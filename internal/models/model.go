package models

import (
	"time"

	"gorm.io/gorm"
)

type Page struct {
	ID        uint `gorm:"primaryKey"`
	Title     string
	Slug      string `gorm:"uniqueIndex"`
	Type      string // "page" | "post"
	Content   string // JSON or markdown
	LayoutID  uint
	CreatedAt time.Time
	UpdatedAt time.Time

	// SEO
	MetaTitle          string // overrides <title> / og:title / twitter:title if set
	MetaDescription    string
	CanonicalURL       string // absolute override; falls back to SITE_URL + slug
	FocusKeyword       string // stored for future on-page analysis, not scored in v1
	MetaRobotsNoindex  bool
	MetaRobotsNofollow bool
	OGTitle            string // falls back to MetaTitle -> Title
	OGDescription      string // falls back to MetaDescription
	OGImage            string
	TwitterCard        string // "summary" | "summary_large_image"
	TwitterTitle       string // falls back to OGTitle
	TwitterDescription string // falls back to OGDescription
	TwitterImage       string // falls back to OGImage
}

type Layout struct {
	ID        uint   `gorm:"primaryKey"`
	Name      string `gorm:"uniqueIndex"`
	Structure string // JSON for rows/columns/components
	Template  string // HTML template (Go template syntax)
	CreatedAt time.Time
}

type Menu struct {
	ID              uint `gorm:"primaryKey"`
	Menu            string
	Path            string
	Icon            string // emoji or css class, free text
	Status          uint8  // 1 = active (shown in sidebar / routable)
	ParentID        uint
	ListOrder       uint32
	MenuDescription string
	MenuType        string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       gorm.DeletedAt `gorm:"index"`
}

type Component struct {
	ID       uint   `gorm:"primaryKey"`
	Name     string `gorm:"uniqueIndex"`
	Schema   string // JSON schema untuk props
	Template string // HTML template (Go template syntax)
}

// Revision is a point-in-time snapshot of a Page/Layout/Component, captured
// right before an update overwrites it. Only the last maxRevisionsPerEntity
// (see handlers.saveRevision) are kept per entity.
type Revision struct {
	ID         uint   `gorm:"primaryKey"`
	EntityType string `gorm:"index:idx_revision_lookup"` // "page" | "layout" | "component"
	EntityID   uint   `gorm:"index:idx_revision_lookup"`
	Data       string // JSON snapshot of the entity as it was before the update
	UserID     uint
	UserName   string
	CreatedAt  time.Time
}
