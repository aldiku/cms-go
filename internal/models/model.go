package models

import (
	"encoding/json"
	"strings"
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

// ApiEndpoint is an admin-defined REST route backed by raw SQL. GroupName is
// an optional path-prefix segment (e.g. "blog"), joined with Path to form
// the full public route under config.APIBasePath() — see FullPath(). SQL
// scope is intentionally unrestricted (SELECT/INSERT/UPDATE/DELETE); the
// only injection defense is that parameter values are always bound as real
// positional query args (see internal/apiengine), never string-concatenated
// into SQLText.
type ApiEndpoint struct {
	ID        uint   `gorm:"primaryKey"`
	Name      string // sidebar label + search key, not DB-unique
	GroupName string `gorm:"uniqueIndex:idx_method_group_path"` // optional prefix, e.g. "blog"
	Path      string `gorm:"uniqueIndex:idx_method_group_path"` // "/posts/:id" — ":name" = path param
	Method    string `gorm:"uniqueIndex:idx_method_group_path"` // GET/POST/PUT/PATCH/DELETE
	Tags      string // comma-separated free text; "auth" is the only one enforced

	Status uint8 // 1 = active/routable

	ParamsJSON         string // JSON array of ApiParameter
	SQLText            string // raw SQL with :name placeholders, never templated
	ResponseConfigJSON string // JSON ResponseConfig

	CreatedAt time.Time
	UpdatedAt time.Time
}

// ApiParameter is one row of the Parameters tab. Stored as an element of
// ApiEndpoint.ParamsJSON, not its own table.
type ApiParameter struct {
	Name     string `json:"name"`
	Type     string `json:"type"` // string|integer|float|boolean|date|json
	Required bool   `json:"required"`
	Default  string `json:"default"`
	Source   string `json:"source"` // query|body|form|path
}

// ResponseConfig shapes the Response Output Builder tab. Built exclusively
// via encoding/json (never raw string templating) so a misconfiguration
// can't ship broken JSON to a live caller.
type ResponseConfig struct {
	Envelope      string            `json:"envelope"` // raw|data|custom
	SuccessField  string            `json:"success_field,omitempty"`
	FieldRenames  map[string]string `json:"field_renames,omitempty"` // db column -> output key
	SingleRow     bool              `json:"single_row"`              // unwrap []row -> object when count==1
	EmptyMode     string            `json:"empty_mode"`              // empty_array|null|custom_message
	EmptyMessage  string            `json:"empty_message,omitempty"`
	ErrorMode     string            `json:"error_mode"` // generic|detailed
	ErrorMessage  string            `json:"error_message,omitempty"`
	IncludeTiming bool              `json:"include_timing"` // opt-in query time in LIVE responses; default false
}

func (e ApiEndpoint) Parameters() ([]ApiParameter, error) {
	var p []ApiParameter
	if e.ParamsJSON == "" {
		return p, nil
	}
	err := json.Unmarshal([]byte(e.ParamsJSON), &p)
	return p, err
}

func (e *ApiEndpoint) SetParameters(p []ApiParameter) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	e.ParamsJSON = string(b)
	return nil
}

func (e ApiEndpoint) Response() (ResponseConfig, error) {
	var r ResponseConfig
	if e.ResponseConfigJSON == "" {
		return r, nil
	}
	err := json.Unmarshal([]byte(e.ResponseConfigJSON), &r)
	return r, err
}

func (e *ApiEndpoint) SetResponse(r ResponseConfig) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	e.ResponseConfigJSON = string(b)
	return nil
}

// HasTag checks the comma-separated Tags field for an exact tag match.
func (e ApiEndpoint) HasTag(tag string) bool {
	for _, t := range strings.Split(e.Tags, ",") {
		if strings.TrimSpace(t) == tag {
			return true
		}
	}
	return false
}

// FullPath is the endpoint's public path relative to config.APIBasePath():
// GroupName (if set) joined with Path, e.g. "blog" + "/posts/:id" ->
// "/blog/posts/:id".
func (e ApiEndpoint) FullPath() string {
	g := strings.Trim(e.GroupName, "/")
	if g == "" {
		return e.Path
	}
	return "/" + g + e.Path
}
