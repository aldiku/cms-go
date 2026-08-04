package auth

import (
	_ "embed"
	"time"

	"cms-go/internal/db"
	"cms-go/internal/models"
)

//go:embed seedpages/blank-layout.html
var blankLayoutTemplate string

//go:embed seedpages/login.html
var loginPageContent string

//go:embed seedpages/register.html
var registerPageContent string

//go:embed seedpages/forgot-password.html
var forgotPasswordPageContent string

//go:embed seedpages/reset-password.html
var resetPasswordPageContent string

const blankLayoutStructure = `{"name":"blank-layout","rows":[{"columns":[{"components":[{"type":"content","props":{"html":"<p>This is default placeholder.</p>"}}]}]}]}`

// SeedAuthPages bootstraps the header/footer-less "blank-layout" and the
// four public JSON-auth pages (/login, /register, /forgot-password,
// /reset-password) that back internal/handlers/auth_api.go, so a fresh
// database (e.g. a second dev machine) gets a working auth UI without
// manual setup. Idempotent — only creates rows that don't already exist by
// name/slug, and never overwrites content an admin has since edited. Call
// after SeedAuth (needs the superadmin role/user it seeds).
func SeedAuthPages() {
	// GORM's belongs-to association on Page.FeaturedImage auto-creates a
	// foreign key on featured_image_id with no NULL escape hatch (it's a
	// plain uint, not a pointer); a freshly-built Page{} leaves that field
	// at its zero value (0 = "no featured image", same convention as
	// RoleID/LayoutID/ReferralID elsewhere in this app), which would
	// violate the FK on a database that's never had this row before. This
	// sentinel makes 0 a valid, permanently-empty reference.
	db.DB.Exec(`INSERT INTO media (id, created_at, updated_at) VALUES (0, now(), now()) ON CONFLICT (id) DO NOTHING`)

	blankLayout := models.Layout{
		Name:      "blank-layout",
		Structure: blankLayoutStructure,
		Template:  blankLayoutTemplate,
	}
	db.DB.Where("name = ?", blankLayout.Name).Attrs(blankLayout).FirstOrCreate(&blankLayout)

	var role models.Role
	db.DB.Where("role = ?", SuperadminRole).First(&role)
	var admin models.User
	db.DB.Where("role_id = ?", role.ID).Order("id asc").First(&admin)

	now := time.Now()
	pages := []models.Page{
		{Title: "Login", Slug: "/login", Type: "html", Status: models.PageStatusPublish, LayoutID: blankLayout.ID, AuthorID: admin.ID, PublishedAt: &now, MetaTitle: "Login", Content: loginPageContent},
		{Title: "Register", Slug: "/register", Type: "html", Status: models.PageStatusPublish, LayoutID: blankLayout.ID, AuthorID: admin.ID, PublishedAt: &now, MetaTitle: "Register", Content: registerPageContent},
		{Title: "Forgot Password", Slug: "/forgot-password", Type: "html", Status: models.PageStatusPublish, LayoutID: blankLayout.ID, AuthorID: admin.ID, PublishedAt: &now, MetaTitle: "Forgot Password", Content: forgotPasswordPageContent},
		{Title: "Reset Password", Slug: "/reset-password", Type: "html", Status: models.PageStatusPublish, LayoutID: blankLayout.ID, AuthorID: admin.ID, PublishedAt: &now, MetaTitle: "Reset Password", Content: resetPasswordPageContent},
	}
	for _, p := range pages {
		var existing models.Page
		db.DB.Where("slug = ?", p.Slug).Attrs(p).FirstOrCreate(&existing)
	}
}
