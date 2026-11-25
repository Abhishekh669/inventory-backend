// NEW FILE: internals/services/supplier_bank_account.go
package services

import (
	"context"
	"inventory/internals/models"
	"inventory/internals/repository"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

type SupplierBankAccountService interface {
	GetSupplierBankAccounts(ctx context.Context, supplierID uuid.UUID) ([]models.SupplierBankAccount, error)
	CreateSupplierBankAccount(ctx context.Context, supplierID uuid.UUID, req models.CreateSupplierBankAccountRequest) (uuid.UUID, error)
	UpdateSupplierBankAccount(ctx context.Context, supplierID, accountID uuid.UUID, req models.UpdateSupplierBankAccountRequest) error
	DeleteSupplierBankAccount(ctx context.Context, supplierID, accountID uuid.UUID) error
}

type supplierBankAccountService struct {
	repo repository.SupplierBankAccountRepository
}

func NewSupplierBankAccountService(repo repository.SupplierBankAccountRepository) SupplierBankAccountService {
	return &supplierBankAccountService{repo: repo}
}

// the limits match the VARCHAR sizes of the table
const (
	maxBankNameLen      = 100
	maxAccountNameLen   = 150
	maxAccountNumberLen = 50
	maxBranchLen        = 100
)

// normalizeBankFields trims and validates the fields shared by create and update.
func normalizeBankFields(bankName, accountName, accountNumber *string, branch **string) error {
	bad := models.NewValidationError

	*bankName = strings.TrimSpace(*bankName)
	*accountName = strings.TrimSpace(*accountName)
	*accountNumber = strings.TrimSpace(*accountNumber)

	if *bankName == "" || utf8.RuneCountInString(*bankName) > maxBankNameLen {
		return bad("bank name is required (max 100 characters)")
	}
	if *accountName == "" || utf8.RuneCountInString(*accountName) > maxAccountNameLen {
		return bad("account name is required (max 150 characters)")
	}
	if *accountNumber == "" || utf8.RuneCountInString(*accountNumber) > maxAccountNumberLen {
		return bad("account number is required (max 50 characters)")
	}

	if *branch != nil {
		b := strings.TrimSpace(**branch)
		switch {
		case b == "":
			*branch = nil
		case utf8.RuneCountInString(b) > maxBranchLen:
			return bad("branch is too long (max 100 characters)")
		default:
			*branch = &b
		}
	}
	return nil
}

func (s *supplierBankAccountService) GetSupplierBankAccounts(ctx context.Context, supplierID uuid.UUID) ([]models.SupplierBankAccount, error) {
	if supplierID == uuid.Nil {
		return nil, models.NewValidationError("invalid supplier id")
	}
	return s.repo.GetSupplierBankAccounts(ctx, supplierID)
}

func (s *supplierBankAccountService) CreateSupplierBankAccount(ctx context.Context, supplierID uuid.UUID, req models.CreateSupplierBankAccountRequest) (uuid.UUID, error) {
	if supplierID == uuid.Nil {
		return uuid.Nil, models.NewValidationError("invalid supplier id")
	}
	if err := normalizeBankFields(&req.BankName, &req.AccountName, &req.AccountNumber, &req.Branch); err != nil {
		return uuid.Nil, err
	}
	return s.repo.CreateSupplierBankAccount(ctx, supplierID, &req)
}

func (s *supplierBankAccountService) UpdateSupplierBankAccount(ctx context.Context, supplierID, accountID uuid.UUID, req models.UpdateSupplierBankAccountRequest) error {
	if supplierID == uuid.Nil || accountID == uuid.Nil {
		return models.NewValidationError("invalid id")
	}
	if err := normalizeBankFields(&req.BankName, &req.AccountName, &req.AccountNumber, &req.Branch); err != nil {
		return err
	}
	return s.repo.UpdateSupplierBankAccount(ctx, supplierID, accountID, &req)
}

func (s *supplierBankAccountService) DeleteSupplierBankAccount(ctx context.Context, supplierID, accountID uuid.UUID) error {
	if supplierID == uuid.Nil || accountID == uuid.Nil {
		return models.NewValidationError("invalid id")
	}
	return s.repo.DeleteSupplierBankAccount(ctx, supplierID, accountID)
}

func bankAccountForMethod(method models.PaymentMethod, id *uuid.UUID) *uuid.UUID {
	if id == nil || *id == uuid.Nil {
		return nil
	}
	if method == models.PaymentMethodCash || method == "" {
		return nil
	}
	return id
}
