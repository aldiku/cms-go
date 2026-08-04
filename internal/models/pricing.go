package models

import "time"

// PriceOverride is a custom price for one ProductVariant, granted by
// SetByUserID (an admin, or a reseller) to TargetUserID (a reseller, or
// one of that reseller's own clients, per User.ReferralID). Resolving the
// effective price for a (variant, user) pair is a direct lookup on
// TargetUserID — no ancestor chain walking; see internal/pricing.EffectivePrice.
type PriceOverride struct {
	ID           uint `gorm:"primaryKey"`
	VariantID    uint `gorm:"uniqueIndex:idx_variant_target"`
	TargetUserID uint `gorm:"uniqueIndex:idx_variant_target"`
	SetByUserID  uint
	Price        int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
