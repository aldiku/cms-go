package handlers

import (
	"net/http"
	"strings"

	"cms-go/internal/db"
	"cms-go/internal/models"

	"github.com/labstack/echo/v4"
)

// GET /admin/email-templates
func AdminEmailTemplates(c echo.Context) error {
	var templates []models.EmailTemplate
	db.DB.Order("name asc").Find(&templates)

	data := map[string]interface{}{
		"Templates": templates,
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/email_templates.html", data)
}

// GET /admin/email-templates/new and /admin/email-templates/:id/edit
func AdminEmailTemplateForm(c echo.Context) error {
	var tpl models.EmailTemplate
	if id := c.Param("id"); id != "" {
		if err := db.DB.First(&tpl, id).Error; err != nil {
			return c.String(http.StatusNotFound, "Email template not found")
		}
	}

	params, _ := tpl.Parameters()

	data := map[string]interface{}{
		"Template":   tpl,
		"Parameters": params,
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/email_template_form.html", data)
}

// bindEmailTemplateFromForm reads the shared fields, plus the repeatable
// parameter rows submitted as parallel arrays (param_key[]/
// param_description[] — one row per index, zipped together here). Rows
// with a blank key are dropped, so users can leave trailing empty rows in
// the UI without polluting the stored list.
func bindEmailTemplateFromForm(c echo.Context, tpl *models.EmailTemplate) error {
	tpl.Name = c.FormValue("name")
	tpl.Description = c.FormValue("description")
	tpl.Subject = c.FormValue("subject")
	tpl.Body = c.FormValue("body")

	form, _ := c.FormParams()
	keys := form["param_key[]"]
	descriptions := form["param_description[]"]

	params := make([]models.EmailTemplateParam, 0, len(keys))
	for i, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		desc := ""
		if i < len(descriptions) {
			desc = strings.TrimSpace(descriptions[i])
		}
		params = append(params, models.EmailTemplateParam{Key: key, Description: desc})
	}
	return tpl.SetParameters(params)
}

// POST /admin/email-templates/new
func AdminCreateEmailTemplate(c echo.Context) error {
	var tpl models.EmailTemplate
	if err := bindEmailTemplateFromForm(c, &tpl); err != nil {
		return c.String(http.StatusInternalServerError, "Failed to process parameters")
	}
	if tpl.Name == "" || tpl.Subject == "" {
		return c.String(http.StatusBadRequest, "Name and subject are required")
	}
	if err := db.DB.Create(&tpl).Error; err != nil {
		return c.String(http.StatusBadRequest, "Failed to create email template (name may already exist)")
	}
	return c.Redirect(http.StatusSeeOther, "/admin/email-templates")
}

// POST /admin/email-templates/:id/edit
func AdminUpdateEmailTemplate(c echo.Context) error {
	var tpl models.EmailTemplate
	if err := db.DB.First(&tpl, c.Param("id")).Error; err != nil {
		return c.String(http.StatusNotFound, "Email template not found")
	}
	if err := bindEmailTemplateFromForm(c, &tpl); err != nil {
		return c.String(http.StatusInternalServerError, "Failed to process parameters")
	}
	if err := db.DB.Save(&tpl).Error; err != nil {
		return c.String(http.StatusBadRequest, "Failed to update email template (name may already exist)")
	}
	return c.Redirect(http.StatusSeeOther, "/admin/email-templates")
}

// POST /admin/email-templates/:id/delete
func AdminDeleteEmailTemplate(c echo.Context) error {
	var tpl models.EmailTemplate
	if err := db.DB.First(&tpl, c.Param("id")).Error; err != nil {
		return c.String(http.StatusNotFound, "Email template not found")
	}

	var inUse int64
	db.DB.Model(&models.NotificationHook{}).Where("email_template_id = ?", tpl.ID).Count(&inUse)
	if inUse > 0 {
		return c.String(http.StatusBadRequest, "Cannot delete: still used by one or more notification hooks")
	}

	db.DB.Delete(&tpl)
	return c.Redirect(http.StatusSeeOther, "/admin/email-templates")
}
