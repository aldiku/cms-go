package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"cms-go/internal/db"
	"cms-go/internal/models"

	"github.com/labstack/echo/v4"
)

// GET /admin/transactions
func AdminTransactionsList(c echo.Context) error {
	pageStr := c.QueryParam("page")
	if pageStr == "" {
		pageStr = "1"
	}
	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}

	// Filters
	userFilter := c.QueryParam("user")
	statusFilter := c.QueryParam("status")
	dateFromStr := c.QueryParam("date_from")
	dateToStr := c.QueryParam("date_to")

	pageSize := 20
	offset := (page - 1) * pageSize

	query := db.DB.Model(&models.Transaction{})

	// Apply filters
	if userFilter != "" {
		query = query.Where("user_id = ?", userFilter)
	}
	if statusFilter != "" {
		query = query.Where("status = ?", statusFilter)
	}
	if dateFromStr != "" {
		if t, err := time.Parse("2006-01-02", dateFromStr); err == nil {
			query = query.Where("created_at >= ?", t)
		}
	}
	if dateToStr != "" {
		if t, err := time.Parse("2006-01-02", dateToStr); err == nil {
			query = query.Where("created_at <= ?", t.Add(24*time.Hour))
		}
	}

	// Get total count
	var total int64
	query.Count(&total)

	// Get paginated results
	var transactions []models.Transaction
	query.Preload("User").
		Order("created_at desc").
		Offset(offset).
		Limit(pageSize).
		Find(&transactions)

	// Calculate stats
	var stats struct {
		TotalTransactions int64
		TotalRevenue      string // formatted string for template
		PendingCount      int64
		PaidCount         int64
	}
	var totalRevenue int64
	db.DB.Model(&models.Transaction{}).Count(&stats.TotalTransactions)
	db.DB.Model(&models.Transaction{}).Select("COALESCE(SUM(grand_total), 0)").Row().Scan(&totalRevenue)
	stats.TotalRevenue = fmt.Sprintf("Rp %.0f", float64(totalRevenue)) // format in handler
	db.DB.Model(&models.Transaction{}).Where("status = ?", "pending").Count(&stats.PendingCount)
	db.DB.Model(&models.Transaction{}).Where("status = ?", "paid").Count(&stats.PaidCount)

	// Get unique statuses for filter dropdown
	var statuses []string
	db.DB.Model(&models.Transaction{}).Distinct("status").Order("status asc").Pluck("status", &statuses)

	totalPages := (int(total) + pageSize - 1) / pageSize

	data := map[string]interface{}{
		"Transactions": transactions,
		"Stats":        stats,
		"Page":         page,
		"PageSize":     pageSize,
		"TotalPages":   totalPages,
		"Total":        total,
		"Statuses":     statuses,
		"Filters": map[string]string{
			"user":      userFilter,
			"status":    statusFilter,
			"date_from": dateFromStr,
			"date_to":   dateToStr,
		},
	}

	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/transactions_list.html", data)
}

// GET /admin/transactions/:id
func AdminTransactionDetail(c echo.Context) error {
	var txn models.Transaction
	if err := db.DB.Preload("User").First(&txn, c.Param("id")).Error; err != nil {
		return c.String(http.StatusNotFound, "Transaction not found")
	}

	// Get linked orders
	var links []models.TransactionOrder
	db.DB.Where("transaction_id = ?", txn.ID).Find(&links)
	orderIDs := make([]string, 0, len(links))
	for _, l := range links {
		orderIDs = append(orderIDs, l.OrderID)
	}

	var orders []models.Order
	if len(orderIDs) > 0 {
		db.DB.Where("id IN ?", orderIDs).Preload("User").Find(&orders)
	}

	// Get payment gateway logs
	var logs []models.PaymentGatewayLog
	db.DB.Where("subject_type = ? AND subject_id = ?", "transaction", txn.ID).
		Order("created_at desc").
		Find(&logs)

	data := map[string]interface{}{
		"Transaction": txn,
		"Orders":      orders,
		"Logs":        logs,
	}

	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/transaction_detail.html", data)
}

// POST /admin/transactions/:id/edit
func AdminUpdateTransaction(c echo.Context) error {
	var txn models.Transaction
	if err := db.DB.First(&txn, c.Param("id")).Error; err != nil {
		return c.String(http.StatusNotFound, "Transaction not found")
	}

	// Only allow status updates
	if newStatus := c.FormValue("status"); newStatus != "" {
		txn.Status = newStatus
	}

	if err := db.DB.Save(&txn).Error; err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to update transaction: %v", err))
	}

	return c.Redirect(http.StatusSeeOther, "/admin/transactions/"+txn.ID)
}

// GET /admin/transactions/import
func AdminTransactionImportForm(c echo.Context) error {
	data := map[string]interface{}{}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/transaction_import.html", data)
}

// POST /admin/transactions/import
func AdminTransactionImportProcess(c echo.Context) error {
	// TODO: Implement CSV/Excel import
	return c.String(http.StatusNotImplemented, "Import functionality coming soon")
}

// GET /admin/transactions/custom/new
func AdminTransactionCustomForm(c echo.Context) error {
	var users []models.User
	db.DB.Where("status = ?", 1).Order("firstname asc").Find(&users)

	data := map[string]interface{}{
		"Users": users,
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/transaction_custom.html", data)
}

// POST /admin/transactions/custom/new
func AdminTransactionCustomCreate(c echo.Context) error {
	// TODO: Implement custom transaction creation
	return c.String(http.StatusNotImplemented, "Custom transaction creation coming soon")
}
