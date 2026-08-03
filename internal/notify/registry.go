// Package notify holds the system's notification building blocks: the
// fixed registry of hooks (events) the app can send mail for, HTML
// placeholder rendering, and SMTP delivery. The admin-configured pairing of
// hook -> SMTP config -> email template -> field mapping lives in
// models.NotificationHook; this package supplies what those rows reference.
package notify

// HookField is one piece of data a hook makes available at fire time, e.g.
// the new user's email address for "register_user".
type HookField struct {
	Key         string
	Description string
}

// HookDef describes one system event that can trigger notifications.
type HookDef struct {
	Key         string
	Label       string
	Description string
	Fields      []HookField
}

// Registry is the fixed list of hooks available to bind notifications to.
// Adding a new hook here makes it selectable in the Notification Manager;
// actually firing it (calling Dispatch at the right point in the app) is a
// separate step.
var Registry = []HookDef{
	{
		Key:         "register_user",
		Label:       "User Registered",
		Description: "Fires when a new user account is created in the admin panel.",
		Fields: []HookField{
			{Key: "user_name", Description: "Full name of the new user"},
			{Key: "user_email", Description: "Email address of the new user"},
			{Key: "user_role", Description: "Assigned role name"},
			{Key: "site_name", Description: "Site name"},
			{Key: "site_url", Description: "Site URL"},
		},
	},
	{
		Key:         "page_published",
		Label:       "Page Published",
		Description: "Fires when a page or post's status changes to Publish.",
		Fields: []HookField{
			{Key: "page_title", Description: "Title of the page"},
			{Key: "page_url", Description: "Public URL of the page"},
			{Key: "author_name", Description: "Author's name"},
		},
	},
	{
		Key:         "password_reset",
		Label:       "Password Reset Requested",
		Description: "Fires when a user requests a password reset.",
		Fields: []HookField{
			{Key: "user_name", Description: "Full name of the requesting user"},
			{Key: "user_email", Description: "Email address of the requesting user"},
			{Key: "reset_url", Description: "One-time password reset link"},
		},
	},
}

// Find looks up a hook definition by key.
func Find(key string) (HookDef, bool) {
	for _, h := range Registry {
		if h.Key == key {
			return h, true
		}
	}
	return HookDef{}, false
}
