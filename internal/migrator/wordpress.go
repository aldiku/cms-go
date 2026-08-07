package migrator

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"cms-go/internal/db"
	"cms-go/internal/models"
)

const wpBaseURL = "https://adsqoo.id/wp/wp-json/wp/v2"

type wpPost struct {
	ID             int    `json:"id"`
	Date           string `json:"date"`
	Slug           string `json:"slug"`
	Status         string `json:"status"`
	Type           string `json:"type"`
	Title          wpText `json:"title"`
	Content        wpText `json:"content"`
	Excerpt        wpText `json:"excerpt"`
	FeaturedMedia  int    `json:"featured_media"`
	Categories     []int  `json:"categories"`
	Tags           []int  `json:"tags"`
	Embedded       wpEmbedded `json:"_embedded"`
}

type wpText struct {
	Rendered string `json:"rendered"`
}

type wpEmbedded struct {
	FeaturedMedia []wpMedia `json:"wp:featuredmedia"`
	Terms         [][]wpTerm `json:"wp:term"`
	Author        []wpAuthor `json:"author"`
}

type wpMedia struct {
	ID        int    `json:"id"`
	SourceURL string `json:"source_url"`
	AltText   string `json:"alt_text"`
	Title     wpText `json:"title"`
	Details   wpMediaDetails `json:"media_details"`
}

type wpMediaDetails struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type wpTerm struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Taxonomy string `json:"taxonomy"`
}

type wpAuthor struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// fetchWPPosts retrieves paginated posts from WordPress with embedded media/terms/author.
func fetchWPPosts(page int, perPage int) ([]wpPost, error) {
	url := fmt.Sprintf("%s/posts?per_page=%d&page=%d&_embed", wpBaseURL, perPage, page)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch posts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fetch posts: status %d: %s", resp.StatusCode, string(body))
	}

	var posts []wpPost
	if err := json.NewDecoder(resp.Body).Decode(&posts); err != nil {
		return nil, fmt.Errorf("decode posts: %w", err)
	}
	return posts, nil
}

// fetchWPPages retrieves paginated pages from WordPress with embedded media/author.
func fetchWPPages(page int, perPage int) ([]wpPost, error) {
	url := fmt.Sprintf("%s/pages?per_page=%d&page=%d&_embed", wpBaseURL, perPage, page)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch pages: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fetch pages: status %d: %s", resp.StatusCode, string(body))
	}

	var pages []wpPost
	if err := json.NewDecoder(resp.Body).Decode(&pages); err != nil {
		return nil, fmt.Errorf("decode pages: %w", err)
	}
	return pages, nil
}

// findOrCreateAuthor finds an author by WP name or creates one as "Unknown Author" if not found.
// Returns the user ID to assign to the page as AuthorID.
func findOrCreateAuthor(wpAuthors []wpAuthor, defaultAuthorID uint) uint {
	if len(wpAuthors) == 0 {
		return defaultAuthorID
	}

	authorName := wpAuthors[0].Name
	var user models.User
	if err := db.DB.Where("firstname = ?", authorName).First(&user).Error; err != nil {
		// Author doesn't exist; try to find a default admin user
		if defaultAuthorID != 0 {
			return defaultAuthorID
		}
		var fallback models.User
		if err := db.DB.Order("id asc").First(&fallback).Error; err == nil {
			return fallback.ID
		}
		return 0
	}
	return user.ID
}

// findOrCreateCategories retrieves existing categories by slug or name, or creates new ones.
func findOrCreateCategories(terms []wpTerm) []models.Category {
	categories := make([]models.Category, 0)
	for _, t := range terms {
		if t.Taxonomy != "category" {
			continue
		}
		var cat models.Category
		if err := db.DB.Where("slug = ? OR name = ?", t.Slug, t.Name).First(&cat).Error; err != nil {
			cat = models.Category{Name: t.Name, Slug: t.Slug}
			if err := db.DB.Create(&cat).Error; err != nil {
				continue
			}
		}
		categories = append(categories, cat)
	}
	return categories
}

// findOrCreateTags retrieves existing tags by slug or name, or creates new ones.
func findOrCreateTags(terms []wpTerm) []models.Tag {
	tags := make([]models.Tag, 0)
	for _, t := range terms {
		if t.Taxonomy != "post_tag" {
			continue
		}
		var tag models.Tag
		if err := db.DB.Where("slug = ? OR name = ?", t.Slug, t.Name).First(&tag).Error; err != nil {
			tag = models.Tag{Name: t.Name, Slug: t.Slug}
			if err := db.DB.Create(&tag).Error; err != nil {
				continue
			}
		}
		tags = append(tags, tag)
	}
	return tags
}

// createOrUpdateMediaFromWP creates a Media record with a URL reference (no file download).
func createOrUpdateMediaFromWP(wpMedia wpMedia) (uint, error) {
	var existing models.Media
	path := extractPathFromURL(wpMedia.SourceURL)
	if err := db.DB.Where("url = ?", wpMedia.SourceURL).First(&existing).Error; err == nil {
		return existing.ID, nil
	}

	media := models.Media{
		OriginalName: wpMedia.Title.Rendered,
		Path:         path,
		URL:          wpMedia.SourceURL,
		Type:         models.MediaTypeImage,
		MimeType:     "image/jpeg",
		Width:        wpMedia.Details.Width,
		Height:       wpMedia.Details.Height,
		Title:        wpMedia.Title.Rendered,
		AltText:      wpMedia.AltText,
	}
	if err := db.DB.Create(&media).Error; err != nil {
		return 0, fmt.Errorf("create media: %w", err)
	}
	return media.ID, nil
}

// extractPathFromURL extracts a relative path from a full WP media URL.
// e.g., https://adsqoo.id/wp/wp-content/uploads/2026/08/foo.jpg => uploads/2026/08/foo.jpg
func extractPathFromURL(url string) string {
	parts := strings.Split(url, "/wp-content/uploads/")
	if len(parts) > 1 {
		return "uploads/" + parts[1]
	}
	return url
}

// MigrateWPData fetches 5 posts and 5 pages from WordPress and saves them to the DB.
// pageType should be "post" or "page". defaultAuthorID is used if no author is found in WP data.
func MigrateWPData(defaultAuthorID uint) error {
	fmt.Println("Starting WordPress migration...")

	// Fetch posts
	fmt.Println("Fetching posts from WordPress...")
	posts, err := fetchWPPosts(1, 5)
	if err != nil {
		return fmt.Errorf("fetch posts: %w", err)
	}
	fmt.Printf("Fetched %d posts\n", len(posts))

	// Fetch pages
	fmt.Println("Fetching pages from WordPress...")
	pages, err := fetchWPPages(1, 5)
	if err != nil {
		return fmt.Errorf("fetch pages: %w", err)
	}
	fmt.Printf("Fetched %d pages\n", len(pages))

	// Migrate posts
	postCount := 0
	for _, wp := range posts {
		if err := migrateSinglePost(wp, defaultAuthorID, "post"); err != nil {
			fmt.Printf("Error migrating post %d: %v\n", wp.ID, err)
			continue
		}
		postCount++
	}
	fmt.Printf("Successfully migrated %d posts\n", postCount)

	// Migrate pages
	pageCount := 0
	for _, wp := range pages {
		if err := migrateSinglePost(wp, defaultAuthorID, "page"); err != nil {
			fmt.Printf("Error migrating page %d: %v\n", wp.ID, err)
			continue
		}
		pageCount++
	}
	fmt.Printf("Successfully migrated %d pages\n", pageCount)

	fmt.Println("WordPress migration complete!")
	return nil
}

// preparePageContent prepends Elementor CSS links to page content.
func preparePageContent(wpID int, content string) string {
	cssLinks := fmt.Sprintf(`<link rel="stylesheet" href="https://adsqoo.id/wp-content/uploads/elementor/css/post-%d.css">
<link rel="stylesheet" href="https://adsqoo.id/wp-content/uploads/elementor/css/post-6.css?ver=1754297852">
<link rel="stylesheet" href="https://adsqoo.id/wp-content/plugins/elementor/assets/css/frontend.min.css">
`, wpID)
	return cssLinks + content
}

// preparePostContent prepends featured image to post content.
func preparePostContent(wpMedia *wpMedia, postTitle string, content string) string {
	if wpMedia == nil {
		return content
	}
	imgTag := fmt.Sprintf(`<img src="%s" alt="%s" style="width: 100%%; margin-bottom: 1rem;">
`, wpMedia.SourceURL, postTitle)
	return imgTag + content
}

// migrateSinglePost converts a WordPress post/page into a local Page record.
func migrateSinglePost(wp wpPost, defaultAuthorID uint, pageType string) error {
	// Check if already migrated
	var existing models.Page
	if err := db.DB.Where("slug = ?", wp.Slug).First(&existing).Error; err == nil {
		fmt.Printf("  Skipping %s (already exists): %s\n", wp.Type, wp.Slug)
		return nil
	}

	// Create featured image if present
	var featuredImageID uint
	if len(wp.Embedded.FeaturedMedia) > 0 && wp.FeaturedMedia != 0 {
		if id, err := createOrUpdateMediaFromWP(wp.Embedded.FeaturedMedia[0]); err == nil {
			featuredImageID = id
		}
	}

	// Find or create author
	authorID := findOrCreateAuthor(wp.Embedded.Author, defaultAuthorID)

	// Parse published date
	var publishedAt *time.Time
	if wp.Status == "publish" {
		if t, err := time.Parse(time.RFC3339, wp.Date); err == nil {
			publishedAt = &t
		}
	}

	// Prepare content: add CSS for pages, featured image for posts
	content := wp.Content.Rendered
	if pageType == "page" {
		content = preparePageContent(wp.ID, content)
	} else if pageType == "post" && len(wp.Embedded.FeaturedMedia) > 0 {
		content = preparePostContent(&wp.Embedded.FeaturedMedia[0], wp.Title.Rendered, content)
	}

	// Create page record
	page := models.Page{
		Title:           wp.Title.Rendered,
		Slug:            wp.Slug,
		Type:            pageType,
		Content:         content,
		Status:          wp.Status,
		AuthorID:        authorID,
		FeaturedImageID: featuredImageID,
		PublishedAt:     publishedAt,
	}

	if err := db.DB.Create(&page).Error; err != nil {
		return fmt.Errorf("create page: %w", err)
	}

	// Apply categories and tags (only for "post" type in WP)
	if len(wp.Embedded.Terms) > 0 && pageType == "post" {
		allTerms := make([]wpTerm, 0)
		for _, termGroup := range wp.Embedded.Terms {
			allTerms = append(allTerms, termGroup...)
		}

		categories := findOrCreateCategories(allTerms)
		if len(categories) > 0 {
			db.DB.Model(&page).Association("Categories").Append(categories)
		}

		tags := findOrCreateTags(allTerms)
		if len(tags) > 0 {
			db.DB.Model(&page).Association("Tags").Append(tags)
		}
	}

	fmt.Printf("  Migrated %s: %s (%d)\n", pageType, wp.Slug, page.ID)
	return nil
}
