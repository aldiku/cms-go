package models

import "time"

// PaymentGateway is a named external payment-gateway credential set (e.g.
// "Midtrans Production", "Xendit Sandbox"). APIKey and CallbackToken are
// stored encrypted — see utils.SimpleEncrypt and config.AppKey — never in
// plaintext. Provider is free text (e.g. "midtrans", "xendit", "manual");
// nothing here enforces a fixed set, same convention as ApiEndpoint.Tags and
// Menu.MenuType elsewhere in this codebase.
//
// Charges are opened with no per-provider Go code (see
// internal/paymentgateway.createGenericCharge) — RequestPath +
// RequestBodyTemplate + ResponseFieldMap fully describe the call, the same
// "configure it, don't code it" spirit as the workflow builder described in
// payment-gateway.md. Provider "manual" (or unset) skips all of this and
// uses a locally-generated stub VA instead, for installs with no gateway
// set up yet.
type PaymentGateway struct {
	ID       uint   `gorm:"primaryKey"`
	Name     string `gorm:"uniqueIndex"`
	Provider string
	Sandbox  bool
	Status   uint8 // 1 = active

	BaseURL       string
	APIKey        string // encrypted; sent as the HTTP Basic Auth username (empty password)
	CallbackToken string // encrypted; verifies inbound webhook authenticity (future: webhook trigger)

	// RequestPath is appended to BaseURL for the charge-creation call, e.g.
	// "/callback_virtual_accounts" (Xendit) or "/v2/charge" (Midtrans).
	RequestPath string

	// RequestBodyTemplate is the JSON POST body, with "{{field}}"
	// placeholders substituted via the same {{key}} engine as
	// notify.Render. Available fields: order_id, amount, bank_code,
	// customer_name, customer_email. Numeric JSON fields should omit the
	// surrounding quotes, e.g. "expected_amount": {{amount}}.
	RequestBodyTemplate string

	// ResponseFieldMap is a JSON object mapping this app's normalized
	// charge-result fields to a dot-path into the gateway's JSON response
	// (numeric segments index arrays, e.g. "va_numbers.0.va_number").
	// Recognized keys: va_number, bank_code, payment_url, reference_id,
	// expires_at. A key with no mapping (or a path that doesn't resolve)
	// just leaves that result field empty.
	ResponseFieldMap string

	Notes string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// PaymentGatewayLog is an audit record of one call to (or, later, webhook
// from) a PaymentGateway — generalized across whatever it's for (today just
// Transaction checkout via internal/paymentgateway.CreateCharge; later also
// ChannelTopup and inbound webhook processing). This is the "save payment
// gateway log" step from payment-gateway.md's workflow description, written
// automatically by every caller rather than needing an explicit action.
type PaymentGatewayLog struct {
	ID               uint   `gorm:"primaryKey"`
	PaymentGatewayID uint   `gorm:"index"` // 0 = no gateway configured (manual/stub fallback)
	SubjectType      string `gorm:"index"` // "transaction" | "channel_topup" (future)
	SubjectID        string `gorm:"index"` // e.g. Transaction.ID
	Direction        string // "outbound" | "inbound"
	RequestJSON      string
	ResponseJSON     string
	Success          bool
	ErrorMessage     string
	CreatedAt        time.Time
}
