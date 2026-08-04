package models

import "time"

// ProductCategory is a flat, top-level grouping for the product catalog
// (e.g. "SMS Advertising", "WhatsApp Advertising"). Distinct from Category
// (the WP-style Page/post taxonomy) — this one backs its own admin section
// under /admin/product-categories.
type ProductCategory struct {
	ID          uint   `gorm:"primaryKey"`
	Name        string `gorm:"uniqueIndex"`
	Slug        string `gorm:"uniqueIndex"`
	Description string
	ImageID     uint  // 0 = none; see the media id=0 sentinel row, internal/auth/seed_pages.go
	Image       Media `gorm:"foreignKey:ImageID"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Product is a node in the catalog tree under a ProductCategory.
// Self-referencing via ParentID (0 = root of the category's tree), same
// adjacency-list + ListOrder pattern as Menu — supports arbitrary depth,
// e.g. SMS Advertising > SMS LBA > TELKOMSEL > Telkomsel Ads.
type Product struct {
	ID                uint   `gorm:"primaryKey"`
	Code              string `gorm:"uniqueIndex"`
	ProductCategoryID uint
	ParentID          uint // 0 = root of this category's tree
	Name              string
	Description       string
	ThumbnailID       uint  // 0 = none
	Thumbnail         Media `gorm:"foreignKey:ThumbnailID"`
	HPP               int64
	Price             int64
	Status            uint8 // 1 = active
	ListOrder         uint32
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// ProductVariant is the sellable unit under a Product node (normally a
// leaf). PricingMode selects whether Price (fixed) or the
// ProductVariantTier rows (tiering) apply.
type ProductVariant struct {
	ID          uint `gorm:"primaryKey"`
	ProductID   uint `gorm:"index"`
	Name        string
	PricingMode string // "fixed" | "tiering"
	HPP         int64
	Price       int64 // used when PricingMode == "fixed"
	Status      uint8 // 1 = active
	ListOrder   uint32
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ProductVariantTier is one price point of a "tiering" ProductVariant
// (e.g. WABA session bundles: 10.000 / 20.000 / 50.000 sesi).
type ProductVariantTier struct {
	ID        uint `gorm:"primaryKey"`
	VariantID uint `gorm:"index"`
	Label     string
	Price     int64
	IsCustom  bool // "custom" tier — price negotiated, not from the fixed list
	CreatedAt time.Time
}
