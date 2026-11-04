package repository

import (
	"context"
	"errors"
	"fmt"
	"inventory/internals/database"
	"inventory/internals/models"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProductRepo interface {
	GetProductsByCategory(
		ctx context.Context,
		slug string,
		limit, page int,
		toDate, fromDate *time.Time,
		name *string,
		currentStock *int,
		greaterThan *bool,
	) (*models.ProductsListTypeResponse, error)

	GetProducts(
		ctx context.Context,
		limit int,
		page int,
		toDate *time.Time,
		fromDate *time.Time,
		name *string,
		currentStock *int,
		greaterThan *bool,
	) (*models.ProductsListTypeResponse, error)

	CreateNewProduct(
		ctx context.Context,
		newProducts *[]models.CreateProductRequestType,
		userId uuid.UUID,
	) error

	UpdateNewProduct(
		ctx context.Context,
		updatedProduct *models.UpdateProductRequestType,
	) error

	DeleteNewProducts(
		ctx context.Context,
		ids []uuid.UUID,
	) error
}
type productRepo struct {
	pool *pgxpool.Pool
}

var (
	ErrProductNotFound       = errors.New("product not found")
	ErrProductSKUExists      = errors.New("a product with this sku already exists")
	ErrInvalidCategoryOrUnit = errors.New("invalid category or unit")
)

func (r *productRepo) CreateNewProduct(ctx context.Context, newProducts *[]models.CreateProductRequestType, userId uuid.UUID) error {
	if newProducts == nil || len(*newProducts) == 0 {
		return fmt.Errorf("no products provided")
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	const insertProduct = `
        INSERT INTO products (
            sku, name, category_id, unit_id,
            vat_rate, cost_price, cost_with_vat,
            margin_percent, selling_price,
            re_order_level, current_stock, is_active
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
        RETURNING id
    `

	const insertPriceHistory = `
        INSERT INTO product_price_history (
            product_id, purchase_item_id,
            unit_cost, unit_cost_with_vat,
            margin_percent, selling_price
        )
        VALUES ($1, NULL, $2, $3, $4, $5)
    `

	// Skipped when opening stock is 0, because stock_movements requires qty_change <> 0.
	const insertOpeningStock = `
        INSERT INTO stock_movements (
            product_id, type, qty_change, balance_after,
            ref_type, ref_id, note, user_id
        )
        SELECT $1::uuid, 'opening', $2::numeric, $2::numeric,
               NULL, NULL, 'Opening stock', $3::uuid
        WHERE $2::numeric <> 0
    `

	var movementUser uuid.UUID
	if userId != uuid.Nil {
		movementUser = userId
	}

	for i, p := range *newProducts {
		if strings.TrimSpace(p.SKU) == "" || strings.TrimSpace(p.Name) == "" {
			return fmt.Errorf("product at index %d: sku and name are required", i)
		}

		var productID string
		err := tx.QueryRow(ctx, insertProduct,
			strings.TrimSpace(p.SKU), strings.TrimSpace(p.Name),
			p.CategoryID, p.UnitID,
			p.VatRate, p.CostPrice, p.CostWithVat,
			p.MarginPercent, p.SellingPrice,
			p.ReOrderLevel, p.CurrentStock, p.IsActive,
		).Scan(&productID)
		if err != nil {
			if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
				switch pgErr.Code {
				case "23505":
					return fmt.Errorf("product at index %d: %w (sku: %q)", i, ErrProductSKUExists, p.SKU)
				case "23503":
					return fmt.Errorf("product at index %d: %w", i, ErrInvalidCategoryOrUnit)
				}
			}
			return fmt.Errorf("failed to insert product at index %d: %w", i, err)
		}

		if _, err = tx.Exec(ctx, insertPriceHistory,
			productID,
			p.CostPrice, p.CostWithVat,
			p.MarginPercent, p.SellingPrice,
		); err != nil {
			return fmt.Errorf("failed to insert price history for product at index %d: %w", i, err)
		}

		if _, err = tx.Exec(ctx, insertOpeningStock,
			productID, p.CurrentStock, movementUser,
		); err != nil {
			return fmt.Errorf("failed to insert opening stock for product at index %d: %w", i, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r *productRepo) UpdateNewProduct(ctx context.Context, updatedProduct *models.UpdateProductRequestType) error {
	if updatedProduct == nil {
		return fmt.Errorf("no product provided")
	}

	sku := strings.TrimSpace(updatedProduct.SKU)
	name := strings.TrimSpace(updatedProduct.Name)
	if sku == "" || name == "" {
		return fmt.Errorf("sku and name are required")
	}

	result, err := r.pool.Exec(ctx, `
		UPDATE products SET
			sku             = $2,
			name            = $3,
			category_id     = $4,
			unit_id         = $5,
			vat_rate        = $6,
			cost_price      = $7,
			cost_with_vat   = $8,
			margin_percent  = $9,
			selling_price   = $10,
			re_order_level  = $11,
			current_stock   = $12,
			is_active       = $13,
			updated_at      = now()
		WHERE id = $1
	`,
		updatedProduct.ID,
		sku, name,
		updatedProduct.CategoryID, updatedProduct.UnitID,
		updatedProduct.VatRate,
		updatedProduct.CostPrice, updatedProduct.CostWithVat,
		updatedProduct.MarginPercent, updatedProduct.SellingPrice,
		updatedProduct.ReOrderLevel, updatedProduct.CurrentStock,
		updatedProduct.IsActive,
	)
	if err != nil {
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
			switch pgErr.Code {
			case "23505": // unique_violation
				return fmt.Errorf("%w (sku: %q)", ErrProductSKUExists, sku)
			case "23503": // foreign_key_violation
				return ErrInvalidCategoryOrUnit
			}
		}
		return fmt.Errorf("failed to update product: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrProductNotFound
	}

	return nil
}
func (r *productRepo) DeleteNewProducts(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return fmt.Errorf("no product ids provided")
	}

	idStrings := make([]string, len(ids))
	for i, id := range ids {
		idStrings[i] = id.String()
	}

	result, err := r.pool.Exec(ctx, `
		UPDATE products
		SET is_active = FALSE,
		    updated_at = now()
		WHERE id = ANY($1::uuid[])
		  AND is_active = TRUE
	`, idStrings)
	if err != nil {
		return fmt.Errorf("failed to delete products: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrProductNotFound
	}
	return nil
}

func (r *productRepo) GetProductsByCategory(
	ctx context.Context,
	slug string,
	limit int,
	page int,
	toDate *time.Time,
	fromDate *time.Time,
	name *string,
	currentStock *int,
	greaterThan *bool,
) (*models.ProductsListTypeResponse, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, fmt.Errorf("category slug is required")
	}

	if limit <= 0 {
		limit = 20
	}
	if page < 0 {
		page = 0
	}
	offset := page * limit

	// Slug is compulsory, so it is always $1
	where := " WHERE c.slug = $1"
	args := []interface{}{slug}
	argCounter := 2

	if fromDate != nil {
		where += fmt.Sprintf(" AND p.created_at >= $%d", argCounter)
		args = append(args, *fromDate)
		argCounter++
	}

	if toDate != nil {
		where += fmt.Sprintf(" AND p.created_at <= $%d", argCounter)
		args = append(args, *toDate)
		argCounter++
	}

	if name != nil {
		trimmed := strings.TrimSpace(*name)
		if trimmed != "" {
			where += fmt.Sprintf(" AND (p.name ILIKE $%d OR p.sku ILIKE $%d)", argCounter, argCounter)
			args = append(args, "%"+trimmed+"%")
			argCounter++
		}
	}

	if currentStock != nil {
		op := "<="
		if greaterThan != nil && *greaterThan {
			op = ">="
		}
		where += fmt.Sprintf(" AND p.current_stock %s $%d", op, argCounter)
		args = append(args, *currentStock)
		argCounter++
	}

	// Count query (needs the categories join for the slug filter)
	var total int
	countQuery := `
		SELECT COUNT(*)
		FROM products p
		JOIN categories c ON c.id = p.category_id
	` + where
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to get total count: %w", err)
	}

	dataQuery := fmt.Sprintf(`
		SELECT
			p.id, p.sku, p.name,
			p.category_id, c.name AS category_name, c.slug AS category_slug,
			p.unit_id, u.name AS unit_name, u.allow_decimal,
			p.vat_rate, p.cost_price, p.cost_with_vat,
			p.margin_percent, p.selling_price, p.profit_per_unit,
			p.re_order_level, p.current_stock, p.is_active,
			p.created_at, p.updated_at
		FROM products p
		JOIN categories c ON c.id = p.category_id
		JOIN units u ON u.id = p.unit_id
		%s
		ORDER BY p.created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argCounter, argCounter+1)

	dataArgs := make([]interface{}, 0, len(args)+2)
	dataArgs = append(dataArgs, args...)
	dataArgs = append(dataArgs, limit, offset)

	rows, err := r.pool.Query(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch products by category: %w", err)
	}

	products, err := pgx.CollectRows(rows, pgx.RowToStructByName[models.ProductsListType])
	if err != nil {
		return nil, fmt.Errorf("failed to scan products: %w", err)
	}
	if products == nil {
		products = []models.ProductsListType{}
	}

	return &models.ProductsListTypeResponse{
		Products:   products,
		Total:      total,
		HasMore:    (page+1)*limit < total,
		NextOffset: page + 1,
	}, nil
}

func (r *productRepo) GetProducts(
	ctx context.Context,
	limit int,
	page int,
	toDate *time.Time,
	fromDate *time.Time,
	name *string,
	currentStock *int,
	greaterThan *bool,
) (*models.ProductsListTypeResponse, error) {
	if limit <= 0 {
		limit = 20
	}
	if page < 0 {
		page = 0
	}
	offset := page * limit

	// Build shared WHERE clause (all filters optional)
	where := " WHERE 1=1"
	args := []interface{}{}
	argCounter := 1

	if fromDate != nil {
		where += fmt.Sprintf(" AND p.created_at >= $%d", argCounter)
		args = append(args, *fromDate)
		argCounter++
	}

	if toDate != nil {
		where += fmt.Sprintf(" AND p.created_at <= $%d", argCounter)
		args = append(args, *toDate)
		argCounter++
	}

	if name != nil {
		trimmed := strings.TrimSpace(*name)
		if trimmed != "" {
			where += fmt.Sprintf(" AND (p.name ILIKE $%d OR p.sku ILIKE $%d)", argCounter, argCounter)
			args = append(args, "%"+trimmed+"%")
			argCounter++
		}
	}

	// Stock filter: only applies when a stock value is given
	if currentStock != nil {
		op := "<=" // default when greaterThan is not provided
		if greaterThan != nil && *greaterThan {
			op = ">="
		}
		where += fmt.Sprintf(" AND p.current_stock %s $%d", op, argCounter)
		args = append(args, *currentStock)
		argCounter++
	}

	// Count query
	var total int
	countQuery := "SELECT COUNT(*) FROM products p" + where
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to get total count: %w", err)
	}

	// Data query
	dataQuery := fmt.Sprintf(`
		SELECT
			p.id, p.sku, p.name,
			p.category_id, c.name AS category_name, c.slug AS category_slug,
			p.unit_id, u.name AS unit_name, u.allow_decimal,
			p.vat_rate, p.cost_price, p.cost_with_vat,
			p.margin_percent, p.selling_price, p.profit_per_unit,
			p.re_order_level, p.current_stock, p.is_active,
			p.created_at, p.updated_at
		FROM products p
		JOIN categories c ON c.id = p.category_id
		JOIN units u ON u.id = p.unit_id
		%s
		ORDER BY p.created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argCounter, argCounter+1)

	dataArgs := make([]interface{}, 0, len(args)+2)
	dataArgs = append(dataArgs, args...)
	dataArgs = append(dataArgs, limit, offset)

	rows, err := r.pool.Query(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch products: %w", err)
	}

	products, err := pgx.CollectRows(rows, pgx.RowToStructByName[models.ProductsListType])
	if err != nil {
		return nil, fmt.Errorf("failed to scan products: %w", err)
	}
	if products == nil {
		products = []models.ProductsListType{}
	}

	return &models.ProductsListTypeResponse{
		Products:   products,
		Total:      total,
		HasMore:    (page+1)*limit < total,
		NextOffset: page + 1,
	}, nil
}

func NewProductRepository() ProductRepo {
	pool, err := database.GetPostgresPool()
	if err != nil {
		log.Println("Failed to initialize pool pool:", err)
		return nil
	}

	return &productRepo{
		pool: pool,
	}
}
