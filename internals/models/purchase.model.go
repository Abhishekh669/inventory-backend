package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type PurchaseOrderStatus string

const (
	PurchaseOrderDraft     PurchaseOrderStatus = "draft"
	PurchaseOrderSent      PurchaseOrderStatus = "sent"
	PurchaseOrderPartial   PurchaseOrderStatus = "partial"
	PurchaseOrderReceived  PurchaseOrderStatus = "received"
	PurchaseOrderCancelled PurchaseOrderStatus = "cancelled"
)

type PurchaseOrder struct {
	ID           uuid.UUID           `json:"id" db:"id"`
	PONo         string              `json:"po_no" db:"po_no"`
	SupplierID   uuid.UUID           `json:"supplier_id" db:"supplier_id"`
	Title        *string             `json:"title" db:"title"`
	OrderDate    time.Time           `json:"order_date" db:"order_date"`
	ExpectedDate *time.Time          `json:"expected_date" db:"expected_date"`
	Status       PurchaseOrderStatus `json:"status" db:"status"`
	Note         *string             `json:"note" db:"note"`
	CreatedBy    *uuid.UUID          `json:"created_by" db:"created_by"`
	CreatedAt    time.Time           `json:"created_at" db:"created_at"`
}

type PurchaseOrderItem struct {
	ID               uuid.UUID        `json:"id" db:"id"`
	PurchaseOrderID  uuid.UUID        `json:"purchase_order_id" db:"purchase_order_id"`
	ProductID        uuid.UUID        `json:"product_id" db:"product_id"`
	QtyOrdered       decimal.Decimal  `json:"qty_ordered" db:"qty_ordered"`
	QtyReceived      decimal.Decimal  `json:"qty_received" db:"qty_received"`
	ExpectedUnitCost *decimal.Decimal `json:"expected_unit_cost" db:"expected_unit_cost"`
}

type CreatePurchaseOrderItem struct {
	PurchaseOrderID  uuid.UUID        `json:"purchase_order_id" db:"purchase_order_id"`
	ProductID        uuid.UUID        `json:"product_id" db:"product_id"`
	QtyOrdered       decimal.Decimal  `json:"qty_ordered" db:"qty_ordered"`
	QtyReceived      decimal.Decimal  `json:"qty_received" db:"qty_received"`
	ExpectedUnitCost *decimal.Decimal `json:"expected_unit_cost" db:"expected_unit_cost"`
}

type CreatePurchaseOrder struct {
	PONo               string                  `json:"po_no" db:"po_no"`
	SupplierID         uuid.UUID               `json:"supplier_id" db:"supplier_id"`
	Title              *string                 `json:"title" db:"title"`
	OrderDate          time.Time               `json:"order_date" db:"order_date"`
	ExpectedDate       *time.Time              `json:"expected_date" db:"expected_date"`
	Status             PurchaseOrderStatus     `json:"status" db:"status"`
	Note               *string                 `json:"note" db:"note"`
	PurchaseOrderItems CreatePurchaseOrderItem `json:"purchase_order_items" db:"purchase_order_items"`
}

// PurchaseOrderSummary is the light row shown in the supplier's order list.
type PurchaseOrderSummary struct {
	ID            uuid.UUID           `json:"id"`
	PONo          string              `json:"po_no"`
	Title         *string             `json:"title"`
	Status        PurchaseOrderStatus `json:"status"`
	Note          *string             `json:"note"`
	CreatedBy     *uuid.UUID          `json:"created_by"`
	CreatedByName *string             `json:"created_by_name"`
	CreatedAt     time.Time           `json:"created_at"`
}

// PurchaseOrderCursor is the position of the last row of a page.
// It is sent to the client as an opaque base64 string.
type PurchaseOrderCursor struct {
	CreatedAt time.Time `json:"t"`
	ID        uuid.UUID `json:"id"`
}

type PurchaseOrderListResponse struct {
	PurchaseOrders []PurchaseOrderSummary `json:"purchase_orders"`
	Total          int                    `json:"total"`
	NextCursor     *string                `json:"next_cursor"`
	HasMore        bool                   `json:"has_more"`
}

// PurchaseOrderItemDetail is one line of an order, joined with product and unit info.
type PurchaseOrderItemDetail struct {
	ID               uuid.UUID        `json:"id"`
	ProductID        uuid.UUID        `json:"product_id"`
	ProductName      string           `json:"product_name"`
	ProductSKU       string           `json:"product_sku"`
	UnitName         string           `json:"unit_name"`
	AllowDecimal     bool             `json:"allow_decimal"`
	QtyOrdered       decimal.Decimal  `json:"qty_ordered"`
	QtyReceived      decimal.Decimal  `json:"qty_received"`
	ExpectedUnitCost *decimal.Decimal `json:"expected_unit_cost"`
	LineTotal        *decimal.Decimal `json:"line_total"`
}

// ---------- request bodies (create / update) ----------

// PurchaseOrderItemInput is one product line sent by the client.
// ID is set for an existing line when updating, and empty for a new line.
type PurchaseOrderItemInput struct {
	ID               *uuid.UUID       `json:"id"`
	ProductID        uuid.UUID        `json:"product_id"`
	QtyOrdered       decimal.Decimal  `json:"qty_ordered"`
	ExpectedUnitCost *decimal.Decimal `json:"expected_unit_cost"`
}

// PurchaseOrderInput is the body of create and update.
// SupplierID is required on create and ignored on update.
type PurchaseOrderInput struct {
	SupplierID   uuid.UUID                `json:"supplier_id"`
	Title        *string                  `json:"title"`
	OrderDate    string                   `json:"order_date"`    // YYYY-MM-DD, empty means today
	ExpectedDate *string                  `json:"expected_date"` // YYYY-MM-DD
	Status       *PurchaseOrderStatus     `json:"status"`
	Note         *string                  `json:"note"`
	Items        []PurchaseOrderItemInput `json:"items"`
}

type DeletePurchaseOrdersRequest struct {
	IDs []uuid.UUID `json:"ids"`
}

// ---------- validated values passed from the service to the repository ----------

type PurchaseOrderItemWrite struct {
	ID               *uuid.UUID
	ProductID        uuid.UUID
	QtyOrdered       decimal.Decimal
	ExpectedUnitCost *decimal.Decimal
}

type PurchaseOrderWrite struct {
	SupplierID   uuid.UUID
	Title        *string
	OrderDate    time.Time // date only, UTC midnight
	ExpectedDate *time.Time
	Status       *PurchaseOrderStatus
	Note         *string
	Items        []PurchaseOrderItemWrite
}

// ---------- full purchase order (used to fill the edit form) ----------

type PurchaseOrderDetail struct {
	ID            uuid.UUID                 `json:"id"`
	PONo          string                    `json:"po_no"`
	SupplierID    uuid.UUID                 `json:"supplier_id"`
	Title         *string                   `json:"title"`
	OrderDate     string                    `json:"order_date"`    // YYYY-MM-DD
	ExpectedDate  *string                   `json:"expected_date"` // YYYY-MM-DD
	Status        PurchaseOrderStatus       `json:"status"`
	Note          *string                   `json:"note"`
	CreatedBy     *uuid.UUID                `json:"created_by"`
	CreatedByName *string                   `json:"created_by_name"`
	CreatedAt     time.Time                 `json:"created_at"`
	Items         []PurchaseOrderItemDetail `json:"items"`
}

type PurchaseOrderListQuery struct {
	SupplierID uuid.UUID
	Status     string
	Search     string // matches title or note
	FromDate   string
	ToDate     string
	Cursor     string
	Limit      int
}

type PurchaseOrderListFilter struct {
	SupplierID uuid.UUID
	Status     *PurchaseOrderStatus
	Search     *string // already trimmed and validated
	FromDate   *time.Time
	ToDate     *time.Time
	Cursor     *PurchaseOrderCursor
	Limit      int
}
