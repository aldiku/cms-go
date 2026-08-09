package handlers

import (
	"net/http"

	"cms-go/internal/migrator"

	"github.com/labstack/echo/v4"
)

// AdminSetupPostsAPI creates the "Get Posts Paginated" API endpoint
// GET /admin/api-setup/posts - displays setup info
// POST /admin/api-setup/posts - creates the endpoint
func AdminSetupPostsAPI(c echo.Context) error {
	if c.Request().Method == "POST" {
		if err := migrator.SetupGetPostsPaginatedAPI(); err != nil {
			return c.String(http.StatusInternalServerError, "Failed to create API: "+err.Error())
		}
		return c.Redirect(http.StatusSeeOther, "/admin/api-builder?setup_success=posts")
	}

	data := map[string]interface{}{
		"Title": "Setup Posts API",
		"APIName": "Get Posts Paginated",
		"Path": "/posts",
		"Method": "GET",
		"Description": "Returns paginated list of published posts with metadata (title, description, tags, thumbnail, author, published date)",
		"BaseURL": "/api/public",
	}

	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/api_setup_posts.html", data)
}
