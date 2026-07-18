package handlers

import (
	"net/http"

	"cms-go/internal/db"
	"cms-go/internal/models"

	"github.com/labstack/echo/v4"
)

// GET /admin/roles
func AdminRoles(c echo.Context) error {
	var roles []models.Role
	db.DB.Order("id asc").Find(&roles)

	data := map[string]interface{}{
		"Roles": roles,
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/roles.html", data)
}

// GET /admin/roles/new and /admin/roles/:id/edit
func AdminRoleForm(c echo.Context) error {
	var role models.Role
	if id := c.Param("id"); id != "" {
		if err := db.DB.First(&role, id).Error; err != nil {
			return c.String(http.StatusNotFound, "Role not found")
		}
	}

	data := map[string]interface{}{
		"Role": role,
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/role_form.html", data)
}

// POST /admin/roles/new
func AdminCreateRole(c echo.Context) error {
	role := models.Role{Role: c.FormValue("role")}
	if c.FormValue("status") == "on" {
		role.Status = 1
	}
	if role.Role == "" {
		return c.String(http.StatusBadRequest, "Role name is required")
	}
	if err := db.DB.Create(&role).Error; err != nil {
		return c.String(http.StatusBadRequest, "Failed to create role (name may already exist)")
	}
	return c.Redirect(http.StatusSeeOther, "/admin/roles")
}

// POST /admin/roles/:id/edit
func AdminUpdateRole(c echo.Context) error {
	var role models.Role
	if err := db.DB.First(&role, c.Param("id")).Error; err != nil {
		return c.String(http.StatusNotFound, "Role not found")
	}
	role.Role = c.FormValue("role")
	if c.FormValue("status") == "on" {
		role.Status = 1
	} else {
		role.Status = 0
	}
	if err := db.DB.Save(&role).Error; err != nil {
		return c.String(http.StatusBadRequest, "Failed to update role")
	}
	return c.Redirect(http.StatusSeeOther, "/admin/roles")
}

// POST /admin/roles/:id/delete
func AdminDeleteRole(c echo.Context) error {
	var role models.Role
	if err := db.DB.First(&role, c.Param("id")).Error; err != nil {
		return c.String(http.StatusNotFound, "Role not found")
	}

	var userCount int64
	db.DB.Model(&models.User{}).Where("role_id = ?", role.ID).Count(&userCount)
	if userCount > 0 {
		return c.String(http.StatusBadRequest, "Cannot delete a role that still has users")
	}

	db.DB.Delete(&role)
	db.DB.Delete(&models.Permission{}, "role_id = ?", role.ID)
	return c.Redirect(http.StatusSeeOther, "/admin/roles")
}
