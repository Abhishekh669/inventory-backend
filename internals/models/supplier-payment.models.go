package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type PaymentMethod string

const (
	PaymentMethodCash   PaymentMethod = "cash"
	PaymentMethodBank   PaymentMethod = "bank"
	PaymentMethodCheque PaymentMethod = "cheque"
	PaymentMethodOnline PaymentMethod = "online"
)

type PaymentType string

const (
	PaymentTypeAdvance PaymentType = "advance"
	PaymentTypeRegular PaymentType = "regular"
)

type SupplierPayment struct {
	ID            uuid.UUID       `json:"id"`
	SupplierID    uuid.UUID       `json:"supplier_id"`
	Amount        decimal.Decimal `json:"amount"`
	PaymentDate   time.Time       `json:"payment_date"`
	PaymentMethod PaymentMethod   `json:"payment_method"`
	PaymentType   PaymentType     `json:"payment_type"`
	ReferenceNo   *string         `json:"reference_no"`
	Note          *string         `json:"note"`
	CreatedBy     *uuid.UUID      `json:"created_by"`
	CreatedAt     time.Time       `json:"created_at"`
}

type PurchasePaymentAllocation struct {
	ID                uuid.UUID       `json:"id"`
	SupplierPaymentID uuid.UUID       `json:"supplier_payment_id"`
	PurchaseID        uuid.UUID       `json:"purchase_id"`
	Amount            decimal.Decimal `json:"amount"`
	CreatedAt         time.Time       `json:"created_at"`
}

type CreateAdvancePaymentType struct {
	SupplierID    uuid.UUID       `json:"supplier_id" binding:"required"`
	Amount        decimal.Decimal `json:"amount" binding:"required"`
	PaymentMethod PaymentMethod   `json:"payment_method" binding:"required"`
	Note          *string         `json:"note"`
}

type SupplierPaymentCreationType struct {
	SupplierID    uuid.UUID       `json:"supplier_id" binding:"required"`
	Amount        decimal.Decimal `json:"amount" binding:"required"`
	PaymentDate   *time.Time      `json:"payment_date"`
	PaymentMethod PaymentMethod   `json:"payment_method" binding:"required"`
	PaymentType   PaymentType     `json:"payment_type" binding:"required"`
	Note          *string         `json:"note"`
}
