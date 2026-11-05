package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Supplier struct {
	ID             uuid.UUID       `db:"id" json:"id"`
	Name           string          `db:"name" json:"name"`
	ContactPerson  *string         `db:"contact_person" json:"contact_person"`
	Phone          *string         `db:"phone" json:"phone"`
	Address        *string         `db:"address" json:"address"`
	PanNo          *string         `db:"pan_no" json:"pan_no"`
	OpeningBalance decimal.Decimal `db:"opening_balance" json:"opening_balance"`
	Balance        decimal.Decimal `db:"balance" json:"balance"`
	IsActive       bool            `db:"is_active" json:"is_active"`
	CreatedAt      time.Time       `db:"created_at" json:"created_at"`
}
type CreateSupplierInput struct {
	Name           string          `json:"name"`
	ContactPerson  *string         `json:"contact_person"`
	Phone          *string         `json:"phone"`
	Address        *string         `json:"address"`
	PanNo          *string         `json:"pan_no"`
	OpeningBalance decimal.Decimal `json:"opening_balance"`
}

// nil field = leave unchanged
type UpdateSupplierInput struct {
	Name           *string          `json:"name"`
	ContactPerson  *string          `json:"contact_person"`
	Phone          *string          `json:"phone"`
	Address        *string          `json:"address"`
	PanNo          *string          `json:"pan_no"`
	OpeningBalance *decimal.Decimal `json:"opening_balance"`
	IsActive       *bool            `json:"is_active"`
}

type DeleteSuppliers struct {
	IDs []string `json:"ids"`
}
