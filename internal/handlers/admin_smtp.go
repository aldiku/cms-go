package handlers

import (
	"net/http"
	"strconv"

	"cms-go/internal/config"
	"cms-go/internal/db"
	"cms-go/internal/models"
	"cms-go/internal/notify"
	"cms-go/internal/utils"

	"github.com/labstack/echo/v4"
)

// GET /admin/smtp
func AdminSMTPConfigs(c echo.Context) error {
	var configs []models.SMTPConfig
	db.DB.Order("name asc").Find(&configs)

	data := map[string]interface{}{
		"Configs": configs,
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/smtp_configs.html", data)
}

// GET /admin/smtp/new and /admin/smtp/:id/edit
func AdminSMTPConfigForm(c echo.Context) error {
	var cfg models.SMTPConfig
	if id := c.Param("id"); id != "" {
		if err := db.DB.First(&cfg, id).Error; err != nil {
			return c.String(http.StatusNotFound, "SMTP config not found")
		}
	}

	data := map[string]interface{}{
		"Config": cfg,
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/smtp_config_form.html", data)
}

func bindSMTPConfigFromForm(c echo.Context, cfg *models.SMTPConfig) error {
	cfg.Name = c.FormValue("name")
	cfg.Host = c.FormValue("host")
	cfg.Username = c.FormValue("username")
	cfg.Encryption = c.FormValue("encryption")
	cfg.FromEmail = c.FormValue("from_email")
	cfg.FromName = c.FormValue("from_name")

	if port, err := strconv.Atoi(c.FormValue("port")); err == nil {
		cfg.Port = port
	}

	// Blank password on edit = keep the current one (same convention as
	// user account passwords — see bindUserFromForm).
	if password := c.FormValue("password"); password != "" {
		encrypted, err := utils.SimpleEncrypt(password, config.AppKey())
		if err != nil {
			return err
		}
		cfg.Password = encrypted
	}
	return nil
}

// POST /admin/smtp/new
func AdminCreateSMTPConfig(c echo.Context) error {
	var cfg models.SMTPConfig
	if err := bindSMTPConfigFromForm(c, &cfg); err != nil {
		return c.String(http.StatusInternalServerError, "Failed to process password")
	}
	if cfg.Name == "" || cfg.Host == "" || cfg.FromEmail == "" {
		return c.String(http.StatusBadRequest, "Name, host and from-email are required")
	}
	if err := db.DB.Create(&cfg).Error; err != nil {
		return c.String(http.StatusBadRequest, "Failed to create SMTP config (name may already exist)")
	}
	return c.Redirect(http.StatusSeeOther, "/admin/smtp")
}

// POST /admin/smtp/:id/edit
func AdminUpdateSMTPConfig(c echo.Context) error {
	var cfg models.SMTPConfig
	if err := db.DB.First(&cfg, c.Param("id")).Error; err != nil {
		return c.String(http.StatusNotFound, "SMTP config not found")
	}
	if err := bindSMTPConfigFromForm(c, &cfg); err != nil {
		return c.String(http.StatusInternalServerError, "Failed to process password")
	}
	if err := db.DB.Save(&cfg).Error; err != nil {
		return c.String(http.StatusBadRequest, "Failed to update SMTP config (name may already exist)")
	}
	return c.Redirect(http.StatusSeeOther, "/admin/smtp")
}

// POST /admin/smtp/:id/delete
func AdminDeleteSMTPConfig(c echo.Context) error {
	var cfg models.SMTPConfig
	if err := db.DB.First(&cfg, c.Param("id")).Error; err != nil {
		return c.String(http.StatusNotFound, "SMTP config not found")
	}

	var inUse int64
	db.DB.Model(&models.NotificationHook{}).Where("smtp_config_id = ?", cfg.ID).Count(&inUse)
	if inUse > 0 {
		return c.String(http.StatusBadRequest, "Cannot delete: still used by one or more notification hooks")
	}

	db.DB.Delete(&cfg)
	return c.Redirect(http.StatusSeeOther, "/admin/smtp")
}

// POST /admin/smtp/:id/test — sends a plain test email through this config
// to prove it's actually reachable/authenticates, independent of any
// template or hook.
func AdminTestSMTPConfig(c echo.Context) error {
	var cfg models.SMTPConfig
	if err := db.DB.First(&cfg, c.Param("id")).Error; err != nil {
		return c.String(http.StatusNotFound, "SMTP config not found")
	}

	to := c.FormValue("to")
	if to == "" {
		return c.String(http.StatusBadRequest, "Recipient address is required")
	}

	password, err := utils.SimpleDecrypt(cfg.Password, config.AppKey())
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to decrypt stored password")
	}

	body := "<p>This is a test email from the <strong>" + cfg.Name + "</strong> SMTP configuration.</p>"
	if err := notify.Send(cfg, password, to, "SMTP test from "+cfg.Name, body); err != nil {
		return c.String(http.StatusBadGateway, "Send failed: "+err.Error())
	}
	return c.String(http.StatusOK, "Test email sent to "+to)
}
