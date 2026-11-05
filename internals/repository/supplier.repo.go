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

var ErrSupplierNotFound = errors.New("supplier not found")
var ErrSupplierInUse = errors.New("one or more suppliers have purchases or payments; deactivate them instead")

type SupplierRepo interface {
	CreateSuppliers(ctx context.Context, in []models.CreateSupplierInput) ([]models.Supplier, error)
	UpdateSupplier(ctx context.Context, id uuid.UUID, in models.UpdateSupplierInput) (*models.Supplier, error)
	GetSupplierById(ctx context.Context, id uuid.UUID) (*models.Supplier, error)
	GetSuppliers(ctx context.Context) ([]models.Supplier, error)
	DeleteSuppliers(ctx context.Context, ids []uuid.UUID) (int64, error)
}

type supplierRepo struct {
	pool *pgxpool.Pool
}

const supplierColumns = `id, name, contact_person, phone, address, pan_no,
	opening_balance, balance, is_active, created_at`

func scanSupplier(row pgx.Row) (*models.Supplier, error) {
	var s models.Supplier
	err := row.Scan(
		&s.ID, &s.Name, &s.ContactPerson, &s.Phone, &s.Address, &s.PanNo,
		&s.OpeningBalance, &s.Balance, &s.IsActive, &s.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSupplierNotFound
		}
		return nil, err
	}
	return &s, nil
}

func (r *supplierRepo) CreateSuppliers(ctx context.Context, inputs []models.CreateSupplierInput) ([]models.Supplier, error) {
	if len(inputs) == 0 {
		return []models.Supplier{}, nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("create suppliers: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after a successful commit

	// the starting balance equals the opening balance
	query := `
		INSERT INTO suppliers
			(name, contact_person, phone, address, pan_no, opening_balance, balance)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
		RETURNING ` + supplierColumns

	batch := &pgx.Batch{}
	for _, in := range inputs {
		batch.Queue(query, in.Name, in.ContactPerson, in.Phone, in.Address, in.PanNo, in.OpeningBalance)
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

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("create suppliers: commit: %w", err)
	}
	return created, nil
}

func (r *supplierRepo) UpdateSupplier(ctx context.Context, id uuid.UUID, in models.UpdateSupplierInput) (*models.Supplier, error) {
	// COALESCE keeps the old value when the field is nil.
	// If opening_balance changes, balance shifts by the same difference.
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

	s, err := scanSupplier(r.pool.QueryRow(ctx, query,
		id, in.Name, in.ContactPerson, in.Phone, in.Address, in.PanNo,
		in.OpeningBalance, in.IsActive,
	))
	if err != nil {
		if errors.Is(err, ErrSupplierNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("update supplier: %w", err)
	}
	return s, nil
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
