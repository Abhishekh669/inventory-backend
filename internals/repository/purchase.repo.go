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
	"github.com/shopspring/decimal"
)

type PurchaseRepository interface {
	GetAllPurchase(ctx context.Context, f models.PurchaseListFilter) ([]models.PurchaseListItem, int, error)

	CreatePurchase(ctx context.Context, userID uuid.UUID, calc *models.PurchaseCalc) (uuid.UUID, error)
}

type purchaseRepository struct {
	pool *pgxpool.Pool
}

func parseDec(s string) (decimal.Decimal, error) {
	return decimal.NewFromString(s)
}

//TODO: wile creating add pruchaseid in purchase roder and while deiing pruchasdeleteforthe prucase order too

func NewPurchaseRepository() PurchaseRepository {

	pool, err := database.GetPostgresPool()
	if err != nil {
		log.Println("Failed to initialize pool pool:", err)
		return nil
	}

	return &purchaseRepository{pool: pool}
}

func (r *purchaseRepository) GetAllPurchase(ctx context.Context, f models.PurchaseListFilter) ([]models.PurchaseListItem, int, error) {
	conds := []string{"po.status <> 'draft'"}
	args := []any{}

	if f.PurchaseOrderStatus != nil {
		args = append(args, string(*f.PurchaseOrderStatus))
		conds = append(conds, fmt.Sprintf("po.status = $%d", len(args)))
	}

	if f.PurchaseOrderNumber != nil {
		args = append(args, "%"+escapeLike(*f.PurchaseOrderNumber)+"%")
		conds = append(conds, fmt.Sprintf("po.po_no ILIKE $%d", len(args)))
	}

	// Filtering on purchase fields hides POs that have no purchase.
	if f.PurchaseStatus != nil {
		args = append(args, string(*f.PurchaseStatus))
		conds = append(conds, fmt.Sprintf("p.status = $%d", len(args)))
	}

	if f.PaymentStatus != nil {
		args = append(args, string(*f.PaymentStatus))
		conds = append(conds, fmt.Sprintf("p.payment_status = $%d", len(args)))
	}

	if f.Search != nil {
		args = append(args, "%"+escapeLike(*f.Search)+"%")
		ph := fmt.Sprintf("$%d", len(args))

		conds = append(conds, fmt.Sprintf(
			"(po.po_no ILIKE %[1]s OR po.title ILIKE %[1]s OR p.supplier_bill_no ILIKE %[1]s)",
			ph,
		))
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

	// PO is the main row. The purchase is optional, and the lateral join
	// picks at most ONE purchase per PO so rows are never duplicated.
	const fromSQL = `
		FROM purchase_orders po
		JOIN suppliers s ON s.id = po.supplier_id
		LEFT JOIN users u ON u.id = po.created_by
		LEFT JOIN LATERAL (
			SELECT *
			FROM purchases
			WHERE purchase_order_id = po.id
			ORDER BY created_at DESC, id DESC
			LIMIT 1
		) p ON TRUE
	`

	// Count ignores cursor so total stays consistent across pages.
	var total int
	if err := r.pool.QueryRow(
		ctx,
		"SELECT COUNT(*) "+fromSQL+" WHERE "+filterSQL,
		args...,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count purchase list: %w", err)
	}

	if total == 0 {
		return []models.PurchaseListItem{}, 0, nil
	}

	listSQL := filterSQL
	listArgs := append([]any{}, args...)

	if f.Cursor != nil {
		listArgs = append(listArgs, f.Cursor.CreatedAt, f.Cursor.ID)
		listSQL += fmt.Sprintf(
			" AND (po.created_at, po.id) < ($%d, $%d)",
			len(listArgs)-1,
			len(listArgs),
		)
	}

	listArgs = append(listArgs, f.Limit+1)

	query := fmt.Sprintf(`
		SELECT
			po.id,
			po.po_no,
			po.supplier_id,
			s.name,
			u.name,
			po.created_at,
			po.status,
			po.title,

			p.id,
			p.supplier_bill_no,
			p.purchase_date,
			p.total,
			p.paid_amount,
			p.returned_amount,
			p.due_amount,
			p.status,
			p.payment_status,
			p.created_at,
			p.note
		%s
		WHERE %s
		ORDER BY po.created_at DESC, po.id DESC
		LIMIT $%d
	`, fromSQL, listSQL, len(listArgs))

	rows, err := r.pool.Query(ctx, query, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query purchase list: %w", err)
	}
	defer rows.Close()

	items := make([]models.PurchaseListItem, 0, f.Limit+1)

	for rows.Next() {
		var (
			po       models.PurchaseOrderData
			poStatus string

			// purchase columns are all NULL when the PO has no purchase
			pID            *uuid.UUID
			pBillNo        *string
			pDate          *time.Time
			pTotal         decimal.NullDecimal
			pPaid          decimal.NullDecimal
			pReturned      decimal.NullDecimal
			pDue           decimal.NullDecimal
			pStatus        *string
			pPaymentStatus *string
			pCreatedAt     *time.Time
			pNote          *string
		)

		if err := rows.Scan(
			&po.PurchaseOrderID,
			&po.PurchaseOrderNumber,
			&po.SupplierID,
			&po.SupplierName,
			&po.PurchaseOrderByName,
			&po.PurchaseOrderedAt,
			&poStatus,
			&po.PurchaseOrderTitle,

			&pID,
			&pBillNo,
			&pDate,
			&pTotal,
			&pPaid,
			&pReturned,
			&pDue,
			&pStatus,
			&pPaymentStatus,
			&pCreatedAt,
			&pNote,
		); err != nil {
			return nil, 0, fmt.Errorf("scan purchase list: %w", err)
		}

		po.PurchaseOrderStatus = models.PurchaseOrderStatus(poStatus)
		item := models.PurchaseListItem{PurchaseOrderData: po}

		if pID != nil {
			pd := &models.PurchaseData{
				PurchaseID:     *pID,
				Total:          pTotal.Decimal,
				PaidAmount:     pPaid.Decimal,
				ReturnedAmount: pReturned.Decimal,
				DueAmount:      pDue.Decimal, // zero value if NULL
				PurchaseNote:   pNote,
			}
			if pBillNo != nil {
				pd.SupplierBillNo = *pBillNo
			}
			if pDate != nil {
				pd.PurchaseDate = *pDate
			}
			if pStatus != nil {
				pd.PurchaseStatus = models.PurchaseStatus(*pStatus)
			}
			if pPaymentStatus != nil {
				pd.PaymentStatus = models.PaymentStatus(*pPaymentStatus)
			}
			if pCreatedAt != nil {
				pd.PurchaseCreatedAt = *pCreatedAt
			}
			item.PurchaseData = pd
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("read purchase list: %w", err)
	}

	return items, total, nil
}

func (r *purchaseRepository) CreatePurchase(ctx context.Context, userID uuid.UUID, calc *models.PurchaseCalc) (uuid.UUID, error) {
	req := calc.Req
	bad := models.NewValidationError
	hundred := decimal.NewFromInt(100)
	one := decimal.NewFromInt(1)

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. lock the purchase order; it must be 'sent' and not already purchased
	var supplierID uuid.UUID
	var poStatus string
	var linkedPurchase *uuid.UUID

	err = tx.QueryRow(ctx,
		`SELECT supplier_id, status, purchase_id FROM purchase_orders WHERE id = $1 FOR UPDATE`,
		req.PurchaseOrderID,
	).Scan(&supplierID, &poStatus, &linkedPurchase)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, bad("purchase order not found")
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("load purchase order: %w", err)
	}
	if poStatus != string(models.PurchaseOrderSent) || linkedPurchase != nil {
		return uuid.Nil, bad("a purchase can only be created from a purchase order with status 'sent'")
	}

	// 2. purchase header
	var purchaseID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO purchases (
			supplier_id, purchase_order_id, supplier_bill_no, purchase_date,
			excise_duty, special_discount, discount, vat_rate, freight_charge,
			gross_amount, taxable_amount, vat_amount, total, paid_amount,
			payment_status, due_date, status, note, created_by
		) VALUES (
			$1, $2, $3, $4::date,
			$5::numeric, $6::numeric, $7::numeric, $8::numeric, $9::numeric,
			$10::numeric, $11::numeric, $12::numeric, $13::numeric, $14::numeric,
			$15, $16::date, $17, $18, $19
		) RETURNING id`,
		supplierID, req.PurchaseOrderID, req.SupplierBillNo, req.PurchaseDate,
		req.ExciseDuty.String(), req.SpecialDiscount.String(), req.Discount.String(),
		req.VatRate.String(), req.FreightCharge.String(),
		calc.GrossAmount.String(), calc.TaxableAmount.String(), calc.VatAmount.String(),
		calc.Total.String(), req.PaidAmount.String(),
		string(calc.PaymentStatus), req.DueDate, string(models.PurchasePosted), req.Note, userID,
	).Scan(&purchaseID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return uuid.Nil, bad("this supplier already has a purchase with that bill number")
		}
		return uuid.Nil, fmt.Errorf("insert purchase: %w", err)
	}

	// 3. items: purchase_items, PO received qty (PO lines only), stock, movement, price update.
	// The quantity is what the supplier actually sent: it may be more or less than ordered.
	for _, it := range calc.Items {
		var expected *decimal.Decimal // PO expected cost, when the line has one

		if it.PurchaseOrderItemID != nil {
			var poiProduct uuid.UUID
			var expectedS *string

			err = tx.QueryRow(ctx, `
				SELECT product_id, expected_unit_cost::text
				FROM purchase_order_items
				WHERE id = $1 AND purchase_order_id = $2
				FOR UPDATE`,
				*it.PurchaseOrderItemID, req.PurchaseOrderID,
			).Scan(&poiProduct, &expectedS)
			if errors.Is(err, pgx.ErrNoRows) {
				return uuid.Nil, bad("an item does not belong to this purchase order")
			}
			if err != nil {
				return uuid.Nil, fmt.Errorf("load purchase order item: %w", err)
			}
			if poiProduct != it.ProductID {
				return uuid.Nil, bad("product does not match the purchase order item")
			}
			if expectedS != nil {
				var e decimal.Decimal
				if e, err = parseDec(*expectedS); err != nil {
					return uuid.Nil, fmt.Errorf("parse expected_unit_cost: %w", err)
				}
				expected = &e
			}
		}

		// lock the product (and read its unit) so concurrent sales/purchases cannot corrupt stock
		var stockS, vatS, costS, marginS string
		var allowDecimal bool
		err = tx.QueryRow(ctx, `
			SELECT p.current_stock::text, p.vat_rate::text, p.cost_price::text,
			       p.margin_percent::text, u.allow_decimal
			FROM products p
			JOIN units u ON u.id = p.unit_id
			WHERE p.id = $1
			FOR UPDATE OF p`,
			it.ProductID,
		).Scan(&stockS, &vatS, &costS, &marginS, &allowDecimal)
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, bad("product not found")
		}
		if err != nil {
			return uuid.Nil, fmt.Errorf("load product: %w", err)
		}
		if !allowDecimal && !it.Qty.IsInteger() {
			return uuid.Nil, bad("quantity must be a whole number for this product's unit")
		}

		stock, err1 := parseDec(stockS)
		productVat, err2 := parseDec(vatS)
		currentCost, err3 := parseDec(costS)
		margin, err4 := parseDec(marginS)
		if err := errors.Join(err1, err2, err3, err4); err != nil {
			return uuid.Nil, fmt.Errorf("parse product values: %w", err)
		}

		// purchase item row (matches the slim purchase_items table)
		var purchaseItemID uuid.UUID
		err = tx.QueryRow(ctx, `
			INSERT INTO purchase_items (
				purchase_id, product_id, poi_id, qty, rate,
				vat_amount, unit_cost, unit_cost_with_vat
			) VALUES (
				$1, $2, $3, $4::numeric, $5::numeric,
				$6::numeric, $7::numeric, $8::numeric
			) RETURNING id`,
			purchaseID, it.ProductID, it.PurchaseOrderItemID, it.Qty.String(), it.Rate.String(),
			it.VatAmount.String(), it.UnitCost.String(), it.UnitCostWithVat.String(),
		).Scan(&purchaseItemID)
		if err != nil {
			return uuid.Nil, fmt.Errorf("insert purchase item: %w", err)
		}

		if it.PurchaseOrderItemID != nil {
			if _, err = tx.Exec(ctx,
				`UPDATE purchase_order_items SET qty_received = qty_received + $2::numeric WHERE id = $1`,
				*it.PurchaseOrderItemID, it.Qty.String(),
			); err != nil {
				return uuid.Nil, fmt.Errorf("update received qty: %w", err)
			}
		}

		newStock := stock.Add(it.Qty)

		// price changed = rate differs from the PO expected cost;
		// no expected cost (added product, or blank on the PO): compare with the product's current cost
		var priceChanged bool
		if expected != nil {
			priceChanged = !it.Rate.Equal(*expected)
		} else {
			priceChanged = !it.Rate.Equal(currentCost)
		}

		if priceChanged {
			newCost := it.Rate
			newCostWithVat := newCost.Mul(one.Add(productVat.Div(hundred))).Round(4)
			newSelling := newCostWithVat.Mul(one.Add(margin.Div(hundred))).Round(2)

			if _, err = tx.Exec(ctx, `
				UPDATE products
				SET current_stock = $2::numeric, cost_price = $3::numeric, cost_with_vat = $4::numeric,
				    selling_price = $5::numeric, updated_at = now()
				WHERE id = $1`,
				it.ProductID, newStock.String(), newCost.String(), newCostWithVat.String(), newSelling.String(),
			); err != nil {
				return uuid.Nil, fmt.Errorf("update product stock and price: %w", err)
			}

			if _, err = tx.Exec(ctx, `
				INSERT INTO product_price_history (
					product_id, purchase_item_id, unit_cost, unit_cost_with_vat,
					margin_percent, selling_price, effective_date
				) VALUES ($1, $2, $3::numeric, $4::numeric, $5::numeric, $6::numeric, $7::date)`,
				it.ProductID, purchaseItemID, newCost.String(), newCostWithVat.String(),
				margin.String(), newSelling.String(), req.PurchaseDate,
			); err != nil {
				return uuid.Nil, fmt.Errorf("insert price history: %w", err)
			}
		} else {
			if _, err = tx.Exec(ctx,
				`UPDATE products SET current_stock = $2::numeric, updated_at = now() WHERE id = $1`,
				it.ProductID, newStock.String(),
			); err != nil {
				return uuid.Nil, fmt.Errorf("update product stock: %w", err)
			}
		}

		if _, err = tx.Exec(ctx, `
			INSERT INTO stock_movements (
				product_id, type, qty_change, balance_after, ref_type, ref_id, note, user_id, movement_date
			) VALUES ($1, 'purchase', $2::numeric, $3::numeric, 'purchase', $4, $5, $6, $7::date)`,
			it.ProductID, it.Qty.String(), newStock.String(), purchaseID,
			"Purchase bill "+req.SupplierBillNo, userID, req.PurchaseDate,
		); err != nil {
			return uuid.Nil, fmt.Errorf("insert stock movement: %w", err)
		}
	}

	// 4. purchase order status + link to the purchase:
	// 'received' only when every ordered line arrived in full, otherwise 'partial'
	var allReceived bool
	if err = tx.QueryRow(ctx, `
		SELECT COALESCE(bool_and(qty_received >= qty_ordered), false)
		FROM purchase_order_items WHERE purchase_order_id = $1`,
		req.PurchaseOrderID,
	).Scan(&allReceived); err != nil {
		return uuid.Nil, fmt.Errorf("check order completion: %w", err)
	}
	newPOStatus := models.PurchaseOrderPartial
	if allReceived {
		newPOStatus = models.PurchaseOrderReceived
	}
	if _, err = tx.Exec(ctx,
		`UPDATE purchase_orders SET status = $2, purchase_id = $3 WHERE id = $1`,
		req.PurchaseOrderID, string(newPOStatus), purchaseID,
	); err != nil {
		return uuid.Nil, fmt.Errorf("update purchase order: %w", err)
	}

	// 5. payment made together with the purchase
	if req.PaidAmount.IsPositive() {
		var paymentID uuid.UUID
		err = tx.QueryRow(ctx, `
			INSERT INTO supplier_payments (
				supplier_id, amount, payment_date, payment_method, payment_type, note, created_by
			) VALUES ($1, $2::numeric, $3::date, $4, 'regular', $5, $6)
			RETURNING id`,
			supplierID, req.PaidAmount.String(), req.PurchaseDate, string(req.PaymentMethod),
			"Payment for purchase bill "+req.SupplierBillNo, userID,
		).Scan(&paymentID)
		if err != nil {
			return uuid.Nil, fmt.Errorf("insert supplier payment: %w", err)
		}
		if _, err = tx.Exec(ctx, `
			INSERT INTO purchase_payment_allocations (supplier_payment_id, purchase_id, amount)
			VALUES ($1, $2, $3::numeric)`,
			paymentID, purchaseID, req.PaidAmount.String(),
		); err != nil {
			return uuid.Nil, fmt.Errorf("insert payment allocation: %w", err)
		}
	}

	// 6. supplier balance (what we owe) and summary
	if _, err = tx.Exec(ctx,
		`UPDATE suppliers SET balance = balance + $2::numeric WHERE id = $1`,
		supplierID, calc.Total.Sub(req.PaidAmount).String(),
	); err != nil {
		return uuid.Nil, fmt.Errorf("update supplier balance: %w", err)
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO supplier_financial_summaries (supplier_id, total_purchases, total_payments)
		VALUES ($1, $2::numeric, $3::numeric)
		ON CONFLICT (supplier_id) DO UPDATE SET
			total_purchases = supplier_financial_summaries.total_purchases + EXCLUDED.total_purchases,
			total_payments  = supplier_financial_summaries.total_payments  + EXCLUDED.total_payments,
			updated_at = now()`,
		supplierID, calc.Total.String(), req.PaidAmount.String(),
	); err != nil {
		return uuid.Nil, fmt.Errorf("update supplier summary: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return uuid.Nil, fmt.Errorf("commit: %w", err)
	}
	return purchaseID, nil
}
