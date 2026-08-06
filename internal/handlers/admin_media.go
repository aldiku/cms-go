package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"cms-go/internal/auth"
	"cms-go/internal/db"
	"cms-go/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

const (
	mediaUploadRoot    = "assets/uploads"
	adminMediasPerPage = 24
)

// extTypeMap classifies uploads by file extension (lowercased, with dot)
// into the WordPress-style Media Library buckets. Anything not listed here
// falls back to models.MediaTypeOther.
var extTypeMap = map[string]string{
	".jpg": models.MediaTypeImage, ".jpeg": models.MediaTypeImage, ".png": models.MediaTypeImage,
	".gif": models.MediaTypeImage, ".webp": models.MediaTypeImage, ".svg": models.MediaTypeImage,
	".bmp": models.MediaTypeImage, ".ico": models.MediaTypeImage, ".tiff": models.MediaTypeImage,

	".mp4": models.MediaTypeVideo, ".mov": models.MediaTypeVideo, ".avi": models.MediaTypeVideo,
	".mkv": models.MediaTypeVideo, ".webm": models.MediaTypeVideo, ".flv": models.MediaTypeVideo,
	".wmv": models.MediaTypeVideo,

	".mp3": models.MediaTypeAudio, ".wav": models.MediaTypeAudio, ".ogg": models.MediaTypeAudio,
	".m4a": models.MediaTypeAudio, ".flac": models.MediaTypeAudio, ".aac": models.MediaTypeAudio,

	".pdf": models.MediaTypeDocument, ".doc": models.MediaTypeDocument, ".docx": models.MediaTypeDocument,
	".xls": models.MediaTypeDocument, ".xlsx": models.MediaTypeDocument, ".ppt": models.MediaTypeDocument,
	".pptx": models.MediaTypeDocument, ".txt": models.MediaTypeDocument, ".csv": models.MediaTypeDocument,
	".odt": models.MediaTypeDocument, ".rtf": models.MediaTypeDocument,

	".zip": models.MediaTypeArchive, ".rar": models.MediaTypeArchive, ".7z": models.MediaTypeArchive,
	".tar": models.MediaTypeArchive, ".gz": models.MediaTypeArchive, ".bz2": models.MediaTypeArchive,
}

func classifyMediaType(ext string) string {
	if t, ok := extTypeMap[strings.ToLower(ext)]; ok {
		return t
	}
	return models.MediaTypeOther
}

// randomHex returns n random bytes hex-encoded, used to keep uploaded
// filenames collision-free without needing a filesystem existence check.
func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// Extremely unlikely; fall back to a timestamp so uploads never hard-fail.
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b)
}

// GET /admin/medias
func AdminMedias(c echo.Context) error {
	query := strings.TrimSpace(c.QueryParam("q"))
	mediaType := c.QueryParam("type")
	sortBy := c.QueryParam("sort")
	view := c.QueryParam("view")
	if view != "table" {
		view = "grid"
	}

	pageNum, err := strconv.Atoi(c.QueryParam("page"))
	if err != nil || pageNum < 1 {
		pageNum = 1
	}

	// withSearch returns a fresh query (scoped to the search term only) every
	// call. Each of the three queries below is terminal (Scan/Count/Find),
	// so each gets its own instance rather than sharing/mutating one — gorm
	// clones lazily, and a shared *gorm.DB here would let one query's
	// Select/Group leak into the next.
	withSearch := func() *gorm.DB {
		q := db.DB.Model(&models.Media{})
		if query != "" {
			like := "%" + query + "%"
			q = q.Where("original_name ILIKE ? OR title ILIKE ?", like, like)
		}
		return q
	}

	// Type counts for the filter tabs, computed before the type filter itself
	// is applied so every tab shows its own total (WordPress "All / Images /
	// Documents ..." style), but after the search filter so counts stay
	// consistent with an active search.
	type typeCount struct {
		Type  string
		Count int64
	}
	var counts []typeCount
	withSearch().Select("type, count(*) as count").Group("type").Scan(&counts)
	countByType := make(map[string]int64, len(counts))
	var totalCount int64
	for _, tc := range counts {
		countByType[tc.Type] = tc.Count
		totalCount += tc.Count
	}

	findQuery := withSearch()
	if mediaType != "" && mediaType != "all" {
		findQuery = findQuery.Where("type = ?", mediaType)
	}

	var total int64
	findQuery.Count(&total)

	totalPages := int((total + adminMediasPerPage - 1) / adminMediasPerPage)
	if totalPages < 1 {
		totalPages = 1
	}
	if pageNum > totalPages {
		pageNum = totalPages
	}

	order := "created_at desc"
	switch sortBy {
	case "oldest":
		order = "created_at asc"
	case "name_asc":
		order = "original_name asc"
	case "name_desc":
		order = "original_name desc"
	case "size_desc":
		order = "size desc"
	case "size_asc":
		order = "size asc"
	}

	var mediaItems []models.Media
	findQuery.Preload("Uploader").Order(order).
		Limit(adminMediasPerPage).
		Offset((pageNum - 1) * adminMediasPerPage).
		Find(&mediaItems)

	data := map[string]interface{}{
		"Media":       mediaItems,
		"Query":       query,
		"Type":        mediaType,
		"Sort":        sortBy,
		"View":        view,
		"CurrentPage": pageNum,
		"TotalPages":  totalPages,
		"HasPrev":     pageNum > 1,
		"HasNext":     pageNum < totalPages,
		"PrevPage":    pageNum - 1,
		"NextPage":    pageNum + 1,
		"TotalCount":  totalCount,
		"CountByType": countByType,
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/medias.html", data)
}

// mediaJSON is the shape returned by AdminMediasJSON — just enough for a
// picker UI to render a thumbnail/label and know what to insert.
type mediaJSON struct {
	ID           uint   `json:"id"`
	OriginalName string `json:"originalName"`
	URL          string `json:"url"`
	Type         string `json:"type"`
	SizeStr      string `json:"sizeStr"`
}

// GET /admin/medias/json — lightweight listing for in-page pickers (e.g. the
// page editor's block inserter's Media block), independent of the full
// admin Medias page's pagination/view-mode state.
func AdminMediasJSON(c echo.Context) error {
	query := strings.TrimSpace(c.QueryParam("q"))
	mediaType := c.QueryParam("type")

	q := db.DB.Model(&models.Media{})
	if query != "" {
		like := "%" + query + "%"
		q = q.Where("original_name ILIKE ? OR title ILIKE ?", like, like)
	}
	if mediaType != "" && mediaType != "all" {
		q = q.Where("type = ?", mediaType)
	}

	var items []models.Media
	q.Order("created_at desc").Limit(60).Find(&items)

	out := make([]mediaJSON, 0, len(items))
	for _, m := range items {
		out = append(out, mediaJSON{
			ID: m.ID, OriginalName: m.OriginalName, URL: m.URL,
			Type: m.Type, SizeStr: m.SizeStr(),
		})
	}
	return c.JSON(http.StatusOK, out)
}

// POST /admin/medias/upload — multipart upload, one or more files under the
// "files" field (WordPress-style bulk upload).
func AdminMediaUpload(c echo.Context) error {
	form, err := c.MultipartForm()
	if err != nil {
		return c.String(http.StatusBadRequest, "Invalid upload")
	}

	files := form.File["files"]
	if len(files) == 0 {
		return c.String(http.StatusBadRequest, "No files provided")
	}

	var uploaderID uint
	if user, ok := c.Get(auth.CtxUser).(models.User); ok {
		uploaderID = user.ID
	}

	now := time.Now()
	subDir := filepath.Join(mediaUploadRoot, now.Format("2006"), now.Format("01"))
	if err := os.MkdirAll(subDir, 0755); err != nil {
		return c.String(http.StatusInternalServerError, "Failed to create upload directory")
	}

	created := make([]models.Media, 0, len(files))
	for _, fh := range files {
		media, err := saveUploadedMedia(fh, subDir, uploaderID)
		if err != nil {
			fmt.Println("media upload error:", err)
			continue
		}
		created = append(created, media)
	}

	if len(created) == 0 {
		return c.String(http.StatusInternalServerError, "All uploads failed")
	}

	// The page editor's block-inserter uploads from a modal via fetch() and
	// needs the new items back to select from, without navigating away from
	// the page being edited — the plain Medias page instead does a normal
	// form POST and expects the usual redirect back to the library.
	if c.QueryParam("ajax") == "1" {
		out := make([]mediaJSON, 0, len(created))
		for _, m := range created {
			out = append(out, mediaJSON{
				ID: m.ID, OriginalName: m.OriginalName, URL: m.URL,
				Type: m.Type, SizeStr: m.SizeStr(),
			})
		}
		return c.JSON(http.StatusOK, out)
	}
	return c.Redirect(http.StatusSeeOther, "/admin/medias")
}

func saveUploadedMedia(fh *multipart.FileHeader, subDir string, uploaderID uint) (models.Media, error) {
	src, err := fh.Open()
	if err != nil {
		return models.Media{}, err
	}
	defer src.Close()

	ext := filepath.Ext(fh.Filename)
	base := slugify(strings.TrimSuffix(fh.Filename, ext))
	if base == "" {
		base = "file"
	}
	fileName := fmt.Sprintf("%s-%s%s", base, randomHex(4), strings.ToLower(ext))
	relPath := filepath.Join(subDir, fileName)

	dst, err := os.Create(relPath)
	if err != nil {
		return models.Media{}, err
	}
	defer dst.Close()

	size, err := io.Copy(dst, src)
	if err != nil {
		return models.Media{}, err
	}

	mediaType := classifyMediaType(ext)

	width, height := 0, 0
	if mediaType == models.MediaTypeImage {
		if f, err := os.Open(relPath); err == nil {
			if cfg, _, err := image.DecodeConfig(f); err == nil {
				width, height = cfg.Width, cfg.Height
			}
			f.Close()
		}
	}

	// relPath is rooted at the repo root (e.g. "assets/uploads/2026/08/x.jpg"),
	// which is exactly the URL "/assets/uploads/2026/08/x.jpg" serves given
	// e.Static("/assets", "assets") mounts that directory at "/assets".
	urlPath := "/" + filepath.ToSlash(relPath)

	media := models.Media{
		FileName:     fileName,
		OriginalName: fh.Filename,
		Path:         strings.TrimPrefix(filepath.ToSlash(relPath), "assets/"),
		URL:          urlPath,
		MimeType:     fh.Header.Get("Content-Type"),
		Type:         mediaType,
		Size:         size,
		Width:        width,
		Height:       height,
		UploaderID:   uploaderID,
	}
	if err := db.DB.Create(&media).Error; err != nil {
		return models.Media{}, err
	}
	return media, nil
}

// POST /admin/medias/:id/delete
func AdminDeleteMedia(c echo.Context) error {
	var media models.Media
	if err := db.DB.First(&media, c.Param("id")).Error; err != nil {
		return c.String(http.StatusNotFound, "Media not found")
	}

	// Media rows are referenced by belongs-to FKs (Page.FeaturedImage,
	// ProductCategory.Image, Product.Thumbnail, GeneralSetting.Favicon)
	// with no ON DELETE clause, so a straight DELETE 500s with a FK
	// violation that was previously swallowed — the page would redirect
	// as if it succeeded while the row (and file) stayed put. Check usage
	// up front and report it instead of silently no-op'ing.
	var inUse []string
	var count int64

	db.DB.Model(&models.Page{}).Where("featured_image_id = ?", media.ID).Count(&count)
	if count > 0 {
		inUse = append(inUse, fmt.Sprintf("%d page(s)", count))
	}
	db.DB.Model(&models.ProductCategory{}).Where("image_id = ?", media.ID).Count(&count)
	if count > 0 {
		inUse = append(inUse, fmt.Sprintf("%d product categor(y/ies)", count))
	}
	db.DB.Model(&models.Product{}).Where("thumbnail_id = ?", media.ID).Count(&count)
	if count > 0 {
		inUse = append(inUse, fmt.Sprintf("%d product(s)", count))
	}
	db.DB.Model(&models.GeneralSetting{}).Where("favicon_id = ?", media.ID).Count(&count)
	if count > 0 {
		inUse = append(inUse, "the site favicon")
	}
	if len(inUse) > 0 {
		return c.String(http.StatusBadRequest, "Cannot delete: this file is still used by "+strings.Join(inUse, ", "))
	}

	if err := db.DB.Delete(&media).Error; err != nil {
		return c.String(http.StatusInternalServerError, "Failed to delete media: "+err.Error())
	}
	os.Remove(filepath.Join("assets", media.Path))
	return c.Redirect(http.StatusSeeOther, "/admin/medias")
}
