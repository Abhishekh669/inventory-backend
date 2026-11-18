// NEW FILE: internals/repository/stock_movement.go
package repository

import (
	"context"
	"fmt"
	"inventory/internals/database"
	"inventory/internals/models"
	"log"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type StockMovementRepository interface {
	GetAllStockMovements(ctx context.Context, f models.StockMovementListFilter) ([]models.StockMovementItem, int, error)
}

type stockMovementRepository struct {
	pool *pgxpool.Pool
}

func NewStockMovementRepository() StockMovementRepository {
	pool, err := database.GetPostgresPool()
	if err != nil {
		log.Println("Failed to initialize pool:", err)
		return nil
	}
	return &stockMovementRepository{pool: pool}
}

func (r *stockMovementRepository) GetAllStockMovements(ctx context.Context, f models.StockMovementListFilter) ([]models.StockMovementItem, int, error) {
	conds := []string{"TRUE"}
	args := []any{}

	if f.ProductID != nil {
		args = append(args, *f.ProductID)
		conds = append(conds, fmt.Sprintf("sm.product_id = $%d", len(args)))
	}
	if f.Type != nil {
		args = append(args, string(*f.Type))
		conds = append(conds, fmt.Sprintf("sm.type = $%d", len(args)))
	}
	if f.FromDate != nil {
		args = append(args, f.FromDate.Format("2006-01-02"))
		conds = append(conds, fmt.Sprintf("sm.movement_date >= $%d::date", len(args)))
	}
	if f.ToDate != nil {
		args = append(args, f.ToDate.Format("2006-01-02"))
		conds = append(conds, fmt.Sprintf("sm.movement_date <= $%d::date", len(args)))
	}
	filterSQL := strings.Join(conds, " AND ")

	const fromSQL = `
		FROM stock_movements sm
		JOIN products p ON p.id = sm.product_id
		LEFT JOIN users u ON u.id = sm.user_id
	`

	// the count ignores the cursor, so the total is the same on every page
	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) "+fromSQL+" WHERE "+filterSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count stock movements: %w", err)
	}
	if total == 0 {
		return []models.StockMovementItem{}, 0, nil
	}

	listSQL := filterSQL
	listArgs := append([]any{}, args...)
	if f.Cursor != nil {
		listArgs = append(listArgs, f.Cursor.Date, f.Cursor.CreatedAt, f.Cursor.ID)
		n := len(listArgs)
		listSQL += fmt.Sprintf(
			" AND (sm.movement_date, sm.created_at, sm.id) < ($%d::date, $%d::timestamptz, $%d::uuid)", n-2, n-1, n)
	}
	listArgs = append(listArgs, f.Limit+1) // one extra row tells us if there is a next page

	query := fmt.Sprintf(`
		SELECT sm.id, sm.product_id, p.name, p.sku, sm.type, sm.qty_change, sm.balance_after,
		       sm.ref_type, sm.ref_id, sm.note, sm.user_id, u.name,
		       to_char(sm.movement_date, 'YYYY-MM-DD'), sm.created_at
		%s
		WHERE %s
		ORDER BY sm.movement_date DESC, sm.created_at DESC, sm.id DESC
		LIMIT $%d`, fromSQL, listSQL, len(listArgs))

	rows, err := r.pool.Query(ctx, query, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query stock movements: %w", err)
	}
	defer rows.Close()

	items := make([]models.StockMovementItem, 0, f.Limit+1)
	for rows.Next() {
		var it models.StockMovementItem
		if err := rows.Scan(
			&it.ID, &it.ProductID, &it.ProductName, &it.ProductSKU, &it.Type, &it.QtyChange, &it.BalanceAfter,
			&it.RefType, &it.RefID, &it.Note, &it.UserID, &it.UserName,
			&it.MovementDate, &it.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan stock movement: %w", err)
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("read stock movements: %w", err)
	}
	return items, total, nil
}
