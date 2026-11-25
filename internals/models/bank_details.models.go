// NEW FILE: internals/models/supplier_bank_account.go
package models

import (
	"time"

	"github.com/google/uuid"
)

type SupplierBankAccount struct {
	ID            uuid.UUID `json:"id"`
	SupplierID    uuid.UUID `json:"supplier_id"`
	BankName      string    `json:"bank_name"`
	AccountName   string    `json:"account_name"`
	AccountNumber string    `json:"account_number"`
	Branch        *string   `json:"branch"`
	IsActive      bool      `json:"is_active"`
	CreatedAt     time.Time `json:"created_at"`
}

type CreateSupplierBankAccountRequest struct {
	BankName      string  `json:"bank_name"`
	AccountName   string  `json:"account_name"`
	AccountNumber string  `json:"account_number"`
	Branch        *string `json:"branch"`
}

type UpdateSupplierBankAccountRequest struct {
	BankName      string  `json:"bank_name"`
	AccountName   string  `json:"account_name"`
	AccountNumber string  `json:"account_number"`
	Branch        *string `json:"branch"`
	IsActive      *bool   `json:"is_active"` // nil = leave as it is
}
