package repository

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"inventory/internals/database"
	"inventory/internals/models"
)

type PurchaseOrderRepository interface {
	// GetBySupplier returns up to Limit+1 rows (the extra row tells the service
	// there is another page) and the total number of orders matching the filters.
	GetBySupplier(ctx context.Context, f models.PurchaseOrderListFilter) ([]models.PurchaseOrderSummary, int, error)
	GetItems(ctx context.Context, purchaseOrderID uuid.UUID) ([]models.PurchaseOrderItemDetail, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.PurchaseOrderDetail, error)
	Create(ctx context.Context, in models.PurchaseOrderWrite, createdBy uuid.UUID) (uuid.UUID, error)
	Update(ctx context.Context, id uuid.UUID, in models.PurchaseOrderWrite) error
	Delete(ctx context.Context, ids []uuid.UUID) error
}

type purchaseOrderRepository struct {
	pool *pgxpool.Pool
}

var (
	ErrPurchaseOrderNotFound        = errors.New("purchase order not found")
	ErrPurchaseOrderItemNotFound    = errors.New("purchase order item not found")
	ErrPurchaseOrderLocked          = errors.New("purchase order can't be changed in its current status")
	ErrPurchaseOrderProductNotFound = errors.New("one or more products do not exist")
	ErrPurchaseOrderQuantity        = errors.New("quantity must be a whole number for products sold in whole units")
	ErrPurchaseOrderItemProduct     = errors.New("the product of an existing item can't be changed")
)

func NewPurchaseOrderRepository() PurchaseOrderRepository {

	pool, err := database.GetPostgresPool()
	if err != nil {
		log.Println("Failed to initialize pool pool:", err)
		return nil
	}

	return &purchaseOrderRepository{pool: pool}
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func escapeLike(s string) string { return likeEscaper.Replace(s) }

func (r *purchaseOrderRepository) GetBySupplier(
	ctx context.Context,
	f models.PurchaseOrderListFilter,
) ([]models.PurchaseOrderSummary, int, error) {
	conds := []string{"po.supplier_id = $1"}
	args := []any{f.SupplierID}

	if f.Status != nil {
		args = append(args, string(*f.Status))
		conds = append(conds, fmt.Sprintf("po.status = $%d", len(args)))
	}
	if f.Search != nil {
		args = append(args, "%"+escapeLike(*f.Search)+"%")
		conds = append(conds, fmt.Sprintf("(po.title ILIKE $%d OR po.note ILIKE $%d)", len(args), len(args)))
	}

	if f.FromDate != nil {
		args = append(args, *f.FromDate)
		conds = append(conds, fmt.Sprintf("po.created_at >= $%d", len(args)))
	}
	if f.ToDate != nil {
		args = append(args, *f.ToDate)
		conds = append(conds, fmt.Sprintf("po.created_at < $%d", len(args)))
	}
	filterSQL := strings.Join(conds, " AND ")

	// total ignores the cursor so it stays the same on every page
	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM purchase_orders po WHERE "+filterSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count purchase orders: %w", err)
	}
	if total == 0 {
		return []models.PurchaseOrderSummary{}, 0, nil
	}

	listSQL := filterSQL
	listArgs := append([]any{}, args...)
	if f.Cursor != nil {
		listArgs = append(listArgs, f.Cursor.CreatedAt, f.Cursor.ID)
		listSQL += fmt.Sprintf(" AND (po.created_at, po.id) < ($%d, $%d)", len(listArgs)-1, len(listArgs))
	}
	listArgs = append(listArgs, f.Limit+1)

	query := fmt.Sprintf(`
		SELECT po.id, po.po_no, po.title, po.status, po.note,
		       po.created_by, u.name, po.created_at
		FROM purchase_orders po
		LEFT JOIN users u ON u.id = po.created_by
		WHERE %s
		ORDER BY po.created_at DESC, po.id DESC
		LIMIT $%d`, listSQL, len(listArgs))

	rows, err := r.pool.Query(ctx, query, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query purchase orders: %w", err)
	}
	defer rows.Close()

	orders := make([]models.PurchaseOrderSummary, 0, f.Limit+1)
	for rows.Next() {
		var (
			po     models.PurchaseOrderSummary
			status string
		)
		if err := rows.Scan(
			&po.ID, &po.PONo, &po.Title, &status, &po.Note,
			&po.CreatedBy, &po.CreatedByName, &po.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan purchase order: %w", err)
		}
		po.Status = models.PurchaseOrderStatus(status)
		orders = append(orders, po)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("read purchase orders: %w", err)
	}

	return orders, total, nil
}

func (r *purchaseOrderRepository) GetItems(
	ctx context.Context,
	purchaseOrderID uuid.UUID,
) ([]models.PurchaseOrderItemDetail, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT i.id, i.product_id, p.name, p.sku, un.name, un.allow_decimal,
		       i.qty_ordered, i.qty_received, i.expected_unit_cost,
		       i.expected_unit_cost * i.qty_ordered
		FROM purchase_order_items i
		JOIN products p ON p.id = i.product_id
		JOIN units un   ON un.id = p.unit_id
		WHERE i.purchase_order_id = $1
		ORDER BY p.name, i.id`, purchaseOrderID)
	if err != nil {
		return nil, fmt.Errorf("query purchase order items: %w", err)
	}
	defer rows.Close()

	items := make([]models.PurchaseOrderItemDetail, 0)
	for rows.Next() {
		var it models.PurchaseOrderItemDetail
		if err := rows.Scan(
			&it.ID, &it.ProductID, &it.ProductName, &it.ProductSKU, &it.UnitName, &it.AllowDecimal,
			&it.QtyOrdered, &it.QtyReceived, &it.ExpectedUnitCost, &it.LineTotal,
		); err != nil {
			return nil, fmt.Errorf("scan purchase order item: %w", err)
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read purchase order items: %w", err)
	}

	return items, nil
}

func (r *purchaseOrderRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.PurchaseOrderDetail, error) {
	var (
		po     models.PurchaseOrderDetail
		status string
	)

	err := r.pool.QueryRow(ctx, `
		SELECT po.id, po.po_no, po.supplier_id, po.title,
		       po.order_date::text, po.expected_date::text,
		       po.status, po.note, po.created_by, u.name, po.created_at
		FROM purchase_orders po
		LEFT JOIN users u ON u.id = po.created_by
		WHERE po.id = $1`, id,
	).Scan(
		&po.ID, &po.PONo, &po.SupplierID, &po.Title,
		&po.OrderDate, &po.ExpectedDate,
		&status, &po.Note, &po.CreatedBy, &po.CreatedByName, &po.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPurchaseOrderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get purchase order: %w", err)
	}
	po.Status = models.PurchaseOrderStatus(status)

	items, err := r.GetItems(ctx, id)
	if err != nil {
		return nil, err
	}
	po.Items = items

	return &po, nil
}

// ---------- writes ----------

func (r *purchaseOrderRepository) Create(
	ctx context.Context,
	in models.PurchaseOrderWrite,
	createdBy uuid.UUID,
) (uuid.UUID, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("begin create purchase order: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := validatePurchaseOrderItems(ctx, tx, in.Items); err != nil {
		return uuid.Nil, err
	}

	// the number is generated in the same transaction, so a rollback gives it back
	poNo, err := nextPONo(ctx, tx, in.OrderDate)
	if err != nil {
		return uuid.Nil, err
	}

	status := models.PurchaseOrderDraft
	if in.Status != nil {
		status = *in.Status
	}

	var id uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO purchase_orders
			(po_no, supplier_id, title, order_date, expected_date, status, note, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id`,
		poNo, in.SupplierID, in.Title, in.OrderDate, in.ExpectedDate, string(status), in.Note, createdBy,
	).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert purchase order: %w", err)
	}

	for _, item := range in.Items {
		if err := insertPurchaseOrderItem(ctx, tx, id, item); err != nil {
			return uuid.Nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, fmt.Errorf("commit create purchase order: %w", err)
	}

	return id, nil
}

func (r *purchaseOrderRepository) Update(ctx context.Context, id uuid.UUID, in models.PurchaseOrderWrite) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin update purchase order: %w", err)
	}
	defer tx.Rollback(ctx)

	// lock the row so two edits (or an edit and a delete) can't run at the same time
	var current string
	err = tx.QueryRow(ctx, `SELECT status FROM purchase_orders WHERE id = $1 FOR UPDATE`, id).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPurchaseOrderNotFound
	}
	if err != nil {
		return fmt.Errorf("lock purchase order: %w", err)
	}
	if !isEditablePurchaseOrderStatus(current) {
		return ErrPurchaseOrderLocked
	}

	if err := validatePurchaseOrderItems(ctx, tx, in.Items); err != nil {
		return err
	}

	var status *string
	if in.Status != nil {
		s := string(*in.Status)
		status = &s
	}

	if _, err := tx.Exec(ctx, `
		UPDATE purchase_orders
		SET title = $2, order_date = $3, expected_date = $4,
		    status = COALESCE($5, status), note = $6
		WHERE id = $1`,
		id, in.Title, in.OrderDate, in.ExpectedDate, status, in.Note,
	); err != nil {
		return fmt.Errorf("update purchase order: %w", err)
	}

	// existing lines: item id -> product id
	existing := make(map[uuid.UUID]uuid.UUID)
	rows, err := tx.Query(ctx, `SELECT id, product_id FROM purchase_order_items WHERE purchase_order_id = $1`, id)
	if err != nil {
		return fmt.Errorf("query existing items: %w", err)
	}
	for rows.Next() {
		var itemID, productID uuid.UUID
		if err := rows.Scan(&itemID, &productID); err != nil {
			rows.Close()
			return fmt.Errorf("scan existing item: %w", err)
		}
		existing[itemID] = productID
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read existing items: %w", err)
	}

	kept := make(map[uuid.UUID]bool, len(in.Items))
	for _, item := range in.Items {
		if item.ID == nil {
			continue
		}
		productID, ok := existing[*item.ID]
		if !ok {
			return ErrPurchaseOrderItemNotFound
		}
		if productID != item.ProductID {
			return ErrPurchaseOrderItemProduct
		}
		kept[*item.ID] = true
	}

	// 1. delete the lines that were removed (before inserts, so a product can be re-added)
	for itemID := range existing {
		if kept[itemID] {
			continue
		}
		if _, err := tx.Exec(ctx, `DELETE FROM purchase_order_items WHERE id = $1 AND purchase_order_id = $2`, itemID, id); err != nil {
			return fmt.Errorf("delete purchase order item: %w", err)
		}
	}

	// 2. update the kept lines and insert the new ones
	for _, item := range in.Items {
		if item.ID == nil {
			if err := insertPurchaseOrderItem(ctx, tx, id, item); err != nil {
				return err
			}
			continue
		}
		if _, err := tx.Exec(ctx, `
			UPDATE purchase_order_items
			SET qty_ordered = $3, expected_unit_cost = $4
			WHERE id = $1 AND purchase_order_id = $2`,
			*item.ID, id, item.QtyOrdered, item.ExpectedUnitCost,
		); err != nil {
			return fmt.Errorf("update purchase order item: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit update purchase order: %w", err)
	}

	return nil
}

func (r *purchaseOrderRepository) Delete(ctx context.Context, ids []uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete purchase orders: %w", err)
	}
	defer tx.Rollback(ctx)

	idStrings := uuidStrings(ids)

	rows, err := tx.Query(ctx, `SELECT status FROM purchase_orders WHERE id = ANY($1::uuid[]) FOR UPDATE`, idStrings)
	if err != nil {
		return fmt.Errorf("lock purchase orders: %w", err)
	}
	found := 0
	locked := false
	for rows.Next() {
		var status string
		if err := rows.Scan(&status); err != nil {
			rows.Close()
			return fmt.Errorf("scan purchase order status: %w", err)
		}
		found++
		if !isDeletablePurchaseOrderStatus(status) {
			locked = true
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read purchase order status: %w", err)
	}

	if found != len(ids) {
		return ErrPurchaseOrderNotFound
	}
	if locked {
		return ErrPurchaseOrderLocked
	}

	// the items are removed by ON DELETE CASCADE
	if _, err := tx.Exec(ctx, `DELETE FROM purchase_orders WHERE id = ANY($1::uuid[])`, idStrings); err != nil {
		return fmt.Errorf("delete purchase orders: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete purchase orders: %w", err)
	}

	return nil
}

// ---------- helpers ----------

// only orders that nothing was received against can be edited
func isEditablePurchaseOrderStatus(status string) bool {
	return status == string(models.PurchaseOrderDraft) || status == string(models.PurchaseOrderSent)
}

func isDeletablePurchaseOrderStatus(status string) bool {
	return status == string(models.PurchaseOrderDraft) || status == string(models.PurchaseOrderCancelled)
}

func uuidStrings(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

// validatePurchaseOrderItems checks that every product exists and that the quantity
// is a whole number when the product's unit does not allow decimals.
func validatePurchaseOrderItems(ctx context.Context, tx pgx.Tx, items []models.PurchaseOrderItemWrite) error {
	productIDs := make([]string, len(items))
	for i, item := range items {
		productIDs[i] = item.ProductID.String()
	}

	rows, err := tx.Query(ctx, `
		SELECT p.id, un.allow_decimal
		FROM products p
		JOIN units un ON un.id = p.unit_id
		WHERE p.id = ANY($1::uuid[])`, productIDs)
	if err != nil {
		return fmt.Errorf("query products: %w", err)
	}
	defer rows.Close()

	allowDecimal := make(map[uuid.UUID]bool, len(items))
	for rows.Next() {
		var (
			id      uuid.UUID
			allowed bool
		)
		if err := rows.Scan(&id, &allowed); err != nil {
			return fmt.Errorf("scan product: %w", err)
		}
		allowDecimal[id] = allowed
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read products: %w", err)
	}

	for _, item := range items {
		allowed, ok := allowDecimal[item.ProductID]
		if !ok {
			return ErrPurchaseOrderProductNotFound
		}
		if !allowed && !item.QtyOrdered.Equal(item.QtyOrdered.Truncate(0)) {
			return ErrPurchaseOrderQuantity
		}
	}

	return nil
}

func insertPurchaseOrderItem(
	ctx context.Context,
	tx pgx.Tx,
	purchaseOrderID uuid.UUID,
	item models.PurchaseOrderItemWrite,
) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO purchase_order_items (purchase_order_id, product_id, qty_ordered, expected_unit_cost)
		VALUES ($1, $2, $3, $4)`,
		purchaseOrderID, item.ProductID, item.QtyOrdered, item.ExpectedUnitCost,
	); err != nil {
		return fmt.Errorf("insert purchase order item: %w", err)
	}
	return nil
}

// nextPONo returns the next number like PO-2026-0001. It must run inside the same
// transaction that inserts the purchase order.
func nextPONo(ctx context.Context, tx pgx.Tx, orderDate time.Time) (string, error) {
	year := orderDate.Year()

	var n int
	err := tx.QueryRow(ctx, `
		INSERT INTO po_counters (po_year, last_no)
		VALUES ($1, 1)
		ON CONFLICT (po_year) DO UPDATE SET last_no = po_counters.last_no + 1
		RETURNING last_no`, year).Scan(&n)
	if err != nil {
		return "", fmt.Errorf("generate po number: %w", err)
	}

	return fmt.Sprintf("PO-%d-%04d", year, n), nil
}
