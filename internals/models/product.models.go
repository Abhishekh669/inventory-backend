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
