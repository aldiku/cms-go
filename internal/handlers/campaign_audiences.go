package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"cms-go/internal/db"
	"cms-go/internal/models"
	"cms-go/internal/utils"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labstack/echo/v4"
)

// isDuplicateKeyError reports whether err is a Postgres unique-violation
// (SQLSTATE 23505) — the race-condition fallback behind the upfront
// name-uniqueness check in CampaignAudienceCreate/CampaignCreativeCreate
// (and their Update counterparts): two concurrent requests can both pass
// the pre-check before either commits, so the DB constraint
// (idx_audience_user_sandbox_name / idx_creative_user_sandbox_name) is the
// actual source of truth — this just turns that into a clean 409 instead of
// a raw driver error.
func isDuplicateKeyError(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// audienceJSON is the wire shape for the Audience JSON API — gender,
// interests and whitelist_phones are arrays over the wire but stored as a
// comma-joined string (gender/interests) or a JSON array string
// (whitelist_phones) on the model, see models.Audience.
type audienceJSON struct {
	ID              string   `json:"audience_id,omitempty"`
	Name            string   `json:"name"`
	ProductID       uint     `json:"product_id"`
	ProductType     string   `json:"product_type"`
	LocationAddress string   `json:"location_address"`
	MinAge          int      `json:"min_age"`
	MaxAge          int      `json:"max_age"`
	Gender          []string `json:"gender"`
	Interests       []string `json:"interests"`
	Latitude        float64  `json:"latitude"`
	Longitude       float64  `json:"longitude"`
	Radius          int      `json:"radius"`
	ProvID          uint     `json:"prov_id"`
	KabID           uint     `json:"kab_id"`
	KecID           uint     `json:"kec_id"`
	KelID           uint     `json:"kel_id"`
	WhitelistPhones []string `json:"whitelist_phones"`
	FileURL         string   `json:"file_url"`
	Sandbox         bool     `json:"sandbox"`
	Source          string   `json:"source"`
}

func audienceToJSON(a models.Audience) audienceJSON {
	var whitelist []string
	json.Unmarshal([]byte(a.WhitelistPhones), &whitelist)

	var gender, interests []string
	if a.Gender != "" {
		gender = strings.Split(a.Gender, ",")
	}
	if a.Interests != "" {
		interests = strings.Split(a.Interests, ",")
	}

	return audienceJSON{
		ID: a.ID, Name: a.Name, ProductID: a.ProductID, ProductType: a.ProductType,
		LocationAddress: a.LocationAddress, MinAge: a.MinAge, MaxAge: a.MaxAge,
		Gender: gender, Interests: interests,
		Latitude: a.Latitude, Longitude: a.Longitude, Radius: a.Radius,
		ProvID: a.ProvID, KabID: a.KabID, KecID: a.KecID, KelID: a.KelID,
		WhitelistPhones: whitelist, FileURL: a.FileURL,
		Sandbox: a.Sandbox, Source: a.Source,
	}
}

func bindAudienceFromJSON(req audienceJSON, a *models.Audience) {
	a.Name = req.Name
	a.ProductID = req.ProductID
	a.ProductType = req.ProductType
	a.LocationAddress = req.LocationAddress
	a.MinAge = req.MinAge
	a.MaxAge = req.MaxAge
	a.Gender = strings.Join(req.Gender, ",")
	a.Interests = strings.Join(req.Interests, ",")
	a.Latitude = req.Latitude
	a.Longitude = req.Longitude
	a.Radius = req.Radius
	a.ProvID = req.ProvID
	a.KabID = req.KabID
	a.KecID = req.KecID
	a.KelID = req.KelID
	if b, err := json.Marshal(req.WhitelistPhones); err == nil {
		a.WhitelistPhones = string(b)
	}
	a.FileURL = req.FileURL
}

// GET /campaign/api/audiences?q=&page=
func CampaignAudienceList(c echo.Context) error {
	user, sandbox, _ := campaignActor(c)

	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		page = 1
	}
	const perPage = 20

	q := db.DB.Model(&models.Audience{}).Where("user_id = ? AND sandbox = ?", user.ID, sandbox)
	if search := c.QueryParam("q"); search != "" {
		q = q.Where("name ILIKE ?", "%"+search+"%")
	}

	var total int64
	q.Count(&total)

	var audiences []models.Audience
	q.Order("created_at desc").Limit(perPage).Offset((page - 1) * perPage).Find(&audiences)

	out := make([]audienceJSON, 0, len(audiences))
	for _, a := range audiences {
		out = append(out, audienceToJSON(a))
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"data": out, "total": total, "page": page, "per_page": perPage})
}

// audienceNameTaken reports whether user/sandbox already has a non-deleted
// Audience named name — excludeID skips one row (itself) when checking on
// update, and should be "" when checking on create.
func audienceNameTaken(userID uint, sandbox bool, name, excludeID string) bool {
	q := db.DB.Model(&models.Audience{}).Where("user_id = ? AND sandbox = ? AND name = ?", userID, sandbox, name)
	if excludeID != "" {
		q = q.Where("id <> ?", excludeID)
	}
	var count int64
	q.Count(&count)
	return count > 0
}

// POST /campaign/api/audiences
func CampaignAudienceCreate(c echo.Context) error {
	user, sandbox, source := campaignActor(c)

	var req audienceJSON
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid payload"})
	}
	if req.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "name is required"})
	}
	if audienceNameTaken(user.ID, sandbox, req.Name, "") {
		return c.JSON(http.StatusConflict, map[string]string{"error": "An audience named \"" + req.Name + "\" already exists"})
	}

	audience := models.Audience{UserID: user.ID, Sandbox: sandbox, Source: source}
	bindAudienceFromJSON(req, &audience)

	for attempt := 0; attempt < 5; attempt++ {
		audience.ID = utils.GenerateEntityID("AUD-")
		err := db.DB.Create(&audience).Error
		if err == nil {
			return c.JSON(http.StatusCreated, audienceToJSON(audience))
		}
		if isDuplicateKeyError(err) {
			return c.JSON(http.StatusConflict, map[string]string{"error": "An audience named \"" + req.Name + "\" already exists"})
		}
	}
	return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create audience"})
}

// ownedAudience loads the :id audience, scoped to the caller's own
// user_id+sandbox — cross-user access looks identical to "not found".
func ownedAudience(c echo.Context, user models.User, sandbox bool) (models.Audience, error) {
	var a models.Audience
	err := db.DB.Where("id = ? AND user_id = ? AND sandbox = ?", c.Param("id"), user.ID, sandbox).First(&a).Error
	return a, err
}

// GET /campaign/api/audiences/:id
func CampaignAudienceGet(c echo.Context) error {
	user, sandbox, _ := campaignActor(c)
	a, err := ownedAudience(c, user, sandbox)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "audience not found"})
	}
	return c.JSON(http.StatusOK, audienceToJSON(a))
}

// PUT /campaign/api/audiences/:id
func CampaignAudienceUpdate(c echo.Context) error {
	user, sandbox, _ := campaignActor(c)
	a, err := ownedAudience(c, user, sandbox)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "audience not found"})
	}

	var req audienceJSON
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid payload"})
	}
	if req.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "name is required"})
	}
	if audienceNameTaken(user.ID, sandbox, req.Name, a.ID) {
		return c.JSON(http.StatusConflict, map[string]string{"error": "An audience named \"" + req.Name + "\" already exists"})
	}
	bindAudienceFromJSON(req, &a)
	if err := db.DB.Save(&a).Error; err != nil {
		if isDuplicateKeyError(err) {
			return c.JSON(http.StatusConflict, map[string]string{"error": "An audience named \"" + req.Name + "\" already exists"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update audience"})
	}
	return c.JSON(http.StatusOK, audienceToJSON(a))
}

// DELETE /campaign/api/audiences/:id — soft delete (models.Audience has
// DeletedAt), per cart-transaction.md's "semua crud delete, hanya akan soft delete".
func CampaignAudienceDelete(c echo.Context) error {
	user, sandbox, _ := campaignActor(c)
	a, err := ownedAudience(c, user, sandbox)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "audience not found"})
	}
	db.DB.Delete(&a)
	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}
