package models

import (
	"time"

	"github.com/google/uuid"
)

type Setting struct {
	ID              uuid.UUID `db:"id" json:"id"`
	Name            string    `db:"name" json:"name"`
	Address         *string   `db:"address" json:"address"`
	Phone           *string   `db:"phone" json:"phone"`
	PanNo           *string   `db:"pan_no" json:"pan_no"`
	VatNo           *string   `db:"vat_no" json:"vat_no"`
	DefaultVatRate  float64   `db:"default_vat_rate" json:"default_vat_rate"`
	LowStockDefault float64   `db:"low_stock_default" json:"low_stock_default"`
	BillFooter      *string   `db:"bill_footer" json:"bill_footer"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
}

// UpdateSettingRequest is the payload for updating the settings.
type UpdateSettingRequest struct {
	Name            string  `json:"name" db:"name"`
	Address         *string `json:"address" db:"address"`
	Phone           *string `json:"phone" db:"phone"`
	PanNo           *string `json:"pan_no" db:"pan_no"`
	VatNo           *string `json:"vat_no" db:"vat_no"`
	DefaultVatRate  float64 `json:"default_vat_rate" db:"default_vat_rate"`
	LowStockDefault float64 `json:"low_stock_default" db:"low_stock_default"`
	BillFooter      *string `json:"bill_footer" db:"bill_footer"`
}
