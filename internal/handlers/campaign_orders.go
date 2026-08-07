package handlers

import (
	"net/http"
	"strconv"
	"time"

	"cms-go/internal/db"
	"cms-go/internal/models"
	"cms-go/internal/pricing"
	"cms-go/internal/utils"

	"github.com/labstack/echo/v4"
)

type orderDetailJSON struct {
	AudienceID string `json:"audience_id"`
	CreativeID string `json:"creative_id"`
	Qty        int64  `json:"qty"`
}

type orderJSON struct {
	ID                  string            `json:"order_id,omitempty"`
	CampaignName        string            `json:"campaign_name"`
	CampaignDescription string            `json:"campaign_description"`
	ProductID           uint              `json:"product_id"`
	ProductVariantID    uint              `json:"product_variant_id"`
	ScheduleStart       *time.Time        `json:"schedule_start"`
	ScheduleEnd         *time.Time        `json:"schedule_end"`
	Taxable             bool              `json:"taxable"`
	Status              uint8             `json:"status"`
	Qty                 int64             `json:"qty"`
	GrandTotal          int64             `json:"grand_total"`
	OriginalCost        int64             `json:"original_cost,omitempty"`
	ResellerCost        int64             `json:"reseller_cost,omitempty"`
	Sandbox             bool              `json:"sandbox"`
	Source              string            `json:"source"`
	Details             []orderDetailJSON `json:"details"`
}

func orderToJSON(o models.Order, details []models.OrderDetail, includeCosts bool) orderJSON {
	detailsJSON := make([]orderDetailJSON, 0, len(details))
	for _, d := range details {
		detailsJSON = append(detailsJSON, orderDetailJSON{AudienceID: d.AudienceID, CreativeID: d.CreativeID, Qty: d.Qty})
	}
	out := orderJSON{
		ID: o.ID, CampaignName: o.CampaignName, CampaignDescription: o.CampaignDescription,
		ProductID: o.ProductID, ProductVariantID: o.ProductVariantID,
		ScheduleStart: o.ScheduleStart, ScheduleEnd: o.ScheduleEnd,
		Taxable: o.Taxable, Status: o.Status, Qty: o.Qty, GrandTotal: o.GrandTotal,
		Sandbox: o.Sandbox, Source: o.Source, Details: detailsJSON,
	}
	// original_cost/reseller_cost are cost-basis figures — only the order's
	// own owner (querying their own draft) or an admin view should ever
	// request them; callers that don't need them just omit the field.
	if includeCosts {
		out.OriginalCost = o.OriginalCost
		out.ResellerCost = o.ResellerCost
	}
	return out
}

// GET /campaign/api/orders?status=&page=
func CampaignOrderList(c echo.Context) error {
	user, sandbox, _ := campaignActor(c)

	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		page = 1
	}
	const perPage = 20

	q := db.DB.Model(&models.Order{}).Where("user_id = ? AND sandbox = ?", user.ID, sandbox)
	if statusStr := c.QueryParam("status"); statusStr != "" {
		if status, err := strconv.Atoi(statusStr); err == nil {
			q = q.Where("status = ?", status)
		}
	}

	var total int64
	q.Count(&total)

	var orders []models.Order
	q.Order("created_at desc").Limit(perPage).Offset((page - 1) * perPage).Find(&orders)

	out := make([]orderJSON, 0, len(orders))
	for _, o := range orders {
		out = append(out, orderToJSON(o, nil, true))
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"data": out, "total": total, "page": page, "per_page": perPage})
}

// ownedOrder loads the :id order (or the given id string), scoped to the
// caller's own user_id+sandbox.
func ownedOrder(id string, user models.User, sandbox bool) (models.Order, error) {
	var o models.Order
	err := db.DB.Where("id = ? AND user_id = ? AND sandbox = ?", id, user.ID, sandbox).First(&o).Error
	return o, err
}

func loadOrderDetails(orderID string) []models.OrderDetail {
	var details []models.OrderDetail
	db.DB.Where("order_id = ?", orderID).Find(&details)
	return details
}

// GET /campaign/api/orders/:id
func CampaignOrderGet(c echo.Context) error {
	user, sandbox, _ := campaignActor(c)
	o, err := ownedOrder(c.Param("id"), user, sandbox)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "order not found"})
	}
	return c.JSON(http.StatusOK, orderToJSON(o, loadOrderDetails(o.ID), true))
}

// resolveOrderPricing loads the chosen Product+ProductVariant and returns
// the unit price this user pays, its HPP cost basis, and whether the
// product is taxable (IsCampaignable, per cart-transaction.md's "Taxable
// flag (only product campaignable is taxable)").
func resolveOrderPricing(productID, variantID uint, user models.User) (price, hpp int64, taxable bool, err error) {
	var product models.Product
	if err = db.DB.First(&product, productID).Error; err != nil {
		return
	}
	var variant models.ProductVariant
	if err = db.DB.Where("id = ? AND product_id = ?", variantID, productID).First(&variant).Error; err != nil {
		return
	}
	price, err = pricing.EffectivePrice(variant.ID, user.ID)
	if err != nil {
		price = variant.Price
		err = nil
	}
	hpp = variant.HPP
	taxable = product.IsCampaignable
	return
}

// POST /campaign/api/orders — create a new draft, or (if "order_id" is set
// and owned by the caller and still a draft) autosave over it in place, per
// cart-transaction.md's "order bersifat draftable ... auto save".
func CampaignOrderSave(c echo.Context) error {
	user, sandbox, source := campaignActor(c)

	var req orderJSON
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid payload"})
	}
	if req.ProductID == 0 || req.ProductVariantID == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "product_id and product_variant_id are required"})
	}

	price, hpp, taxable, err := resolveOrderPricing(req.ProductID, req.ProductVariantID, user)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "product/variant not found"})
	}

	// Ownership-check every referenced audience/creative up front so a
	// tampered payload can't attach someone else's targeting/creative to
	// this order.
	for _, d := range req.Details {
		if d.AudienceID != "" {
			if err := db.DB.Where("id = ? AND user_id = ? AND sandbox = ?", d.AudienceID, user.ID, sandbox).
				First(&models.Audience{}).Error; err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "audience not found: " + d.AudienceID})
			}
		}
		if d.CreativeID != "" {
			if err := db.DB.Where("id = ? AND user_id = ? AND sandbox = ?", d.CreativeID, user.ID, sandbox).
				First(&models.Creative{}).Error; err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "creative not found: " + d.CreativeID})
			}
		}
	}

	var qty int64
	for _, d := range req.Details {
		qty += d.Qty
	}

	var order models.Order
	isNew := true
	if req.ID != "" {
		if existing, err := ownedOrder(req.ID, user, sandbox); err == nil && existing.Status == models.OrderStatusDraft {
			order = existing
			isNew = false
		}
	}

	order.CampaignName = req.CampaignName
	order.CampaignDescription = req.CampaignDescription
	order.ProductID = req.ProductID
	order.ProductVariantID = req.ProductVariantID
	order.ScheduleStart = req.ScheduleStart
	order.ScheduleEnd = req.ScheduleEnd
	order.Taxable = taxable
	order.Status = models.OrderStatusDraft
	order.Qty = qty
	order.GrandTotal = qty * price
	order.OriginalCost = qty * hpp
	if user.ReferralID != 0 {
		if resellerPrice, err := pricing.EffectivePrice(req.ProductVariantID, user.ReferralID); err == nil {
			order.ResellerCost = qty * resellerPrice
		}
	}
	order.UserID = user.ID
	order.Sandbox = sandbox
	order.Source = source

	tx := db.DB.Begin()
	if isNew {
		for attempt := 0; attempt < 5; attempt++ {
			order.ID = utils.GenerateEntityID("")
			if err := tx.Create(&order).Error; err == nil {
				break
			}
		}
		if order.ID == "" {
			tx.Rollback()
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create order"})
		}
	} else {
		if err := tx.Save(&order).Error; err != nil {
			tx.Rollback()
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update order"})
		}
		tx.Where("order_id = ?", order.ID).Delete(&models.OrderDetail{})
	}

	for _, d := range req.Details {
		detail := models.OrderDetail{OrderID: order.ID, AudienceID: d.AudienceID, CreativeID: d.CreativeID, Qty: d.Qty}
		if err := tx.Create(&detail).Error; err != nil {
			tx.Rollback()
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to save order details"})
		}
	}
	tx.Commit()

	return c.JSON(http.StatusOK, orderToJSON(order, loadOrderDetails(order.ID), true))
}

// DELETE /campaign/api/orders/:id — soft delete.
func CampaignOrderDelete(c echo.Context) error {
	user, sandbox, _ := campaignActor(c)
	o, err := ownedOrder(c.Param("id"), user, sandbox)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "order not found"})
	}
	db.DB.Delete(&o)
	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}
