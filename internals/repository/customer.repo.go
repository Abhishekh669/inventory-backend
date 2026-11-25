// NEW FILE: internals/repository/customer.go
package repository

import (
	"context"
	"errors"
	"fmt"
	"inventory/internals/database"
	"inventory/internals/models"
	"log"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CustomerRepository interface {
	GetAllCustomers(ctx context.Context, f models.CustomerListFilter) ([]models.Customer, int, error)
	CreateCustomer(ctx context.Context, req *models.CreateCustomerRequest) (*models.Customer, error)
	UpdateCustomer(ctx context.Context, customerID uuid.UUID, req *models.UpdateCustomerRequest) (*models.Customer, error)
	DeleteCustomer(ctx context.Context, customerID uuid.UUID) error
}

type customerRepository struct {
	pool *pgxpool.Pool
}

func NewCustomerRepository() CustomerRepository {
	pool, err := database.GetPostgresPool()
	if err != nil {
		log.Println("Failed to initialize pool:", err)
		return nil
	}
	return &customerRepository{pool: pool}
}

const (
	customerColumns = `id, name, contact_person, phone, email, address, pan_no,
		opening_balance, balance, credit_balance, is_active, created_at`
	customerColumnsC = `c.id, c.name, c.contact_person, c.phone, c.email, c.address, c.pan_no,
		c.opening_balance, c.balance, c.credit_balance, c.is_active, c.created_at`

	duplicateCustomerPanMsg = "a customer with that PAN number already exists"
)

// works for a single row and for rows.Next()
type customerRowScanner interface {
	Scan(dest ...any) error
}

func scanCustomer(row customerRowScanner) (models.Customer, error) {
	var c models.Customer
	err := row.Scan(&c.ID, &c.Name, &c.ContactPerson, &c.Phone, &c.Email, &c.Address, &c.PanNo,
		&c.OpeningBalance, &c.Balance, &c.CreditBalance, &c.IsActive, &c.CreatedAt)
	return c, err
}

func customerPgCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

var customerBalanceOps = map[models.BalanceOp]string{
	models.BalanceGT:  ">",
	models.BalanceGTE: ">=",
	models.BalanceLT:  "<",
	models.BalanceLTE: "<=",
	models.BalanceEQ:  "=",
}

func (r *customerRepository) GetAllCustomers(ctx context.Context, f models.CustomerListFilter) ([]models.Customer, int, error) {
	conds := []string{"TRUE"}
	args := []any{}

	// one search box for name, phone and email
	if f.Search != nil {
		args = append(args, "%"+escapeLike(*f.Search)+"%")
		conds = append(conds, fmt.Sprintf(
			"(c.name ILIKE $%[1]d OR c.phone ILIKE $%[1]d OR c.email ILIKE $%[1]d)", len(args)))
	}

	// balance filter: how much the customer still has to pay (more than / less than an amount)
	if f.BalanceOp != nil && f.BalanceAmount != nil {
		sqlOp, ok := customerBalanceOps[*f.BalanceOp]
		if !ok {
			return nil, 0, models.NewValidationError("unknown balance filter")
		}
		args = append(args, f.BalanceAmount.String())
		conds = append(conds, fmt.Sprintf("c.balance %s $%d::numeric", sqlOp, len(args)))
	}

	filterSQL := strings.Join(conds, " AND ")

	// the count ignores the cursor, so the total is the same on every page
	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM customers c WHERE "+filterSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count customers: %w", err)
	}
	if total == 0 {
		return []models.Customer{}, 0, nil
	}

	listSQL := filterSQL
	listArgs := append([]any{}, args...)
	if f.Cursor != nil {
		listArgs = append(listArgs, f.Cursor.CreatedAt, f.Cursor.ID)
		n := len(listArgs)
		listSQL += fmt.Sprintf(" AND (c.created_at, c.id) < ($%d::timestamptz, $%d::uuid)", n-1, n)
	}
	listArgs = append(listArgs, f.Limit+1) // one extra row tells us if there is a next page

	query := fmt.Sprintf(`
		SELECT %s
		FROM customers c
		WHERE %s
		ORDER BY c.created_at DESC, c.id DESC
		LIMIT $%d`, customerColumnsC, listSQL, len(listArgs))

	rows, err := r.pool.Query(ctx, query, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query customers: %w", err)
	}
	defer rows.Close()

	items := make([]models.Customer, 0, f.Limit+1)
	for rows.Next() {
		c, err := scanCustomer(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan customer: %w", err)
		}
		items = append(items, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("read customers: %w", err)
	}
	return items, total, nil
}

func (r *customerRepository) CreateCustomer(ctx context.Context, req *models.CreateCustomerRequest) (*models.Customer, error) {
	// the opening balance is what the customer already owes us, so it is also the starting balance
	c, err := scanCustomer(r.pool.QueryRow(ctx, `
		INSERT INTO customers (name, contact_person, phone, email, address, pan_no, opening_balance, balance)
		VALUES ($1, $2, $3, $4, $5, $6, $7::numeric, $7::numeric)
		RETURNING `+customerColumns,
		req.Name, req.ContactPerson, req.Phone, req.Email, req.Address, req.PanNo,
		req.OpeningBalance.String(),
	))
	if err != nil {
		if customerPgCode(err) == "23505" {
			return nil, models.NewValidationError(duplicateCustomerPanMsg)
		}
		return nil, fmt.Errorf("insert customer: %w", err)
	}
	return &c, nil
}

func (r *customerRepository) UpdateCustomer(ctx context.Context, customerID uuid.UUID, req *models.UpdateCustomerRequest) (*models.Customer, error) {
	c, err := scanCustomer(r.pool.QueryRow(ctx, `
		UPDATE customers
		SET name = $2, contact_person = $3, phone = $4, email = $5, address = $6, pan_no = $7,
		    is_active = COALESCE($8, is_active)
		WHERE id = $1
		RETURNING `+customerColumns,
		customerID, req.Name, req.ContactPerson, req.Phone, req.Email, req.Address, req.PanNo, req.IsActive,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, models.ErrNotFound
	}
	if err != nil {
		if customerPgCode(err) == "23505" {
			return nil, models.NewValidationError(duplicateCustomerPanMsg)
		}
		return nil, fmt.Errorf("update customer: %w", err)
	}
	return &c, nil
}

func (r *customerRepository) DeleteCustomer(ctx context.Context, customerID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM customers WHERE id = $1`, customerID)
	if err != nil {
		// sales, returns and payments reference the customer, so one with history cannot be removed
		if customerPgCode(err) == "23503" {
			return models.NewValidationError(
				"this customer has sales or payments, so it cannot be deleted. Mark it inactive instead")
		}
		return fmt.Errorf("delete customer: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return models.ErrNotFound
	}
	return nil
}