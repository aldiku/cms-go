package auth

import (
	"log"
	"os"

	"cms-go/internal/db"
	"cms-go/internal/models"
)

// SeedAuth bootstraps the auth tables on first boot: a superadmin role, an
// admin user from ADMIN_EMAIL/ADMIN_PASSWORD, the admin menus, and full-CRUD
// permissions for superadmin. Each block only runs when its table is empty,
// so partial re-seeds work and existing data is never touched.
func SeedAuth() {
	var role models.Role

	var roleCount int64
	db.DB.Model(&models.Role{}).Count(&roleCount)
	if roleCount == 0 {
		role = models.Role{Role: SuperadminRole, Status: 1}
		if err := db.DB.Create(&role).Error; err != nil {
			log.Printf("seed: create role failed: %v", err)
			return
		}
		log.Printf("seed: created role %q", role.Role)
	} else {
		db.DB.Where("role = ?", SuperadminRole).First(&role)
	}

	var userCount int64
	db.DB.Model(&models.User{}).Count(&userCount)
	if userCount == 0 {
		email := os.Getenv("ADMIN_EMAIL")
		if email == "" {
			email = "admin@example.com"
		}
		password := os.Getenv("ADMIN_PASSWORD")
		if password == "" {
			password = "admin123"
		}

		hash, err := HashPassword(password)
		if err != nil {
			log.Printf("seed: hash password failed: %v", err)
			return
		}
		user := models.User{
			Firstname: "Admin",
			Email:     email,
			Password:  hash,
			RoleID:    role.ID,
			Status:    1,
		}
		if err := db.DB.Create(&user).Error; err != nil {
			log.Printf("seed: create admin user failed: %v", err)
			return
		}
		log.Printf("seed: created admin user %s", email)
		if os.Getenv("ADMIN_PASSWORD") == "" {
			log.Println("⚠️  seed: admin password is the default 'admin123' — set ADMIN_PASSWORD in .env and change it immediately")
		}
	}

	var menuCount int64
	db.DB.Model(&models.Menu{}).Count(&menuCount)
	if menuCount == 0 {
		seedMenus := []models.Menu{
			{Menu: "Dashboard", Path: "/admin", Icon: "🏠", MenuType: "module"},
			{Menu: "Pages", Path: "/admin/pages", Icon: "📄", MenuType: "module"},
			{Menu: "Components", Path: "/admin/components", Icon: "🧩", MenuType: "module"},
			{Menu: "Layouts", Path: "/admin/layouts", Icon: "🖼️", MenuType: "module"},
			{Menu: "Users", Path: "/admin/users", Icon: "👤", MenuType: "settings"},
			{Menu: "Roles", Path: "/admin/roles", Icon: "🛡️", MenuType: "settings"},
			{Menu: "Menus", Path: "/admin/menus", Icon: "📋", MenuType: "settings"},
			{Menu: "Permissions", Path: "/admin/permissions", Icon: "🔑", MenuType: "settings"},
		}
		for i := range seedMenus {
			seedMenus[i].Status = 1
			seedMenus[i].ListOrder = uint32(i + 1)
		}
		if err := db.DB.Create(&seedMenus).Error; err != nil {
			log.Printf("seed: create menus failed: %v", err)
			return
		}
		log.Printf("seed: created %d menus", len(seedMenus))
	}

	var permCount int64
	db.DB.Model(&models.Permission{}).Count(&permCount)
	if permCount == 0 && role.ID != 0 {
		var menus []models.Menu
		db.DB.Find(&menus)
		perms := make([]models.Permission, 0, len(menus))
		for _, m := range menus {
			perms = append(perms, models.Permission{
				Permission: role.Role + ":" + m.Menu,
				RoleID:     role.ID,
				MenuID:     m.ID,
				CanCreate:  true,
				CanRead:    true,
				CanUpdate:  true,
				CanDelete:  true,
			})
		}
		if len(perms) > 0 {
			if err := db.DB.Create(&perms).Error; err != nil {
				log.Printf("seed: create permissions failed: %v", err)
				return
			}
			log.Printf("seed: created %d permissions for %s", len(perms), role.Role)
		}
	}
}
