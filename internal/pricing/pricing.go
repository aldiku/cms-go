// Package pricing resolves what a given user actually pays for a product
// variant, accounting for per-user custom pricing (see models.PriceOverride).
package pricing

import (
	"cms-go/internal/db"
	"cms-go/internal/models"
)

// EffectivePrice returns the price userID pays for variantID: their own
// override if one exists for this variant, else the variant's default
// Price. This is a direct lookup on TargetUserID only — the User.ReferralID
// hierarchy is not walked here; it only governs who may grant an override
// to whom (see internal/handlers/auth_pricing.go).
func EffectivePrice(variantID, userID uint) (int64, error) {
	var override models.PriceOverride
	err := db.DB.Where("variant_id = ? AND target_user_id = ?", variantID, userID).First(&override).Error
	if err == nil {
		return override.Price, nil
	}

	var variant models.ProductVariant
	if err := db.DB.First(&variant, variantID).Error; err != nil {
		return 0, err
	}
	return variant.Price, nil
}
