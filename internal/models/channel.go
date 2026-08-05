package models

import "time"

// Channel is a registered sending identity (SMS/WABA/Email — email is
// wired in the schema but not selectable in the UI yet, per note.md §9.1).
// It's the entity form of the "Sender ID" produced by the registration
// flow in note.md §6.
type Channel struct {
	ID          uint   `gorm:"primaryKey"`
	Type        string // "sms" | "waba" | "email"
	SenderID    string // display sender name, e.g. "PROMO-ADSQOO"
	Identifier  string // meaning varies by type (WA number, approval code, ...) — free text
	OwnerUserID uint   `gorm:"index"`
	Status      string // "pending" | "active" | "suspended" | "rejected"
	Balance     int64  // credits (sms) or sessions (waba) — unit is per-channel-type

	// Registration detail (note.md §6.1/§6.2) — superset fields, same
	// convention as Creative (internal/models/model.go): not all used per
	// type. DocumentURL is common to both flows; the rest are WABA-only.
	DocumentURL       string
	PICName           string
	PICEmail          string
	PICGender         string
	Company           string
	Phone             string
	Website           string
	BusinessManagerID string
	FBPageURL         string
	// InitialProduct is the WABA product code (waba-service/utility/
	// marketing/authentication) selected at registration (note.md §6.2)
	// — SMS has no equivalent, it just has an initial token (Balance).
	// Always "waba-service" for self-service registration since that
	// product is mandatory there (see InitialProductSelections for the
	// full picked set, including the optional extras and quantities).
	InitialProduct string
	// InitialProductSelections is a JSON snapshot (`[{"code","name",
	// "quantity","unit_price","amount"}, ...]`) of every WABA product the
	// registrant checked on the /register-whatsapp card picker, captured
	// at submission time so admins reviewing a pending registration can
	// see the requested quantities/estimated total without them being
	// re-derived from the (possibly since-changed) product catalog.
	// Informational only — never parsed back by application code.
	InitialProductSelections string `gorm:"type:text"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

// ChannelTopup is a topup request against a Channel — the "history
// transaksi" ledger (note.md §9.3). No payment gateway exists yet, so this
// is a manual admin-reviewed queue: a request starts "pending", and an
// admin marks it "completed" (crediting Channel.Balance) or "rejected".
type ChannelTopup struct {
	ID            uint   `gorm:"primaryKey"`
	ChannelID     uint   `gorm:"index"`
	VariantID     uint   // ProductVariant that priced this (smstopup, or one of the WABA topup variants)
	TierID        uint   // ProductVariantTier if tiering mode; 0 = not tiered
	Quantity      int64  // SMS credits, or 1 for a WABA session-bundle purchase
	Amount        int64  // Rupiah price paid — resolved via pricing.EffectivePrice at request time
	Status        string // "pending" | "completed" | "rejected"
	Note          string // optional, e.g. rejection reason
	RequestedByID uint   // who requested it (channel owner, or admin on their behalf)
	ReviewedByID  uint   // admin who completed/rejected it; 0 = not yet reviewed
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
