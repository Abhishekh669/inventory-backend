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
	SupplierID            uuid.UUID       `json:"supplier_id" binding:"required"`
	Amount                decimal.Decimal `json:"amount" binding:"required"`
	PaymentMethod         PaymentMethod   `json:"payment_method" binding:"required"`
	SupplierBankAccountID *uuid.UUID      `json:"supplier_bank_account_id"` // NEW, optional
	Note                  *string         `json:"note"`
}

type SupplierPaymentCreationType struct {
	SupplierID            uuid.UUID       `json:"supplier_id" binding:"required"`
	Amount                decimal.Decimal `json:"amount" binding:"required"`
	PaymentDate           *time.Time      `json:"payment_date"`
	PaymentMethod         PaymentMethod   `json:"payment_method" binding:"required"`
	PaymentType           PaymentType     `json:"payment_type" binding:"required"`
	Note                  *string         `json:"note"`
	SupplierBankAccountID *uuid.UUID      `json:"supplier_bank_account_id"` // NEW

}

type SupplierPaymentStatus string

const (
	SupplierPaymentUnallocated SupplierPaymentStatus = "unallocated" // nothing used yet
	SupplierPaymentPartial     SupplierPaymentStatus = "partial"     // some used
	SupplierPaymentAllocated   SupplierPaymentStatus = "allocated"   // fully used
)

// ---------- response ----------

type SupplierPaymentAllocationResponse struct {
	PurchaseID      uuid.UUID       `json:"purchase_id"`
	SupplierBillNo  string          `json:"supplier_bill_no"`
	PurchaseDate    time.Time       `json:"purchase_date"`
	PurchaseTotal   decimal.Decimal `json:"purchase_total"`
	AllocatedAmount decimal.Decimal `json:"allocated_amount"`
}

type SupplierIDPaymentResponse struct {
	ID            uuid.UUID       `json:"id"`
	SupplierID    uuid.UUID       `json:"supplier_id"`
	Amount        decimal.Decimal `json:"amount"`
	PaymentDate   time.Time       `json:"payment_date"`
	PaymentMethod PaymentMethod   `json:"payment_method"`
	PaymentType   PaymentType     `json:"payment_type"`
	Note          *string         `json:"note"`

	AllocatedAmount   decimal.Decimal `json:"allocated_amount"`
	UnallocatedAmount decimal.Decimal `json:"unallocated_amount"`

	Allocations []SupplierPaymentAllocationResponse `json:"allocations"`

	CreatedBy     *uuid.UUID `json:"created_by"`
	CreatedByName *string    `json:"created_by_name"`
	CreatedAt     time.Time  `json:"created_at"`

	SupplierBankAccountID *uuid.UUID `json:"supplier_bank_account_id"`
	BankName              *string    `json:"bank_name"`
	AccountName           *string    `json:"account_name"` // NEW
	AccountNumber         *string    `json:"account_number"`
}

// SupplierFinancialSummary mirrors supplier_financial_summaries plus two derived numbers.
type SupplierFinancialSummary struct {
	TotalPurchases      decimal.Decimal `json:"total_purchases"`
	TotalPayments       decimal.Decimal `json:"total_payments"`
	TotalReturns        decimal.Decimal `json:"total_returns"`
	UnallocatedPayments decimal.Decimal `json:"unallocated_payments"` // advance still available

	AllocatedPayments decimal.Decimal `json:"allocated_payments"` // payments already used on purchases
	TotalDue          decimal.Decimal `json:"total_due"`          // still owed on purchases
}

// Calculate fills the derived fields.
//
//	allocated = total_payments - unallocated_payments
//	due       = total_purchases - total_returns - allocated
func (s *SupplierFinancialSummary) Calculate() {
	s.AllocatedPayments = s.TotalPayments.Sub(s.UnallocatedPayments)
	s.TotalDue = s.TotalPurchases.Sub(s.TotalReturns).Sub(s.AllocatedPayments)
}

type SupplierPaymentListResponse struct {
	Payments   []SupplierIDPaymentResponse `json:"payments"`
	Total      int                         `json:"total"`
	HasMore    bool                        `json:"has_more"`
	NextCursor *string                     `json:"next_cursor"`
	Summary    SupplierFinancialSummary    `json:"summary"`
}

// ---------- query ----------

// raw values from the HTTP query string
type SupplierPaymentListQuery struct {
	SupplierID uuid.UUID
	Status     string
	FromDate   string
	ToDate     string
	Cursor     string
	Limit      int
}

// validated values used by the repository
type SupplierPaymentListFilter struct {
	SupplierID uuid.UUID
	Status     *SupplierPaymentStatus
	FromDate   *time.Time
	ToDate     *time.Time
	Cursor     *SupplierPaymentCursor
	Limit      int
}

// order is payment_date DESC, created_at DESC, id DESC
type SupplierPaymentCursor struct {
	PaymentDate string    `json:"d"` // YYYY-MM-DD
	CreatedAt   time.Time `json:"c"`
	ID          uuid.UUID `json:"i"`
}

type SupplierLedgerEntryType string

const (
	SupplierLedgerPurchase          SupplierLedgerEntryType = "purchase"
	SupplierLedgerAdvance           SupplierLedgerEntryType = "advance" // original advance transfer
	SupplierLedgerPayment           SupplierLedgerEntryType = "payment" // regular payment only
	SupplierLedgerReturn            SupplierLedgerEntryType = "return"
	SupplierLedgerAdvanceAllocation SupplierLedgerEntryType = "advance_allocation"
)

// One line of the supplier account.
//
//	balance = what we owe the supplier after this line
//	credit  = advance the supplier holds for us after this line (supplier owes us)
//	Only one of the two is above zero.

type SupplierLedgerEntry struct {
	ID          uuid.UUID               `json:"id"` // purchase / payment / return / allocation id
	EntryType   SupplierLedgerEntryType `json:"entry_type"`
	EntryDate   time.Time               `json:"entry_date"`
	Reference   *string                 `json:"reference"`
	Description string                  `json:"description"`
	Note        *string                 `json:"note"`
	Amount      decimal.Decimal         `json:"amount"`
	Balance     decimal.Decimal         `json:"balance"`
	Credit      decimal.Decimal         `json:"credit"`
	CreatedAt   time.Time               `json:"created_at"`

	// NEW: the bank account of the payment (null for purchases, returns and cash)
	SupplierBankAccountID *uuid.UUID `json:"supplier_bank_account_id"`
	BankName              *string    `json:"bank_name"`
	AccountName           *string    `json:"account_name"`
	AccountNumber         *string    `json:"account_number"`

	SortRank int `json:"-"` // tie-break, only used for the cursor
}

type SupplierLedgerSupplier struct {
	ID             uuid.UUID       `json:"id"`
	Name           string          `json:"name"`
	OpeningBalance decimal.Decimal `json:"opening_balance"`
}

type SupplierLedgerResponse struct {
	Supplier SupplierLedgerSupplier `json:"supplier"`

	// figures for the selected period (the whole history when no dates are given)
	OpeningBalance decimal.Decimal `json:"opening_balance"` // we owed this before from_date
	OpeningCredit  decimal.Decimal `json:"opening_credit"`  // advance before from_date
	TotalPurchases decimal.Decimal `json:"total_purchases"`
	TotalPayments  decimal.Decimal `json:"total_payments"` // regular payments + advance applied
	TotalAdvances  decimal.Decimal `json:"total_advances"` // advances paid in the period
	TotalReturns   decimal.Decimal `json:"total_returns"`
	ClosingBalance decimal.Decimal `json:"closing_balance"` // we owe at the end of the period
	ClosingCredit  decimal.Decimal `json:"closing_credit"`  // advance still held at the end of the period

	Entries    []SupplierLedgerEntry `json:"entries"`
	Total      int                   `json:"total"`
	HasMore    bool                  `json:"has_more"`
	NextCursor *string               `json:"next_cursor"`
}

// raw values from the HTTP query string
type SupplierLedgerQuery struct {
	SupplierID uuid.UUID
	FromDate   string
	ToDate     string
	Cursor     string
	Limit      int
}

// validated values used by the repository
type SupplierLedgerFilter struct {
	SupplierID uuid.UUID
	FromDate   *time.Time
	ToDate     *time.Time
	Cursor     *SupplierLedgerCursor
	Limit      int
}

// order is entry_date DESC, created_at DESC, id DESC (newest first)
type SupplierLedgerCursor struct {
	EntryDate string    `json:"d"` // YYYY-MM-DD
	CreatedAt time.Time `json:"c"`
	Rank      int       `json:"k"`
	ID        uuid.UUID `json:"i"`
}
