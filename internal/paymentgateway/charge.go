// Package paymentgateway is the "run the payment gateway" step of
// payment-gateway.md's workflow: given a resolved models.PaymentGateway and
// a provider-agnostic ChargeRequest, CreateCharge opens a real payment with
// that gateway (or, with no gateway configured, falls back to the
// locally-generated stub VA this app always used). Each adapter is written
// side-effect-isolated — pure request in, response/error out, no DB writes
// of its own — so it can be lifted into a workflow-engine action node later
// without changing shape; callers are responsible for persisting the result
// and logging the call (see models.PaymentGatewayLog).
package paymentgateway

import (
	"fmt"
	"time"

	"cms-go/internal/config"
	"cms-go/internal/models"
	"cms-go/internal/utils"
)

// ChargeRequest is what CreateCharge needs to open a payment — provider-
// agnostic; each adapter maps it onto its own API shape.
type ChargeRequest struct {
	OrderID       string // used as the gateway's own order/reference id (Transaction.ID)
	Amount        int64  // Rupiah
	BankCode      string // e.g. "bca", "bni" — bank_transfer/VA methods only
	CustomerName  string
	CustomerEmail string
}

// ChargeResponse is the gateway's answer to a charge request, normalized
// across providers. Raw is the provider's full response body, kept only
// for PaymentGatewayLog — never surfaced to the end user.
type ChargeResponse struct {
	GatewayReferenceID string
	VANumber           string
	BankCode           string
	PaymentURL         string
	ExpiresAt          *time.Time
	Raw                string
}

// CreateCharge opens a charge through gw. Provider "manual" (or unset) uses
// the built-in stub VA (no HTTP call); every other provider goes through
// createGenericCharge, driven entirely by gw's RequestPath/
// RequestBodyTemplate/ResponseFieldMap — there's no per-provider Go code to
// add when onboarding a new gateway.
func CreateCharge(gw models.PaymentGateway, req ChargeRequest) (ChargeResponse, error) {
	if gw.Provider == "manual" || gw.Provider == "" {
		return createManualCharge(gw, req)
	}
	return createGenericCharge(gw, req)
}

// decryptAPIKey decrypts gw.APIKey with the app-wide encryption key — same
// convention as SMTPConfig.Password (see utils.SimpleEncrypt/SimpleDecrypt).
func decryptAPIKey(gw models.PaymentGateway) (string, error) {
	if gw.APIKey == "" {
		return "", fmt.Errorf("payment gateway %q has no API key configured", gw.Name)
	}
	return utils.SimpleDecrypt(gw.APIKey, config.AppKey())
}
