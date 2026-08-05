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

	// The "Backend Menu" group is the system menu location: it drives the
	// admin sidebar and every RBAC permission check. It always exists and
	// every other seeded/legacy menu row belongs to it.
	backendGroup := models.MenuGroup{Name: "Backend Menu", Slug: "backend", IsSystem: true}
	db.DB.Where("slug = ?", backendGroup.Slug).Attrs(backendGroup).FirstOrCreate(&backendGroup)

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

	// "member" is the default role for self-service registrations
	// (POST /auth/register) — it starts with zero granted permissions, so a
	// member who somehow reaches /admin is refused by RequirePermission.
	memberRole := models.Role{Role: "member", Status: 1}
	db.DB.Where("role = ?", memberRole.Role).Attrs(memberRole).FirstOrCreate(&memberRole)

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
			seedMenus[i].MenuGroupID = backendGroup.ID
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

	// The menu-seed block above only runs once, on a fully empty menus
	// table, so it won't fire for installs that predate a given feature.
	// Ensure newer menus exist idempotently, outside that one-time gate.
	apiBuilderMenu := models.Menu{
		Menu: "API Builder", Path: "/admin/api-builder", Icon: "🔌",
		MenuType: "module", Status: 1, ListOrder: 9, MenuGroupID: backendGroup.ID,
	}
	db.DB.Where("path = ?", apiBuilderMenu.Path).Attrs(apiBuilderMenu).FirstOrCreate(&apiBuilderMenu)

	// Granting CanUpdate here grants full arbitrary SQL execution (including
	// DDL) — there's no separate "execute" RBAC verb in this app. Treat
	// granting DB Manager access at all as a superadmin-tier decision.
	dbManagerMenu := models.Menu{
		Menu: "DB Manager", Path: "/admin/db-manager", Icon: "🗄️",
		MenuType: "module", Status: 1, ListOrder: 10, MenuGroupID: backendGroup.ID,
		MenuDescription: "Direct Postgres table browser and SQL console — grants full arbitrary-SQL execution to any role with update access.",
	}
	db.DB.Where("path = ?", dbManagerMenu.Path).Attrs(dbManagerMenu).FirstOrCreate(&dbManagerMenu)

	categoriesMenu := models.Menu{
		Menu: "Categories", Path: "/admin/categories", Icon: "🗂️",
		MenuType: "module", Status: 1, ListOrder: 11, MenuGroupID: backendGroup.ID,
	}
	db.DB.Where("path = ?", categoriesMenu.Path).Attrs(categoriesMenu).FirstOrCreate(&categoriesMenu)

	tagsMenu := models.Menu{
		Menu: "Tags", Path: "/admin/tags", Icon: "🏷️",
		MenuType: "module", Status: 1, ListOrder: 12, MenuGroupID: backendGroup.ID,
	}
	db.DB.Where("path = ?", tagsMenu.Path).Attrs(tagsMenu).FirstOrCreate(&tagsMenu)

	mediasMenu := models.Menu{
		Menu: "Medias", Path: "/admin/medias", Icon: "🖼️",
		MenuType: "module", Status: 1, ListOrder: 13, MenuGroupID: backendGroup.ID,
	}
	db.DB.Where("path = ?", mediasMenu.Path).Attrs(mediasMenu).FirstOrCreate(&mediasMenu)

	// "Settings" is a pure grouping menu (no route of its own, Path "#"),
	// matched by name rather than path since "#" isn't unique — the
	// existing "ACL" group already uses it too.
	settingsMenu := models.Menu{
		Menu: "Settings", Path: "#", Icon: "⚙️",
		MenuType: "module", Status: 1, ListOrder: 14, MenuGroupID: backendGroup.ID,
	}
	db.DB.Where("menu = ?", settingsMenu.Menu).Attrs(settingsMenu).FirstOrCreate(&settingsMenu)

	generalSettingsMenu := models.Menu{
		Menu: "General Settings", Path: "/admin/general-settings", Icon: "🛠️",
		MenuType: "module", Status: 1, ListOrder: 0, ParentID: settingsMenu.ID, MenuGroupID: backendGroup.ID,
	}
	db.DB.Where("path = ?", generalSettingsMenu.Path).Attrs(generalSettingsMenu).FirstOrCreate(&generalSettingsMenu)

	smtpMenu := models.Menu{
		Menu: "Email SMTP", Path: "/admin/smtp", Icon: "📧",
		MenuType: "module", Status: 1, ListOrder: 1, ParentID: settingsMenu.ID, MenuGroupID: backendGroup.ID,
	}
	db.DB.Where("path = ?", smtpMenu.Path).Attrs(smtpMenu).FirstOrCreate(&smtpMenu)

	emailTemplateMenu := models.Menu{
		Menu: "Email Template", Path: "/admin/email-templates", Icon: "✉️",
		MenuType: "module", Status: 1, ListOrder: 15, MenuGroupID: backendGroup.ID,
	}
	db.DB.Where("path = ?", emailTemplateMenu.Path).Attrs(emailTemplateMenu).FirstOrCreate(&emailTemplateMenu)

	notificationManagerMenu := models.Menu{
		Menu: "Notification Manager", Path: "/admin/notification-hooks", Icon: "🔔",
		MenuType: "module", Status: 1, ListOrder: 16, MenuGroupID: backendGroup.ID,
	}
	db.DB.Where("path = ?", notificationManagerMenu.Path).Attrs(notificationManagerMenu).FirstOrCreate(&notificationManagerMenu)

	productCategoriesMenu := models.Menu{
		Menu: "Product Categories", Path: "/admin/product-categories", Icon: "🗃️",
		MenuType: "module", Status: 1, ListOrder: 17, MenuGroupID: backendGroup.ID,
	}
	db.DB.Where("path = ?", productCategoriesMenu.Path).Attrs(productCategoriesMenu).FirstOrCreate(&productCategoriesMenu)

	productsMenu := models.Menu{
		Menu: "Products", Path: "/admin/products", Icon: "📦",
		MenuType: "module", Status: 1, ListOrder: 18, MenuGroupID: backendGroup.ID,
	}
	db.DB.Where("path = ?", productsMenu.Path).Attrs(productsMenu).FirstOrCreate(&productsMenu)

	customPricingMenu := models.Menu{
		Menu: "Custom Pricing", Path: "/admin/custom-pricing", Icon: "💰",
		MenuType: "module", Status: 1, ListOrder: 19, MenuGroupID: backendGroup.ID,
	}
	db.DB.Where("path = ?", customPricingMenu.Path).Attrs(customPricingMenu).FirstOrCreate(&customPricingMenu)

	channelsMenu := models.Menu{
		Menu: "Channels", Path: "/admin/channels", Icon: "📡",
		MenuType: "module", Status: 1, ListOrder: 20, MenuGroupID: backendGroup.ID,
	}
	db.DB.Where("path = ?", channelsMenu.Path).Attrs(channelsMenu).FirstOrCreate(&channelsMenu)

	// Backfill: any menu row without a group (pre-dates MenuGroupID, or was
	// just created above without one) belongs to the system Backend group.
	db.DB.Model(&models.Menu{}).Where("menu_group_id = 0 OR menu_group_id IS NULL").Update("menu_group_id", backendGroup.ID)

	if os.Getenv("API_KEY") == "" {
		log.Println("⚠️  seed: API_KEY is not set — all \"auth\"-tagged API Builder endpoints will reject every request")
	}
	if os.Getenv("APP_KEY") == "" {
		log.Println("⚠️  seed: APP_KEY is not set — SMTP config passwords will be encrypted with a guessable key")
	}
}
