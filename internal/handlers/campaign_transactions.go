package handlers

import (
	"crypto/rand"
	"net/http"
	"time"

	"cms-go/internal/db"
	"cms-go/internal/models"
	"cms-go/internal/utils"

	"github.com/labstack/echo/v4"
)

// adminFee is a flat processing fee added to every transaction, matching
// the "Fee"/"Biaya Admin" line in the cart/invoice screenshots. No payment
// gateway exists yet (note.md §10.11), so this is a fixed placeholder, not
// a gateway-quoted amount.
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
		VANumber: txn.VANumber, ExpiresAt: txn.ExpiresAt, CreatedAt: txn.CreatedAt,
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

func generateVANumber() string {
	digits := "0123456789"
	buf := make([]byte, 16)
	rand.Read(buf)
	for i, b := range buf {
		buf[i] = digits[int(b)%len(digits)]
	}
	return string(buf)
}

// POST /campaign/api/transactions — bundles one or more draft Orders into a
// single payable invoice (cart-transaction.md's "post api dengan isi
// orderids []"). No real payment gateway: creates a "pending" record with a
// stub VA number, mirroring the existing ChannelTopup manual-review shape.
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

	txn := models.Transaction{
		UserID: user.ID, Subtotal: subtotal, Tax: tax, Fee: adminFee, GrandTotal: grandTotal,
		Status: "pending", PaymentMethod: req.PaymentMethod, BankCode: req.BankCode,
		VANumber: generateVANumber(), Sandbox: sandbox, Source: source,
	}
	expires := time.Now().Add(24 * time.Hour)
	txn.ExpiresAt = &expires

	dbTx := db.DB.Begin()
	for attempt := 0; attempt < 5; attempt++ {
		txn.ID = utils.GenerateEntityID("TRX-")
		if err := dbTx.Create(&txn).Error; err == nil {
			break
		}
	}
	if txn.ID == "" {
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
