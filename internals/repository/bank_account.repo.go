// NEW FILE: internals/repository/supplier_bank_account.go
package repository

import (
	"context"
	"errors"
	"fmt"
	"inventory/internals/database"
	"inventory/internals/models"
	"log"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SupplierBankAccountRepository interface {
	GetSupplierBankAccounts(ctx context.Context, supplierID uuid.UUID) ([]models.SupplierBankAccount, error)
	CreateSupplierBankAccount(ctx context.Context, supplierID uuid.UUID, req *models.CreateSupplierBankAccountRequest) (uuid.UUID, error)
	UpdateSupplierBankAccount(ctx context.Context, supplierID, accountID uuid.UUID, req *models.UpdateSupplierBankAccountRequest) error
	DeleteSupplierBankAccount(ctx context.Context, supplierID, accountID uuid.UUID) error
}

type bankQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type supplierBankAccountRepository struct {
	pool *pgxpool.Pool
}

func NewSupplierBankAccountRepository() SupplierBankAccountRepository {
	pool, err := database.GetPostgresPool()
	if err != nil {
		log.Println("Failed to initialize pool:", err)
		return nil
	}
	return &supplierBankAccountRepository{pool: pool}
}

func supplierBankPgCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

const duplicateBankAccountMsg = "this supplier already has an account with that number in that bank"

// A supplier has only a few accounts, so there is no pagination: active accounts first.
func (r *supplierBankAccountRepository) GetSupplierBankAccounts(ctx context.Context, supplierID uuid.UUID) ([]models.SupplierBankAccount, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, supplier_id, bank_name, account_name, account_number, branch, is_active, created_at
		FROM supplier_bank_accounts
		WHERE supplier_id = $1
		ORDER BY is_active DESC, bank_name, created_at`,
		supplierID,
	)
	if err != nil {
		return nil, fmt.Errorf("query supplier bank accounts: %w", err)
	}
	defer rows.Close()

	accounts := make([]models.SupplierBankAccount, 0)
	for rows.Next() {
		var a models.SupplierBankAccount
		if err := rows.Scan(&a.ID, &a.SupplierID, &a.BankName, &a.AccountName, &a.AccountNumber,
			&a.Branch, &a.IsActive, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan supplier bank account: %w", err)
		}
		accounts = append(accounts, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read supplier bank accounts: %w", err)
	}
	return accounts, nil
}

func (r *supplierBankAccountRepository) CreateSupplierBankAccount(ctx context.Context, supplierID uuid.UUID, req *models.CreateSupplierBankAccountRequest) (uuid.UUID, error) {
	bad := models.NewValidationError

	var id uuid.UUID
	err := r.pool.QueryRow(ctx, `
		INSERT INTO supplier_bank_accounts (supplier_id, bank_name, account_name, account_number, branch)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`,
		supplierID, req.BankName, req.AccountName, req.AccountNumber, req.Branch,
	).Scan(&id)
	if err != nil {
		switch supplierBankPgCode(err) {
		case "23505":
			return uuid.Nil, bad(duplicateBankAccountMsg)
		case "23503":
			return uuid.Nil, bad("supplier not found")
		}
		return uuid.Nil, fmt.Errorf("insert supplier bank account: %w", err)
	}
	return id, nil
}

func (r *supplierBankAccountRepository) UpdateSupplierBankAccount(ctx context.Context, supplierID, accountID uuid.UUID, req *models.UpdateSupplierBankAccountRequest) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE supplier_bank_accounts
		SET bank_name = $3, account_name = $4, account_number = $5, branch = $6,
		    is_active = COALESCE($7, is_active)
		WHERE id = $1 AND supplier_id = $2`,
		accountID, supplierID, req.BankName, req.AccountName, req.AccountNumber, req.Branch, req.IsActive,
	)
	if err != nil {
		if supplierBankPgCode(err) == "23505" {
			return models.NewValidationError(duplicateBankAccountMsg)
		}
		return fmt.Errorf("update supplier bank account: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return models.ErrNotFound
	}
	return nil
}

func (r *supplierBankAccountRepository) DeleteSupplierBankAccount(ctx context.Context, supplierID, accountID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM supplier_bank_accounts WHERE id = $1 AND supplier_id = $2`,
		accountID, supplierID,
	)
	if err != nil {
		// supplier_payments references the account, so one that was already used cannot be removed
		if supplierBankPgCode(err) == "23503" {
			return models.NewValidationError(
				"this bank account is used by supplier payments, so it cannot be deleted. Mark it inactive instead")
		}
		return fmt.Errorf("delete supplier bank account: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return models.ErrNotFound
	}
	return nil
}

func verifyPaymentBankAccount(ctx context.Context, q bankQuerier, supplierID uuid.UUID, accountID *uuid.UUID, allowInactive bool) error {
	if accountID == nil {
		return nil
	}

	var active bool
	err := q.QueryRow(ctx,
		`SELECT is_active FROM supplier_bank_accounts WHERE id = $1 AND supplier_id = $2`,
		*accountID, supplierID,
	).Scan(&active)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.NewValidationError("the selected bank account does not belong to this supplier")
	}
	if err != nil {
		return fmt.Errorf("load bank account: %w", err)
	}
	if !active && !allowInactive {
		return models.NewValidationError("the selected bank account is inactive")
	}
	return nil
}
