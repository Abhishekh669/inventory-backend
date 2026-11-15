package repository

import (
	"context"
	"fmt"
	"inventory/internals/database"
	"inventory/internals/models"
	"log"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SupplierPaymentRepository interface {
	CreateAdvancePaymentForSupplier(ctx context.Context, data *models.SupplierPaymentCreationType, userID uuid.UUID) (uuid.UUID, error)
}

type supplierPaymentRepository struct {
	pool *pgxpool.Pool
}

func NewSupplierPaymentRepository() SupplierPaymentRepository {
	pool, err := database.GetPostgresPool()
	if err != nil {
		log.Println("Failed to initialize pool:", err)
		return nil
	}

	return &supplierPaymentRepository{pool: pool}
}
func (r *supplierPaymentRepository) GetAllPaymentsSummaries(ctx context.Context) {}
func (r *supplierPaymentRepository) GetAllPayments(ctx context.Context)          {}
func (r *supplierPaymentRepository) CreateAdvancePaymentForSupplier(ctx context.Context, data *models.SupplierPaymentCreationType, userID uuid.UUID) (uuid.UUID, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("begin advance payment transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Increase the advance we hold with the supplier. Runs first so an
	// unknown supplier fails early and the row is locked until commit.
	tag, err := tx.Exec(ctx, `
		UPDATE suppliers
		SET credit_balance = credit_balance + $1
		WHERE id = $2
	`, data.Amount, data.SupplierID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("update supplier credit balance: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return uuid.Nil, ErrSupplierNotFound
	}

	// 2. Insert the advance payment
	var paymentID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO supplier_payments (
			supplier_id, amount, payment_date,
			payment_method, payment_type, note, created_by
		)
		VALUES ($1, $2, COALESCE($3::date, CURRENT_DATE), $4, 'advance', $5, $6)
		RETURNING id
	`,
		data.SupplierID,
		data.Amount,
		data.PaymentDate,
		string(data.PaymentMethod),
		data.Note,
		userID,
	).Scan(&paymentID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert advance payment: %w", err)
	}

	// 3. Update the financial summary. An advance is fully unallocated.
	_, err = tx.Exec(ctx, `
		INSERT INTO supplier_financial_summaries (
			supplier_id, total_payments, unallocated_payments
		)
		VALUES ($1, $2, $2)
		ON CONFLICT (supplier_id)
		DO UPDATE SET
			total_payments       = supplier_financial_summaries.total_payments + EXCLUDED.total_payments,
			unallocated_payments = supplier_financial_summaries.unallocated_payments + EXCLUDED.unallocated_payments,
			updated_at           = NOW()
	`, data.SupplierID, data.Amount)
	if err != nil {
		return uuid.Nil, fmt.Errorf("update supplier financial summary: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, fmt.Errorf("commit advance payment transaction: %w", err)
	}

	return paymentID, nil
}
