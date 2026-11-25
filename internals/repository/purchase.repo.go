package repository

import (
	"context"
	"errors"
	"fmt"
	"inventory/internals/database"
	"inventory/internals/models"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

type PurchaseRepository interface {
	GetPurchaseDataById(ctx context.Context, purchaseID uuid.UUID) (*models.PurchaseByIdResponse, error)

	GetAllPurchase(ctx context.Context, f models.PurchaseListFilter) ([]models.PurchaseListItem, int, error)

	CreatePurchase(ctx context.Context, userID uuid.UUID, calc *models.PurchaseCalc) (uuid.UUID, error)
	UpdatePurchase(ctx context.Context, userID, purchaseID uuid.UUID, calc *models.PurchaseCalc) error
	DeletePurchase(ctx context.Context, userID, purchaseID uuid.UUID) error

	CreatePurchaseReturn(ctx context.Context, userID, purchaseID uuid.UUID, calc *models.PurchaseReturnCalc) (uuid.UUID, error)
	UpdatePurchaseReturn(ctx context.Context, userID, purchaseID, returnID uuid.UUID, calc *models.PurchaseReturnCalc) error
	DeletePurchaseReturn(ctx context.Context, userID, purchaseID, returnID uuid.UUID) error

	CreatePurchasePayment(ctx context.Context, userID, purchaseID uuid.UUID, req *models.CreatePurchasePaymentRequest) (uuid.UUID, error)
	UpdatePurchasePayment(ctx context.Context, userID, purchaseID, paymentID uuid.UUID, req *models.UpdatePurchasePaymentRequest) error
	DeletePurchasePayment(ctx context.Context, userID, purchaseID, paymentID uuid.UUID) error

	GetSupplierPurchases(ctx context.Context, f models.SupplierPurchaseListFilter) ([]models.SupplierPurchaseItem, int, error)
	GetSupplierReturns(ctx context.Context, f models.SupplierReturnListFilter) ([]models.SupplierReturnItem, int, error)
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
		if err = verifyPaymentBankAccount(ctx, tx, supplierID, req.SupplierBankAccountID, false); err != nil {
			return uuid.Nil, err
		}
		var paymentID uuid.UUID
		err = tx.QueryRow(ctx, `
			INSERT INTO supplier_payments (
				supplier_id, amount, payment_date, payment_method, payment_type, note, created_by,
				supplier_bank_account_id
			) VALUES ($1, $2::numeric, $3::date, $4, 'regular', $5, $6, $7)
			RETURNING id`,
			supplierID, req.PaidAmount.String(), req.PurchaseDate, string(req.PaymentMethod),
			"Payment for purchase bill "+req.SupplierBillNo, userID, req.SupplierBankAccountID,
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
func (r *purchaseRepository) GetPurchaseDataById(ctx context.Context, purchaseID uuid.UUID) (*models.PurchaseByIdResponse, error) {
	resp := &models.PurchaseByIdResponse{
		PurchaseItems:      []models.PurchaseItemDetail{},
		PurchaseOrderItems: []models.PurchaseOrderItemDetail{},
		Payments:           []models.PurchasePayment{},
		Returns:            []models.PurchaseReturnDetail{},
	}

	// 1. purchase + supplier + who created it
	p := &resp.PurchaseData
	s := &resp.SupplierData
	err := r.pool.QueryRow(ctx, `
		SELECT p.id, p.supplier_id, p.purchase_order_id, p.supplier_bill_no, p.purchase_date,
		       p.excise_duty, p.special_discount, p.discount, p.vat_rate, p.freight_charge,
		       p.gross_amount, p.taxable_amount, p.vat_amount, p.total,
		       p.paid_amount, p.returned_amount, p.due_amount,
		       p.payment_status, p.due_date, p.status, p.note, p.created_by, p.created_at,
		       cu.name,
		       s.name, s.contact_person, s.phone, s.address, s.pan_no, s.is_active
		FROM purchases p
		JOIN suppliers s ON s.id = p.supplier_id
		LEFT JOIN users cu ON cu.id = p.created_by
		WHERE p.id = $1`,
		purchaseID,
	).Scan(
		&p.ID, &p.SupplierID, &p.PurchaseOrderID, &p.SupplierBillNo, &p.PurchaseDate,
		&p.ExciseDuty, &p.SpecialDiscount, &p.Discount, &p.VATRate, &p.FreightCharge,
		&p.GrossAmount, &p.TaxableAmount, &p.VATAmount, &p.Total,
		&p.PaidAmount, &p.ReturnedAmount, &p.DueAmount,
		&p.PaymentStatus, &p.DueDate, &p.Status, &p.Note, &p.CreatedBy, &p.CreatedAt,
		&p.CreatedByName,
		&s.Name, &s.ContactPerson, &s.Phone, &s.Address, &s.PanNo, &s.IsActive,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, models.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load purchase: %w", err)
	}
	s.ID = p.SupplierID

	// 2. purchase items, joined with the product, unit, the PO line they came from,
	// and how much of each line has been returned
	itemRows, err := r.pool.Query(ctx, `
		SELECT pi.id, pi.product_id, pr.name, pr.sku, un.name, un.allow_decimal, pi.poi_id,
		       pi.qty, pi.rate, pi.vat_amount, pi.unit_cost, pi.unit_cost_with_vat,
		       ROUND(pi.qty * pi.rate, 4),
		       poi.qty_ordered, poi.expected_unit_cost,
		       COALESCE((SELECT SUM(pri.qty) FROM purchase_return_items pri WHERE pri.purchase_item_id = pi.id), 0)
		FROM purchase_items pi
		JOIN products pr ON pr.id = pi.product_id
		JOIN units un ON un.id = pr.unit_id
		LEFT JOIN purchase_order_items poi ON poi.id = pi.poi_id
		WHERE pi.purchase_id = $1
		ORDER BY pr.name, pi.id`,
		purchaseID,
	)
	if err != nil {
		return nil, fmt.Errorf("load purchase items: %w", err)
	}
	defer itemRows.Close()
	for itemRows.Next() {
		var it models.PurchaseItemDetail
		if err := itemRows.Scan(
			&it.ID, &it.ProductID, &it.ProductName, &it.ProductSKU, &it.UnitName, &it.AllowDecimal, &it.POIID,
			&it.Qty, &it.Rate, &it.VATAmount, &it.UnitCost, &it.UnitCostWithVAT,
			&it.LineValue,
			&it.QtyOrdered, &it.ExpectedUnitCost,
			&it.ReturnedQty,
		); err != nil {
			return nil, fmt.Errorf("scan purchase item: %w", err)
		}
		resp.PurchaseItems = append(resp.PurchaseItems, it)
	}
	if err := itemRows.Err(); err != nil {
		return nil, fmt.Errorf("read purchase items: %w", err)
	}
	itemRows.Close()

	// 3. purchase order and all of its lines (including lines that were not received)
	if p.PurchaseOrderID != nil {
		var po models.PurchaseOrderInfo
		err = r.pool.QueryRow(ctx, `
			SELECT po.id, po.po_no, po.title,
			       to_char(po.order_date, 'YYYY-MM-DD'), to_char(po.expected_date, 'YYYY-MM-DD'),
			       po.status, po.note, cu.name, po.created_at
			FROM purchase_orders po
			LEFT JOIN users cu ON cu.id = po.created_by
			WHERE po.id = $1`,
			*p.PurchaseOrderID,
		).Scan(&po.ID, &po.PONo, &po.Title, &po.OrderDate, &po.ExpectedDate,
			&po.Status, &po.Note, &po.CreatedByName, &po.CreatedAt)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("load purchase order: %w", err)
		}
		if err == nil {
			resp.PurchaseOrderData = &po
		}

		poiRows, err := r.pool.Query(ctx, `
			SELECT poi.id, poi.product_id, pr.name, pr.sku, un.name, un.allow_decimal,
			       poi.qty_ordered, poi.qty_received, poi.expected_unit_cost,
			       CASE WHEN poi.expected_unit_cost IS NULL THEN NULL
			            ELSE ROUND(poi.qty_ordered * poi.expected_unit_cost, 2) END
			FROM purchase_order_items poi
			JOIN products pr ON pr.id = poi.product_id
			JOIN units un ON un.id = pr.unit_id
			WHERE poi.purchase_order_id = $1
			ORDER BY pr.name, poi.id`,
			*p.PurchaseOrderID,
		)
		if err != nil {
			return nil, fmt.Errorf("load purchase order items: %w", err)
		}
		defer poiRows.Close()
		for poiRows.Next() {
			var it models.PurchaseOrderItemDetail
			if err := poiRows.Scan(
				&it.ID, &it.ProductID, &it.ProductName, &it.ProductSKU, &it.UnitName, &it.AllowDecimal,
				&it.QtyOrdered, &it.QtyReceived, &it.ExpectedUnitCost, &it.LineTotal,
			); err != nil {
				return nil, fmt.Errorf("scan purchase order item: %w", err)
			}
			resp.PurchaseOrderItems = append(resp.PurchaseOrderItems, it)
		}
		if err := poiRows.Err(); err != nil {
			return nil, fmt.Errorf("read purchase order items: %w", err)
		}
		poiRows.Close()
	}

	// 4. payments allocated to this purchase
	payRows, err := r.pool.Query(ctx, `
		SELECT sp.id, ppa.amount, to_char(sp.payment_date, 'YYYY-MM-DD'),
		       sp.payment_method, sp.payment_type, sp.note, cu.name, sp.created_at,
		       sp.supplier_bank_account_id, ba.bank_name, ba.account_name, ba.account_number
		FROM purchase_payment_allocations ppa
		JOIN supplier_payments sp ON sp.id = ppa.supplier_payment_id
		LEFT JOIN users cu ON cu.id = sp.created_by
		LEFT JOIN supplier_bank_accounts ba ON ba.id = sp.supplier_bank_account_id
		WHERE ppa.purchase_id = $1
		ORDER BY sp.payment_date DESC, sp.created_at DESC`,
		purchaseID,
	)
	if err != nil {
		return nil, fmt.Errorf("load payments: %w", err)
	}
	defer payRows.Close()
	for payRows.Next() {
		var pay models.PurchasePayment
		if err := payRows.Scan(&pay.ID, &pay.Amount, &pay.PaymentDate,
			&pay.PaymentMethod, &pay.PaymentType, &pay.Note, &pay.CreatedByName, &pay.CreatedAt,
			&pay.SupplierBankAccountID, &pay.BankName, &pay.AccountName, &pay.AccountNumber); err != nil {
			return nil, fmt.Errorf("scan payment: %w", err)
		}
		resp.Payments = append(resp.Payments, pay)
	}
	if err := payRows.Err(); err != nil {
		return nil, fmt.Errorf("read payments: %w", err)
	}
	payRows.Close()

	// 5. returns, newest first, each with its items
	retRows, err := r.pool.Query(ctx, `
		SELECT r.id, r.return_no, to_char(r.return_date, 'YYYY-MM-DD'), r.total, r.note, cu.name, r.created_at
		FROM purchase_returns r
		LEFT JOIN users cu ON cu.id = r.created_by
		WHERE r.purchase_id = $1
		ORDER BY r.return_date DESC, r.created_at DESC`,
		purchaseID,
	)
	if err != nil {
		return nil, fmt.Errorf("load returns: %w", err)
	}
	defer retRows.Close()
	returnIndex := map[uuid.UUID]int{}
	for retRows.Next() {
		var ret models.PurchaseReturnDetail
		if err := retRows.Scan(&ret.ID, &ret.ReturnNo, &ret.ReturnDate, &ret.Total,
			&ret.Note, &ret.CreatedByName, &ret.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan return: %w", err)
		}
		ret.Items = []models.PurchaseReturnItemDetail{}
		returnIndex[ret.ID] = len(resp.Returns)
		resp.Returns = append(resp.Returns, ret)
	}
	if err := retRows.Err(); err != nil {
		return nil, fmt.Errorf("read returns: %w", err)
	}
	retRows.Close()

	if len(resp.Returns) > 0 {
		riRows, err := r.pool.Query(ctx, `
			SELECT pri.id, pri.purchase_return_id, pri.purchase_item_id, pri.product_id,
			       pr.name, pr.sku, un.name, un.allow_decimal, pri.qty, pri.amount, pri.reason
			FROM purchase_return_items pri
			JOIN purchase_returns r ON r.id = pri.purchase_return_id
			JOIN products pr ON pr.id = pri.product_id
			JOIN units un ON un.id = pr.unit_id
			WHERE r.purchase_id = $1
			ORDER BY pr.name`,
			purchaseID,
		)
		if err != nil {
			return nil, fmt.Errorf("load return items: %w", err)
		}
		defer riRows.Close()
		for riRows.Next() {
			var returnID uuid.UUID
			var it models.PurchaseReturnItemDetail
			if err := riRows.Scan(&it.ID, &returnID, &it.PurchaseItemID, &it.ProductID,
				&it.ProductName, &it.ProductSKU, &it.UnitName, &it.AllowDecimal,
				&it.Qty, &it.Amount, &it.Reason); err != nil {
				return nil, fmt.Errorf("scan return item: %w", err)
			}
			if idx, ok := returnIndex[returnID]; ok {
				resp.Returns[idx].Items = append(resp.Returns[idx].Items, it)
			}
		}
		if err := riRows.Err(); err != nil {
			return nil, fmt.Errorf("read return items: %w", err)
		}
	}

	return resp, nil
}

func (r *purchaseRepository) UpdatePurchase(ctx context.Context, userID, purchaseID uuid.UUID, calc *models.PurchaseCalc) error {
	req := calc.Req
	bad := models.NewValidationError

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	cur, err := lockPurchase(ctx, tx, purchaseID)
	if err != nil {
		return err
	}
	if calc.Total.LessThan(cur.Paid) {
		return bad("the new total is less than the amount already paid on this purchase")
	}

	// current lines, keyed by product, with how much of each has been returned
	type oldLine struct {
		ID          uuid.UUID
		ProductID   uuid.UUID
		POIID       *uuid.UUID
		Qty         decimal.Decimal
		Rate        decimal.Decimal
		ReturnedQty decimal.Decimal
	}
	oldRows, err := tx.Query(ctx, `
		SELECT pi.id, pi.product_id, pi.poi_id, pi.qty, pi.rate, COALESCE(SUM(pri.qty), 0)
		FROM purchase_items pi
		LEFT JOIN purchase_return_items pri ON pri.purchase_item_id = pi.id
		WHERE pi.purchase_id = $1
		GROUP BY pi.id
		ORDER BY pi.product_id`, purchaseID)
	if err != nil {
		return fmt.Errorf("load purchase items: %w", err)
	}
	old := map[uuid.UUID]oldLine{}
	for oldRows.Next() {
		var l oldLine
		if err := oldRows.Scan(&l.ID, &l.ProductID, &l.POIID, &l.Qty, &l.Rate, &l.ReturnedQty); err != nil {
			oldRows.Close()
			return fmt.Errorf("scan purchase item: %w", err)
		}
		old[l.ProductID] = l
	}
	oldRows.Close()
	if err := oldRows.Err(); err != nil {
		return fmt.Errorf("read purchase items: %w", err)
	}

	newBy := map[uuid.UUID]models.PurchaseItemCalc{}
	for _, it := range calc.Items {
		newBy[it.ProductID] = it
	}

	// bill header
	if _, err = tx.Exec(ctx, `
		UPDATE purchases SET
			supplier_bill_no = $2, purchase_date = $3::date,
			excise_duty = $4::numeric, special_discount = $5::numeric, discount = $6::numeric,
			vat_rate = $7::numeric, freight_charge = $8::numeric,
			gross_amount = $9::numeric, taxable_amount = $10::numeric, vat_amount = $11::numeric, total = $12::numeric,
			payment_status = $13, due_date = $14::date, note = $15
		WHERE id = $1`,
		purchaseID, req.SupplierBillNo, req.PurchaseDate,
		req.ExciseDuty.String(), req.SpecialDiscount.String(), req.Discount.String(),
		req.VatRate.String(), req.FreightCharge.String(),
		calc.GrossAmount.String(), calc.TaxableAmount.String(), calc.VatAmount.String(), calc.Total.String(),
		string(paymentStatusFor(calc.Total, cur.Paid, cur.Returned)), req.DueDate, req.Note,
	); err != nil {
		if isUniqueViolation(err) {
			return bad("this supplier already has a purchase with that bill number")
		}
		return fmt.Errorf("update purchase: %w", err)
	}

	// every product that was or is on the purchase, in a fixed order (avoids deadlocks)
	seen := map[uuid.UUID]bool{}
	ids := make([]uuid.UUID, 0, len(old)+len(newBy))
	for id := range old {
		seen[id] = true
		ids = append(ids, id)
	}
	for id := range newBy {
		if !seen[id] {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })

	note := "Purchase bill " + req.SupplierBillNo + " edited"

	for _, pid := range ids {
		o, hadOld := old[pid]
		n, hasNew := newBy[pid]

		prod, err := lockProduct(ctx, tx, pid)
		if err != nil {
			return err
		}
		if hasNew && !prod.AllowDecimal && !n.Qty.IsInteger() {
			return bad(prod.Name + ": quantity must be a whole number")
		}

		// a line that has returns keeps at least the returned quantity and cannot be removed
		if hadOld && o.ReturnedQty.IsPositive() {
			if !hasNew {
				return bad(prod.Name + " has returns, so it cannot be removed. Delete its returns first.")
			}
			if n.Qty.LessThan(o.ReturnedQty) {
				return bad(fmt.Sprintf("%s: %s already returned, so the quantity cannot be lower", prod.Name, o.ReturnedQty.String()))
			}
		}

		oldQty, newQty := decimal.Zero, decimal.Zero
		if hadOld {
			oldQty = o.Qty
		}
		if hasNew {
			newQty = n.Qty
		}
		delta := newQty.Sub(oldQty)

		// returns already took their quantity off stock, so only the difference moves
		if err := changeStock(ctx, tx, pid, prod, delta, "adjustment", "purchase_edit", purchaseID, note, userID); err != nil {
			return err
		}

		switch {
		case hadOld && hasNew:
			if _, err = tx.Exec(ctx, `
				UPDATE purchase_items
				SET qty = $2::numeric, rate = $3::numeric, vat_amount = $4::numeric,
				    unit_cost = $5::numeric, unit_cost_with_vat = $6::numeric
				WHERE id = $1`,
				o.ID, n.Qty.String(), n.Rate.String(), n.VatAmount.String(), n.UnitCost.String(), n.UnitCostWithVat.String(),
			); err != nil {
				return fmt.Errorf("update purchase item: %w", err)
			}
			if o.POIID != nil && !delta.IsZero() {
				if _, err = tx.Exec(ctx,
					`UPDATE purchase_order_items SET qty_received = GREATEST(qty_received + $2::numeric, 0) WHERE id = $1`,
					*o.POIID, delta.String(),
				); err != nil {
					return fmt.Errorf("update received qty: %w", err)
				}
			}
			// the product price follows this line only while it still carries the line's old price
			if !n.Rate.Equal(o.Rate) && prod.Cost.Equal(o.Rate) {
				if err := applyProductPrice(ctx, tx, pid, prod, n.Rate, o.ID, req.PurchaseDate); err != nil {
					return err
				}
			}

		case hadOld && !hasNew:
			if err := restoreProductPrice(ctx, tx, pid, prod, o.ID, o.Rate); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `DELETE FROM purchase_items WHERE id = $1`, o.ID); err != nil {
				return fmt.Errorf("delete purchase item: %w", err)
			}
			if o.POIID != nil {
				if _, err = tx.Exec(ctx,
					`UPDATE purchase_order_items SET qty_received = GREATEST(qty_received - $2::numeric, 0) WHERE id = $1`,
					*o.POIID, o.Qty.String(),
				); err != nil {
					return fmt.Errorf("update received qty: %w", err)
				}
			}

		default: // a product added during the edit
			// if it is on the purchase order (it was left out when the purchase was made) link it back;
			// if the order never had it, it stays an "added later" line with no order line
			var poiID *uuid.UUID
			var expected *decimal.Decimal
			if cur.PurchaseOrderID != nil {
				var id uuid.UUID
				var exp *decimal.Decimal
				err = tx.QueryRow(ctx, `
					SELECT id, expected_unit_cost FROM purchase_order_items
					WHERE purchase_order_id = $1 AND product_id = $2 FOR UPDATE`,
					*cur.PurchaseOrderID, pid,
				).Scan(&id, &exp)
				if err == nil {
					poiID, expected = &id, exp
				} else if !errors.Is(err, pgx.ErrNoRows) {
					return fmt.Errorf("load purchase order item: %w", err)
				}
			}

			var itemID uuid.UUID
			if err = tx.QueryRow(ctx, `
				INSERT INTO purchase_items (
					purchase_id, product_id, poi_id, qty, rate, vat_amount, unit_cost, unit_cost_with_vat
				) VALUES ($1, $2, $3, $4::numeric, $5::numeric, $6::numeric, $7::numeric, $8::numeric)
				RETURNING id`,
				purchaseID, pid, poiID, n.Qty.String(), n.Rate.String(), n.VatAmount.String(),
				n.UnitCost.String(), n.UnitCostWithVat.String(),
			).Scan(&itemID); err != nil {
				return fmt.Errorf("insert purchase item: %w", err)
			}
			if poiID != nil {
				if _, err = tx.Exec(ctx,
					`UPDATE purchase_order_items SET qty_received = qty_received + $2::numeric WHERE id = $1`,
					*poiID, n.Qty.String(),
				); err != nil {
					return fmt.Errorf("update received qty: %w", err)
				}
			}

			var changed bool
			if expected != nil {
				changed = !n.Rate.Equal(*expected)
			} else {
				changed = !n.Rate.Equal(prod.Cost)
			}
			if changed {
				if err := applyProductPrice(ctx, tx, pid, prod, n.Rate, itemID, req.PurchaseDate); err != nil {
					return err
				}
			}
		}
	}

	// purchase order status follows what has now been received
	if cur.PurchaseOrderID != nil {
		var allReceived bool
		if err = tx.QueryRow(ctx, `
			SELECT COALESCE(bool_and(qty_received >= qty_ordered), false)
			FROM purchase_order_items WHERE purchase_order_id = $1`,
			*cur.PurchaseOrderID,
		).Scan(&allReceived); err != nil {
			return fmt.Errorf("check order completion: %w", err)
		}
		poStatus := models.PurchaseOrderPartial
		if allReceived {
			poStatus = models.PurchaseOrderReceived
		}
		if _, err = tx.Exec(ctx,
			`UPDATE purchase_orders SET status = $2 WHERE id = $1`,
			*cur.PurchaseOrderID, string(poStatus),
		); err != nil {
			return fmt.Errorf("update purchase order: %w", err)
		}
	}

	// supplier balance and summary follow the change in the purchase total
	diff := calc.Total.Sub(cur.Total)
	if err := adjustSupplier(ctx, tx, cur.SupplierID, diff, decimal.Zero); err != nil {
		return err
	}
	if err := bumpSummary(ctx, tx, cur.SupplierID, diff, decimal.Zero, decimal.Zero, decimal.Zero); err != nil {
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (r *purchaseRepository) CreatePurchaseReturn(ctx context.Context, userID, purchaseID uuid.UUID, calc *models.PurchaseReturnCalc) (uuid.UUID, error) {
	req := calc.Req
	bad := models.NewValidationError

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	cur, err := lockPurchase(ctx, tx, purchaseID)
	if err != nil {
		return uuid.Nil, err
	}

	returnNo := fmt.Sprintf("PRT-%s-%s",
		strings.ReplaceAll(req.ReturnDate, "-", ""),
		strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", "")[:6]))

	var returnID uuid.UUID
	if err = tx.QueryRow(ctx, `
		INSERT INTO purchase_returns (return_no, purchase_id, supplier_id, return_date, total, note, created_by)
		VALUES ($1, $2, $3, $4::date, $5::numeric, $6, $7)
		RETURNING id`,
		returnNo, purchaseID, cur.SupplierID, req.ReturnDate, calc.Total.String(), req.Note, userID,
	).Scan(&returnID); err != nil {
		return uuid.Nil, fmt.Errorf("insert purchase return: %w", err)
	}

	for _, it := range req.Items {
		var productID uuid.UUID
		var boughtQty decimal.Decimal
		err = tx.QueryRow(ctx, `
			SELECT product_id, qty FROM purchase_items WHERE id = $1 AND purchase_id = $2 FOR UPDATE`,
			it.PurchaseItemID, purchaseID,
		).Scan(&productID, &boughtQty)
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, bad("an item does not belong to this purchase")
		}
		if err != nil {
			return uuid.Nil, fmt.Errorf("load purchase item: %w", err)
		}

		var already decimal.Decimal
		if err = tx.QueryRow(ctx,
			`SELECT COALESCE(SUM(qty), 0) FROM purchase_return_items WHERE purchase_item_id = $1`,
			it.PurchaseItemID,
		).Scan(&already); err != nil {
			return uuid.Nil, fmt.Errorf("load returned qty: %w", err)
		}
		if it.Qty.GreaterThan(boughtQty.Sub(already)) {
			return uuid.Nil, bad("return quantity is more than what is left to return")
		}

		prod, err := lockProduct(ctx, tx, productID)
		if err != nil {
			return uuid.Nil, err
		}
		if !prod.AllowDecimal && !it.Qty.IsInteger() {
			return uuid.Nil, bad(prod.Name + ": quantity must be a whole number")
		}
		// the returned quantity leaves stock
		if err := changeStock(ctx, tx, productID, prod, it.Qty.Neg(), "purchase_return", "purchase_return",
			returnID, "Return "+returnNo, userID); err != nil {
			return uuid.Nil, err
		}

		if _, err = tx.Exec(ctx, `
			INSERT INTO purchase_return_items (purchase_return_id, purchase_item_id, product_id, qty, amount, reason)
			VALUES ($1, $2, $3, $4::numeric, $5::numeric, $6)`,
			returnID, it.PurchaseItemID, productID, it.Qty.String(), it.Amount.String(), it.Reason,
		); err != nil {
			return uuid.Nil, fmt.Errorf("insert purchase return item: %w", err)
		}
	}

	// only the purchase's returned amount changes: no supplier balance, credit or summary
	newReturned := cur.Returned.Add(calc.Total)
	if _, err = tx.Exec(ctx,
		`UPDATE purchases SET returned_amount = $2::numeric, payment_status = $3 WHERE id = $1`,
		purchaseID, newReturned.String(), string(paymentStatusFor(cur.Total, cur.Paid, newReturned)),
	); err != nil {
		return uuid.Nil, fmt.Errorf("update purchase: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return uuid.Nil, fmt.Errorf("commit: %w", err)
	}
	return returnID, nil
}

func (r *purchaseRepository) UpdatePurchaseReturn(ctx context.Context, userID, purchaseID, returnID uuid.UUID, calc *models.PurchaseReturnCalc) error {
	req := calc.Req
	bad := models.NewValidationError

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	cur, err := lockPurchase(ctx, tx, purchaseID)
	if err != nil {
		return err
	}

	var returnNo string
	var oldTotal decimal.Decimal
	err = tx.QueryRow(ctx,
		`SELECT return_no, total FROM purchase_returns WHERE id = $1 AND purchase_id = $2 FOR UPDATE`,
		returnID, purchaseID,
	).Scan(&returnNo, &oldTotal)
	if errors.Is(err, pgx.ErrNoRows) {
		return bad("return not found on this purchase")
	}
	if err != nil {
		return fmt.Errorf("load return: %w", err)
	}

	// the lines this return holds now, keyed by purchase item
	type oldItem struct {
		ID  uuid.UUID
		Qty decimal.Decimal
	}
	rows, err := tx.Query(ctx,
		`SELECT id, purchase_item_id, qty FROM purchase_return_items WHERE purchase_return_id = $1`, returnID)
	if err != nil {
		return fmt.Errorf("load return items: %w", err)
	}
	old := map[uuid.UUID]oldItem{}
	for rows.Next() {
		var id, itemID uuid.UUID
		var qty decimal.Decimal
		if err := rows.Scan(&id, &itemID, &qty); err != nil {
			rows.Close()
			return fmt.Errorf("scan return item: %w", err)
		}
		old[itemID] = oldItem{ID: id, Qty: qty}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read return items: %w", err)
	}

	newBy := map[uuid.UUID]models.CreatePurchaseReturnItemRequest{}
	for _, it := range req.Items {
		newBy[it.PurchaseItemID] = it
	}

	seen := map[uuid.UUID]bool{}
	ids := make([]uuid.UUID, 0, len(old)+len(newBy))
	for id := range old {
		seen[id] = true
		ids = append(ids, id)
	}
	for id := range newBy {
		if !seen[id] {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })

	note := "Return " + returnNo + " edited"

	for _, itemID := range ids {
		o, hadOld := old[itemID]
		n, hasNew := newBy[itemID]

		var productID uuid.UUID
		var boughtQty decimal.Decimal
		err = tx.QueryRow(ctx, `
			SELECT product_id, qty FROM purchase_items WHERE id = $1 AND purchase_id = $2 FOR UPDATE`,
			itemID, purchaseID,
		).Scan(&productID, &boughtQty)
		if errors.Is(err, pgx.ErrNoRows) {
			return bad("an item does not belong to this purchase")
		}
		if err != nil {
			return fmt.Errorf("load purchase item: %w", err)
		}

		if hasNew {
			// what other returns of this line already took
			var others decimal.Decimal
			if err = tx.QueryRow(ctx, `
				SELECT COALESCE(SUM(qty), 0) FROM purchase_return_items
				WHERE purchase_item_id = $1 AND purchase_return_id <> $2`,
				itemID, returnID,
			).Scan(&others); err != nil {
				return fmt.Errorf("load returned qty: %w", err)
			}
			if n.Qty.GreaterThan(boughtQty.Sub(others)) {
				return bad("return quantity is more than what is left to return")
			}
		}

		oldQty, newQty := decimal.Zero, decimal.Zero
		if hadOld {
			oldQty = o.Qty
		}
		if hasNew {
			newQty = n.Qty
		}

		prod, err := lockProduct(ctx, tx, productID)
		if err != nil {
			return err
		}
		if hasNew && !prod.AllowDecimal && !n.Qty.IsInteger() {
			return bad(prod.Name + ": quantity must be a whole number")
		}
		// returning more takes more off stock; returning less puts the difference back
		if err := changeStock(ctx, tx, productID, prod, newQty.Sub(oldQty).Neg(),
			"purchase_return", "purchase_return", returnID, note, userID); err != nil {
			return err
		}

		switch {
		case hadOld && hasNew:
			if _, err = tx.Exec(ctx, `
				UPDATE purchase_return_items
				SET qty = $2::numeric, amount = $3::numeric, reason = $4 WHERE id = $1`,
				o.ID, n.Qty.String(), n.Amount.String(), n.Reason,
			); err != nil {
				return fmt.Errorf("update return item: %w", err)
			}
		case hadOld && !hasNew:
			if _, err = tx.Exec(ctx, `DELETE FROM purchase_return_items WHERE id = $1`, o.ID); err != nil {
				return fmt.Errorf("delete return item: %w", err)
			}
		default:
			if _, err = tx.Exec(ctx, `
				INSERT INTO purchase_return_items (purchase_return_id, purchase_item_id, product_id, qty, amount, reason)
				VALUES ($1, $2, $3, $4::numeric, $5::numeric, $6)`,
				returnID, itemID, productID, n.Qty.String(), n.Amount.String(), n.Reason,
			); err != nil {
				return fmt.Errorf("insert return item: %w", err)
			}
		}
	}

	if _, err = tx.Exec(ctx, `
		UPDATE purchase_returns SET return_date = $2::date, total = $3::numeric, note = $4 WHERE id = $1`,
		returnID, req.ReturnDate, calc.Total.String(), req.Note,
	); err != nil {
		return fmt.Errorf("update return: %w", err)
	}

	// only the purchase's returned amount changes: no supplier balance, credit or summary
	newReturned := decimal.Max(cur.Returned.Add(calc.Total.Sub(oldTotal)), decimal.Zero)
	if _, err = tx.Exec(ctx,
		`UPDATE purchases SET returned_amount = $2::numeric, payment_status = $3 WHERE id = $1`,
		purchaseID, newReturned.String(), string(paymentStatusFor(cur.Total, cur.Paid, newReturned)),
	); err != nil {
		return fmt.Errorf("update purchase: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (r *purchaseRepository) DeletePurchaseReturn(ctx context.Context, userID, purchaseID, returnID uuid.UUID) error {
	bad := models.NewValidationError

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	cur, err := lockPurchase(ctx, tx, purchaseID)
	if err != nil {
		return err
	}

	var returnNo string
	var total decimal.Decimal
	err = tx.QueryRow(ctx,
		`SELECT return_no, total FROM purchase_returns WHERE id = $1 AND purchase_id = $2 FOR UPDATE`,
		returnID, purchaseID,
	).Scan(&returnNo, &total)
	if errors.Is(err, pgx.ErrNoRows) {
		return bad("return not found on this purchase")
	}
	if err != nil {
		return fmt.Errorf("load return: %w", err)
	}

	type item struct {
		ProductID uuid.UUID
		Qty       decimal.Decimal
	}
	rows, err := tx.Query(ctx,
		`SELECT product_id, qty FROM purchase_return_items WHERE purchase_return_id = $1 ORDER BY product_id`, returnID)
	if err != nil {
		return fmt.Errorf("load return items: %w", err)
	}
	var items []item
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.ProductID, &it.Qty); err != nil {
			rows.Close()
			return fmt.Errorf("scan return item: %w", err)
		}
		items = append(items, it)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read return items: %w", err)
	}

	// the returned quantities come back into stock
	note := "Return " + returnNo + " deleted"
	for _, it := range items {
		prod, err := lockProduct(ctx, tx, it.ProductID)
		if err != nil {
			return err
		}
		if err := changeStock(ctx, tx, it.ProductID, prod, it.Qty, "adjustment", "purchase_return_delete",
			returnID, note, userID); err != nil {
			return err
		}
	}

	// the return's items go with it (ON DELETE CASCADE)
	if _, err = tx.Exec(ctx, `DELETE FROM purchase_returns WHERE id = $1`, returnID); err != nil {
		return fmt.Errorf("delete return: %w", err)
	}

	// only the purchase's returned amount changes: no supplier balance, credit or summary
	newReturned := decimal.Max(cur.Returned.Sub(total), decimal.Zero)
	if _, err = tx.Exec(ctx,
		`UPDATE purchases SET returned_amount = $2::numeric, payment_status = $3 WHERE id = $1`,
		purchaseID, newReturned.String(), string(paymentStatusFor(cur.Total, cur.Paid, newReturned)),
	); err != nil {
		return fmt.Errorf("update purchase: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (r *purchaseRepository) CreatePurchasePayment(ctx context.Context, userID, purchaseID uuid.UUID, req *models.CreatePurchasePaymentRequest) (uuid.UUID, error) {
	bad := models.NewValidationError

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	cur, err := lockPurchase(ctx, tx, purchaseID)
	if err != nil {
		return uuid.Nil, err
	}

	// the supplier row lock serialises every credit and payment change for this supplier
	var credit decimal.Decimal
	if err = tx.QueryRow(ctx,
		`SELECT credit_balance FROM suppliers WHERE id = $1 FOR UPDATE`, cur.SupplierID,
	).Scan(&credit); err != nil {
		return uuid.Nil, fmt.Errorf("load supplier: %w", err)
	}

	due := cur.Total.Sub(cur.Paid).Sub(cur.Returned)
	if !due.IsPositive() {
		return uuid.Nil, bad("this purchase has nothing left to pay")
	}
	if req.Amount.GreaterThan(due) {
		return uuid.Nil, bad("the amount is more than the amount due on this purchase")
	}

	note := "Payment for purchase bill " + cur.BillNo
	if req.Note != nil {
		note = *req.Note
	}

	var paymentID uuid.UUID

	switch req.Source {
	case models.PaymentTypeRegular:
		if err = verifyPaymentBankAccount(ctx, tx, cur.SupplierID, req.SupplierBankAccountID, false); err != nil {
			return uuid.Nil, err
		}
		if err = tx.QueryRow(ctx, `
			INSERT INTO supplier_payments (
				supplier_id, amount, payment_date, payment_method, payment_type, note, created_by,
				supplier_bank_account_id
			) VALUES ($1, $2::numeric, $3::date, $4, 'regular', $5, $6, $7)
			RETURNING id`,
			cur.SupplierID, req.Amount.String(), req.PaymentDate, string(req.PaymentMethod), note, userID,
			req.SupplierBankAccountID,
		).Scan(&paymentID); err != nil {
			return uuid.Nil, fmt.Errorf("insert supplier payment: %w", err)
		}
		if err := allocatePayment(ctx, tx, paymentID, purchaseID, req.Amount); err != nil {
			return uuid.Nil, err
		}
		if err := adjustSupplier(ctx, tx, cur.SupplierID, req.Amount.Neg(), decimal.Zero); err != nil {
			return uuid.Nil, err
		}
		if err := bumpSummary(ctx, tx, cur.SupplierID, decimal.Zero, req.Amount, decimal.Zero, decimal.Zero); err != nil {
			return uuid.Nil, err
		}

	case models.PaymentTypeAdvance:
		if req.Amount.GreaterThan(credit) {
			return uuid.Nil, bad("the supplier does not have enough advance credit for this amount")
		}

		// payments that still have an unallocated part, oldest first
		type openPayment struct {
			ID        uuid.UUID
			Remaining decimal.Decimal
		}
		rows, err := tx.Query(ctx, `
			SELECT sp.id, sp.amount - COALESCE(SUM(a.amount), 0)
			FROM supplier_payments sp
			LEFT JOIN purchase_payment_allocations a ON a.supplier_payment_id = sp.id
			WHERE sp.supplier_id = $1
			GROUP BY sp.id, sp.amount, sp.payment_date, sp.created_at
			HAVING sp.amount - COALESCE(SUM(a.amount), 0) > 0
			ORDER BY sp.payment_date, sp.created_at`,
			cur.SupplierID,
		)
		if err != nil {
			return uuid.Nil, fmt.Errorf("load advance payments: %w", err)
		}
		var open []openPayment
		for rows.Next() {
			var p openPayment
			if err := rows.Scan(&p.ID, &p.Remaining); err != nil {
				rows.Close()
				return uuid.Nil, fmt.Errorf("scan advance payment: %w", err)
			}
			open = append(open, p)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return uuid.Nil, fmt.Errorf("read advance payments: %w", err)
		}

		left := req.Amount
		for _, p := range open {
			if !left.IsPositive() {
				break
			}
			take := decimal.Min(p.Remaining, left)
			if err := allocatePayment(ctx, tx, p.ID, purchaseID, take); err != nil {
				return uuid.Nil, err
			}
			paymentID = p.ID
			left = left.Sub(take)
		}
		if left.IsPositive() {
			return uuid.Nil, bad("the advance payment records do not cover this amount")
		}

		// using credit reduces both the supplier's advance credit and what we owe
		if err := adjustSupplier(ctx, tx, cur.SupplierID, req.Amount.Neg(), req.Amount.Neg()); err != nil {
			return uuid.Nil, err
		}
		if err := bumpSummary(ctx, tx, cur.SupplierID, decimal.Zero, decimal.Zero, decimal.Zero, req.Amount.Neg()); err != nil {
			return uuid.Nil, err
		}

	default:
		return uuid.Nil, bad("invalid payment source")
	}

	newPaid := cur.Paid.Add(req.Amount)
	if _, err = tx.Exec(ctx,
		`UPDATE purchases SET paid_amount = $2::numeric, payment_status = $3 WHERE id = $1`,
		purchaseID, newPaid.String(), string(paymentStatusFor(cur.Total, newPaid, cur.Returned)),
	); err != nil {
		return uuid.Nil, fmt.Errorf("update purchase: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return uuid.Nil, fmt.Errorf("commit: %w", err)
	}
	return paymentID, nil
}

// UpdatePurchasePayment changes one payment of this purchase.
//   - a regular payment can change amount, date, method and note
//   - a payment taken from advance credit can only change the amount applied to this purchase
func (r *purchaseRepository) UpdatePurchasePayment(ctx context.Context, userID, purchaseID, paymentID uuid.UUID, req *models.UpdatePurchasePaymentRequest) error {
	bad := models.NewValidationError

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	cur, err := lockPurchase(ctx, tx, purchaseID)
	if err != nil {
		return err
	}

	var credit decimal.Decimal
	if err = tx.QueryRow(ctx,
		`SELECT credit_balance FROM suppliers WHERE id = $1 FOR UPDATE`, cur.SupplierID,
	).Scan(&credit); err != nil {
		return fmt.Errorf("load supplier: %w", err)
	}

	pay, err := loadPurchasePayment(ctx, tx, purchaseID, paymentID)
	if err != nil {
		return err
	}

	diff := req.Amount.Sub(pay.AllocAmount) // positive = more is paid on this purchase
	due := cur.Total.Sub(cur.Paid).Sub(cur.Returned)
	if diff.IsPositive() && diff.GreaterThan(due) {
		return bad("the new amount is more than what is due on this purchase")
	}

	switch pay.Type {
	case models.PaymentTypeRegular:
		if pay.AllocCount != 1 || !pay.AllocAmount.Equal(pay.PaymentAmount) {
			return bad("this payment is shared with other purchases, so it cannot be edited here")
		}
		if err = verifyPaymentBankAccount(ctx, tx, cur.SupplierID, req.SupplierBankAccountID, true); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `
			UPDATE supplier_payments
			SET amount = $2::numeric,
			    payment_date = COALESCE(NULLIF($3, '')::date, payment_date),
			    payment_method = COALESCE(NULLIF($4, ''), payment_method),
			    supplier_bank_account_id = CASE
			        WHEN COALESCE(NULLIF($4, ''), payment_method) = 'cash' THEN NULL
			        ELSE $6::uuid END,
			    note = $5
			WHERE id = $1`,
			paymentID, req.Amount.String(), req.PaymentDate, string(req.PaymentMethod), req.Note,
			req.SupplierBankAccountID,
		); err != nil {
			return fmt.Errorf("update supplier payment: %w", err)
		}
		if _, err = tx.Exec(ctx, `
			UPDATE purchase_payment_allocations SET amount = $3::numeric
			WHERE supplier_payment_id = $1 AND purchase_id = $2`,
			paymentID, purchaseID, req.Amount.String(),
		); err != nil {
			return fmt.Errorf("update allocation: %w", err)
		}
		// more paid means we owe less
		if err := adjustSupplier(ctx, tx, cur.SupplierID, diff.Neg(), decimal.Zero); err != nil {
			return err
		}
		if err := bumpSummary(ctx, tx, cur.SupplierID, decimal.Zero, diff, decimal.Zero, decimal.Zero); err != nil {
			return err
		}

	case models.PaymentTypeAdvance:
		if diff.IsPositive() {
			remaining := pay.PaymentAmount.Sub(pay.AllocSum)
			if diff.GreaterThan(remaining) {
				return bad("that advance payment does not have enough left to apply this amount")
			}
			if diff.GreaterThan(credit) {
				return bad("the supplier does not have enough advance credit for this amount")
			}
		}
		if _, err = tx.Exec(ctx, `
			UPDATE purchase_payment_allocations SET amount = $3::numeric
			WHERE supplier_payment_id = $1 AND purchase_id = $2`,
			paymentID, purchaseID, req.Amount.String(),
		); err != nil {
			return fmt.Errorf("update allocation: %w", err)
		}
		// applying more uses more credit; applying less frees it again
		if err := adjustSupplier(ctx, tx, cur.SupplierID, diff.Neg(), diff.Neg()); err != nil {
			return err
		}
		if err := bumpSummary(ctx, tx, cur.SupplierID, decimal.Zero, decimal.Zero, decimal.Zero, diff.Neg()); err != nil {
			return err
		}

	default:
		return bad("invalid payment type")
	}

	newPaid := cur.Paid.Add(diff)
	if _, err = tx.Exec(ctx,
		`UPDATE purchases SET paid_amount = $2::numeric, payment_status = $3 WHERE id = $1`,
		purchaseID, newPaid.String(), string(paymentStatusFor(cur.Total, newPaid, cur.Returned)),
	); err != nil {
		return fmt.Errorf("update purchase: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// DeletePurchasePayment removes one payment of this purchase.
//   - a regular payment is deleted completely (as if it was never paid)
//   - a payment taken from advance credit only gives the amount back to the supplier's credit
func (r *purchaseRepository) DeletePurchasePayment(ctx context.Context, userID, purchaseID, paymentID uuid.UUID) error {
	bad := models.NewValidationError

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	cur, err := lockPurchase(ctx, tx, purchaseID)
	if err != nil {
		return err
	}

	if _, err = tx.Exec(ctx, `SELECT 1 FROM suppliers WHERE id = $1 FOR UPDATE`, cur.SupplierID); err != nil {
		return fmt.Errorf("lock supplier: %w", err)
	}

	pay, err := loadPurchasePayment(ctx, tx, purchaseID, paymentID)
	if err != nil {
		return err
	}
	amount := pay.AllocAmount

	switch pay.Type {
	case models.PaymentTypeRegular:
		if pay.AllocCount != 1 || !pay.AllocAmount.Equal(pay.PaymentAmount) {
			return bad("this payment is shared with other purchases, so it cannot be deleted here")
		}
		if _, err = tx.Exec(ctx,
			`DELETE FROM purchase_payment_allocations WHERE supplier_payment_id = $1 AND purchase_id = $2`,
			paymentID, purchaseID,
		); err != nil {
			return fmt.Errorf("delete allocation: %w", err)
		}
		if _, err = tx.Exec(ctx, `DELETE FROM supplier_payments WHERE id = $1`, paymentID); err != nil {
			return fmt.Errorf("delete supplier payment: %w", err)
		}
		// the money was never paid, so we owe it again
		if err := adjustSupplier(ctx, tx, cur.SupplierID, amount, decimal.Zero); err != nil {
			return err
		}
		if err := bumpSummary(ctx, tx, cur.SupplierID, decimal.Zero, amount.Neg(), decimal.Zero, decimal.Zero); err != nil {
			return err
		}

	case models.PaymentTypeAdvance:
		if _, err = tx.Exec(ctx,
			`DELETE FROM purchase_payment_allocations WHERE supplier_payment_id = $1 AND purchase_id = $2`,
			paymentID, purchaseID,
		); err != nil {
			return fmt.Errorf("delete allocation: %w", err)
		}
		// the advance is available again, and the bill is unpaid by that amount
		if err := adjustSupplier(ctx, tx, cur.SupplierID, amount, amount); err != nil {
			return err
		}
		if err := bumpSummary(ctx, tx, cur.SupplierID, decimal.Zero, decimal.Zero, decimal.Zero, amount); err != nil {
			return err
		}

	default:
		return bad("invalid payment type")
	}

	newPaid := decimal.Max(cur.Paid.Sub(amount), decimal.Zero)
	if _, err = tx.Exec(ctx,
		`UPDATE purchases SET paid_amount = $2::numeric, payment_status = $3 WHERE id = $1`,
		purchaseID, newPaid.String(), string(paymentStatusFor(cur.Total, newPaid, cur.Returned)),
	); err != nil {
		return fmt.Errorf("update purchase: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (r *purchaseRepository) DeletePurchase(ctx context.Context, userID, purchaseID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	cur, err := lockPurchase(ctx, tx, purchaseID)
	if err != nil {
		return err
	}

	type line struct {
		ID        uuid.UUID
		ProductID uuid.UUID
		POIID     *uuid.UUID
		Qty       decimal.Decimal
		Rate      decimal.Decimal
		Returned  decimal.Decimal
	}
	rows, err := tx.Query(ctx, `
		SELECT pi.id, pi.product_id, pi.poi_id, pi.qty, pi.rate,
		       COALESCE((SELECT SUM(pri.qty) FROM purchase_return_items pri WHERE pri.purchase_item_id = pi.id), 0)
		FROM purchase_items pi
		WHERE pi.purchase_id = $1
		ORDER BY pi.product_id`, purchaseID)
	if err != nil {
		return fmt.Errorf("load purchase items: %w", err)
	}
	var lines []line
	for rows.Next() {
		var l line
		if err := rows.Scan(&l.ID, &l.ProductID, &l.POIID, &l.Qty, &l.Rate, &l.Returned); err != nil {
			rows.Close()
			return fmt.Errorf("scan purchase item: %w", err)
		}
		lines = append(lines, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read purchase items: %w", err)
	}

	note := "Purchase bill " + cur.BillNo + " deleted"
	for _, l := range lines {
		prod, err := lockProduct(ctx, tx, l.ProductID)
		if err != nil {
			return err
		}
		// returns already took their quantity off stock
		net := l.Qty.Sub(l.Returned)
		if err := changeStock(ctx, tx, l.ProductID, prod, net.Neg(), "adjustment", "purchase_delete",
			purchaseID, note, userID); err != nil {
			return err
		}
		if err := restoreProductPrice(ctx, tx, l.ProductID, prod, l.ID, l.Rate); err != nil {
			return err
		}
		if l.POIID != nil {
			if _, err = tx.Exec(ctx,
				`UPDATE purchase_order_items SET qty_received = GREATEST(qty_received - $2::numeric, 0) WHERE id = $1`,
				*l.POIID, l.Qty.String(),
			); err != nil {
				return fmt.Errorf("update received qty: %w", err)
			}
		}
	}

	// the purchase order can be purchased again
	if cur.PurchaseOrderID != nil {
		if _, err = tx.Exec(ctx,
			`UPDATE purchase_orders SET status = $2, purchase_id = NULL WHERE id = $1`,
			*cur.PurchaseOrderID, string(models.PurchaseOrderSent),
		); err != nil {
			return fmt.Errorf("reset purchase order: %w", err)
		}
	}

	// returns go with the purchase (their items cascade)
	if _, err = tx.Exec(ctx, `DELETE FROM purchase_returns WHERE purchase_id = $1`, purchaseID); err != nil {
		return fmt.Errorf("delete returns: %w", err)
	}

	// payments stay on record; what was paid for this bill becomes supplier credit again
	if _, err = tx.Exec(ctx, `DELETE FROM purchase_payment_allocations WHERE purchase_id = $1`, purchaseID); err != nil {
		return fmt.Errorf("delete allocations: %w", err)
	}

	// returns never touched the supplier, so only the total and the payments are reversed
	if err := adjustSupplier(ctx, tx, cur.SupplierID, cur.Paid.Sub(cur.Total), cur.Paid); err != nil {
		return err
	}
	if err := bumpSummary(ctx, tx, cur.SupplierID, cur.Total.Neg(), decimal.Zero, decimal.Zero, cur.Paid); err != nil {
		return err
	}

	if _, err = tx.Exec(ctx, `DELETE FROM purchases WHERE id = $1`, purchaseID); err != nil {
		return fmt.Errorf("delete purchase: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

var (
	dHundred = decimal.NewFromInt(100)
	dOne     = decimal.NewFromInt(1)
)

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// ---------- purchase ----------

type lockedPurchase struct {
	SupplierID      uuid.UUID
	PurchaseOrderID *uuid.UUID
	BillNo          string
	Status          models.PurchaseStatus
	Total           decimal.Decimal
	Paid            decimal.Decimal
	Returned        decimal.Decimal
}

func lockPurchase(ctx context.Context, tx pgx.Tx, purchaseID uuid.UUID) (*lockedPurchase, error) {
	var p lockedPurchase
	err := tx.QueryRow(ctx, `
		SELECT supplier_id, purchase_order_id, supplier_bill_no, status, total, paid_amount, returned_amount
		FROM purchases WHERE id = $1 FOR UPDATE`,
		purchaseID,
	).Scan(&p.SupplierID, &p.PurchaseOrderID, &p.BillNo, &p.Status, &p.Total, &p.Paid, &p.Returned)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, models.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load purchase: %w", err)
	}
	if p.Status != models.PurchasePosted {
		return nil, models.NewValidationError("only posted purchases can be changed")
	}
	return &p, nil
}

func paymentStatusFor(total, paid, returned decimal.Decimal) models.PaymentStatus {
	net := total.Sub(returned)
	switch {
	case paid.GreaterThanOrEqual(net):
		return models.PaymentPaid
	case paid.IsPositive():
		return models.PaymentPartial
	default:
		return models.PaymentUnpaid
	}
}

type lockedProduct struct {
	Name         string
	Stock        decimal.Decimal
	Vat          decimal.Decimal
	Cost         decimal.Decimal
	Margin       decimal.Decimal
	AllowDecimal bool
}

func lockProduct(ctx context.Context, tx pgx.Tx, productID uuid.UUID) (*lockedProduct, error) {
	var p lockedProduct
	err := tx.QueryRow(ctx, `
		SELECT p.name, p.current_stock, p.vat_rate, p.cost_price, p.margin_percent, u.allow_decimal
		FROM products p
		JOIN units u ON u.id = p.unit_id
		WHERE p.id = $1
		FOR UPDATE OF p`,
		productID,
	).Scan(&p.Name, &p.Stock, &p.Vat, &p.Cost, &p.Margin, &p.AllowDecimal)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, models.NewValidationError("product not found")
	}
	if err != nil {
		return nil, fmt.Errorf("load product: %w", err)
	}
	return &p, nil
}

// changeStock applies a signed quantity and records the stock movement.
func changeStock(ctx context.Context, tx pgx.Tx, productID uuid.UUID, p *lockedProduct, delta decimal.Decimal,
	movementType, refType string, refID uuid.UUID, note string, userID uuid.UUID) error {
	if delta.IsZero() {
		return nil
	}
	newStock := p.Stock.Add(delta)
	if newStock.IsNegative() {
		return models.NewValidationError(fmt.Sprintf(
			"%s: only %s in stock, so this change cannot be made (items were already sold or used)",
			p.Name, p.Stock.String()))
	}
	if _, err := tx.Exec(ctx,
		`UPDATE products SET current_stock = $2::numeric, updated_at = now() WHERE id = $1`,
		productID, newStock.String(),
	); err != nil {
		return fmt.Errorf("update stock: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO stock_movements (product_id, type, qty_change, balance_after, ref_type, ref_id, note, user_id)
		VALUES ($1, $2, $3::numeric, $4::numeric, $5, $6, $7, $8)`,
		productID, movementType, delta.String(), newStock.String(), refType, refID, note, userID,
	); err != nil {
		return fmt.Errorf("insert stock movement: %w", err)
	}
	p.Stock = newStock
	return nil
}

// applyProductPrice sets the product's cost from a purchase rate (keeping its VAT and margin)
// and records the price history row.
func applyProductPrice(ctx context.Context, tx pgx.Tx, productID uuid.UUID, p *lockedProduct,
	rate decimal.Decimal, itemID uuid.UUID, date string) error {
	costWithVat := rate.Mul(dOne.Add(p.Vat.Div(dHundred))).Round(4)
	selling := costWithVat.Mul(dOne.Add(p.Margin.Div(dHundred))).Round(2)

	if _, err := tx.Exec(ctx, `
		UPDATE products
		SET cost_price = $2::numeric, cost_with_vat = $3::numeric, selling_price = $4::numeric, updated_at = now()
		WHERE id = $1`,
		productID, rate.String(), costWithVat.String(), selling.String(),
	); err != nil {
		return fmt.Errorf("update product price: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO product_price_history (
			product_id, purchase_item_id, unit_cost, unit_cost_with_vat, margin_percent, selling_price, effective_date
		) VALUES ($1, $2, $3::numeric, $4::numeric, $5::numeric, $6::numeric, $7::date)`,
		productID, itemID, rate.String(), costWithVat.String(), p.Margin.String(), selling.String(), date,
	); err != nil {
		return fmt.Errorf("insert price history: %w", err)
	}
	p.Cost = rate
	return nil
}

// restoreProductPrice removes the price history written by a purchase line and, when the
// product still carries that line's price, goes back to the latest earlier price.
func restoreProductPrice(ctx context.Context, tx pgx.Tx, productID uuid.UUID, p *lockedProduct,
	itemID uuid.UUID, rate decimal.Decimal) error {
	tag, err := tx.Exec(ctx, `DELETE FROM product_price_history WHERE purchase_item_id = $1`, itemID)
	if err != nil {
		return fmt.Errorf("delete price history: %w", err)
	}
	if tag.RowsAffected() == 0 || !p.Cost.Equal(rate) {
		return nil
	}

	var cost, costWithVat, margin, selling decimal.Decimal
	err = tx.QueryRow(ctx, `
		SELECT unit_cost, unit_cost_with_vat, margin_percent, selling_price
		FROM product_price_history
		WHERE product_id = $1
		ORDER BY effective_date DESC, created_at DESC
		LIMIT 1`,
		productID,
	).Scan(&cost, &costWithVat, &margin, &selling)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // no earlier price on record: keep the current one
	}
	if err != nil {
		return fmt.Errorf("load previous price: %w", err)
	}
	if _, err = tx.Exec(ctx, `
		UPDATE products
		SET cost_price = $2::numeric, cost_with_vat = $3::numeric, margin_percent = $4::numeric,
		    selling_price = $5::numeric, updated_at = now()
		WHERE id = $1`,
		productID, cost.String(), costWithVat.String(), margin.String(), selling.String(),
	); err != nil {
		return fmt.Errorf("restore product price: %w", err)
	}
	p.Cost = cost
	return nil
}

// ---------- supplier ----------

func adjustSupplier(ctx context.Context, tx pgx.Tx, supplierID uuid.UUID, balanceDelta, creditDelta decimal.Decimal) error {
	if _, err := tx.Exec(ctx, `
		UPDATE suppliers
		SET balance = balance + $2::numeric, credit_balance = credit_balance + $3::numeric
		WHERE id = $1`,
		supplierID, balanceDelta.String(), creditDelta.String(),
	); err != nil {
		return fmt.Errorf("update supplier: %w", err)
	}
	return nil
}

func bumpSummary(ctx context.Context, tx pgx.Tx, supplierID uuid.UUID, purchases, payments, returns, unallocated decimal.Decimal) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO supplier_financial_summaries AS s
			(supplier_id, total_purchases, total_payments, total_returns, unallocated_payments)
		VALUES ($1, GREATEST($2::numeric, 0), GREATEST($3::numeric, 0), GREATEST($4::numeric, 0), GREATEST($5::numeric, 0))
		ON CONFLICT (supplier_id) DO UPDATE SET
			total_purchases      = GREATEST(s.total_purchases      + $2::numeric, 0),
			total_payments       = GREATEST(s.total_payments       + $3::numeric, 0),
			total_returns        = GREATEST(s.total_returns        + $4::numeric, 0),
			unallocated_payments = GREATEST(s.unallocated_payments + $5::numeric, 0),
			updated_at = now()`,
		supplierID, purchases.String(), payments.String(), returns.String(), unallocated.String(),
	); err != nil {
		return fmt.Errorf("update supplier summary: %w", err)
	}
	return nil
}

func allocatePayment(ctx context.Context, tx pgx.Tx, paymentID, purchaseID uuid.UUID, amount decimal.Decimal) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO purchase_payment_allocations (supplier_payment_id, purchase_id, amount)
		VALUES ($1, $2, $3::numeric)
		ON CONFLICT (supplier_payment_id, purchase_id)
		DO UPDATE SET amount = purchase_payment_allocations.amount + EXCLUDED.amount`,
		paymentID, purchaseID, amount.String(),
	); err != nil {
		return fmt.Errorf("allocate payment: %w", err)
	}
	return nil
}

type purchasePaymentInfo struct {
	Type          models.PaymentType
	PaymentAmount decimal.Decimal // the whole payment
	AllocAmount   decimal.Decimal // the part applied to this purchase
	AllocCount    int             // how many purchases the payment is applied to
	AllocSum      decimal.Decimal // the part applied to all purchases
}

// loadPurchasePayment locks and reads a payment together with its allocation to this purchase.
func loadPurchasePayment(ctx context.Context, tx pgx.Tx, purchaseID, paymentID uuid.UUID) (*purchasePaymentInfo, error) {
	var p purchasePaymentInfo
	err := tx.QueryRow(ctx, `
		SELECT sp.payment_type, sp.amount, pa.amount,
		       (SELECT COUNT(*) FROM purchase_payment_allocations x WHERE x.supplier_payment_id = sp.id),
		       (SELECT COALESCE(SUM(x.amount), 0) FROM purchase_payment_allocations x WHERE x.supplier_payment_id = sp.id)
		FROM supplier_payments sp
		JOIN purchase_payment_allocations pa ON pa.supplier_payment_id = sp.id AND pa.purchase_id = $2
		WHERE sp.id = $1
		FOR UPDATE OF sp, pa`,
		paymentID, purchaseID,
	).Scan(&p.Type, &p.PaymentAmount, &p.AllocAmount, &p.AllocCount, &p.AllocSum)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, models.NewValidationError("payment not found on this purchase")
	}
	if err != nil {
		return nil, fmt.Errorf("load payment: %w", err)
	}
	return &p, nil
}

// NEW FILE: internals/repository/purchase_supplier_lists.go
//
// Also add these two lines to the PurchaseRepository interface:
//
//	GetSupplierPurchases(ctx context.Context, f models.SupplierPurchaseListFilter) ([]models.SupplierPurchaseItem, int, error)
//	GetSupplierReturns(ctx context.Context, f models.SupplierReturnListFilter) ([]models.SupplierReturnItem, int, error)

const dateLayout = "2006-01-02"

func (r *purchaseRepository) GetSupplierPurchases(ctx context.Context, f models.SupplierPurchaseListFilter) ([]models.SupplierPurchaseItem, int, error) {
	conds := []string{"p.supplier_id = $1"}
	args := []any{f.SupplierID}

	if f.PaymentStatus != nil {
		args = append(args, string(*f.PaymentStatus))
		conds = append(conds, fmt.Sprintf("p.payment_status = $%d", len(args)))
	}
	if f.PurchaseStatus != nil {
		args = append(args, string(*f.PurchaseStatus))
		conds = append(conds, fmt.Sprintf("p.status = $%d", len(args)))
	}
	if f.FromDate != nil {
		args = append(args, f.FromDate.Format(dateLayout))
		conds = append(conds, fmt.Sprintf("p.purchase_date >= $%d::date", len(args)))
	}
	if f.ToDate != nil {
		args = append(args, f.ToDate.Format(dateLayout))
		conds = append(conds, fmt.Sprintf("p.purchase_date <= $%d::date", len(args)))
	}
	filterSQL := strings.Join(conds, " AND ")

	const fromSQL = `
		FROM purchases p
		JOIN suppliers s ON s.id = p.supplier_id
		LEFT JOIN purchase_orders po ON po.id = p.purchase_order_id
	`

	// the count ignores the cursor, so the total is the same on every page
	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) "+fromSQL+" WHERE "+filterSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count supplier purchases: %w", err)
	}
	if total == 0 {
		return []models.SupplierPurchaseItem{}, 0, nil
	}

	listSQL := filterSQL
	listArgs := append([]any{}, args...)
	if f.Cursor != nil {
		listArgs = append(listArgs, f.Cursor.Date, f.Cursor.CreatedAt, f.Cursor.ID)
		n := len(listArgs)
		listSQL += fmt.Sprintf(
			" AND (p.purchase_date, p.created_at, p.id) < ($%d::date, $%d::timestamptz, $%d::uuid)", n-2, n-1, n)
	}
	listArgs = append(listArgs, f.Limit+1) // one extra row tells us if there is a next page

	query := fmt.Sprintf(`
		SELECT p.id, p.supplier_id, s.name, p.purchase_order_id, po.po_no, p.supplier_bill_no,
		       to_char(p.purchase_date, 'YYYY-MM-DD'),
		       p.total, p.paid_amount, p.due_amount, p.payment_status, p.status, p.note, p.created_at
		%s
		WHERE %s
		ORDER BY p.purchase_date DESC, p.created_at DESC, p.id DESC
		LIMIT $%d`, fromSQL, listSQL, len(listArgs))

	rows, err := r.pool.Query(ctx, query, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query supplier purchases: %w", err)
	}
	defer rows.Close()

	items := make([]models.SupplierPurchaseItem, 0, f.Limit+1)
	for rows.Next() {
		var it models.SupplierPurchaseItem
		if err := rows.Scan(
			&it.PurchaseID, &it.SupplierID, &it.SupplierName, &it.PurchaseOrderID, &it.PurchaseOrderNumber,
			&it.SupplierBillNo, &it.PurchaseDate,
			&it.Total, &it.PaidAmount, &it.DueAmount, &it.PaymentStatus, &it.Status, &it.Note, &it.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan supplier purchase: %w", err)
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("read supplier purchases: %w", err)
	}
	return items, total, nil
}

func (r *purchaseRepository) GetSupplierReturns(ctx context.Context, f models.SupplierReturnListFilter) ([]models.SupplierReturnItem, int, error) {
	conds := []string{"r.supplier_id = $1"}
	args := []any{f.SupplierID}

	if f.FromDate != nil {
		args = append(args, f.FromDate.Format(dateLayout))
		conds = append(conds, fmt.Sprintf("r.return_date >= $%d::date", len(args)))
	}
	if f.ToDate != nil {
		args = append(args, f.ToDate.Format(dateLayout))
		conds = append(conds, fmt.Sprintf("r.return_date <= $%d::date", len(args)))
	}
	filterSQL := strings.Join(conds, " AND ")

	const fromSQL = `
		FROM purchase_returns r
		JOIN purchases p ON p.id = r.purchase_id
	`

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) "+fromSQL+" WHERE "+filterSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count supplier returns: %w", err)
	}
	if total == 0 {
		return []models.SupplierReturnItem{}, 0, nil
	}

	listSQL := filterSQL
	listArgs := append([]any{}, args...)
	if f.Cursor != nil {
		listArgs = append(listArgs, f.Cursor.Date, f.Cursor.CreatedAt, f.Cursor.ID)
		n := len(listArgs)
		listSQL += fmt.Sprintf(
			" AND (r.return_date, r.created_at, r.id) < ($%d::date, $%d::timestamptz, $%d::uuid)", n-2, n-1, n)
	}
	listArgs = append(listArgs, f.Limit+1)

	query := fmt.Sprintf(`
		SELECT r.id, r.return_no, r.purchase_id, p.supplier_bill_no, r.supplier_id,
		       to_char(r.return_date, 'YYYY-MM-DD'), r.total, r.note, r.created_at
		%s
		WHERE %s
		ORDER BY r.return_date DESC, r.created_at DESC, r.id DESC
		LIMIT $%d`, fromSQL, listSQL, len(listArgs))

	rows, err := r.pool.Query(ctx, query, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query supplier returns: %w", err)
	}
	defer rows.Close()

	items := make([]models.SupplierReturnItem, 0, f.Limit+1)
	for rows.Next() {
		var it models.SupplierReturnItem
		if err := rows.Scan(
			&it.PurchaseReturnID, &it.ReturnNo, &it.PurchaseID, &it.SupplierBillNo, &it.SupplierID,
			&it.ReturnDate, &it.Total, &it.Note, &it.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan supplier return: %w", err)
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("read supplier returns: %w", err)
	}
	return items, total, nil
}
