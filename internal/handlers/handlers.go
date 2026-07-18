package handlers

import (
	"html/template"
	"strings"

	"cms-go/internal/auth"
	"cms-go/internal/db"
	"cms-go/internal/models"

	"github.com/labstack/echo/v4"
)

// NavItem is one sidebar entry: a menu the current role can read, with its
// children and whether it matches the current request path.
type NavItem struct {
	Menu     models.Menu
	Active   bool
	Children []NavItem
}

func AdminDashboard(c echo.Context) error {
	var pageCount, layoutCount, componentCount, userCount int64
	db.DB.Model(&models.Page{}).Count(&pageCount)
	db.DB.Model(&models.Layout{}).Count(&layoutCount)
	db.DB.Model(&models.Component{}).Count(&componentCount)
	db.DB.Model(&models.User{}).Count(&userCount)

	data := map[string]interface{}{
		"PageCount":      pageCount,
		"LayoutCount":    layoutCount,
		"ComponentCount": componentCount,
		"UserCount":      userCount,
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/dashboard.html", data)
}

// buildNav turns the role's readable menus into a parent/child tree and marks
// the menu with the longest path-prefix match of the current path as active.
func buildNav(menus []models.Menu, currentPath string) []NavItem {
	activeID := uint(0)
	bestLen := -1
	for _, m := range menus {
		if m.Path == "" {
			continue
		}
		if currentPath == m.Path || strings.HasPrefix(currentPath, m.Path+"/") {
			if len(m.Path) > bestLen {
				activeID = m.ID
				bestLen = len(m.Path)
			}
		}
	}

	childrenOf := map[uint][]models.Menu{}
	for _, m := range menus {
		childrenOf[m.ParentID] = append(childrenOf[m.ParentID], m)
	}

	var build func(parentID uint) []NavItem
	build = func(parentID uint) []NavItem {
		var items []NavItem
		for _, m := range childrenOf[parentID] {
			item := NavItem{
				Menu:     m,
				Active:   m.ID == activeID,
				Children: build(m.ID),
			}
			// A parent with an active child counts as active too, so the
			// section stays highlighted while working inside it.
			for _, child := range item.Children {
				if child.Active {
					item.Active = true
				}
			}
			items = append(items, item)
		}
		return items
	}
	return build(0)
}

// renderWithLayout renders an admin view inside the admin layout, injecting
// the current user and their role's sidebar menus from the auth middleware.
func renderWithLayout(c echo.Context, layoutPath, viewPath string, data map[string]interface{}) error {
	tmpl, err := template.ParseFiles(layoutPath, viewPath)
	if err != nil {
		return err
	}

	if data == nil {
		data = map[string]interface{}{}
	}

	// Wrap HTML fields to prevent escaping
	for k, v := range data {
		if str, ok := v.(string); ok {
			data[k] = template.HTML(str)
		}
	}

	if user, ok := c.Get(auth.CtxUser).(models.User); ok {
		data["CurrentUser"] = user
	}
	if menus, ok := c.Get(auth.CtxNavMenus).([]models.Menu); ok {
		data["NavMenus"] = buildNav(menus, c.Request().URL.Path)
	}

	// Execute the layout template (layout.html should have {{ template "content" . }})
	return tmpl.ExecuteTemplate(c.Response().Writer, "layout", data)
}
