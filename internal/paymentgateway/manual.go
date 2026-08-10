package paymentgateway

import (
	"crypto/rand"
	"encoding/json"

	"cms-go/internal/models"
)

// createManualCharge preserves the app's original behavior — a locally
// generated fake VA number, no real gateway call — used when the resolved
// gateway's Provider is "manual", or when no active PaymentGateway is
// configured at all. This keeps checkout working unchanged on installs that
// haven't set up a real gateway yet.
func createManualCharge(gw models.PaymentGateway, req ChargeRequest) (ChargeResponse, error) {
	va := generateVANumber()
	bank := req.BankCode
	if bank == "" {
		bank = "manual"
	}
	raw, _ := json.Marshal(map[string]string{"va_number": va, "bank": bank, "mode": "manual"})
	return ChargeResponse{
		GatewayReferenceID: req.OrderID,
		VANumber:           va,
		BankCode:           bank,
		Raw:                string(raw),
	}, nil
}

func generateVANumber() string {
	digits := "0123456789"
	buf := make([]byte, 16)
	rand.Read(buf)
	for i, b := range buf {
		buf[i] = digits[int(b)%len(digits)]
	}
	return string(buf)
}
