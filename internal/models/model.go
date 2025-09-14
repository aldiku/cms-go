package models

import "time"

type Page struct {
	ID        uint `gorm:"primaryKey"`
	Title     string
	Slug      string `gorm:"uniqueIndex"`
	Type      string // "page" | "post"
	Content   string // JSON or markdown
	LayoutID  uint
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Layout struct {
	ID        uint   `gorm:"primaryKey"`
	Name      string `gorm:"uniqueIndex"`
	Structure string // JSON for rows/columns/components
	Template  string // HTML template (Go template syntax)
	CreatedAt time.Time
}

type Menu struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"uniqueIndex"`
}

type Component struct {
	ID       uint   `gorm:"primaryKey"`
	Name     string `gorm:"uniqueIndex"`
	Schema   string // JSON schema untuk props
	Template string // HTML template (Go template syntax)
}
