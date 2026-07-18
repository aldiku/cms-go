package handlers

import (
	"net/http"
	"strconv"

	"cms-go/internal/auth"
	"cms-go/internal/db"
	"cms-go/internal/models"

	"github.com/labstack/echo/v4"
)

// GET /admin/users
func AdminUsers(c echo.Context) error {
	var users []models.User
	db.DB.Preload("Role").Order("id asc").Find(&users)

	data := map[string]interface{}{
		"Users": users,
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/users.html", data)
}

// GET /admin/users/new and /admin/users/:id/edit
func AdminUserForm(c echo.Context) error {
	var user models.User
	if id := c.Param("id"); id != "" {
		if err := db.DB.First(&user, id).Error; err != nil {
			return c.String(http.StatusNotFound, "User not found")
		}
	}

	var roles []models.Role
	db.DB.Order("id asc").Find(&roles)

	data := map[string]interface{}{
		"User":  user,
		"Roles": roles,
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/user_form.html", data)
}

func bindUserFromForm(c echo.Context, user *models.User) error {
	user.Firstname = c.FormValue("firstname")
	user.Lastname = c.FormValue("lastname")
	user.Email = c.FormValue("email")
	user.Phone = c.FormValue("phone")
	user.Address = c.FormValue("address")
	user.Company = c.FormValue("company")
	user.EmployeeID = c.FormValue("employee_id")
	user.Avatar = c.FormValue("avatar")

	if roleID, err := strconv.ParseUint(c.FormValue("role_id"), 10, 64); err == nil {
		user.RoleID = uint(roleID)
	}
	if c.FormValue("status") == "on" {
		user.Status = 1
	} else {
		user.Status = 0
	}

	// Blank password on edit = keep the current hash.
	if password := c.FormValue("password"); password != "" {
		hash, err := auth.HashPassword(password)
		if err != nil {
			return err
		}
		user.Password = hash
	}
	return nil
}

// POST /admin/users/new
func AdminCreateUser(c echo.Context) error {
	var user models.User
	if err := bindUserFromForm(c, &user); err != nil {
		return c.String(http.StatusInternalServerError, "Failed to process password")
	}
	if user.Email == "" || user.Password == "" {
		return c.String(http.StatusBadRequest, "Email and password are required")
	}
	if err := db.DB.Create(&user).Error; err != nil {
		return c.String(http.StatusBadRequest, "Failed to create user (email may already exist)")
	}
	return c.Redirect(http.StatusSeeOther, "/admin/users")
}

// POST /admin/users/:id/edit
func AdminUpdateUser(c echo.Context) error {
	var user models.User
	if err := db.DB.First(&user, c.Param("id")).Error; err != nil {
		return c.String(http.StatusNotFound, "User not found")
	}
	if err := bindUserFromForm(c, &user); err != nil {
		return c.String(http.StatusInternalServerError, "Failed to process password")
	}
	if err := db.DB.Save(&user).Error; err != nil {
		return c.String(http.StatusBadRequest, "Failed to update user (email may already exist)")
	}
	return c.Redirect(http.StatusSeeOther, "/admin/users")
}

// POST /admin/users/:id/delete
func AdminDeleteUser(c echo.Context) error {
	var user models.User
	if err := db.DB.First(&user, c.Param("id")).Error; err != nil {
		return c.String(http.StatusNotFound, "User not found")
	}

	if current, ok := c.Get(auth.CtxUser).(models.User); ok && current.ID == user.ID {
		return c.String(http.StatusBadRequest, "You cannot delete your own account")
	}

	db.DB.Delete(&user)
	// Their sessions are no longer valid.
	db.DB.Delete(&models.Session{}, "user_id = ?", user.ID)
	return c.Redirect(http.StatusSeeOther, "/admin/users")
}
