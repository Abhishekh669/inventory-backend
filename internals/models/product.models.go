package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Product struct {
	ID            uuid.UUID       `db:"id" json:"id"`
	SKU           string          `db:"sku" json:"sku"`
	Name          string          `db:"name" json:"name"`
	CategoryID    uuid.UUID       `db:"category_id" json:"category_id"`
	UnitID        uuid.UUID       `db:"unit_id" json:"unit_id"`
	VatRate       decimal.Decimal `db:"vat_rate" json:"vat_rate"`
	CostPrice     decimal.Decimal `db:"cost_price" json:"cost_price"`
	CostWithVat   decimal.Decimal `db:"cost_with_vat" json:"cost_with_vat"`
	MarginPercent decimal.Decimal `db:"margin_percent" json:"margin_percent"`
	SellingPrice  decimal.Decimal `db:"selling_price" json:"selling_price"`
	ProfitPerUnit decimal.Decimal `db:"profit_per_unit" json:"profit_per_unit"`
	ReOrderLevel  decimal.Decimal `db:"re_order_level" json:"re_order_level"`
	CurrentStock  decimal.Decimal `db:"current_stock" json:"current_stock"`
	IsActive      bool            `db:"is_active" json:"is_active"`
	CreatedAt     time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time       `db:"updated_at" json:"updated_at"`
}

type ProductsListType struct {
	ID   uuid.UUID `db:"id" json:"id"`
	SKU  string    `db:"sku" json:"sku"`
	Name string    `db:"name" json:"name"`

	CategoryID   uuid.UUID `db:"category_id" json:"category_id"`
	CategoryName string    `db:"category_name" json:"category_name"`
	CategorySlug string    `db:"category_slug" json:"category_slug"`

	UnitID       uuid.UUID `db:"unit_id" json:"unit_id"`
	UnitName     string    `db:"unit_name" json:"unit_name"`
	AllowDecimal bool      `db:"allow_decimal" json:"allow_decimal"`

	VatRate     decimal.Decimal `db:"vat_rate" json:"vat_rate"`
	CostPrice   decimal.Decimal `db:"cost_price" json:"cost_price"`
	CostWithVat decimal.Decimal `db:"cost_with_vat" json:"cost_with_vat"`

	MarginPercent decimal.Decimal `db:"margin_percent" json:"margin_percent"`
	SellingPrice  decimal.Decimal `db:"selling_price" json:"selling_price"`
	ProfitPerUnit decimal.Decimal `db:"profit_per_unit" json:"profit_per_unit"`

	ReOrderLevel decimal.Decimal `db:"re_order_level" json:"re_order_level"`
	CurrentStock decimal.Decimal `db:"current_stock" json:"current_stock"`
	IsActive     bool            `db:"is_active" json:"is_active"`
	CreatedAt    time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at" json:"updated_at"`
}

type CreateProductRequestType struct {
	SKU           string          `db:"sku" json:"sku"`
	Name          string          `db:"name" json:"name"`
	CategoryID    uuid.UUID       `db:"category_id" json:"category_id"`
	UnitID        uuid.UUID       `db:"unit_id" json:"unit_id"`
	VatRate       decimal.Decimal `db:"vat_rate" json:"vat_rate"`
	CostPrice     decimal.Decimal `db:"cost_price" json:"cost_price"`
	CostWithVat   decimal.Decimal `db:"cost_with_vat" json:"cost_with_vat"`
	MarginPercent decimal.Decimal `db:"margin_percent" json:"margin_percent"`
	SellingPrice  decimal.Decimal `db:"selling_price" json:"selling_price"`
	ReOrderLevel  decimal.Decimal `db:"re_order_level" json:"re_order_level"`
	CurrentStock  decimal.Decimal `db:"current_stock" json:"current_stock"`
	IsActive      bool            `db:"is_active" json:"is_active"`
}

type UpdateProductRequestType struct {
	ID            uuid.UUID       `db:"id" json:"id"`
	SKU           string          `db:"sku" json:"sku"`
	Name          string          `db:"name" json:"name"`
	CategoryID    uuid.UUID       `db:"category_id" json:"category_id"`
	UnitID        uuid.UUID       `db:"unit_id" json:"unit_id"`
	VatRate       decimal.Decimal `db:"vat_rate" json:"vat_rate"`
	CostPrice     decimal.Decimal `db:"cost_price" json:"cost_price"`
	CostWithVat   decimal.Decimal `db:"cost_with_vat" json:"cost_with_vat"`
	MarginPercent decimal.Decimal `db:"margin_percent" json:"margin_percent"`
	SellingPrice  decimal.Decimal `db:"selling_price" json:"selling_price"`
	ReOrderLevel  decimal.Decimal `db:"re_order_level" json:"re_order_level"`
	CurrentStock  decimal.Decimal `db:"current_stock" json:"current_stock"`
	IsActive      bool            `db:"is_active" json:"is_active"`
}

type DeleteProductRequestType struct {
	IDs []string `json:"ids"`
}

type ProductQueryType struct {
	Name         string    `json:"name"`
	Limit        int32     `json:"limit"`
	FromDate     time.Time `json:"from_date"`
	ToDate       time.Time `json:"to_date"`
	CurrentStock int64     `json:"current_stock"`
}

type ProductsListTypeResponse struct {
	Products   []ProductsListType `json:"products"`
	Total      int                `json:"total"`
	HasMore    bool               `json:"has_more"`
	NextOffset int                `json:"next_offset"`
}

type ProductPriceHistory struct {
	ID              uuid.UUID       `db:"id" json:"id"`
	ProductID       uuid.UUID       `db:"product_id" json:"product_id"`
	PurchaseItemID  *uuid.UUID      `db:"purchase_item_id" json:"purchase_item_id"` // nullable
	UnitCost        decimal.Decimal `db:"unit_cost" json:"unit_cost"`
	UnitCostWithVat decimal.Decimal `db:"unit_cost_with_vat" json:"unit_cost_with_vat"`
	MarginPercent   decimal.Decimal `db:"margin_percent" json:"margin_percent"`
	SellingPrice    decimal.Decimal `db:"selling_price" json:"selling_price"`
	ProfitPerUnit   decimal.Decimal `db:"profit_per_unit" json:"profit_per_unit"`
	EffectiveDate   time.Time       `db:"effective_date" json:"effective_date"`
	CreatedAt       time.Time       `db:"created_at" json:"created_at"`
}

type ProductPriceHistoryListType struct {
	ID              uuid.UUID       `db:"id" json:"id"`
	ProductID       uuid.UUID       `db:"product_id" json:"product_id"`
	ProductSKU      string          `db:"product_sku" json:"product_sku"`
	ProductName     string          `db:"product_name" json:"product_name"`
	PurchaseItemID  *uuid.UUID      `db:"purchase_item_id" json:"purchase_item_id"`
	UnitCost        decimal.Decimal `db:"unit_cost" json:"unit_cost"`
	UnitCostWithVat decimal.Decimal `db:"unit_cost_with_vat" json:"unit_cost_with_vat"`
	MarginPercent   decimal.Decimal `db:"margin_percent" json:"margin_percent"`
	SellingPrice    decimal.Decimal `db:"selling_price" json:"selling_price"`
	ProfitPerUnit   decimal.Decimal `db:"profit_per_unit" json:"profit_per_unit"`
	EffectiveDate   time.Time       `db:"effective_date" json:"effective_date"`
	CreatedAt       time.Time       `db:"created_at" json:"created_at"`
}

type CreateProductPriceHistoryRequestType struct {
	ProductID       uuid.UUID       `db:"product_id" json:"product_id"`
	PurchaseItemID  *uuid.UUID      `db:"purchase_item_id" json:"purchase_item_id"`
	UnitCost        decimal.Decimal `db:"unit_cost" json:"unit_cost"`
	UnitCostWithVat decimal.Decimal `db:"unit_cost_with_vat" json:"unit_cost_with_vat"`
	MarginPercent   decimal.Decimal `db:"margin_percent" json:"margin_percent"`
	SellingPrice    decimal.Decimal `db:"selling_price" json:"selling_price"`
	EffectiveDate   *time.Time      `db:"effective_date" json:"effective_date"` // nil -> DB default
}

type DeleteProductPriceHistoryRequestType struct {
	IDs []string `json:"ids"`
}

type ProductPriceHistoryQueryType struct {
	ProductID *uuid.UUID `json:"product_id"`
	Limit     int32      `json:"limit"`
	Page      int32      `json:"page"`
	FromDate  *time.Time `json:"from_date"`
	ToDate    *time.Time `json:"to_date"`
}

type ProductPriceHistoryListTypeResponse struct {
	History    []ProductPriceHistoryListType `json:"history"`
	Total      int                           `json:"total"`
	HasMore    bool                          `json:"has_more"`
	NextOffset int                           `json:"next_offset"`
}

type UpdateProdcutPrice struct {
	CurrentMargin decimal.Decimal `json:"current_margin"`
	CurrentVat    decimal.Decimal `json:"current_vat"`
	OldPrice      decimal.Decimal `json:"old_price"`
	NewPrice      decimal.Decimal `json:"new_price"`
	ID            uuid.UUID       `json:"id"`
}

// calculated by the service, saved by the repository
type ProductPriceUpdate struct {
	ID            uuid.UUID
	OldCost       decimal.Decimal // used to detect concurrent changes
	VatRate       decimal.Decimal
	MarginPercent decimal.Decimal
	CostPrice     decimal.Decimal
	CostWithVat   decimal.Decimal
	SellingPrice  decimal.Decimal
}

type ProductPriceResult struct {
	ID            uuid.UUID       `json:"id"`
	VatRate       decimal.Decimal `json:"vat_rate"`
	MarginPercent decimal.Decimal `json:"margin_percent"`
	CostPrice     decimal.Decimal `json:"cost_price"`
	CostWithVat   decimal.Decimal `json:"cost_with_vat"`
	SellingPrice  decimal.Decimal `json:"selling_price"`
	ProfitPerUnit decimal.Decimal `json:"profit_per_unit"`
}
