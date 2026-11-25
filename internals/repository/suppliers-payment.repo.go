package repository

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"inventory/internals/database"
	"inventory/internals/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

type SupplierPaymentRepository interface {
	CreateAdvancePaymentForSupplier(ctx context.Context, data *models.SupplierPaymentCreationType, userID uuid.UUID) (uuid.UUID, error)

	// GetSupplierPayments returns up to Limit+1 payments (the extra row tells the service
	// there is another page), each with its allocations, and the total matching the filters.
	GetSupplierPayments(ctx context.Context, f models.SupplierPaymentListFilter) ([]models.SupplierIDPaymentResponse, int, error)
	GetSupplierFinancialSummary(ctx context.Context, supplierID uuid.UUID) (*models.SupplierFinancialSummary, error)
	GetSupplierLedger(ctx context.Context, f models.SupplierLedgerFilter) (*models.SupplierLedgerResponse, error)
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

// ---------- reads ----------

// al.allocated is the sum of every allocation of the payment (0 when there is none)
const supplierPaymentFromSQL = `
	FROM supplier_payments sp
	LEFT JOIN users u ON u.id = sp.created_by
	LEFT JOIN supplier_bank_accounts ba ON ba.id = sp.supplier_bank_account_id
	LEFT JOIN LATERAL (
		SELECT COALESCE(SUM(a.amount), 0) AS allocated
		FROM purchase_payment_allocations a
		WHERE a.supplier_payment_id = sp.id
	) al ON true`

func (r *supplierPaymentRepository) GetSupplierPayments(
	ctx context.Context,
	f models.SupplierPaymentListFilter,
) ([]models.SupplierIDPaymentResponse, int, error) {
	conds := []string{"sp.supplier_id = $1"}
	args := []any{f.SupplierID}

	if f.Status != nil {
		switch *f.Status {
		case models.SupplierPaymentUnallocated:
			conds = append(conds, "al.allocated = 0")
		case models.SupplierPaymentPartial:
			conds = append(conds, "al.allocated > 0 AND al.allocated < sp.amount")
		case models.SupplierPaymentAllocated:
			conds = append(conds, "al.allocated >= sp.amount")
		}
	}

	if f.FromDate != nil {
		args = append(args, f.FromDate.Format("2006-01-02"))
		conds = append(conds, fmt.Sprintf("sp.payment_date >= $%d::date", len(args)))
	}

	if f.ToDate != nil {
		args = append(args, f.ToDate.Format("2006-01-02"))
		conds = append(conds, fmt.Sprintf("sp.payment_date <= $%d::date", len(args)))
	}

	filterSQL := strings.Join(conds, " AND ")

	// the count ignores the cursor so the total is the same on every page
	var total int
	if err := r.pool.QueryRow(ctx,
		"SELECT COUNT(*) "+supplierPaymentFromSQL+" WHERE "+filterSQL,
		args...,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count supplier payments: %w", err)
	}

	if total == 0 {
		return []models.SupplierIDPaymentResponse{}, 0, nil
	}

	listSQL := filterSQL
	listArgs := append([]any{}, args...)

	if f.Cursor != nil {
		listArgs = append(listArgs, f.Cursor.PaymentDate, f.Cursor.CreatedAt, f.Cursor.ID)
		listSQL += fmt.Sprintf(
			" AND (sp.payment_date, sp.created_at, sp.id) < ($%d::date, $%d, $%d)",
			len(listArgs)-2, len(listArgs)-1, len(listArgs),
		)
	}

	listArgs = append(listArgs, f.Limit+1)

	query := fmt.Sprintf(`
		SELECT
			sp.id, sp.supplier_id, sp.amount, sp.payment_date,
			sp.payment_method, sp.payment_type, sp.note,
			al.allocated,
			sp.created_by, u.name, sp.created_at,
			sp.supplier_bank_account_id, ba.bank_name, ba.account_name, ba.account_number
		%s
		WHERE %s
		ORDER BY sp.payment_date DESC, sp.created_at DESC, sp.id DESC
		LIMIT $%d`, supplierPaymentFromSQL, listSQL, len(listArgs))

	rows, err := r.pool.Query(ctx, query, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query supplier payments: %w", err)
	}

	payments := make([]models.SupplierIDPaymentResponse, 0, f.Limit+1)
	for rows.Next() {
		var (
			p             models.SupplierIDPaymentResponse
			method, pType string
		)
		if err := rows.Scan(
			&p.ID, &p.SupplierID, &p.Amount, &p.PaymentDate,
			&method, &pType, &p.Note,
			&p.AllocatedAmount,
			&p.CreatedBy, &p.CreatedByName, &p.CreatedAt,
			&p.SupplierBankAccountID, &p.BankName, &p.AccountName, &p.AccountNumber,
		); err != nil {
			rows.Close()
			return nil, 0, fmt.Errorf("scan supplier payment: %w", err)
		}
		p.PaymentMethod = models.PaymentMethod(method)
		p.PaymentType = models.PaymentType(pType)
		p.UnallocatedAmount = p.Amount.Sub(p.AllocatedAmount)
		p.Allocations = []models.SupplierPaymentAllocationResponse{}
		payments = append(payments, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("read supplier payments: %w", err)
	}

	if err := r.attachAllocations(ctx, payments); err != nil {
		return nil, 0, err
	}

	return payments, total, nil
}

// attachAllocations loads the allocations of every payment on the page with one query.
func (r *supplierPaymentRepository) attachAllocations(ctx context.Context, payments []models.SupplierIDPaymentResponse) error {
	if len(payments) == 0 {
		return nil
	}

	ids := make([]uuid.UUID, len(payments))
	index := make(map[uuid.UUID]int, len(payments))
	for i, p := range payments {
		ids[i] = p.ID
		index[p.ID] = i
	}

	rows, err := r.pool.Query(ctx, `
		SELECT a.supplier_payment_id, a.purchase_id, p.supplier_bill_no,
		       p.purchase_date, p.total, a.amount
		FROM purchase_payment_allocations a
		JOIN purchases p ON p.id = a.purchase_id
		WHERE a.supplier_payment_id = ANY($1::uuid[])
		ORDER BY p.purchase_date DESC, a.created_at DESC`, uuidStrings(ids))
	if err != nil {
		return fmt.Errorf("query payment allocations: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			paymentID uuid.UUID
			a         models.SupplierPaymentAllocationResponse
		)
		if err := rows.Scan(
			&paymentID, &a.PurchaseID, &a.SupplierBillNo,
			&a.PurchaseDate, &a.PurchaseTotal, &a.AllocatedAmount,
		); err != nil {
			return fmt.Errorf("scan payment allocation: %w", err)
		}
		i := index[paymentID]
		payments[i].Allocations = append(payments[i].Allocations, a)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read payment allocations: %w", err)
	}

	return nil
}

func (r *supplierPaymentRepository) GetSupplierFinancialSummary(
	ctx context.Context,
	supplierID uuid.UUID,
) (*models.SupplierFinancialSummary, error) {
	var s models.SupplierFinancialSummary

	err := r.pool.QueryRow(ctx, `
		SELECT total_purchases, total_payments, total_returns, unallocated_payments
		FROM supplier_financial_summaries
		WHERE supplier_id = $1`, supplierID,
	).Scan(&s.TotalPurchases, &s.TotalPayments, &s.TotalReturns, &s.UnallocatedPayments)

	// a supplier with no activity yet has no row: every number is zero
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("get supplier financial summary: %w", err)
	}

	return &s, nil
}

// ---------- writes ----------

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

	if err := verifyPaymentBankAccount(ctx, tx, data.SupplierID, data.SupplierBankAccountID, false); err != nil {
		return uuid.Nil, err
	}

	// 2. Insert the advance payment
	var paymentID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO supplier_payments (
			supplier_id, amount, payment_date,
			payment_method, payment_type, note, created_by, supplier_bank_account_id
		)
		VALUES ($1, $2, COALESCE($3::date, CURRENT_DATE), $4, 'advance', $5, $6, $7)
		RETURNING id
	`,
		data.SupplierID,
		data.Amount,
		data.PaymentDate,
		string(data.PaymentMethod),
		data.Note,
		userID,
		data.SupplierBankAccountID,
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

const supplierLedgerCTE = `
	WITH entries AS (
		SELECT 'purchase'::text AS entry_type, p.id, p.purchase_date AS entry_date,
		       p.created_at, 1 AS sort_rank,
		       p.supplier_bill_no::text AS reference,
		       'Purchase'::text AS description,
		       p.note::text AS note,
		       p.total AS amount,
		       p.total AS owed_delta,
		       0::numeric AS adv_delta,
		       NULL::uuid AS bank_account_id,
		       NULL::text AS bank_name,
		       NULL::text AS account_name,
		       NULL::text AS account_number
		FROM purchases p
		WHERE p.supplier_id = $1 AND p.status = 'posted'

		UNION ALL

		SELECT 'advance', sp.id, sp.payment_date, sp.created_at, 2,
		       NULL::text,
		       ('Advance paid (' || sp.payment_method || ')')::text,
		       sp.note::text,
		       sp.amount,
		       0::numeric,
		       sp.amount,
		       ba.id, ba.bank_name::text, ba.account_name::text, ba.account_number::text
		FROM supplier_payments sp
		LEFT JOIN supplier_bank_accounts ba ON ba.id = sp.supplier_bank_account_id
		WHERE sp.supplier_id = $1 AND sp.payment_type = 'advance'

		UNION ALL

		SELECT 'payment', sp.id, sp.payment_date, sp.created_at, 3,
		       NULL::text,
		       ('Payment (' || sp.payment_method || ')')::text,
		       sp.note::text,
		       sp.amount,
		       -sp.amount,
		       0::numeric,
		       ba.id, ba.bank_name::text, ba.account_name::text, ba.account_number::text
		FROM supplier_payments sp
		LEFT JOIN supplier_bank_accounts ba ON ba.id = sp.supplier_bank_account_id
		WHERE sp.supplier_id = $1 AND sp.payment_type = 'regular'

		UNION ALL

		SELECT 'advance_allocation', a.id,
		       GREATEST(p.purchase_date, sp.payment_date),
		       a.created_at, 3,
		       p.supplier_bill_no::text,
		       ('Advance applied to purchase ' || p.supplier_bill_no)::text,
		       sp.note::text,
		       a.amount,
		       -a.amount,
		       -a.amount,
		       ba.id, ba.bank_name::text, ba.account_name::text, ba.account_number::text
		FROM purchase_payment_allocations a
		JOIN supplier_payments sp ON sp.id = a.supplier_payment_id
		JOIN purchases p          ON p.id  = a.purchase_id
		LEFT JOIN supplier_bank_accounts ba ON ba.id = sp.supplier_bank_account_id
		WHERE sp.supplier_id = $1
		  AND sp.payment_type = 'advance'
		  AND p.status = 'posted'

		UNION ALL

		SELECT 'return', pr.id, pr.return_date, pr.created_at, 4,
		       pr.return_no::text,
		       'Purchase return'::text,
		       pr.note::text,
		       pr.total,
		       -pr.total,
		       0::numeric,
		       NULL::uuid, NULL::text, NULL::text, NULL::text
		FROM purchase_returns pr
		WHERE pr.supplier_id = $1
	),
	ledger AS (
		SELECT e.*,
		       s.opening_owed + SUM(e.owed_delta) OVER w AS owed_net,
		       s.opening_adv  + SUM(e.adv_delta)  OVER w AS adv_net
		FROM entries e
		CROSS JOIN (
			SELECT GREATEST(opening_balance, 0)  AS opening_owed,
			       GREATEST(-opening_balance, 0) AS opening_adv
			FROM suppliers
			WHERE id = $1
		) s
		WINDOW w AS (
			ORDER BY e.entry_date, e.created_at, e.sort_rank, e.id
			ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
		)
	)`

func (r *supplierPaymentRepository) GetSupplierLedger(
	ctx context.Context,
	f models.SupplierLedgerFilter,
) (*models.SupplierLedgerResponse, error) {
	res := &models.SupplierLedgerResponse{Entries: []models.SupplierLedgerEntry{}}

	// 1. supplier (also the existence check)
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, opening_balance
		FROM suppliers
		WHERE id = $1`, f.SupplierID,
	).Scan(&res.Supplier.ID, &res.Supplier.Name, &res.Supplier.OpeningBalance)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSupplierNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get ledger supplier: %w", err)
	}

	// 2. date filters
	args := []any{f.SupplierID}
	conds := []string{"TRUE"}
	beforeCond := "FALSE" // no from date: nothing happens before the period

	if f.FromDate != nil {
		args = append(args, f.FromDate.Format("2006-01-02"))
		conds = append(conds, fmt.Sprintf("entry_date >= $%d::date", len(args)))
		beforeCond = fmt.Sprintf("entry_date < $%d::date", len(args))
	}
	if f.ToDate != nil {
		args = append(args, f.ToDate.Format("2006-01-02"))
		conds = append(conds, fmt.Sprintf("entry_date <= $%d::date", len(args)))
	}
	rangeSQL := strings.Join(conds, " AND ")

	// 3. period figures (ignore the cursor so they are the same on every page)
	var beforeOwed, beforeAdv, periodOwed, periodAdv decimal.Decimal

	err = r.pool.QueryRow(ctx, fmt.Sprintf(`%[1]s
		SELECT
			COUNT(*) FILTER (WHERE %[2]s),
			COALESCE(SUM(amount) FILTER (WHERE %[2]s AND entry_type = 'purchase'), 0),
			COALESCE(SUM(amount) FILTER (WHERE %[2]s AND entry_type IN ('payment', 'advance_allocation')), 0),
			COALESCE(SUM(amount) FILTER (WHERE %[2]s AND entry_type = 'return'), 0),
			COALESCE(SUM(amount) FILTER (WHERE %[2]s AND entry_type = 'advance'), 0),
			COALESCE(SUM(owed_delta) FILTER (WHERE %[3]s), 0),
			COALESCE(SUM(adv_delta)  FILTER (WHERE %[3]s), 0),
			COALESCE(SUM(owed_delta) FILTER (WHERE %[2]s), 0),
			COALESCE(SUM(adv_delta)  FILTER (WHERE %[2]s), 0)
		FROM entries`, supplierLedgerCTE, rangeSQL, beforeCond),
		args...,
	).Scan(
		&res.Total,
		&res.TotalPurchases,
		&res.TotalPayments,
		&res.TotalReturns,
		&res.TotalAdvances,
		&beforeOwed, &beforeAdv,
		&periodOwed, &periodAdv,
	)
	if err != nil {
		return nil, fmt.Errorf("ledger totals: %w", err)
	}

	// 4. opening and closing positions
	// a positive opening balance is money we owe, a negative one is an advance
	openOwed, openAdv := splitOpening(res.Supplier.OpeningBalance)

	openingOwed := openOwed.Add(beforeOwed)
	openingAdv := openAdv.Add(beforeAdv)
	closingOwed := openingOwed.Add(periodOwed)
	closingAdv := openingAdv.Add(periodAdv)

	res.OpeningBalance, res.OpeningCredit = position(openingOwed, openingAdv)
	res.ClosingBalance, res.ClosingCredit = position(closingOwed, closingAdv)

	if res.Total == 0 {
		return res, nil
	}

	// 5. the page
	listArgs := append([]any{}, args...)
	listSQL := rangeSQL

	if f.Cursor != nil {
		listArgs = append(listArgs,
			f.Cursor.EntryDate, f.Cursor.CreatedAt, f.Cursor.Rank, f.Cursor.ID)
		n := len(listArgs)
		listSQL += fmt.Sprintf(
			" AND (entry_date, created_at, sort_rank, id) < ($%d::date, $%d, $%d::int, $%d)",
			n-3, n-2, n-1, n,
		)
	}
	listArgs = append(listArgs, f.Limit+1)

	rows, err := r.pool.Query(ctx, fmt.Sprintf(`%s
		SELECT entry_type, id, entry_date, created_at, sort_rank,
		       reference, description, note, amount,
		       GREATEST(owed_net, 0),
		       adv_net + GREATEST(-owed_net, 0),
		       bank_account_id, bank_name, account_name, account_number
		FROM ledger
		WHERE %s
		ORDER BY entry_date DESC, created_at DESC, sort_rank DESC, id DESC
		LIMIT $%d`, supplierLedgerCTE, listSQL, len(listArgs)),
		listArgs...,
	)
	if err != nil {
		return nil, fmt.Errorf("query supplier ledger: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			e     models.SupplierLedgerEntry
			eType string
		)
		if err := rows.Scan(
			&eType, &e.ID, &e.EntryDate, &e.CreatedAt, &e.SortRank,
			&e.Reference, &e.Description, &e.Note, &e.Amount,
			&e.Balance, &e.Credit,
			&e.SupplierBankAccountID, &e.BankName, &e.AccountName, &e.AccountNumber,
		); err != nil {
			return nil, fmt.Errorf("scan ledger entry: %w", err)
		}
		e.EntryType = models.SupplierLedgerEntryType(eType)
		res.Entries = append(res.Entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read supplier ledger: %w", err)
	}

	return res, nil
}

// splitOpening turns suppliers.opening_balance into (we owe, advance held).
func splitOpening(opening decimal.Decimal) (owed, advance decimal.Decimal) {
	if opening.IsPositive() {
		return opening, decimal.Zero
	}
	return decimal.Zero, opening.Neg()
}

// position turns the two running tracks into the numbers shown on a line.
// If owed went below zero (a return after full payment) the excess is extra credit.
func position(owed, advance decimal.Decimal) (balance, credit decimal.Decimal) {
	if owed.IsNegative() {
		return decimal.Zero, advance.Add(owed.Neg())
	}
	return owed, advance
}
