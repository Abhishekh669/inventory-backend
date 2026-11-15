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
	"github.com/shopspring/decimal"
)

var ErrSupplierNotFound = errors.New("supplier not found")
var ErrSupplierInUse = errors.New("one or more suppliers have purchases or payments; deactivate them instead")

var (
	ErrNegativeCreditBalance = errors.New("credit balance cannot be negative")
	ErrCreditOutOfSync       = errors.New("credit balance does not match advance payments")
)

type SupplierRepo interface {
	CreateSuppliers(ctx context.Context, in []models.CreateSupplierInput, id uuid.UUID) ([]models.Supplier, error)
	UpdateSupplier(ctx context.Context, id uuid.UUID, in models.UpdateSupplierInput, userId uuid.UUID) (*models.Supplier, error)
	GetSupplierById(ctx context.Context, id uuid.UUID) (*models.Supplier, error)
	GetSuppliers(ctx context.Context) ([]models.Supplier, error)
	DeleteSuppliers(ctx context.Context, ids []uuid.UUID) (int64, error)
}

type supplierRepo struct {
	pool *pgxpool.Pool
}

const supplierColumns = `id, name, contact_person, phone, address, pan_no,
	opening_balance, balance, credit_balance, is_active, created_at`

func scanSupplier(row pgx.Row) (*models.Supplier, error) {
	var s models.Supplier
	err := row.Scan(
		&s.ID, &s.Name, &s.ContactPerson, &s.Phone, &s.Address, &s.PanNo,
		&s.OpeningBalance, &s.Balance, &s.CreditBalance, &s.IsActive, &s.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSupplierNotFound
		}
		return nil, err
	}
	return &s, nil
}
func (r *supplierRepo) CreateSuppliers(
	ctx context.Context,
	inputs []models.CreateSupplierInput,
	userID uuid.UUID,
) ([]models.Supplier, error) {
	if len(inputs) == 0 {
		return []models.Supplier{}, nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("create suppliers: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after a successful commit

	// 1. Insert suppliers. The starting balance equals the opening balance.
	query := `
		INSERT INTO suppliers
			(name, contact_person, phone, address, pan_no, opening_balance, balance, credit_balance)
		VALUES ($1, $2, $3, $4, $5, $6, $6, $7)
		RETURNING ` + supplierColumns

	batch := &pgx.Batch{}
	for _, in := range inputs {
		batch.Queue(query, in.Name, in.ContactPerson, in.Phone, in.Address, in.PanNo, in.OpeningBalance, in.CreditBalance)
	}

	br := tx.SendBatch(ctx, batch)
	created := make([]models.Supplier, 0, len(inputs))
	for i := range inputs {
		s, err := scanSupplier(br.QueryRow())
		if err != nil {
			br.Close()
			return nil, fmt.Errorf("create supplier #%d (%s): %w", i+1, inputs[i].Name, err)
		}
		created = append(created, *s)
	}
	if err := br.Close(); err != nil {
		return nil, fmt.Errorf("create suppliers: %w", err)
	}

	// 2. For every supplier that starts with an advance: record the advance
	// payment and mirror it in the financial summary. Every supplier also
	// gets a summary row (zeros when there is no advance).
	const paymentQuery = `
		INSERT INTO supplier_payments (
			supplier_id, amount, payment_date,
			payment_method, payment_type, note, created_by
		)
		VALUES ($1, $2, CURRENT_DATE, 'cash', 'advance', $3, $4)
	`
	const summaryQuery = `
		INSERT INTO supplier_financial_summaries (
			supplier_id, total_payments, unallocated_payments
		)
		VALUES ($1, $2, $2)
		ON CONFLICT (supplier_id) DO NOTHING
	`

	followUp := &pgx.Batch{}
	queued := 0

	for i, s := range created {
		credit := inputs[i].CreditBalance // same order as created

		if credit.IsPositive() {
			followUp.Queue(paymentQuery, s.ID, credit, "Opening advance (added when supplier was created)", userID)
			queued++
		}
		followUp.Queue(summaryQuery, s.ID, credit)
		queued++
	}

	fr := tx.SendBatch(ctx, followUp)
	for i := 0; i < queued; i++ {
		if _, err := fr.Exec(); err != nil {
			fr.Close()
			return nil, fmt.Errorf("create suppliers: record opening advance: %w", err)
		}
	}
	if err := fr.Close(); err != nil {
		return nil, fmt.Errorf("create suppliers: record opening advance: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("create suppliers: commit: %w", err)
	}
	return created, nil
}

func (r *supplierRepo) UpdateSupplier(
	ctx context.Context,
	id uuid.UUID,
	in models.UpdateSupplierInput,
	userID uuid.UUID,
) (*models.Supplier, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("update supplier: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Lock the supplier and read the current advance held.
	var currentCredit decimal.Decimal
	err = tx.QueryRow(ctx,
		`SELECT credit_balance FROM suppliers WHERE id = $1 FOR UPDATE`, id,
	).Scan(&currentCredit)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSupplierNotFound
		}
		return nil, fmt.Errorf("update supplier: load: %w", err)
	}

	// 2. Credit balance sent AND different? Then fix the payment records.
	//    Same or not sent? Skip all of this.
	if in.CreditBalance != nil && !in.CreditBalance.Equal(currentCredit) {
		if in.CreditBalance.IsNegative() {
			return nil, ErrNegativeCreditBalance
		}
		diff := in.CreditBalance.Sub(currentCredit)
		if err := adjustAdvanceTx(ctx, tx, id, diff, userID); err != nil {
			return nil, err
		}
	}

	// 3. Profile fields (credit_balance is handled in step 2).
	query := `
		UPDATE suppliers SET
			name            = COALESCE($2, name),
			contact_person  = COALESCE($3, contact_person),
			phone           = COALESCE($4, phone),
			address         = COALESCE($5, address),
			pan_no          = COALESCE($6, pan_no),
			balance         = balance + (COALESCE($7::numeric, opening_balance) - opening_balance),
			opening_balance = COALESCE($7::numeric, opening_balance),
			is_active       = COALESCE($8, is_active)
		WHERE id = $1
		RETURNING ` + supplierColumns

	s, err := scanSupplier(tx.QueryRow(ctx, query,
		id, in.Name, in.ContactPerson, in.Phone, in.Address, in.PanNo,
		in.OpeningBalance, in.IsActive,
	))
	if err != nil {
		if errors.Is(err, ErrSupplierNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("update supplier: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("update supplier: commit: %w", err)
	}
	return s, nil
}

// adjustAdvanceTx changes the advance held by diff (positive or negative).
func adjustAdvanceTx(
	ctx context.Context,
	tx pgx.Tx,
	supplierID uuid.UUID,
	diff decimal.Decimal,
	userID uuid.UUID,
) error {
	if diff.IsPositive() {
		// Up: one new advance payment for the extra amount.
		if _, err := tx.Exec(ctx, `
			INSERT INTO supplier_payments
				(supplier_id, amount, payment_date, payment_method, payment_type, note, created_by)
			VALUES ($1, $2, CURRENT_DATE, 'cash', 'advance', $3, $4)
		`, supplierID, diff, "Advance added by editing supplier credit balance", userID); err != nil {
			return fmt.Errorf("insert advance payment: %w", err)
		}
	} else {
		// Down: take the money from the newest advance with unused money, repeat if needed.
		remaining := diff.Neg()

		for remaining.IsPositive() {
			var (
				paymentID   uuid.UUID
				amount      decimal.Decimal
				unallocated decimal.Decimal
			)
			err := tx.QueryRow(ctx, `
				SELECT sp.id, sp.amount,
				       sp.amount - COALESCE((SELECT SUM(a.amount)
				                             FROM purchase_payment_allocations a
				                             WHERE a.supplier_payment_id = sp.id), 0)
				FROM supplier_payments sp
				WHERE sp.supplier_id = $1
				  AND sp.payment_type = 'advance'
				  AND sp.amount > COALESCE((SELECT SUM(a.amount)
				                            FROM purchase_payment_allocations a
				                            WHERE a.supplier_payment_id = sp.id), 0)
				ORDER BY sp.payment_date DESC, sp.created_at DESC, sp.id DESC
				LIMIT 1
				FOR UPDATE OF sp
			`, supplierID).Scan(&paymentID, &amount, &unallocated)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return ErrCreditOutOfSync // nothing left to take from
				}
				return fmt.Errorf("find advance payment: %w", err)
			}

			cut := decimal.Min(unallocated, remaining)

			if cut.Equal(amount) {
				// Nothing from this payment was used, so remove it entirely.
				_, err = tx.Exec(ctx, `DELETE FROM supplier_payments WHERE id = $1`, paymentID)
			} else {
				_, err = tx.Exec(ctx, `UPDATE supplier_payments SET amount = amount - $2 WHERE id = $1`, paymentID, cut)
			}
			if err != nil {
				return fmt.Errorf("reduce advance payment: %w", err)
			}

			remaining = remaining.Sub(cut)
		}
	}

	// Same difference everywhere else.
	if _, err := tx.Exec(ctx,
		`UPDATE suppliers SET credit_balance = credit_balance + $1 WHERE id = $2`, diff, supplierID,
	); err != nil {
		return fmt.Errorf("update credit balance: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO supplier_financial_summaries (supplier_id, total_payments, unallocated_payments)
		VALUES ($1, GREATEST($2, 0), GREATEST($2, 0))
		ON CONFLICT (supplier_id) DO UPDATE SET
			total_payments       = supplier_financial_summaries.total_payments + $2,
			unallocated_payments = supplier_financial_summaries.unallocated_payments + $2,
			updated_at           = NOW()
	`, supplierID, diff); err != nil {
		return fmt.Errorf("update summary: %w", err)
	}

	return nil
}

func (r *supplierRepo) GetSupplierById(ctx context.Context, id uuid.UUID) (*models.Supplier, error) {
	query := `SELECT ` + supplierColumns + ` FROM suppliers WHERE id = $1`
	return scanSupplier(r.pool.QueryRow(ctx, query, id))
}
func (r *supplierRepo) GetSuppliers(ctx context.Context) ([]models.Supplier, error) {
	query := `SELECT ` + supplierColumns + ` FROM suppliers ORDER BY name`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get suppliers: %w", err)
	}
	defer rows.Close()

	suppliers := []models.Supplier{}
	for rows.Next() {
		s, err := scanSupplier(rows)
		if err != nil {
			return nil, fmt.Errorf("scan supplier: %w", err)
		}
		suppliers = append(suppliers, *s)
	}
	return suppliers, rows.Err()
}

// DeleteSuppliers deletes all given ids in one statement.
// Returns how many rows were deleted. If any supplier is referenced by
// purchases/payments (FK RESTRICT), nothing is deleted.
func (r *supplierRepo) DeleteSuppliers(ctx context.Context, ids []uuid.UUID) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}

	tag, err := r.pool.Exec(ctx, `DELETE FROM suppliers WHERE id = ANY($1::uuid[])`, ids)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign_key_violation
			return 0, ErrSupplierInUse
		}
		return 0, fmt.Errorf("delete suppliers: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return 0, ErrSupplierNotFound
	}
	return tag.RowsAffected(), nil
}

func NewSupplierRepository() SupplierRepo {
	pool, err := database.GetPostgresPool()
	if err != nil {
		log.Println("Failed to initialize pool:", err)
		return nil
	}
	return &supplierRepo{pool: pool}
}
