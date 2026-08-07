package handlers

import (
	"cms-go/internal/auth"
	"cms-go/internal/db"
	"cms-go/internal/generator"
	"cms-go/internal/models"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

const adminPagesPerPage = 10

func AdminPages(c echo.Context) error {
	query := strings.TrimSpace(c.QueryParam("q"))
	filterType := c.QueryParam("type")
	filterStatus := c.QueryParam("status")
	filterLayoutID := c.QueryParam("layout_id")
	filterAuthorID := c.QueryParam("author_id")

	pageNum, err := strconv.Atoi(c.QueryParam("page"))
	if err != nil || pageNum < 1 {
		pageNum = 1
	}

	like := "%" + query + "%"

	applyFilters := func(q *gorm.DB) *gorm.DB {
		if query != "" {
			q = q.Where("title ILIKE ? OR slug ILIKE ?", like, like)
		}
		if filterType != "" {
			q = q.Where("type = ?", filterType)
		}
		if filterStatus != "" {
			q = q.Where("status = ?", filterStatus)
		}
		if filterLayoutID != "" {
			q = q.Where("layout_id = ?", filterLayoutID)
		}
		if filterAuthorID != "" {
			q = q.Where("author_id = ?", filterAuthorID)
		}
		return q
	}

	var total int64
	applyFilters(db.DB.Model(&models.Page{})).Count(&total)

	totalPages := int((total + adminPagesPerPage - 1) / adminPagesPerPage)
	if totalPages < 1 {
		totalPages = 1
	}
	if pageNum > totalPages {
		pageNum = totalPages
	}

	var pages []models.Page
	applyFilters(db.DB.Model(&models.Page{}).Preload("Author").Preload("Categories").Preload("FeaturedImage")).
		Order("id desc").
		Limit(adminPagesPerPage).
		Offset((pageNum - 1) * adminPagesPerPage).
		Find(&pages)

	var layouts []models.Layout
	db.DB.Order("name asc").Find(&layouts)
	layoutNames := make(map[uint]string, len(layouts))
	for _, l := range layouts {
		layoutNames[l.ID] = l.Name
	}

	var authors []models.User
	db.DB.Order("firstname asc").Find(&authors)

	data := map[string]interface{}{
		"Pages":          pages,
		"Query":          query,
		"FilterType":     filterType,
		"FilterStatus":   filterStatus,
		"FilterLayoutID": filterLayoutID,
		"FilterAuthorID": filterAuthorID,
		"CurrentPage":    pageNum,
		"TotalPages":     totalPages,
		"HasPrev":        pageNum > 1,
		"HasNext":        pageNum < totalPages,
		"PrevPage":       pageNum - 1,
		"NextPage":       pageNum + 1,
		"Layouts":        layouts,
		"LayoutNames":    layoutNames,
		"Authors":        authors,
	}

	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/pages.html", data)
}

// bindPageFromForm reads the shared create/edit form fields (type, layout,
// author, status, meta + SEO) from the request into page. Categories and
// Tags are many2many associations and are applied separately, once the page
// has an ID — see applyPageTaxonomy.
func bindPageFromForm(c echo.Context, page *models.Page) {
	page.Title = c.FormValue("title")
	page.Slug = c.FormValue("slug")
	page.Content = strings.TrimSpace(c.FormValue("content"))

	if t := c.FormValue("type"); t != "" {
		page.Type = t
	} else if page.Type == "" {
		page.Type = "page"
	}

	if layoutIDStr := c.FormValue("layout_id"); layoutIDStr != "" {
		if layoutID, err := strconv.ParseUint(layoutIDStr, 10, 64); err == nil {
			page.LayoutID = uint(layoutID)
		}
	}

	if status := c.FormValue("status"); status != "" {
		page.Status = status
	} else if page.Status == "" {
		page.Status = models.PageStatusDraft
	}

	if authorIDStr := c.FormValue("author_id"); authorIDStr != "" {
		if authorID, err := strconv.ParseUint(authorIDStr, 10, 64); err == nil {
			page.AuthorID = uint(authorID)
		}
	}

	if featuredIDStr := c.FormValue("featured_image_id"); featuredIDStr != "" {
		if featuredID, err := strconv.ParseUint(featuredIDStr, 10, 64); err == nil {
			page.FeaturedImageID = uint(featuredID)
		}
	} else {
		page.FeaturedImageID = 0 // blank = explicitly cleared via the Featured Image card
	}

	if publishedAtStr := c.FormValue("published_at"); publishedAtStr != "" {
		// <input type="datetime-local"> carries no timezone — it's a "wall
		// clock" value the user typed, meant as local time, not UTC.
		if t, err := time.ParseInLocation("2006-01-02T15:04", publishedAtStr, time.Local); err == nil {
			page.PublishedAt = &t
		}
	}
	if page.Status == models.PageStatusPublish && page.PublishedAt == nil {
		now := time.Now()
		page.PublishedAt = &now
	}

	page.MetaTitle = c.FormValue("meta_title")
	page.MetaDescription = c.FormValue("meta_description")
	page.CanonicalURL = c.FormValue("canonical_url")
	page.FocusKeyword = c.FormValue("focus_keyword")
	page.MetaRobotsNoindex = c.FormValue("meta_robots_noindex") == "on"
	page.MetaRobotsNofollow = c.FormValue("meta_robots_nofollow") == "on"
	page.OGTitle = c.FormValue("og_title")
	page.OGDescription = c.FormValue("og_description")
	page.OGImage = c.FormValue("og_image")
	page.TwitterCard = c.FormValue("twitter_card")
	page.TwitterTitle = c.FormValue("twitter_title")
	page.TwitterDescription = c.FormValue("twitter_description")
	page.TwitterImage = c.FormValue("twitter_image")
}

// applyPageTaxonomy replaces a page's Categories/Tags associations from the
// submitted form. Tag names that don't exist yet are created on the fly
// (WordPress-style free-form tagging); categories must already exist (picked
// from checkboxes populated by AdminPageEditor).
func applyPageTaxonomy(c echo.Context, page *models.Page) {
	form, _ := c.FormParams()

	var categories []models.Category
	if ids := form["category_ids"]; len(ids) > 0 {
		db.DB.Where("id IN ?", ids).Find(&categories)
	}
	db.DB.Model(page).Association("Categories").Replace(categories)

	tags := findOrCreateTags(splitTagNames(c.FormValue("tags")))
	db.DB.Model(page).Association("Tags").Replace(tags)
}

// splitTagNames parses the comma-separated "tags" field into a deduplicated
// (case-insensitive), trimmed list, preserving first-seen casing/order.
func splitTagNames(raw string) []string {
	parts := strings.Split(raw, ",")
	names := make([]string, 0, len(parts))
	seen := make(map[string]bool, len(parts))
	for _, p := range parts {
		name := strings.TrimSpace(p)
		if name == "" || seen[strings.ToLower(name)] {
			continue
		}
		seen[strings.ToLower(name)] = true
		names = append(names, name)
	}
	return names
}

// findOrCreateTags looks up each tag name, creating it if it doesn't exist
// yet.
func findOrCreateTags(names []string) []models.Tag {
	tags := make([]models.Tag, 0, len(names))
	for _, name := range names {
		var tag models.Tag
		if err := db.DB.Where("name = ?", name).First(&tag).Error; err != nil {
			tag = models.Tag{Name: name, Slug: slugify(name)}
			if err := db.DB.Create(&tag).Error; err != nil {
				continue
			}
		}
		tags = append(tags, tag)
	}
	return tags
}

func AdminCreatePage(c echo.Context) error {
	page := models.Page{}
	bindPageFromForm(c, &page)
	if page.AuthorID == 0 {
		if user, ok := c.Get(auth.CtxUser).(models.User); ok {
			page.AuthorID = user.ID
		}
	}

	if err := db.DB.Create(&page).Error; err != nil {
		return c.String(http.StatusBadRequest, "Failed to create page")
	}
	applyPageTaxonomy(c, &page)

	if err := generator.GenerateTemplatesFromDB(); err != nil {
		fmt.Println("template generation error:", err)
	}
	return c.Redirect(http.StatusFound, "/admin/pages")
}

func AdminPageEditor(c echo.Context) error {
	var page models.Page
	var layouts []models.Layout

	id := c.Param("id")
	if id != "" {
		// Editing existing page
		db.DB.Preload("Author").Preload("Categories").Preload("Tags").Preload("FeaturedImage").First(&page, id)
	}

	db.DB.Find(&layouts)

	var users []models.User
	db.DB.Order("firstname asc").Find(&users)

	var categories []models.Category
	db.DB.Order("name asc").Find(&categories)

	selectedCategories := make(map[uint]bool, len(page.Categories))
	for _, cat := range page.Categories {
		selectedCategories[cat.ID] = true
	}

	tagNames := make([]string, len(page.Tags))
	for i, t := range page.Tags {
		tagNames[i] = t.Name
	}

	data := map[string]interface{}{
		"Page":               page,
		"Layouts":            layouts,
		"Users":              users,
		"Categories":         categories,
		"SelectedCategories": selectedCategories,
		"TagsCSV":            strings.Join(tagNames, ", "),
	}
	if page.ID != 0 {
		data["Revisions"] = loadRevisions("page", page.ID)
	}

	return renderWithLayout(
		c,
		"internal/views/admin/admin-layout.html",
		"internal/views/admin/page-editor.html",
		data,
	)
}

// GET /admin/pages/:id/edit
func AdminEditPage(c echo.Context) error {
	id := c.Param("id")
	var page models.Page
	if err := db.DB.First(&page, id).Error; err != nil {
		return c.String(http.StatusNotFound, "Page not found")
	}

	data := map[string]interface{}{
		"Page": page,
	}

	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/edit_page.html", data)
}

// POST /admin/pages/:id/edit
func AdminUpdatePage(c echo.Context) error {
	id := c.Param("id")
	var page models.Page
	if err := db.DB.First(&page, id).Error; err != nil {
		return c.String(http.StatusNotFound, "Page not found")
	}
	before := page

	bindPageFromForm(c, &page)

	if err := db.DB.Save(&page).Error; err != nil {
		return c.String(http.StatusInternalServerError, "Failed to update page")
	}
	applyPageTaxonomy(c, &page)
	saveRevision(c, "page", page.ID, before)

	if err := generator.GenerateTemplatesFromDB(); err != nil {
		fmt.Println("template generation error:", err)
	}

	return c.Redirect(http.StatusSeeOther, "/admin/pages")
}
