package models

import (
	"time"

	"gorm.io/gorm"
)

// Order statuses (cart-transaction.md "statuses" section). Draft is the
// default — an Order starts as an autosaved draft ("cart" is just the list
// of a user's own Orders still in OrderStatusDraft) and only moves forward
// once a Transaction bundles it for payment.
const (
	OrderStatusDraft = iota
	OrderStatusPendingModeration
	OrderStatusInModeration
	OrderStatusWaitingSchedule
	OrderStatusRejected
	OrderStatusLive
	OrderStatusFinished
	OrderStatusToPayment
	OrderStatusAwaitingPayment
)

// Order is one campaign booking — draftable (autosaved while status is
// OrderStatusDraft), created via the /campaign/api/orders JSON endpoint.
// ID is generated as YYMMDD + 6 random uppercase alnum chars (see
// internal/utils.GenerateEntityID), no prefix, e.g. "260805Y4GK6W".
type Order struct {
	ID                  string `gorm:"primaryKey;size:20"`
	CampaignName        string
	CampaignDescription string
	ProductID           uint // leaf Product node selected for this campaign
	ProductVariantID    uint
	ScheduleStart       *time.Time
	ScheduleEnd         *time.Time
	Taxable             bool  // true only when the selected Product.IsCampaignable
	Status              uint8 // OrderStatus* constants above
	Qty                 int64 // sum of OrderDetail.Qty
	GrandTotal          int64
	OriginalCost        int64 // admin-only, from Product/Variant HPP
	ResellerCost        int64 // admin+reseller only, from reseller-tier HPP
	UserID              uint  `gorm:"index"`
	Sandbox             bool
	Source              string // "web" | "iframe" | "api"
	CreatedAt           time.Time
	UpdatedAt           time.Time
	DeletedAt           gorm.DeletedAt `gorm:"index"`
}

// OrderDetail is one targeting+creative pairing within a (multi-target,
// multi-creative) Order, e.g. "20000 sms on Audience A with Creative X".
type OrderDetail struct {
	ID         uint   `gorm:"primaryKey"`
	OrderID    string `gorm:"index;size:20"`
	AudienceID string `gorm:"size:20"`
	CreativeID string `gorm:"size:20"`
	Qty        int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Audience is a reusable targeting definition, created standalone or inline
// from the campaign composer's "New" tab. Not every field is populated for
// every product type — see cart-transaction.md §Audience. ID format:
// "AUD-" + YYMMDD + 6 random uppercase alnum chars.
type Audience struct {
	ID              string `gorm:"primaryKey;size:24"`
	Name            string
	ProductID       uint   // leaf Product this audience was defined for
	ProductType     string // snapshot of Product.Code at creation time
	UserID          uint   `gorm:"index"`
	LocationAddress string
	MinAge          int
	MaxAge          int
	Gender          string // comma-separated
	Interests       string // comma-separated
	Latitude        float64
	Longitude       float64
	Radius          int
	ProvID          uint
	KabID           uint
	KecID           uint
	KelID           uint
	WhitelistPhones string // JSON array of strings
	FileURL         string // CSV/XLSX of phones/emails
	Sandbox         bool
	Source          string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       gorm.DeletedAt `gorm:"index"`
}

// Creative is a reusable ad asset, created standalone or inline from the
// campaign composer's "New" tab. Superset of fields across product types —
// see cart-transaction.md §Creative. ID format: "CRE-" + YYMMDD + 6 random
// uppercase alnum chars.
type Creative struct {
	ID          string `gorm:"primaryKey;size:24"`
	Name        string
	ProductID   uint
	ProductType string
	UserID      uint `gorm:"index"`
	Title       string
	Caption     string
	Body        string
	FooterType  string // "none" | "cta"
	CTAProps    string // raw JSON, e.g. {"type":"url","title":"Buy now","url":"aaa.com"}
	MediaID     uint
	Media       Media `gorm:"foreignKey:MediaID"`
	FileURL     string
	Sandbox     bool
	Source      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

// Transaction bundles one or more Orders into a single payable invoice. No
// payment gateway exists yet (note.md §10.11) — it's created "pending" with
// a stub virtual account number, same manually-reviewable shape as
// ChannelTopup. ID format: "TRX-" + YYMMDD + 6 random uppercase alnum chars.
type Transaction struct {
	ID            string `gorm:"primaryKey;size:24"`
	UserID        uint   `gorm:"index"`
	Subtotal      int64
	Tax           int64 // PPN 11%
	Fee           int64
	GrandTotal    int64
	Status        string // "pending" | "paid" | "expired" | "cancelled"
	PaymentMethod string // e.g. "bank_transfer:bca", "token_adsqoo"
	BankCode      string
	VANumber      string
	ExpiresAt     *time.Time
	Sandbox       bool
	Source        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     gorm.DeletedAt `gorm:"index"`
}

// TransactionOrder is the join row backing a Transaction's order_ids[]
// payload — one Transaction can pay for many Orders at once.
type TransactionOrder struct {
	ID            uint   `gorm:"primaryKey"`
	TransactionID string `gorm:"index;size:24"`
	OrderID       string `gorm:"index;size:20"`
}
