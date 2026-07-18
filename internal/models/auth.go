package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID         uint   `gorm:"primaryKey"`
	Firstname  string
	Lastname   string
	Email      string `gorm:"uniqueIndex"`
	Phone      string
	Password   string // bcrypt hash, never rendered
	Address    string
	Company    string
	EmployeeID string
	Avatar     string
	RoleID     uint
	Status     uint8 // 1 = active
	VerifiedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  gorm.DeletedAt `gorm:"index"`
	Role       Role
}

func (u User) FullName() string {
	if u.Lastname == "" {
		return u.Firstname
	}
	return u.Firstname + " " + u.Lastname
}

type Role struct {
	ID        uint   `gorm:"primaryKey"`
	Role      string `gorm:"uniqueIndex"`
	Status    uint8 // 1 = active
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Permission struct {
	ID         uint   `gorm:"primaryKey"`
	Permission string // label, e.g. "superadmin:pages"
	RoleID     uint   `gorm:"uniqueIndex:idx_role_menu"`
	MenuID     uint   `gorm:"uniqueIndex:idx_role_menu"`
	CanCreate  bool   `gorm:"column:create"`
	CanRead    bool   `gorm:"column:read"`
	CanUpdate  bool   `gorm:"column:update"`
	CanDelete  bool   `gorm:"column:delete"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Session struct {
	Token     string `gorm:"primaryKey;size:64"` // hex of 32 random bytes
	UserID    uint   `gorm:"index"`
	ExpiresAt time.Time
	CreatedAt time.Time
}
