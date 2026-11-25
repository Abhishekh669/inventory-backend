package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type PurchaseStatus string
type PaymentStatus string

const (
	PaymentUnpaid  PaymentStatus = "unpaid"
	PaymentPartial PaymentStatus = "partial"
	PaymentPaid    PaymentStatus = "paid"
)

const (
	PurchaseDraft     PurchaseStatus = "draft"
	PurchasePosted    PurchaseStatus = "posted"
	PurchaseCancelled PurchaseStatus = "cancelled"
)

// purchase
type Purchase struct {
	ID              uuid.UUID  `json:"id" db:"id"`
	SupplierID      uuid.UUID  `json:"supplier_id" db:"supplier_id"`
	PurchaseOrderID *uuid.UUID `json:"purchase_order_id,omitempty" db:"purchase_order_id"`
	SupplierBillNo  string     `json:"supplier_bill_no" db:"supplier_bill_no"`
	PurchaseDate    time.Time  `json:"purchase_date" db:"purchase_date"`

	ExciseDuty      decimal.Decimal `json:"excise_duty" db:"excise_duty"`
	SpecialDiscount decimal.Decimal `json:"special_discount" db:"special_discount"`
	Discount        decimal.Decimal `json:"discount" db:"discount"`
	VATRate         decimal.Decimal `json:"vat_rate" db:"vat_rate"`
	FreightCharge   decimal.Decimal `json:"freight_charge" db:"freight_charge"`

	GrossAmount   decimal.Decimal `json:"gross_amount" db:"gross_amount"`
	TaxableAmount decimal.Decimal `json:"taxable_amount" db:"taxable_amount"`
	VATAmount     decimal.Decimal `json:"vat_amount" db:"vat_amount"`
	Total         decimal.Decimal `json:"total" db:"total"`

	PaidAmount     decimal.Decimal `json:"paid_amount" db:"paid_amount"`
	ReturnedAmount decimal.Decimal `json:"returned_amount" db:"returned_amount"`
	DueAmount      decimal.Decimal `json:"due_amount" db:"due_amount"`

	PaymentStatus PaymentStatus `json:"payment_status" db:"payment_status"`
	DueDate       *time.Time    `json:"due_date,omitempty" db:"due_date"`

	Status PurchaseStatus `json:"status" db:"status"`
	Note   *string        `json:"note,omitempty" db:"note"`

	CreatedBy *uuid.UUID `json:"created_by,omitempty" db:"created_by"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
}

type PurchaseItem struct {
	ID         uuid.UUID  `json:"id" db:"id"`
	PurchaseID uuid.UUID  `json:"purchase_id" db:"purchase_id"`
	ProductID  uuid.UUID  `json:"product_id" db:"product_id"`
	POIID      *uuid.UUID `json:"poi_id,omitempty" db:"poi_id"`

	Qty  decimal.Decimal `json:"qty" db:"qty"`
	Rate decimal.Decimal `json:"rate" db:"rate"`

	VATAmount decimal.Decimal `json:"vat_amount" db:"vat_amount"`

	UnitCost        decimal.Decimal `json:"unit_cost" db:"unit_cost"`
	UnitCostWithVAT decimal.Decimal `json:"unit_cost_with_vat" db:"unit_cost_with_vat"`
}

type PurchaseReturn struct {
	ID         uuid.UUID `json:"id" db:"id"`
	ReturnNo   string    `json:"return_no" db:"return_no"`
	PurchaseID uuid.UUID `json:"purchase_id" db:"purchase_id"`
	SupplierID uuid.UUID `json:"supplier_id" db:"supplier_id"`

	ReturnDate time.Time       `json:"return_date" db:"return_date"`
	Total      decimal.Decimal `json:"total" db:"total"`

	Note *string `json:"note,omitempty" db:"note"`

	CreatedBy *uuid.UUID `json:"created_by,omitempty" db:"created_by"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
}

type PurchaseReturnItem struct {
	ID               uuid.UUID `json:"id" db:"id"`
	PurchaseReturnID uuid.UUID `json:"purchase_return_id" db:"purchase_return_id"`
	PurchaseItemID   uuid.UUID `json:"purchase_item_id" db:"purchase_item_id"`
	ProductID        uuid.UUID `json:"product_id" db:"product_id"`

	Qty    decimal.Decimal `json:"qty" db:"qty"`
	Amount decimal.Decimal `json:"amount" db:"amount"`

	Reason *string `json:"reason,omitempty" db:"reason"`
}

// PurchaseOrderData is always present on a row.
type PurchaseOrderData struct {
	PurchaseOrderID     uuid.UUID           `json:"purchase_order_id"`
	PurchaseOrderNumber string              `json:"purchase_order_number"`
	SupplierID          uuid.UUID           `json:"supplier_id"`
	SupplierName        string              `json:"supplier_name"`
	PurchaseOrderByName *string             `json:"purchase_order_by_name"`
	PurchaseOrderedAt   time.Time           `json:"purchase_ordered_at"`
	PurchaseOrderStatus PurchaseOrderStatus `json:"purchase_order_status"`
	PurchaseOrderTitle  *string             `json:"purchase_order_title"`
}

// PurchaseData is optional: nil when the PO has no purchase yet.
type PurchaseData struct {
	PurchaseID        uuid.UUID       `json:"purchase_id"`
	SupplierBillNo    string          `json:"supplier_bill_no"`
	PurchaseDate      time.Time       `json:"purchase_date"`
	Total             decimal.Decimal `json:"total"`
	PaidAmount        decimal.Decimal `json:"paid_amount"`
	ReturnedAmount    decimal.Decimal `json:"returned_amount"`
	DueAmount         decimal.Decimal `json:"due_amount"`
	PurchaseStatus    PurchaseStatus  `json:"purchase_status"`
	PaymentStatus     PaymentStatus   `json:"payment_status"`
	PurchaseCreatedAt time.Time       `json:"purchase_created_at"`
	PurchaseNote      *string         `json:"purchase_note"`
}

// One row of the list.
type PurchaseListItem struct {
	PurchaseOrderData PurchaseOrderData `json:"purchase_order_data"` // always set
	PurchaseData      *PurchaseData     `json:"purchase_data"`       // null if no purchase
}

// Raw query from the handler (unvalidated).
type PurchaseListQuery struct {
	SupplierID          *uuid.UUID // optional
	PurchaseOrderStatus string
	PurchaseOrderNumber string
	PurchaseStatus      string
	PaymentStatus       string
	Search              string
	FromDate            string
	ToDate              string
	Cursor              string
	Limit               int
}

// Validated filter passed to the repository.
type PurchaseListFilter struct {
	PurchaseOrderStatus *PurchaseOrderStatus
	PurchaseOrderNumber *string
	PurchaseStatus      *PurchaseStatus
	PaymentStatus       *PaymentStatus
	Search              *string    // trimmed and validated
	FromDate            *time.Time // inclusive
	ToDate              *time.Time // exclusive (service adds +1 day)
	Cursor              *PurchaseListCursor
	Limit               int
}

type PurchaseListCursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        uuid.UUID `json:"id"`
}

type GetAllPurchaseResponse struct {
	Items      []PurchaseListItem `json:"items"`
	Total      int                `json:"total"`
	NextCursor *string            `json:"next_cursor"`
	HasMore    bool               `json:"has_more"`
}

type CreatePurchaseItemRequest struct {
	PurchaseOrderItemID *uuid.UUID      `json:"purchase_order_item_id"` // nil = product added to the purchase
	ProductID           uuid.UUID       `json:"product_id"`
	Qty                 decimal.Decimal `json:"qty"`
	Rate                decimal.Decimal `json:"rate"`
}

type CreatePurchaseRequest struct {
	PurchaseOrderID       uuid.UUID                   `json:"purchase_order_id"`
	SupplierBillNo        string                      `json:"supplier_bill_no"`
	PurchaseDate          string                      `json:"purchase_date"`
	DueDate               *string                     `json:"due_date"`
	ExciseDuty            decimal.Decimal             `json:"excise_duty"`
	SpecialDiscount       decimal.Decimal             `json:"special_discount"`
	Discount              decimal.Decimal             `json:"discount"`
	VatRate               decimal.Decimal             `json:"vat_rate"`
	FreightCharge         decimal.Decimal             `json:"freight_charge"`
	PaidAmount            decimal.Decimal             `json:"paid_amount"`
	PaymentMethod         PaymentMethod               `json:"payment_method"`
	SupplierBankAccountID *uuid.UUID                  `json:"supplier_bank_account_id"`
	Note                  *string                     `json:"note"`
	Items                 []CreatePurchaseItemRequest `json:"items"`
}

// ---- computed by the service, persisted by the repository ----

type PurchaseItemCalc struct {
	CreatePurchaseItemRequest
	VatAmount       decimal.Decimal // qty x rate x VAT%
	UnitCost        decimal.Decimal // the purchase rate
	UnitCostWithVat decimal.Decimal // rate x (1 + VAT%)
}

type PurchaseCalc struct {
	Req           CreatePurchaseRequest
	Items         []PurchaseItemCalc
	GrossAmount   decimal.Decimal
	TaxableAmount decimal.Decimal
	VatAmount     decimal.Decimal
	Total         decimal.Decimal
	PaymentStatus PaymentStatus
}

type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }

func NewValidationError(msg string) error { return &ValidationError{Message: msg} }

var ErrNotFound = errors.New("not found")

type PurchaseSupplier struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	ContactPerson *string   `json:"contact_person"`
	Phone         *string   `json:"phone"`
	Address       *string   `json:"address"`
	PanNo         *string   `json:"pan_no"`
	IsActive      bool      `json:"is_active"`
}

// Purchase fields are flattened into the JSON, plus the creator's name.
type PurchaseDetail struct {
	Purchase
	CreatedByName *string `json:"created_by_name"`
}

type PurchaseItemDetail struct {
	ID               uuid.UUID        `json:"id"`
	ProductID        uuid.UUID        `json:"product_id"`
	ProductName      string           `json:"product_name"`
	ProductSKU       string           `json:"product_sku"`
	UnitName         string           `json:"unit_name"`
	AllowDecimal     bool             `json:"allow_decimal"`
	POIID            *uuid.UUID       `json:"poi_id"`
	Qty              decimal.Decimal  `json:"qty"`
	Rate             decimal.Decimal  `json:"rate"`
	VATAmount        decimal.Decimal  `json:"vat_amount"`
	UnitCost         decimal.Decimal  `json:"unit_cost"`
	UnitCostWithVAT  decimal.Decimal  `json:"unit_cost_with_vat"`
	LineValue        decimal.Decimal  `json:"line_value"`         // qty x rate
	QtyOrdered       *decimal.Decimal `json:"qty_ordered"`        // from the PO line, nil for added products
	ExpectedUnitCost *decimal.Decimal `json:"expected_unit_cost"` // from the PO line

	ReturnedQty decimal.Decimal `json:"returned_qty"`
}

type PurchaseOrderInfo struct {
	ID            uuid.UUID           `json:"id"`
	PONo          string              `json:"po_no"`
	Title         *string             `json:"title"`
	OrderDate     string              `json:"order_date"`    // YYYY-MM-DD
	ExpectedDate  *string             `json:"expected_date"` // YYYY-MM-DD
	Status        PurchaseOrderStatus `json:"status"`
	Note          *string             `json:"note"`
	CreatedByName *string             `json:"created_by_name"`
	CreatedAt     time.Time           `json:"created_at"`
}

type PurchasePayment struct {
	ID                    uuid.UUID       `json:"id"`
	Amount                decimal.Decimal `json:"amount"` // the part allocated to this purchase
	PaymentDate           string          `json:"payment_date"`
	PaymentMethod         string          `json:"payment_method"`
	PaymentType           string          `json:"payment_type"`
	Note                  *string         `json:"note"`
	CreatedByName         *string         `json:"created_by_name"`
	CreatedAt             time.Time       `json:"created_at"`
	SupplierBankAccountID *uuid.UUID      `json:"supplier_bank_account_id"`
	BankName              *string         `json:"bank_name"`
	AccountName           *string         `json:"account_name"` // NEW
	AccountNumber         *string         `json:"account_number"`
}
type PurchaseByIdResponse struct {
	SupplierData       PurchaseSupplier          `json:"supplier_data"`
	PurchaseData       PurchaseDetail            `json:"purchase_data"`
	PurchaseItems      []PurchaseItemDetail      `json:"purchase_items"`
	PurchaseOrderData  *PurchaseOrderInfo        `json:"purchase_order_data"`
	PurchaseOrderItems []PurchaseOrderItemDetail `json:"purchase_order_items"`
	Payments           []PurchasePayment         `json:"payments"`

	Returns []PurchaseReturnDetail `json:"returns"`
}

type UpdatePurchaseRequest struct {
	SupplierBillNo  string                      `json:"supplier_bill_no"`
	PurchaseDate    string                      `json:"purchase_date"`
	DueDate         *string                     `json:"due_date"`
	ExciseDuty      decimal.Decimal             `json:"excise_duty"`
	SpecialDiscount decimal.Decimal             `json:"special_discount"`
	Discount        decimal.Decimal             `json:"discount"`
	VatRate         decimal.Decimal             `json:"vat_rate"`
	FreightCharge   decimal.Decimal             `json:"freight_charge"`
	Note            *string                     `json:"note"`
	Items           []CreatePurchaseItemRequest `json:"items"` // matched to existing lines by product_id
}

type CreatePurchaseReturnItemRequest struct {
	PurchaseItemID uuid.UUID       `json:"purchase_item_id"`
	Qty            decimal.Decimal `json:"qty"`
	Amount         decimal.Decimal `json:"amount"`
	Reason         *string         `json:"reason"`
}

type CreatePurchaseReturnRequest struct {
	ReturnDate string                            `json:"return_date"`
	Note       *string                           `json:"note"`
	Items      []CreatePurchaseReturnItemRequest `json:"items"`
}

type PurchaseReturnCalc struct {
	Req   CreatePurchaseReturnRequest
	Total decimal.Decimal
}

type CreatePurchasePaymentRequest struct {
	Amount                decimal.Decimal `json:"amount"`
	PaymentDate           string          `json:"payment_date"`
	Source                PaymentType     `json:"source"`
	PaymentMethod         PaymentMethod   `json:"payment_method"`
	SupplierBankAccountID *uuid.UUID      `json:"supplier_bank_account_id"` // NEW, optional
	Note                  *string         `json:"note"`
}

type PurchaseReturnItemDetail struct {
	ID             uuid.UUID       `json:"id"`
	PurchaseItemID uuid.UUID       `json:"purchase_item_id"`
	ProductID      uuid.UUID       `json:"product_id"`
	ProductName    string          `json:"product_name"`
	ProductSKU     string          `json:"product_sku"`
	UnitName       string          `json:"unit_name"`
	AllowDecimal   bool            `json:"allow_decimal"`
	Qty            decimal.Decimal `json:"qty"`
	Amount         decimal.Decimal `json:"amount"`
	Reason         *string         `json:"reason"`
}

type PurchaseReturnDetail struct {
	ID            uuid.UUID                  `json:"id"`
	ReturnNo      string                     `json:"return_no"`
	ReturnDate    string                     `json:"return_date"`
	Total         decimal.Decimal            `json:"total"`
	Note          *string                    `json:"note"`
	CreatedByName *string                    `json:"created_by_name"`
	CreatedAt     time.Time                  `json:"created_at"`
	Items         []PurchaseReturnItemDetail `json:"items"`
}

type UpdatePurchasePaymentRequest struct {
	Amount                decimal.Decimal `json:"amount"`
	PaymentDate           string          `json:"payment_date"`
	PaymentMethod         PaymentMethod   `json:"payment_method"`
	SupplierBankAccountID *uuid.UUID      `json:"supplier_bank_account_id"` // NEW, optional
	Note                  *string         `json:"note"`
}

// ADD to: internals/models/purchase.go (or keep as its own file in package models)

// Cursor shared by both supplier lists.
// Date is YYYY-MM-DD (purchase_date / return_date); the order is Date DESC, CreatedAt DESC, ID DESC.
type SupplierListCursor struct {
	Date      string    `json:"date"`
	CreatedAt time.Time `json:"created_at"`
	ID        uuid.UUID `json:"id"`
}

// ---------- purchases of one supplier ----------

type SupplierPurchaseListQuery struct { // raw, from the handler
	PaymentStatus  string
	PurchaseStatus string
	FromDate       string
	ToDate         string
	Cursor         string
	Limit          int
}

type SupplierPurchaseListFilter struct { // validated, for the repository
	SupplierID     uuid.UUID
	PaymentStatus  *PaymentStatus
	PurchaseStatus *PurchaseStatus
	FromDate       *time.Time // inclusive
	ToDate         *time.Time // inclusive
	Cursor         *SupplierListCursor
	Limit          int
}

type SupplierPurchaseItem struct {
	PurchaseID          uuid.UUID       `json:"purchase_id"`
	SupplierID          uuid.UUID       `json:"supplier_id"`
	SupplierName        string          `json:"supplier_name"`
	PurchaseOrderID     *uuid.UUID      `json:"purchase_order_id"`
	PurchaseOrderNumber *string         `json:"po_no"`
	SupplierBillNo      string          `json:"supplier_bill_no"`
	PurchaseDate        string          `json:"purchase_date"` // YYYY-MM-DD
	Total               decimal.Decimal `json:"total"`
	PaidAmount          decimal.Decimal `json:"paid_amount"`
	DueAmount           decimal.Decimal `json:"due_amount"`
	PaymentStatus       PaymentStatus   `json:"payment_status"`
	Status              PurchaseStatus  `json:"status"`
	Note                *string         `json:"note"`
	CreatedAt           time.Time       `json:"created_at"`
}

type GetSupplierPurchasesResponse struct {
	Items      []SupplierPurchaseItem `json:"items"`
	Total      int                    `json:"total"`
	NextCursor *string                `json:"next_cursor"`
	HasMore    bool                   `json:"has_more"`
}

// ---------- purchase returns of one supplier ----------

type SupplierReturnListQuery struct { // raw, from the handler
	FromDate string
	ToDate   string
	Cursor   string
	Limit    int
}

type SupplierReturnListFilter struct { // validated, for the repository
	SupplierID uuid.UUID
	FromDate   *time.Time // inclusive
	ToDate     *time.Time // inclusive
	Cursor     *SupplierListCursor
	Limit      int
}

type SupplierReturnItem struct {
	PurchaseReturnID uuid.UUID       `json:"purchase_return_id"`
	ReturnNo         string          `json:"return_no"`
	PurchaseID       uuid.UUID       `json:"purchase_id"`
	SupplierBillNo   string          `json:"supplier_bill_no"`
	SupplierID       uuid.UUID       `json:"supplier_id"`
	ReturnDate       string          `json:"return_date"` // YYYY-MM-DD
	Total            decimal.Decimal `json:"total"`
	Note             *string         `json:"note"`
	CreatedAt        time.Time       `json:"created_at"`
}

type GetSupplierReturnsResponse struct {
	Items      []SupplierReturnItem `json:"items"`
	Total      int                  `json:"total"`
	NextCursor *string              `json:"next_cursor"`
	HasMore    bool                 `json:"has_more"`
}
