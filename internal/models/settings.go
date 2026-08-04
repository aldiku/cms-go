package models

import "time"

// GeneralSetting is a singleton row (always id=1) holding admin-overridable
// site-wide settings that would otherwise only be configurable by editing
// .env or hardcoded defaults — see internal/config.SiteName/SiteTagline/
// DefaultRegisterRoleID, which read this row.
type GeneralSetting struct {
	ID uint `gorm:"primaryKey"`

	SiteTitle string // overrides config.SiteName() (APP_NAME env) when set
	Tagline   string // short site description; used as a default meta description

	FaviconID uint  // 0 = none; FK to Media. Setting this copies the file to assets/favicon.ico
	Favicon   Media `gorm:"foreignKey:FaviconID"`

	// DefaultRoleID is granted to new self-registered users (POST
	// /auth/register). 0 = not configured; callers fall back to the seeded
	// "member" role (see auth.SeedAuth). Deliberately a bare ID with no
	// belongs-to association field (unlike Favicon below) — GORM only
	// generates a foreign key for fields that have one, and this one is
	// never preloaded, so skipping it avoids needing a sentinel row.
	DefaultRoleID uint

	CreatedAt time.Time
	UpdatedAt time.Time
}
