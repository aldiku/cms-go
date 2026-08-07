package handlers

import (
	"net/http"
	"strconv"

	"cms-go/internal/auth"
	"cms-go/internal/db"
	"cms-go/internal/models"
	"cms-go/internal/pricing"

	"github.com/labstack/echo/v4"
)

// campaignActor pulls the user/sandbox/source resolved by
// auth.ResolveKeyActor for the /campaign, /cart and /invoice surfaces.
func campaignActor(c echo.Context) (models.User, bool, string) {
	user, _ := c.Get(auth.CtxCampaignUser).(models.User)
	sandbox, _ := c.Get(auth.CtxCampaignSandbox).(bool)
	source, _ := c.Get(auth.CtxCampaignSource).(string)
	return user, sandbox, source
}

// GET /campaign/api/product-categories — categories that have at least one
// campaignable product, for the campaign composer's type picker.
func CampaignProductCategories(c echo.Context) error {
	var categories []models.ProductCategory
	db.DB.Where("id IN (?)",
		db.DB.Model(&models.Product{}).Distinct("product_category_id").Where("is_campaignable = ?", true),
	).Order("name asc").Find(&categories)
	return c.JSON(http.StatusOK, categories)
}

type campaignProductNode struct {
	models.Product
	HasChildren bool `json:"has_children"`
	HasVariants bool `json:"has_variants"`
	// HasPrice flags a leaf with no variant layer that's priced directly on
	// the Product row itself (Price > 0) — the composer's "Channel" grid
	// treats a leaf as selectable if it has variants OR this, since a leaf
	// with neither has nothing to resolve a price from.
	HasPrice bool `json:"has_price"`
}

// GET /campaign/api/products?category_id=&parent_id= — one level of the
// campaignable product tree at a time, so the composer can fetch children
// as each level is picked (cart-transaction.md: "jika dipilih maka fetch
// api untuk ambil childrennya").
func CampaignProducts(c echo.Context) error {
	categoryID, _ := strconv.ParseUint(c.QueryParam("category_id"), 10, 64)
	parentID, _ := strconv.ParseUint(c.QueryParam("parent_id"), 10, 64)
	if categoryID == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "category_id is required"})
	}

	var products []models.Product
	db.DB.Preload("Thumbnail").
		Where("product_category_id = ? AND parent_id = ? AND is_campaignable = ? AND status = 1", categoryID, parentID, true).
		Order("list_order asc, id asc").Find(&products)

	nodes := make([]campaignProductNode, 0, len(products))
	for _, p := range products {
		var childCount, variantCount int64
		db.DB.Model(&models.Product{}).Where("parent_id = ?", p.ID).Count(&childCount)
		db.DB.Model(&models.ProductVariant{}).Where("product_id = ? AND status = 1", p.ID).Count(&variantCount)
		nodes = append(nodes, campaignProductNode{Product: p, HasChildren: childCount > 0, HasVariants: variantCount > 0, HasPrice: p.Price > 0})
	}
	return c.JSON(http.StatusOK, nodes)
}

// GET /campaign/api/products/:id — a single product's basic info, used by
// the Edit Campaign form to show a human-readable label for the
// already-selected leaf product without requiring the user to re-walk the
// cascading picker just to see what's currently set.
func CampaignProductGet(c echo.Context) error {
	var product models.Product
	if err := db.DB.Preload("Thumbnail").First(&product, c.Param("id")).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "product not found"})
	}
	return c.JSON(http.StatusOK, product)
}

type campaignVariantJSON struct {
	models.ProductVariant
	Price int64                       `json:"resolved_price"`
	Tiers []models.ProductVariantTier `json:"tiers,omitempty"`
}

// GET /campaign/api/products/:id/variants — the leaf's sellable variants,
// priced for the calling user via pricing.EffectivePrice, so the composer
// can pull "harga terakhir" straight into the budget calculation.
func CampaignProductVariants(c echo.Context) error {
	user, _, _ := campaignActor(c)

	productID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid product id"})
	}

	var variants []models.ProductVariant
	db.DB.Where("product_id = ? AND status = 1", productID).Order("list_order asc, id asc").Find(&variants)

	out := make([]campaignVariantJSON, 0, len(variants))
	for _, v := range variants {
		price, err := pricing.EffectivePrice(v.ID, user.ID)
		if err != nil {
			price = v.Price
		}
		item := campaignVariantJSON{ProductVariant: v, Price: price}
		if v.PricingMode == "tiering" {
			db.DB.Where("variant_id = ?", v.ID).Order("id asc").Find(&item.Tiers)
		}
		out = append(out, item)
	}
	return c.JSON(http.StatusOK, out)
}
