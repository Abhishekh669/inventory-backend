// NEW FILE: internals/models/customer.go
package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// balance = what the customer still has to pay us; credit_balance = advance they hold with us.
type Customer struct {
	ID             uuid.UUID       `json:"id"`
	Name           string          `json:"name"`
	ContactPerson  *string         `json:"contact_person"`
	Phone          *string         `json:"phone"`
	Email          *string         `json:"email"`
	Address        *string         `json:"address"`
	PanNo          *string         `json:"pan_no"`
	OpeningBalance decimal.Decimal `json:"opening_balance"`
	Balance        decimal.Decimal `json:"balance"`
	CreditBalance  decimal.Decimal `json:"credit_balance"`
	IsActive       bool            `json:"is_active"`
	CreatedAt      time.Time       `json:"created_at"`
}

type CreateCustomerRequest struct {
	Name           string          `json:"name"`
	ContactPerson  *string         `json:"contact_person"`
	Phone          *string         `json:"phone"`
	Email          *string         `json:"email"`
	Address        *string         `json:"address"`
	PanNo          *string         `json:"pan_no"`
	OpeningBalance decimal.Decimal `json:"opening_balance"` // what they already owe us; becomes the starting balance
}

// The balances are not edited here: they only move through sales and payments.
type UpdateCustomerRequest struct {
	Name          string  `json:"name"`
	ContactPerson *string `json:"contact_person"`
	Phone         *string `json:"phone"`
	Email         *string `json:"email"`
	Address       *string `json:"address"`
	PanNo         *string `json:"pan_no"`
	IsActive      *bool   `json:"is_active"` // nil = leave as it is
}

// ---------- list ----------

type BalanceOp string

const (
	BalanceGT  BalanceOp = "gt"
	BalanceGTE BalanceOp = "gte"
	BalanceLT  BalanceOp = "lt"
	BalanceLTE BalanceOp = "lte"
	BalanceEQ  BalanceOp = "eq"
)

// raw values from the query string (unvalidated)
type CustomerListQuery struct {
	Search        string
	BalanceOp     string
	BalanceAmount string
	Cursor        string
	Limit         int
}

// validated values for the repository; every filter is optional
type CustomerListFilter struct {
	Search        *string
	BalanceOp     *BalanceOp
	BalanceAmount *decimal.Decimal
	Cursor        *CustomerListCursor
	Limit         int
}

// order is created_at DESC, id DESC (newest first)
type CustomerListCursor struct {
	CreatedAt time.Time `json:"c"`
	ID        uuid.UUID `json:"i"`
}

type GetCustomersResponse struct {
	Items      []Customer `json:"items"`
	Total      int        `json:"total"` // customers matching the filters
	NextCursor *string    `json:"next_cursor"`
	HasMore    bool       `json:"has_more"`
}