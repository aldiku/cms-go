package handlers

import (
	"net/http"
	"strconv"

	"cms-go/internal/db"
	"cms-go/internal/models"
	"cms-go/internal/utils"

	"github.com/labstack/echo/v4"
)

type creativeJSON struct {
	ID          string `json:"creative_id,omitempty"`
	Name        string `json:"name"`
	ProductID   uint   `json:"product_id"`
	ProductType string `json:"product_type"`
	Title       string `json:"title"`
	Caption     string `json:"caption"`
	Body        string `json:"body"`
	FooterType  string `json:"footer_type"`
	CTAProps    string `json:"cta_props"`
	MediaID     uint   `json:"media_id"`
	FileURL     string `json:"file_url"`
	Sandbox     bool   `json:"sandbox"`
	Source      string `json:"source"`
}

func creativeToJSON(cr models.Creative) creativeJSON {
	return creativeJSON{
		ID: cr.ID, Name: cr.Name, ProductID: cr.ProductID, ProductType: cr.ProductType,
		Title: cr.Title, Caption: cr.Caption, Body: cr.Body,
		FooterType: cr.FooterType, CTAProps: cr.CTAProps,
		MediaID: cr.MediaID, FileURL: cr.FileURL,
		Sandbox: cr.Sandbox, Source: cr.Source,
	}
}

func bindCreativeFromJSON(req creativeJSON, cr *models.Creative) {
	cr.Name = req.Name
	cr.ProductID = req.ProductID
	cr.ProductType = req.ProductType
	cr.Title = req.Title
	cr.Caption = req.Caption
	cr.Body = req.Body
	cr.FooterType = req.FooterType
	cr.CTAProps = req.CTAProps
	cr.MediaID = req.MediaID
	cr.FileURL = req.FileURL
}

// GET /campaign/api/creatives?q=&page=
func CampaignCreativeList(c echo.Context) error {
	user, sandbox, _ := campaignActor(c)

	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		page = 1
	}
	const perPage = 20

	q := db.DB.Model(&models.Creative{}).Where("user_id = ? AND sandbox = ?", user.ID, sandbox)
	if search := c.QueryParam("q"); search != "" {
		q = q.Where("name ILIKE ?", "%"+search+"%")
	}

	var total int64
	q.Count(&total)

	var creatives []models.Creative
	q.Order("created_at desc").Limit(perPage).Offset((page - 1) * perPage).Find(&creatives)

	out := make([]creativeJSON, 0, len(creatives))
	for _, cr := range creatives {
		out = append(out, creativeToJSON(cr))
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"data": out, "total": total, "page": page, "per_page": perPage})
}

// creativeNameTaken reports whether user/sandbox already has a non-deleted
// Creative named name — excludeID skips one row (itself) when checking on
// update, and should be "" when checking on create.
func creativeNameTaken(userID uint, sandbox bool, name, excludeID string) bool {
	q := db.DB.Model(&models.Creative{}).Where("user_id = ? AND sandbox = ? AND name = ?", userID, sandbox, name)
	if excludeID != "" {
		q = q.Where("id <> ?", excludeID)
	}
	var count int64
	q.Count(&count)
	return count > 0
}

// POST /campaign/api/creatives
func CampaignCreativeCreate(c echo.Context) error {
	user, sandbox, source := campaignActor(c)

	var req creativeJSON
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid payload"})
	}
	if req.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "name is required"})
	}
	if creativeNameTaken(user.ID, sandbox, req.Name, "") {
		return c.JSON(http.StatusConflict, map[string]string{"error": "A creative named \"" + req.Name + "\" already exists"})
	}

	creative := models.Creative{UserID: user.ID, Sandbox: sandbox, Source: source}
	bindCreativeFromJSON(req, &creative)

	for attempt := 0; attempt < 5; attempt++ {
		creative.ID = utils.GenerateEntityID("CRE-")
		err := db.DB.Create(&creative).Error
		if err == nil {
			return c.JSON(http.StatusCreated, creativeToJSON(creative))
		}
		if isDuplicateKeyError(err) {
			return c.JSON(http.StatusConflict, map[string]string{"error": "A creative named \"" + req.Name + "\" already exists"})
		}
	}
	return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create creative"})
}

func ownedCreative(c echo.Context, user models.User, sandbox bool) (models.Creative, error) {
	var cr models.Creative
	err := db.DB.Where("id = ? AND user_id = ? AND sandbox = ?", c.Param("id"), user.ID, sandbox).First(&cr).Error
	return cr, err
}

// GET /campaign/api/creatives/:id
func CampaignCreativeGet(c echo.Context) error {
	user, sandbox, _ := campaignActor(c)
	cr, err := ownedCreative(c, user, sandbox)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "creative not found"})
	}
	return c.JSON(http.StatusOK, creativeToJSON(cr))
}

// PUT /campaign/api/creatives/:id
func CampaignCreativeUpdate(c echo.Context) error {
	user, sandbox, _ := campaignActor(c)
	cr, err := ownedCreative(c, user, sandbox)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "creative not found"})
	}

	var req creativeJSON
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid payload"})
	}
	if req.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "name is required"})
	}
	if creativeNameTaken(user.ID, sandbox, req.Name, cr.ID) {
		return c.JSON(http.StatusConflict, map[string]string{"error": "A creative named \"" + req.Name + "\" already exists"})
	}
	bindCreativeFromJSON(req, &cr)
	if err := db.DB.Save(&cr).Error; err != nil {
		if isDuplicateKeyError(err) {
			return c.JSON(http.StatusConflict, map[string]string{"error": "A creative named \"" + req.Name + "\" already exists"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update creative"})
	}
	return c.JSON(http.StatusOK, creativeToJSON(cr))
}

// DELETE /campaign/api/creatives/:id — soft delete.
func CampaignCreativeDelete(c echo.Context) error {
	user, sandbox, _ := campaignActor(c)
	cr, err := ownedCreative(c, user, sandbox)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "creative not found"})
	}
	db.DB.Delete(&cr)
	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}
