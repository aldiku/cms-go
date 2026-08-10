package auth

import (
	"encoding/json"
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
			{Menu: "WordPress Migration", Path: "/admin/migrate-wordpress", Icon: "📥", MenuType: "module"},
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

	paymentGatewaysMenu := models.Menu{
		Menu: "Payment Gateways", Path: "/admin/payment-gateways", Icon: "💳",
		MenuType: "module", Status: 1, ListOrder: 2, ParentID: settingsMenu.ID, MenuGroupID: backendGroup.ID,
	}
	db.DB.Where("path = ?", paymentGatewaysMenu.Path).Attrs(paymentGatewaysMenu).FirstOrCreate(&paymentGatewaysMenu)

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

	campaignMenu := models.Menu{
		Menu: "Campaign", Path: "/admin/campaign", Icon: "📣",
		MenuType: "module", Status: 1, ListOrder: 21, MenuGroupID: backendGroup.ID,
	}
	db.DB.Where("path = ?", campaignMenu.Path).Attrs(campaignMenu).FirstOrCreate(&campaignMenu)

	workflowsMenu := models.Menu{
		Menu: "Workflow Builder", Path: "/admin/workflows", Icon: "🧵",
		MenuType: "module", Status: 1, ListOrder: 22, MenuGroupID: backendGroup.ID,
		MenuDescription: "Visualizes how a request actually flows through the app end-to-end — trigger, hook, actions, services.",
	}
	db.DB.Where("path = ?", workflowsMenu.Path).Attrs(workflowsMenu).FirstOrCreate(&workflowsMenu)
	seedWorkflows()

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

// cfgJSON marshals a step's ConfigJSON, dropping the error since every call
// site passes a fixed literal map — a marshal failure here would be a typo,
// not a runtime condition.
func cfgJSON(v map[string]string) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// seedWorkflows creates one sample workflow, "Workflow Transaction", that
// documents the real internal/handlers/campaign_transactions.go pipeline —
// trigger, actions, the payment-gateway service call, and the (currently
// unwired) confirmation-email hook — so /admin/workflows has something real
// to show rather than an empty state. Idempotent: only runs if no workflow
// with this slug exists yet, so it never fights hand-edited steps.
func seedWorkflows() {
	var existing models.Workflow
	if err := db.DB.Where("slug = ?", "workflow-transaction").First(&existing).Error; err == nil {
		return
	}

	wf := models.Workflow{
		Name:        "Workflow Transaction",
		Slug:        "workflow-transaction",
		Description: "How a checkout request turns into a real charge with the active payment gateway — from the API call through to the persisted Transaction.",
		Status:      1,
	}
	if err := db.DB.Create(&wf).Error; err != nil {
		log.Printf("seed: create workflow failed: %v", err)
		return
	}

	steps := []models.WorkflowStep{
		{
			WorkflowID: wf.ID, StepOrder: 1, StepType: models.WorkflowStepTrigger,
			Title:       "API Request Received",
			Description: "A campaign-authenticated client bundles one or more draft Orders into a single payable invoice.",
			Icon:        "⚡",
			ConfigJSON: cfgJSON(map[string]string{
				"method":  "POST",
				"path":    "/campaign/api/transactions",
				"auth":    "Campaign API key (query \"key=\" or \"X-API-Key\" header) — see auth.ResolveKeyActor",
				"handler": "CampaignTransactionCreate",
			}),
		},
		{
			WorkflowID: wf.ID, StepOrder: 2, StepType: models.WorkflowStepAction,
			Title:       "Validate & Price Orders",
			Description: "Loads the requested draft Orders for the authenticated user, sums their totals, and applies tax plus a flat admin fee.",
			Icon:        "🧮",
			ConfigJSON: cfgJSON(map[string]string{
				"tax_rate":    "11% (PPN)",
				"admin_fee":   "Rp 4,000 flat",
				"source_file": "internal/handlers/campaign_transactions.go",
			}),
		},
		{
			WorkflowID: wf.ID, StepOrder: 3, StepType: models.WorkflowStepService,
			Title:       "Resolve Active Payment Gateway",
			Description: "Looks up the first active PaymentGateway row. If none is active, falls back to a built-in manual/stub VA generator so checkout still works.",
			Icon:        "🏦",
			ConfigJSON: cfgJSON(map[string]string{
				"function": "resolveActiveGateway()",
				"policy":   "single active gateway, first by id — routing by payment method is a later phase",
				"fallback": "manual (locally-generated stub VA, no HTTP call)",
			}),
		},
		{
			WorkflowID: wf.ID, StepOrder: 4, StepType: models.WorkflowStepAction,
			Title:       "Call Payment Gateway",
			Description: "Renders the gateway's request body template with order_id/amount/bank_code/customer fields and POSTs to Base URL + Request Path with HTTP Basic Auth — no per-provider Go code.",
			Icon:        "🔌",
			ConfigJSON: cfgJSON(map[string]string{
				"package":         "internal/paymentgateway",
				"function":        "CreateCharge",
				"template_engine": "notify.Render (\"{{key}}\" placeholders)",
				"auth":            "HTTP Basic Auth, decrypted API key as username",
			}),
		},
		{
			WorkflowID: wf.ID, StepOrder: 5, StepType: models.WorkflowStepAction,
			Title:       "Log Gateway Call",
			Description: "Writes a PaymentGatewayLog row — request, response, success flag — for every call, success or failure, for auditing and debugging.",
			Icon:        "🧾",
			ConfigJSON: cfgJSON(map[string]string{
				"model":     "PaymentGatewayLog",
				"direction": "outbound",
				"function":  "logGatewayCall()",
			}),
		},
		{
			WorkflowID: wf.ID, StepOrder: 6, StepType: models.WorkflowStepHook,
			Title:       "Notification Hook",
			Description: "No notification hook currently fires after a successful charge. Wire one in the Notification Manager (e.g. a \"Transaction Created\" confirmation email) to add one.",
			Icon:        "🔔",
			ConfigJSON: cfgJSON(map[string]string{
				"registry_key": "not configured",
				"suggested_at": "internal/notify.Registry — add a key such as \"transaction_created\"",
				"see_also":     "/admin/notification-hooks",
			}),
		},
		{
			WorkflowID: wf.ID, StepOrder: 7, StepType: models.WorkflowStepAction,
			Title:       "Persist Transaction & Orders",
			Description: "Inside a single DB transaction: creates the Transaction row with the gateway's real reference/VA/payment URL, links each Order, and moves those Orders to Awaiting Payment.",
			Icon:        "💾",
			ConfigJSON: cfgJSON(map[string]string{
				"models":             "Transaction, TransactionOrder",
				"order_status_after": "Awaiting Payment",
			}),
		},
	}
	if err := db.DB.Create(&steps).Error; err != nil {
		log.Printf("seed: create workflow steps failed: %v", err)
	}
}
