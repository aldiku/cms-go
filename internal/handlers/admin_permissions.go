package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"cms-go/internal/db"
	"cms-go/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm/clause"
)

// PermissionRow is one line of the permissions matrix: a menu plus the
// selected role's current CRUD flags for it.
type PermissionRow struct {
	Menu models.Menu
	Perm models.Permission
}

// GET /admin/permissions?role_id=N — matrix editor: pick a role, get a grid
// of every menu with create/read/update/delete checkboxes.
func AdminPermissions(c echo.Context) error {
	var roles []models.Role
	db.DB.Order("id asc").Find(&roles)

	roleID, _ := strconv.ParseUint(c.QueryParam("role_id"), 10, 64)
	if roleID == 0 && len(roles) > 0 {
		roleID = uint64(roles[0].ID)
	}

	var menus []models.Menu
	db.DB.Order("list_order asc, id asc").Find(&menus)

	var perms []models.Permission
	db.DB.Where("role_id = ?", roleID).Find(&perms)
	permByMenu := make(map[uint]models.Permission, len(perms))
	for _, p := range perms {
		permByMenu[p.MenuID] = p
	}

	rows := make([]PermissionRow, 0, len(menus))
	for _, m := range menus {
		rows = append(rows, PermissionRow{Menu: m, Perm: permByMenu[m.ID]})
	}

	data := map[string]interface{}{
		"Roles":          roles,
		"SelectedRoleID": uint(roleID),
		"Rows":           rows,
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/permissions.html", data)
}

// POST /admin/permissions — upserts one permission row per menu for the
// submitted role. Checkbox names are perm_<menuID>_<flag>.
func AdminSavePermissions(c echo.Context) error {
	roleID, err := strconv.ParseUint(c.FormValue("role_id"), 10, 64)
	if err != nil || roleID == 0 {
		return c.String(http.StatusBadRequest, "Invalid role")
	}

	var role models.Role
	if err := db.DB.First(&role, roleID).Error; err != nil {
		return c.String(http.StatusNotFound, "Role not found")
	}

	var menus []models.Menu
	db.DB.Find(&menus)

	checked := func(menuID uint, flag string) bool {
		return c.FormValue(fmt.Sprintf("perm_%d_%s", menuID, flag)) == "on"
	}

	for _, m := range menus {
		perm := models.Permission{
			Permission: role.Role + ":" + m.Menu,
			RoleID:     role.ID,
			MenuID:     m.ID,
			CanCreate:  checked(m.ID, "create"),
			CanRead:    checked(m.ID, "read"),
			CanUpdate:  checked(m.ID, "update"),
			CanDelete:  checked(m.ID, "delete"),
		}
		db.DB.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "role_id"}, {Name: "menu_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"permission", "create", "read", "update", "delete", "updated_at"}),
		}).Create(&perm)
	}

	return c.Redirect(http.StatusSeeOther, "/admin/permissions?role_id="+strconv.FormatUint(roleID, 10))
}
