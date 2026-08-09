package migrator

import (
	"encoding/json"
	"fmt"

	"cms-go/internal/db"
	"cms-go/internal/models"
)

// SetupGetPostsPaginatedAPI creates the "Get Posts Paginated" API endpoint
func SetupGetPostsPaginatedAPI() error {
	// Check if endpoint already exists
	var existing models.ApiEndpoint
	if err := db.DB.Where("path = ? AND method = ?", "/posts", "GET").First(&existing).Error; err == nil {
		fmt.Println("✓ API endpoint 'Get Posts Paginated' already exists")
		return nil
	}

	// SQL query for paginated posts
	sqlText := `SELECT
  p.id,
  p.title,
  p.meta_description,
  m.url as thumbnail,
  p.published_at,
  u.firstname || ' ' || u.lastname as author,
  string_agg(t.name, ', ') as tags
FROM pages p
LEFT JOIN media m ON p.featured_image_id = m.id
LEFT JOIN users u ON p.author_id = u.id
LEFT JOIN page_tags pt ON p.id = pt.page_id
LEFT JOIN tags t ON pt.tag_id = t.id
WHERE p.type = 'post' AND p.status = 'publish'
GROUP BY p.id, p.title, p.meta_description, m.url, p.published_at, u.firstname, u.lastname
ORDER BY p.published_at DESC
LIMIT :limit OFFSET :offset`

	// API Parameters
	params := []models.ApiParameter{
		{
			Name:     "page",
			Type:     "integer",
			Required: false,
			Default:  "1",
			Source:   "query",
		},
		{
			Name:     "per_page",
			Type:     "integer",
			Required: false,
			Default:  "10",
			Source:   "query",
		},
	}

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("marshal params: %w", err)
	}

	// Response Config
	responseConfig := models.ResponseConfig{
		Envelope:     "data",
		SuccessField: "",
		SingleRow:    false,
		EmptyMode:    "empty_array",
		ErrorMode:    "detailed",
	}

	responseJSON, err := json.Marshal(responseConfig)
	if err != nil {
		return fmt.Errorf("marshal response config: %w", err)
	}

	// Create API Endpoint
	endpoint := models.ApiEndpoint{
		Name:               "Get Posts Paginated",
		GroupName:          "public",
		Path:               "/posts",
		Method:             "GET",
		Tags:               "public",
		Status:             1,
		ParamsJSON:         string(paramsJSON),
		SQLText:            sqlText,
		ResponseConfigJSON: string(responseJSON),
	}

	if err := db.DB.Create(&endpoint).Error; err != nil {
		return fmt.Errorf("create endpoint: %w", err)
	}

	fmt.Printf("✓ Created API endpoint: GET /api/public/posts (paginated)\n")
	return nil
}
