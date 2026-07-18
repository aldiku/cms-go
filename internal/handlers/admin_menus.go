package handlers

import (
	"net/http"
	"strconv"

	"cms-go/internal/db"
	"cms-go/internal/models"

	"github.com/labstack/echo/v4"
)

// GET /admin/menus
func AdminMenus(c echo.Context) error {
	var menus []models.Menu
	db.DB.Order("list_order asc, id asc").Find(&menus)

	names := make(map[uint]string, len(menus))
	for _, m := range menus {
		names[m.ID] = m.Menu
	}
	type menuRow struct {
		models.Menu
		ParentName string
	}
	rows := make([]menuRow, 0, len(menus))
	for _, m := range menus {
		rows = append(rows, menuRow{Menu: m, ParentName: names[m.ParentID]})
	}

	data := map[string]interface{}{
		"Menus": rows,
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/menus.html", data)
}

// GET /admin/menus/new and /admin/menus/:id/edit
func AdminMenuForm(c echo.Context) error {
	var menu models.Menu
	if id := c.Param("id"); id != "" {
		if err := db.DB.First(&menu, id).Error; err != nil {
			return c.String(http.StatusNotFound, "Menu not found")
		}
	}

	var parents []models.Menu
	q := db.DB.Order("list_order asc")
	if menu.ID != 0 {
		q = q.Where("id <> ?", menu.ID) // a menu can't be its own parent
	}
	q.Find(&parents)

	data := map[string]interface{}{
		"Menu":    menu,
		"Parents": parents,
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/menu_form.html", data)
}

func bindMenuFromForm(c echo.Context, menu *models.Menu) {
	menu.Menu = c.FormValue("menu")
	menu.Path = c.FormValue("path")
	menu.Icon = c.FormValue("icon")
	menu.MenuDescription = c.FormValue("menu_description")
	menu.MenuType = c.FormValue("menu_type")

	if parentID, err := strconv.ParseUint(c.FormValue("parent_id"), 10, 64); err == nil {
		menu.ParentID = uint(parentID)
	} else {
		menu.ParentID = 0
	}
	if order, err := strconv.ParseUint(c.FormValue("list_order"), 10, 32); err == nil {
		menu.ListOrder = uint32(order)
	}
	if c.FormValue("status") == "on" {
		menu.Status = 1
	} else {
		menu.Status = 0
	}
}

// POST /admin/menus/new
func AdminCreateMenu(c echo.Context) error {
	var menu models.Menu
	bindMenuFromForm(c, &menu)
	if menu.Menu == "" || menu.Path == "" {
		return c.String(http.StatusBadRequest, "Menu name and path are required")
	}
	if err := db.DB.Create(&menu).Error; err != nil {
		return c.String(http.StatusBadRequest, "Failed to create menu")
	}
	return c.Redirect(http.StatusSeeOther, "/admin/menus")
}

// POST /admin/menus/:id/edit
func AdminUpdateMenu(c echo.Context) error {
	var menu models.Menu
	if err := db.DB.First(&menu, c.Param("id")).Error; err != nil {
		return c.String(http.StatusNotFound, "Menu not found")
	}
	bindMenuFromForm(c, &menu)
	if err := db.DB.Save(&menu).Error; err != nil {
		return c.String(http.StatusBadRequest, "Failed to update menu")
	}
	return c.Redirect(http.StatusSeeOther, "/admin/menus")
}

// POST /admin/menus/:id/delete
func AdminDeleteMenu(c echo.Context) error {
	var menu models.Menu
	if err := db.DB.First(&menu, c.Param("id")).Error; err != nil {
		return c.String(http.StatusNotFound, "Menu not found")
	}

	var childCount int64
	db.DB.Model(&models.Menu{}).Where("parent_id = ?", menu.ID).Count(&childCount)
	if childCount > 0 {
		return c.String(http.StatusBadRequest, "Cannot delete a menu that still has children")
	}

	db.DB.Delete(&menu)
	db.DB.Delete(&models.Permission{}, "menu_id = ?", menu.ID)
	return c.Redirect(http.StatusSeeOther, "/admin/menus")
}
