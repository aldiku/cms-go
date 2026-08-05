package handlers

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"cms-go/internal/auth"
	"cms-go/internal/db"
	"cms-go/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

const adminChannelsPerPage = 12

// channelCardView is the display shape for one channel card/list row — the
// owner's name resolved up front so the template stays a simple range.
type channelCardView struct {
	Channel   models.Channel
	OwnerName string
}

func loadChannelCards(channels []models.Channel) []channelCardView {
	userIDs := make([]uint, 0, len(channels))
	seen := map[uint]bool{}
	for _, ch := range channels {
		if !seen[ch.OwnerUserID] {
			seen[ch.OwnerUserID] = true
			userIDs = append(userIDs, ch.OwnerUserID)
		}
	}
	var users []models.User
	if len(userIDs) > 0 {
		db.DB.Where("id IN ?", userIDs).Find(&users)
	}
	nameByID := make(map[uint]string, len(users))
	for _, u := range users {
		nameByID[u.ID] = u.FullName()
	}

	cards := make([]channelCardView, 0, len(channels))
	for _, ch := range channels {
		cards = append(cards, channelCardView{Channel: ch, OwnerName: nameByID[ch.OwnerUserID]})
	}
	return cards
}

// GET /admin/channels?type=&owner=&page=
func AdminChannels(c echo.Context) error {
	channelType := c.QueryParam("type")
	ownerParam := c.QueryParam("owner")

	pageNum, err := strconv.Atoi(c.QueryParam("page"))
	if err != nil || pageNum < 1 {
		pageNum = 1
	}

	q := db.DB.Model(&models.Channel{})
	if channelType != "" {
		q = q.Where("type = ?", channelType)
	}
	if ownerParam != "" {
		q = q.Where("owner_user_id = ?", ownerParam)
	}

	var total int64
	q.Count(&total)

	totalPages := int((total + adminChannelsPerPage - 1) / adminChannelsPerPage)
	if totalPages < 1 {
		totalPages = 1
	}
	if pageNum > totalPages {
		pageNum = totalPages
	}

	var channels []models.Channel
	q.Order("created_at desc").Limit(adminChannelsPerPage).Offset((pageNum - 1) * adminChannelsPerPage).Find(&channels)

	var ownerName string
	if ownerParam != "" {
		var owner models.User
		if db.DB.First(&owner, ownerParam).Error == nil {
			ownerName = owner.FullName()
		}
	}

	var wabaProducts []models.Product
	db.DB.Where("is_campaignable = ? AND code <> ?", false, "smstopup").Order("name asc").Find(&wabaProducts)

	data := map[string]interface{}{
		"Cards":        loadChannelCards(channels),
		"Type":         channelType,
		"Owner":        ownerParam,
		"OwnerName":    ownerName,
		"CurrentPage":  pageNum,
		"TotalPages":   totalPages,
		"HasPrev":      pageNum > 1,
		"HasNext":      pageNum < totalPages,
		"PrevPage":     pageNum - 1,
		"NextPage":     pageNum + 1,
		"WABAProducts": wabaProducts,
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/channels.html", data)
}

func bindChannelFromForm(c echo.Context, ch *models.Channel) {
	ch.Type = c.FormValue("type")
	ch.SenderID = c.FormValue("sender_id")
	ch.Identifier = c.FormValue("identifier")
	ch.PICName = c.FormValue("pic_name")
	ch.PICEmail = c.FormValue("pic_email")
	ch.PICGender = c.FormValue("pic_gender")
	ch.Company = c.FormValue("company")
	ch.Phone = c.FormValue("phone")
	ch.Website = c.FormValue("website")
	ch.BusinessManagerID = c.FormValue("business_manager_id")
	ch.FBPageURL = c.FormValue("fb_page_url")
	if documentURL := c.FormValue("document_url"); documentURL != "" {
		ch.DocumentURL = documentURL
	}
	if status := c.FormValue("status"); status != "" {
		ch.Status = status
	} else if ch.Status == "" {
		ch.Status = "active"
	}
}

const smsMinInitialToken = 5000

// buildChannelFromRegistrationForm reads the fields common to both
// registration flows in note.md: §6.1 (SMS — sender name, document,
// minimum 5.000 initial token) and §6.2 (WABA — sender name, document,
// PIC/company/contact detail, and an initial product chosen from the WABA
// topup catalog). Shared by the admin-side create form (AdminCreateChannel)
// and the public self-service registration API (RegisterChannel,
// channel_register_api.go) — the only difference between those two
// callers is who ends up owning the row and what status it starts at.
func buildChannelFromRegistrationForm(c echo.Context, uploaderID uint) (models.Channel, error) {
	var ch models.Channel
	bindChannelFromForm(c, &ch)
	if ch.Type == "" || ch.SenderID == "" {
		return ch, errors.New("Type and Sender ID are required")
	}

	switch ch.Type {
	case "sms":
		token, err := strconv.ParseInt(c.FormValue("initial_token"), 10, 64)
		if err != nil || token < smsMinInitialToken {
			return ch, errors.New("Initial token must be at least 5.000")
		}
		ch.Balance = token
	case "waba":
		initialProduct := c.FormValue("initial_product")
		var product models.Product
		if err := db.DB.Where("code = ? AND is_campaignable = ?", initialProduct, false).First(&product).Error; err != nil {
			return ch, errors.New("A valid initial product is required")
		}
		ch.InitialProduct = product.Code
	default:
		return ch, errors.New("Unsupported channel type")
	}

	if fh, err := c.FormFile("document"); err == nil {
		now := time.Now()
		subDir := filepath.Join(mediaUploadRoot, now.Format("2006"), now.Format("01"))
		if err := os.MkdirAll(subDir, 0755); err == nil {
			if media, err := saveUploadedMedia(fh, subDir, uploaderID); err == nil {
				ch.DocumentURL = media.URL
			}
		}
	}

	return ch, nil
}

// POST /admin/channels/new — admin creates a channel on behalf of a user.
// Always created "active" since admin already vetted it by creating it
// directly, unlike public self-service registration (RegisterChannel),
// which starts "pending".
func AdminCreateChannel(c echo.Context) error {
	ownerID, err := strconv.ParseUint(c.FormValue("owner_user_id"), 10, 64)
	if err != nil || ownerID == 0 {
		return c.String(http.StatusBadRequest, "An owner is required")
	}
	var owner models.User
	if err := db.DB.First(&owner, ownerID).Error; err != nil {
		return c.String(http.StatusBadRequest, "Owner not found")
	}

	var uploaderID uint
	if current, ok := c.Get(auth.CtxUser).(models.User); ok {
		uploaderID = current.ID
	}

	ch, err := buildChannelFromRegistrationForm(c, uploaderID)
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	ch.OwnerUserID = owner.ID
	ch.Status = "active"

	if err := db.DB.Create(&ch).Error; err != nil {
		return c.String(http.StatusBadRequest, "Failed to create channel")
	}
	return c.Redirect(http.StatusSeeOther, "/admin/channels")
}

// channelStats is the "usage stats" placeholder (note.md §9.3/§10.9) —
// computed from the topup ledger only, since no send/campaign engine
// exists yet to source real usage metrics from.
type channelStats struct {
	TotalToppedUp  int64
	CompletedCount int64
	PendingCount   int64
	LastTopupAt    *time.Time
}

func loadChannelDetail(c echo.Context) (models.Channel, []models.ChannelTopup, channelStats, error) {
	var ch models.Channel
	if err := db.DB.First(&ch, c.Param("id")).Error; err != nil {
		return ch, nil, channelStats{}, err
	}

	var topups []models.ChannelTopup
	db.DB.Where("channel_id = ?", ch.ID).Order("created_at desc").Find(&topups)

	var stats channelStats
	for _, t := range topups {
		if t.Status == "completed" {
			stats.TotalToppedUp += t.Quantity
			stats.CompletedCount++
			if stats.LastTopupAt == nil || t.CreatedAt.After(*stats.LastTopupAt) {
				at := t.CreatedAt
				stats.LastTopupAt = &at
			}
		} else if t.Status == "pending" {
			stats.PendingCount++
		}
	}

	return ch, topups, stats, nil
}

// GET /admin/channels/:id
func AdminChannelDetail(c echo.Context) error {
	ch, topups, stats, err := loadChannelDetail(c)
	if err != nil {
		return c.String(http.StatusNotFound, "Channel not found")
	}

	var owner models.User
	db.DB.First(&owner, ch.OwnerUserID)

	data := map[string]interface{}{
		"Channel": ch,
		"Owner":   owner,
		"Topups":  topups,
		"Stats":   stats,
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/channel_detail.html", data)
}

// POST /admin/channels/:id/edit
func AdminUpdateChannel(c echo.Context) error {
	var ch models.Channel
	if err := db.DB.First(&ch, c.Param("id")).Error; err != nil {
		return c.String(http.StatusNotFound, "Channel not found")
	}
	bindChannelFromForm(c, &ch)
	if err := db.DB.Save(&ch).Error; err != nil {
		return c.String(http.StatusBadRequest, "Failed to update channel")
	}
	return c.Redirect(http.StatusSeeOther, "/admin/channels/"+c.Param("id"))
}

// POST /admin/channels/:id/topup/:topup_id/review — action=complete credits
// the channel's balance; action=reject leaves it untouched. Either way the
// topup is marked reviewed and can't be actioned again.
func AdminReviewChannelTopup(c echo.Context) error {
	var topup models.ChannelTopup
	if err := db.DB.First(&topup, c.Param("topup_id")).Error; err != nil {
		return c.String(http.StatusNotFound, "Topup not found")
	}
	if topup.Status != "pending" {
		return c.String(http.StatusBadRequest, "Topup has already been reviewed")
	}

	current, _ := c.Get(auth.CtxUser).(models.User)
	action := c.FormValue("action")

	switch action {
	case "complete":
		topup.Status = "completed"
		db.DB.Model(&models.Channel{}).Where("id = ?", topup.ChannelID).
			Update("balance", gorm.Expr("balance + ?", topup.Quantity))
	case "reject":
		topup.Status = "rejected"
		topup.Note = c.FormValue("note")
	default:
		return c.String(http.StatusBadRequest, "Invalid action")
	}
	topup.ReviewedByID = current.ID

	if err := db.DB.Save(&topup).Error; err != nil {
		return c.String(http.StatusInternalServerError, "Failed to review topup")
	}
	return c.Redirect(http.StatusSeeOther, "/admin/channels/"+c.Param("id"))
}
