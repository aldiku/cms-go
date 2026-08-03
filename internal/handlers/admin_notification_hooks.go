package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"cms-go/internal/db"
	"cms-go/internal/models"
	"cms-go/internal/notify"

	"github.com/labstack/echo/v4"
)

// GET /admin/notification-hooks
func AdminNotificationHooks(c echo.Context) error {
	var hooks []models.NotificationHook
	db.DB.Preload("SMTPConfig").Preload("EmailTemplate").Order("id desc").Find(&hooks)

	hookLabels := make(map[string]string, len(notify.Registry))
	for _, h := range notify.Registry {
		hookLabels[h.Key] = h.Label
	}

	data := map[string]interface{}{
		"Hooks":      hooks,
		"HookLabels": hookLabels,
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/notification_hooks.html", data)
}

// GET /admin/notification-hooks/new and /admin/notification-hooks/:id/edit
func AdminNotificationHookForm(c echo.Context) error {
	var hook models.NotificationHook
	if id := c.Param("id"); id != "" {
		if err := db.DB.First(&hook, id).Error; err != nil {
			return c.String(http.StatusNotFound, "Notification hook not found")
		}
	}

	var smtpConfigs []models.SMTPConfig
	db.DB.Order("name asc").Find(&smtpConfigs)

	var templates []models.EmailTemplate
	db.DB.Order("name asc").Find(&templates)

	// Embedded client-side so the param-mapping UI can rebuild itself
	// entirely in JS (which hook fields exist, which params each template
	// declares) without a round trip on every selection change.
	registryJSON, _ := json.Marshal(notify.Registry)
	templateParams := make(map[uint][]models.EmailTemplateParam, len(templates))
	for _, t := range templates {
		params, _ := t.Parameters()
		templateParams[t.ID] = params
	}
	templateParamsJSON, _ := json.Marshal(templateParams)
	mapping, _ := hook.Mapping()
	mappingJSON, _ := json.Marshal(mapping)

	data := map[string]interface{}{
		"Hook":               hook,
		"SMTPConfigs":        smtpConfigs,
		"Templates":          templates,
		"RegistryJSON":       string(registryJSON),
		"TemplateParamsJSON": string(templateParamsJSON),
		"MappingJSON":        string(mappingJSON),
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/notification_hook_form.html", data)
}

func bindNotificationHookFromForm(c echo.Context, hook *models.NotificationHook) error {
	hook.HookKey = c.FormValue("hook_key")
	hook.Name = c.FormValue("name")
	if c.FormValue("status") == "on" {
		hook.Status = 1
	} else {
		hook.Status = 0
	}
	if smtpID, err := strconv.ParseUint(c.FormValue("smtp_config_id"), 10, 64); err == nil {
		hook.SMTPConfigID = uint(smtpID)
	}
	if templateID, err := strconv.ParseUint(c.FormValue("email_template_id"), 10, 64); err == nil {
		hook.EmailTemplateID = uint(templateID)
	}

	form, _ := c.FormParams()
	mapping := map[string]string{}
	for key, vals := range form {
		if strings.HasPrefix(key, "mapping__") && len(vals) > 0 && vals[0] != "" {
			paramKey := strings.TrimPrefix(key, "mapping__")
			mapping[paramKey] = vals[0]
		}
	}
	return hook.SetMapping(mapping)
}

// POST /admin/notification-hooks/new
func AdminCreateNotificationHook(c echo.Context) error {
	var hook models.NotificationHook
	if err := bindNotificationHookFromForm(c, &hook); err != nil {
		return c.String(http.StatusInternalServerError, "Failed to process param mapping")
	}
	if hook.HookKey == "" || hook.SMTPConfigID == 0 || hook.EmailTemplateID == 0 {
		return c.String(http.StatusBadRequest, "Hook, SMTP config and email template are required")
	}
	if err := db.DB.Create(&hook).Error; err != nil {
		return c.String(http.StatusBadRequest, "Failed to create notification hook")
	}
	return c.Redirect(http.StatusSeeOther, "/admin/notification-hooks")
}

// POST /admin/notification-hooks/:id/edit
func AdminUpdateNotificationHook(c echo.Context) error {
	var hook models.NotificationHook
	if err := db.DB.First(&hook, c.Param("id")).Error; err != nil {
		return c.String(http.StatusNotFound, "Notification hook not found")
	}
	if err := bindNotificationHookFromForm(c, &hook); err != nil {
		return c.String(http.StatusInternalServerError, "Failed to process param mapping")
	}
	if hook.HookKey == "" || hook.SMTPConfigID == 0 || hook.EmailTemplateID == 0 {
		return c.String(http.StatusBadRequest, "Hook, SMTP config and email template are required")
	}
	if err := db.DB.Save(&hook).Error; err != nil {
		return c.String(http.StatusBadRequest, "Failed to update notification hook")
	}
	return c.Redirect(http.StatusSeeOther, "/admin/notification-hooks")
}

// POST /admin/notification-hooks/:id/delete
func AdminDeleteNotificationHook(c echo.Context) error {
	var hook models.NotificationHook
	if err := db.DB.First(&hook, c.Param("id")).Error; err != nil {
		return c.String(http.StatusNotFound, "Notification hook not found")
	}
	db.DB.Delete(&hook)
	return c.Redirect(http.StatusSeeOther, "/admin/notification-hooks")
}

// POST /admin/notification-hooks/:id/test — renders the bound template with
// sample data (each hook field's own description, standing in for a real
// value) and sends it to an admin-supplied recipient, proving the whole
// hook -> mapping -> template -> SMTP pipeline actually works end to end.
func AdminTestNotificationHook(c echo.Context) error {
	var hook models.NotificationHook
	if err := db.DB.Preload("SMTPConfig").Preload("EmailTemplate").First(&hook, c.Param("id")).Error; err != nil {
		return c.String(http.StatusNotFound, "Notification hook not found")
	}

	to := c.FormValue("to")
	if to == "" {
		return c.String(http.StatusBadRequest, "Recipient address is required")
	}

	hookDef, ok := notify.Find(hook.HookKey)
	if !ok {
		return c.String(http.StatusBadRequest, "Unknown hook key: "+hook.HookKey)
	}
	sampleData := make(map[string]string, len(hookDef.Fields))
	for _, f := range hookDef.Fields {
		sampleData[f.Key] = "[sample " + f.Key + "]"
	}

	if err := notify.SendTest(hook, to, sampleData); err != nil {
		return c.String(http.StatusBadGateway, "Send failed: "+err.Error())
	}
	return c.String(http.StatusOK, "Test email sent to "+to)
}
