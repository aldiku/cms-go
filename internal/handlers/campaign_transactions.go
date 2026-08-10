package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"cms-go/internal/db"
	"cms-go/internal/models"
	"cms-go/internal/paymentgateway"
	"cms-go/internal/utils"

	"github.com/labstack/echo/v4"
)

// adminFee is a flat processing fee added to every transaction, matching
// the "Fee"/"Biaya Admin" line in the cart/invoice screenshots. Not
// gateway-quoted — gateways don't return a separate admin fee, this is the
// platform's own markup.
const adminFee int64 = 4000

// taxRateBps is PPN 11%, expressed in basis points to avoid float rounding.
const taxRateBps = 1100

type transactionCreateRequest struct {
	OrderIDs      []string `json:"order_ids"`
	PaymentMethod string   `json:"payment_method"`
	BankCode      string   `json:"bank_code"`
}

type transactionOrderLine struct {
	OrderID      string `json:"order_id"`
	CampaignName string `json:"campaign_name"`
	GrandTotal   int64  `json:"grand_total"`
}

type transactionJSON struct {
	ID            string                 `json:"invoice_id,omitempty"`
	Subtotal      int64                  `json:"subtotal"`
	Tax           int64                  `json:"tax"`
	Fee           int64                  `json:"fee"`
	GrandTotal    int64                  `json:"grand_total"`
	Status        string                 `json:"status"`
	PaymentMethod string                 `json:"payment_method"`
	BankCode      string                 `json:"bank_code"`
	VANumber      string                 `json:"va_number"`
	PaymentURL    string                 `json:"payment_url,omitempty"`
	ExpiresAt     *time.Time             `json:"expires_at"`
	CreatedAt     time.Time              `json:"created_at"`
	Sandbox       bool                   `json:"sandbox"`
	Source        string                 `json:"source"`
	Orders        []transactionOrderLine `json:"orders"`
}

func transactionToJSON(txn models.Transaction, lines []transactionOrderLine) transactionJSON {
	return transactionJSON{
		ID: txn.ID, Subtotal: txn.Subtotal, Tax: txn.Tax, Fee: txn.Fee, GrandTotal: txn.GrandTotal,
		Status: txn.Status, PaymentMethod: txn.PaymentMethod, BankCode: txn.BankCode,
		VANumber: txn.VANumber, PaymentURL: txn.PaymentURL, ExpiresAt: txn.ExpiresAt, CreatedAt: txn.CreatedAt,
		Sandbox: txn.Sandbox, Source: txn.Source, Orders: lines,
	}
}

// transactionOrderLines loads the campaign-name/total summary line for each
// Order linked to a Transaction, shared by create + get.
func transactionOrderLines(transactionID string) []transactionOrderLine {
	var links []models.TransactionOrder
	db.DB.Where("transaction_id = ?", transactionID).Find(&links)
	orderIDs := make([]string, 0, len(links))
	for _, l := range links {
		orderIDs = append(orderIDs, l.OrderID)
	}
	var orders []models.Order
	if len(orderIDs) > 0 {
		db.DB.Where("id IN ?", orderIDs).Find(&orders)
	}
	lines := make([]transactionOrderLine, 0, len(orders))
	for _, o := range orders {
		lines = append(lines, transactionOrderLine{OrderID: o.ID, CampaignName: o.CampaignName, GrandTotal: o.GrandTotal})
	}
	return lines
}

// resolveActiveGateway returns the gateway to charge a new Transaction
// through. v1 policy: the single active PaymentGateway, first by ID —
// routing by payment method across multiple active gateways is a later
// phase. No active gateway falls back to an in-memory "manual" one
// (PaymentGatewayID stays 0 on the Transaction), which keeps checkout
// working with the original locally-generated stub VA until a real gateway
// is configured.
func resolveActiveGateway() models.PaymentGateway {
	var gw models.PaymentGateway
	if err := db.DB.Where("status = 1").Order("id asc").First(&gw).Error; err == nil {
		return gw
	}
	return models.PaymentGateway{Provider: "manual"}
}

// logGatewayCall records one call to internal/paymentgateway.CreateCharge —
// success or failure — as the always-on "save payment gateway log" step
// from payment-gateway.md's workflow description. Best-effort: a logging
// failure never blocks or unwinds the checkout itself.
func logGatewayCall(gw models.PaymentGateway, subjectID string, req paymentgateway.ChargeRequest, resp paymentgateway.ChargeResponse, callErr error) {
	reqJSON, _ := json.Marshal(req)
	entry := models.PaymentGatewayLog{
		PaymentGatewayID: gw.ID,
		SubjectType:      "transaction",
		SubjectID:        subjectID,
		Direction:        "outbound",
		RequestJSON:      string(reqJSON),
		ResponseJSON:     resp.Raw,
		Success:          callErr == nil,
	}
	if callErr != nil {
		entry.ErrorMessage = callErr.Error()
	}
	db.DB.Create(&entry)
}

// POST /campaign/api/transactions — bundles one or more draft Orders into a
// single payable invoice (cart-transaction.md's "post api dengan isi
// orderids []"), then runs the payment gateway (internal/paymentgateway) to
// open the actual charge and get a real VA number / payment URL back.
func CampaignTransactionCreate(c echo.Context) error {
	user, sandbox, source := campaignActor(c)

	var req transactionCreateRequest
	if err := c.Bind(&req); err != nil || len(req.OrderIDs) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "order_ids is required"})
	}

	var orders []models.Order
	db.DB.Where("id IN ? AND user_id = ? AND sandbox = ? AND status = ?", req.OrderIDs, user.ID, sandbox, models.OrderStatusDraft).
		Find(&orders)
	if len(orders) != len(req.OrderIDs) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "one or more orders were not found or are not payable"})
	}

	var subtotal int64
	for _, o := range orders {
		subtotal += o.GrandTotal
	}
	tax := subtotal * taxRateBps / 10000
	grandTotal := subtotal + tax + adminFee

	// The transaction ID is generated up front (rather than after the DB
	// write, as before) so the same ID can be handed to the gateway as its
	// order reference — the gateway call happens before any DB write, to
	// avoid holding one open for the duration of an outbound HTTP request.
	txnID := utils.GenerateEntityID("TRX-")

	gw := resolveActiveGateway()
	chargeReq := paymentgateway.ChargeRequest{
		OrderID: txnID, Amount: grandTotal, BankCode: req.BankCode,
		CustomerName: user.FullName(), CustomerEmail: user.Email,
	}
	chargeResp, err := paymentgateway.CreateCharge(gw, chargeReq)
	logGatewayCall(gw, txnID, chargeReq, chargeResp, err)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "failed to open payment with the gateway: " + err.Error()})
	}

	bankCode := chargeResp.BankCode
	if bankCode == "" {
		bankCode = req.BankCode
	}
	expiresAt := chargeResp.ExpiresAt
	if expiresAt == nil {
		fallback := time.Now().Add(24 * time.Hour)
		expiresAt = &fallback
	}

	txn := models.Transaction{
		ID: txnID, UserID: user.ID, Subtotal: subtotal, Tax: tax, Fee: adminFee, GrandTotal: grandTotal,
		Status: "pending", PaymentMethod: req.PaymentMethod, BankCode: bankCode,
		VANumber: chargeResp.VANumber, PaymentGatewayID: gw.ID,
		GatewayReferenceID: chargeResp.GatewayReferenceID, PaymentURL: chargeResp.PaymentURL,
		ExpiresAt: expiresAt, Sandbox: sandbox, Source: source,
	}

	dbTx := db.DB.Begin()
	if err := dbTx.Create(&txn).Error; err != nil {
		dbTx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create transaction"})
	}

	for _, o := range orders {
		if err := dbTx.Create(&models.TransactionOrder{TransactionID: txn.ID, OrderID: o.ID}).Error; err != nil {
			dbTx.Rollback()
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to link orders"})
		}
		if err := dbTx.Model(&models.Order{}).Where("id = ?", o.ID).
			Update("status", models.OrderStatusAwaitingPayment).Error; err != nil {
			dbTx.Rollback()
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update order status"})
		}
	}
	dbTx.Commit()

	return c.JSON(http.StatusCreated, transactionToJSON(txn, transactionOrderLines(txn.ID)))
}

// GET /campaign/api/transactions/:id — feeds the invoice page.
func CampaignTransactionGet(c echo.Context) error {
	user, sandbox, _ := campaignActor(c)

	var txn models.Transaction
	if err := db.DB.Where("id = ? AND user_id = ? AND sandbox = ?", c.Param("id"), user.ID, sandbox).First(&txn).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "transaction not found"})
	}

	return c.JSON(http.StatusOK, transactionToJSON(txn, transactionOrderLines(txn.ID)))
}
