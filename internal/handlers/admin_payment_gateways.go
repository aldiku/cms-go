package handlers

import (
	"net/http"

	"cms-go/internal/config"
	"cms-go/internal/db"
	"cms-go/internal/models"
	"cms-go/internal/utils"

	"github.com/labstack/echo/v4"
)

// GET /admin/payment-gateways
func AdminPaymentGateways(c echo.Context) error {
	var gateways []models.PaymentGateway
	db.DB.Order("name asc").Find(&gateways)

	data := map[string]interface{}{
		"Gateways": gateways,
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/payment_gateways.html", data)
}

// GET /admin/payment-gateways/new and /admin/payment-gateways/:id/edit
func AdminPaymentGatewayForm(c echo.Context) error {
	var gw models.PaymentGateway
	if id := c.Param("id"); id != "" {
		if err := db.DB.First(&gw, id).Error; err != nil {
			return c.String(http.StatusNotFound, "Payment gateway not found")
		}
	}

	data := map[string]interface{}{
		"Gateway": gw,
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/payment_gateway_form.html", data)
}

func bindPaymentGatewayFromForm(c echo.Context, gw *models.PaymentGateway) error {
	gw.Name = c.FormValue("name")
	gw.Provider = c.FormValue("provider")
	gw.BaseURL = c.FormValue("base_url")
	gw.RequestPath = c.FormValue("request_path")
	gw.RequestBodyTemplate = c.FormValue("request_body_template")
	gw.ResponseFieldMap = c.FormValue("response_field_map")
	gw.Notes = c.FormValue("notes")
	gw.Sandbox = c.FormValue("sandbox") == "on"
	if c.FormValue("status") == "on" {
		gw.Status = 1
	} else {
		gw.Status = 0
	}

	// Blank secret fields on edit = keep the current value (same convention
	// as SMTPConfig.Password — see bindSMTPConfigFromForm).
	if apiKey := c.FormValue("api_key"); apiKey != "" {
		encrypted, err := utils.SimpleEncrypt(apiKey, config.AppKey())
		if err != nil {
			return err
		}
		gw.APIKey = encrypted
	}
	if token := c.FormValue("callback_token"); token != "" {
		encrypted, err := utils.SimpleEncrypt(token, config.AppKey())
		if err != nil {
			return err
		}
		gw.CallbackToken = encrypted
	}
	return nil
}

// POST /admin/payment-gateways/new
func AdminCreatePaymentGateway(c echo.Context) error {
	var gw models.PaymentGateway
	if err := bindPaymentGatewayFromForm(c, &gw); err != nil {
		return c.String(http.StatusInternalServerError, "Failed to process credentials")
	}
	if gw.Name == "" || gw.Provider == "" {
		return c.String(http.StatusBadRequest, "Name and provider are required")
	}
	if err := db.DB.Create(&gw).Error; err != nil {
		return c.String(http.StatusBadRequest, "Failed to create payment gateway (name may already exist)")
	}
	return c.Redirect(http.StatusSeeOther, "/admin/payment-gateways")
}

// POST /admin/payment-gateways/:id/edit
func AdminUpdatePaymentGateway(c echo.Context) error {
	var gw models.PaymentGateway
	if err := db.DB.First(&gw, c.Param("id")).Error; err != nil {
		return c.String(http.StatusNotFound, "Payment gateway not found")
	}
	if err := bindPaymentGatewayFromForm(c, &gw); err != nil {
		return c.String(http.StatusInternalServerError, "Failed to process credentials")
	}
	if gw.Name == "" || gw.Provider == "" {
		return c.String(http.StatusBadRequest, "Name and provider are required")
	}
	if err := db.DB.Save(&gw).Error; err != nil {
		return c.String(http.StatusBadRequest, "Failed to update payment gateway (name may already exist)")
	}
	return c.Redirect(http.StatusSeeOther, "/admin/payment-gateways")
}

// POST /admin/payment-gateways/:id/delete
func AdminDeletePaymentGateway(c echo.Context) error {
	var gw models.PaymentGateway
	if err := db.DB.First(&gw, c.Param("id")).Error; err != nil {
		return c.String(http.StatusNotFound, "Payment gateway not found")
	}
	db.DB.Delete(&gw)
	return c.Redirect(http.StatusSeeOther, "/admin/payment-gateways")
}
