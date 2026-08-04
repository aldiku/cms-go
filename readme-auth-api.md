# Auth API

Public JSON API for a decoupled frontend: register, log in, and reset a
forgotten password. Distinct from the server-rendered `/admin/login` (HTML
admin panel login) and the session-gated `/auth/profile`, `/auth/setting`
pages (self-service account pages, also HTML) — everything below is JSON
in, JSON out, and reachable without an existing session.

Implementation: [internal/handlers/auth_api.go](internal/handlers/auth_api.go).
Routes registered in [internal/server/router.go](internal/server/router.go).

## Endpoints at a glance

| Method | Path                    | Body                                                          | Auth needed |
|--------|--------------------------|----------------------------------------------------------------|-------------|
| GET    | `/auth/csrf-token`       | —                                                               | none |
| POST   | `/auth/login`            | `email, password, csrf_token`                                  | none |
| POST   | `/auth/register`         | `email, password, csrf_token` (+ optional `firstname, lastname, phone, referral_id`) | none |
| POST   | `/auth/forgot-password`  | `email`                                                         | none |
| POST   | `/auth/reset-password`   | `reset_token_code, password, confirm_password`                 | none |

All bodies are JSON — send `Content-Type: application/json`. All responses
are JSON, either `{"error": "..."}` on failure or an endpoint-specific
success shape (below).

## CSRF token flow (login & register only)

`login` and `register` are guarded by a double-submit CSRF cookie, since
this is a state-changing JSON POST from a browser frontend. `forgot-password`
and `reset-password` don't require it — they're guarded instead by the
random, single-use reset token itself and by rate limiting.

1. Frontend calls `GET /auth/csrf-token`. This sets a `csrf_token` cookie
   (readable by JS — not `HttpOnly` — so it can be copied into the request
   body) and returns the same value in the JSON body:
   ```json
   { "csrf_token": "e73710d7a10f60b3bd6cf32decb4206b..." }
   ```
2. Frontend includes that value as `csrf_token` in the `login`/`register`
   JSON body. The browser sends the cookie automatically; the server checks
   the two match.
3. Fetch a fresh token before each login/register attempt (or whenever the
   previous one might have expired — 30 minutes) — a mismatch or missing
   token returns `403`.

```js
const { csrf_token } = await fetch('/auth/csrf-token', { credentials: 'include' }).then(r => r.json());

await fetch('/auth/login', {
  method: 'POST',
  credentials: 'include', // required — carries the csrf_token cookie and, on success, the session cookie
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ email, password, csrf_token }),
});
```

## POST /auth/login

**Request**
```json
{ "email": "user@example.com", "password": "secret123", "csrf_token": "..." }
```

**Success — 200**
```json
{
  "token": "a321b656345c06cf2fb1642ff8351e6df0353c0bc9119ec7a9d4b5b1eb3a4b37",
  "user": {
    "id": 6,
    "firstname": "Test",
    "lastname": "User",
    "email": "test.user@example.com",
    "phone": "",
    "referral_id": 1,
    "verified": false
  }
}
```

A session cookie (`cms_session`, `HttpOnly`, 7-day expiry) is also set on
the response — the same cookie the admin panel and `/auth/profile` use. The
`token` field is the raw session token, included for non-browser clients
(mobile apps, server-to-server) that can't rely on cookies; browser
frontends should generally just rely on the cookie.

**Errors**
| Status | Body | Cause |
|--------|------|-------|
| 400 | `{"error":"invalid request body"}` | malformed JSON |
| 400 | `{"error":"email and password are required"}` | missing field |
| 403 | `{"error":"invalid or missing csrf token"}` | see CSRF flow above |
| 401 | `{"error":"invalid email or password"}` | wrong credentials, unknown email, or inactive account (deliberately the same message for all three — doesn't reveal which) |
| 429 | `{"error":"rate limit exceeded"}` | too many attempts from this IP (see Rate limiting) |
| 500 | `{"error":"could not start session"}` | DB error creating the session row |

## POST /auth/register

**Request** — only `email`, `password`, `csrf_token` are required:
```json
{
  "email": "user@example.com",
  "password": "secret123",
  "csrf_token": "...",
  "firstname": "Jane",
  "lastname": "Doe",
  "phone": "+62812...",
  "referral_id": 1
}
```

- `password` must be at least 8 characters.
- `referral_id` (optional) is the numeric `id` of an existing user — the
  "parent" who referred this signup. If provided, it must match a real
  user or the request is rejected; omit it (or send `0`) for no referrer.
- The new account is created active (`status = 1`, can log in immediately)
  but unverified (`verified: false`) and assigned the built-in `member`
  role, which has no admin permissions.
- Fires the `register_user` notification hook (see [Notifications](#notifications-hooks) below) with a one-time email-verification link. Hitting that link (`GET /auth/verify?token=...`) sets `verified: true`.

**Success — 201** — same shape as login's success response (`token` + `user`), and the session cookie is set the same way — registration logs the user in immediately.

**Errors**
| Status | Body | Cause |
|--------|------|-------|
| 400 | `{"error":"invalid request body"}` | malformed JSON |
| 400 | `{"error":"email and password are required"}` | missing field |
| 400 | `{"error":"password must be at least 8 characters"}` | too short |
| 403 | `{"error":"invalid or missing csrf token"}` | see CSRF flow above |
| 400 | `{"error":"invalid referral_id"}` | `referral_id` doesn't match any user |
| 400 | `{"error":"email already registered"}` | email uniqueness violation |
| 429 | `{"error":"rate limit exceeded"}` | too many attempts from this IP |
| 500 | `{"error":"failed to process password"}` / `{"error":"registration temporarily unavailable"}` | server-side error (hashing / role lookup) |

## POST /auth/forgot-password

**Request**
```json
{ "email": "user@example.com" }
```

**Response — always 200**, regardless of whether the email is registered
(this is deliberate — it prevents using this endpoint to discover which
emails have accounts):
```json
{ "message": "If an account exists for this email, a password reset link has been sent." }
```

If (and only if) the email matches an active user, a one-time reset token
is generated (1-hour expiry, invalidates any earlier unused reset token for
the same user) and the `password_reset` notification hook fires with it —
see [Notifications](#notifications-hooks).

`429 {"error":"rate limit exceeded"}` applies here too.

## POST /auth/reset-password

**Request**
```json
{
  "reset_token_code": "7fda543328c74603b81bc0ce1038ac45a158a1aa0309a60bf851f8cfa2dfc6d8",
  "password": "newpass123",
  "confirm_password": "newpass123"
}
```

`reset_token_code` is the raw token from the `password_reset` hook payload
(the `reset_token_code` field, or parsed out of `reset_url`).

**Success — 200**
```json
{ "message": "password has been reset successfully" }
```

On success every existing session for that user is revoked (including the
one that requested the reset) — the user must log in again with the new
password.

**Errors**
| Status | Body | Cause |
|--------|------|-------|
| 400 | `{"error":"invalid request body"}` | malformed JSON |
| 400 | `{"error":"reset_token_code and password are required"}` | missing field |
| 400 | `{"error":"password must be at least 8 characters"}` | too short |
| 400 | `{"error":"password and confirm_password do not match"}` | mismatch |
| 400 | `{"error":"invalid or expired reset token"}` | unknown, already-used (tokens are single-use), or expired (>1h) token |
| 429 | `{"error":"rate limit exceeded"}` | too many attempts from this IP |
| 500 | `{"error":"failed to process password"}` / `{"error":"failed to update password"}` | server-side error |

## Notifications (hooks)

`register` and `forgot-password` don't send email themselves — they fire a
named hook via `internal/notify`, and it's a no-op unless an admin has
bound that hook to an SMTP config + email template in **Admin → Settings →
Notification Manager** (`/admin/notification-hooks`). Nothing needs to be
built here for the API itself to work; this section is for whoever wires up
the actual emails.

**`register_user`** fields available for the email template:
`user_name`, `user_email`, `user_role`, `site_name`, `site_url`,
`verification_url`

**`password_reset`** fields available for the email template:
`user_name`, `user_email`, `reset_url`, `reset_token_code`

`reset_url` is a convenience link (`{SITE_URL}/reset-password?token=...`);
`reset_token_code` is the same token on its own, for a template/frontend
that wants to build its own link or show a raw code. Adjust the frontend
route in `reset_url` to wherever the actual reset-password page ends up
living — the backend doesn't assume a specific path beyond that default.

## Rate limiting

`login`, `register`, `forgot-password`, and `reset-password` share the same
per-IP limiter as the admin login form: burst of 5 requests, then
~1 request per 12 seconds, tracked in memory per source IP.

## Notes for the frontend

- Always send `credentials: 'include'` (or your HTTP client's cookie-jar
  equivalent) — the CSRF flow depends on the cookie set by
  `GET /auth/csrf-token` round-tripping back to the server.
- The session cookie (`cms_session`) set by `login`/`register` is the same
  one used by `/auth/profile`, `/auth/setting`, etc. — a logged-in user
  from this API can hit those pages directly if useful.
- No page, email template, or hook binding is included — only the API.
  The admin still needs to create the actual login/register/reset-password
  pages and bind the `register_user` / `password_reset` hooks to an SMTP
  config + email template for verification/reset emails to actually send.
