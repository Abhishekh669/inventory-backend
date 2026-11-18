// NEW FILE: internals/models/stock_movement.go
package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type StockMovementType string

const (
	StockMovementOpening        StockMovementType = "opening"
	StockMovementPurchase       StockMovementType = "purchase"
	StockMovementSale           StockMovementType = "sale"
	StockMovementPurchaseReturn StockMovementType = "purchase_return"
	StockMovementSaleReturn     StockMovementType = "sale_return"
	StockMovementAdjustment     StockMovementType = "adjustment"
)

// Order is movement_date DESC, created_at DESC, id DESC. Date is YYYY-MM-DD.
type StockMovementCursor struct {
	Date      string    `json:"date"`
	CreatedAt time.Time `json:"created_at"`
	ID        uuid.UUID `json:"id"`
}

// Raw query from the handler (unvalidated).
type StockMovementListQuery struct {
	ProductID string
	Type      string
	FromDate  string
	ToDate    string
	Cursor    string
	Limit     int
}

// Validated filter passed to the repository.
type StockMovementListFilter struct {
	ProductID *uuid.UUID
	Type      *StockMovementType
	FromDate  *time.Time // inclusive
	ToDate    *time.Time // inclusive
	Cursor    *StockMovementCursor
	Limit     int
}

type StockMovementItem struct {
	ID           uuid.UUID         `json:"id"`
	ProductID    uuid.UUID         `json:"product_id"`
	ProductName  string            `json:"product_name"`
	ProductSKU   string            `json:"product_sku"`
	Type         StockMovementType `json:"type"`
	QtyChange    decimal.Decimal   `json:"qty_change"`
	BalanceAfter decimal.Decimal   `json:"balance_after"`
	RefType      *string           `json:"ref_type"`
	RefID        *uuid.UUID        `json:"ref_id"`
	Note         *string           `json:"note"`
	UserID       *uuid.UUID        `json:"user_id"`
	UserName     *string           `json:"user_name"`
	MovementDate string            `json:"movement_date"` // YYYY-MM-DD
	CreatedAt    time.Time         `json:"created_at"`
}

type GetAllStockMovementsResponse struct {
	Items      []StockMovementItem `json:"items"`
	Total      int                 `json:"total"`
	NextCursor *string             `json:"next_cursor"`
	HasMore    bool                `json:"has_more"`
}
