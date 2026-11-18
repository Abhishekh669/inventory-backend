package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"inventory/internals/models"
	"inventory/internals/repository"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type PurchaseService interface {
	GetPurchaseDataById(ctx context.Context, purchaseID uuid.UUID) (*models.PurchaseByIdResponse, error)

	GetAllPurchases(ctx context.Context, q models.PurchaseListQuery) (*models.GetAllPurchaseResponse, error)

	CreatePurchase(ctx context.Context, userID uuid.UUID, req models.CreatePurchaseRequest) (uuid.UUID, error)
	UpdatePurchase(ctx context.Context, userID, purchaseID uuid.UUID, req models.UpdatePurchaseRequest) error
	DeletePurchase(ctx context.Context, userID, purchaseID uuid.UUID) error

	CreatePurchaseReturn(ctx context.Context, userID, purchaseID uuid.UUID, req models.CreatePurchaseReturnRequest) (uuid.UUID, error)
	UpdatePurchaseReturn(ctx context.Context, userID, purchaseID, returnID uuid.UUID, req models.CreatePurchaseReturnRequest) error
	DeletePurchaseReturn(ctx context.Context, userID, purchaseID, returnID uuid.UUID) error

	CreatePurchasePayment(ctx context.Context, userID, purchaseID uuid.UUID, req models.CreatePurchasePaymentRequest) (uuid.UUID, error)
	UpdatePurchasePayment(ctx context.Context, userID, purchaseID, paymentID uuid.UUID, req models.UpdatePurchasePaymentRequest) error
	DeletePurchasePayment(ctx context.Context, userID, purchaseID, paymentID uuid.UUID) error

	GetSupplierPurchases(ctx context.Context, supplierID uuid.UUID, q models.SupplierPurchaseListQuery) (*models.GetSupplierPurchasesResponse, error)
	GetSupplierReturns(ctx context.Context, supplierID uuid.UUID, q models.SupplierReturnListQuery) (*models.GetSupplierReturnsResponse, error)
}
type purchaseService struct {
	repo repository.PurchaseRepository
}

func NewPurchaseService(repo repository.PurchaseRepository) PurchaseService {
	return &purchaseService{repo: repo}
}

const (
	defaultPurchaseListLimit = 20
	maxPurchaseListLimit     = 100
	maxPurchaseSearchLen     = 100
	maxPONumberLen           = 30 // matches po_no VARCHAR(30)
)

func isValidPurchaseStatus(s models.PurchaseStatus) bool {
	switch s {
	case models.PurchaseDraft, models.PurchasePosted, models.PurchaseCancelled:
		return true
	}
	return false
}

func isValidPaymentStatus(s models.PaymentStatus) bool {
	switch s {
	case models.PaymentUnpaid, models.PaymentPartial, models.PaymentPaid:
		return true
	}
	return false
}

func encodePurchaseListCursor(it models.PurchaseListItem) string {
	b, _ := json.Marshal(models.PurchaseListCursor{
		CreatedAt: it.PurchaseOrderData.PurchaseOrderedAt,
		ID:        it.PurchaseOrderData.PurchaseOrderID,
	})
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodePurchaseListCursor(raw string) (*models.PurchaseListCursor, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, invalidf("invalid cursor")
	}

	var c models.PurchaseListCursor
	if err := json.Unmarshal(b, &c); err != nil || c.ID == uuid.Nil || c.CreatedAt.IsZero() {
		return nil, invalidf("invalid cursor")
	}

	return &c, nil
}

func (s *purchaseService) GetAllPurchases(ctx context.Context, q models.PurchaseListQuery) (*models.GetAllPurchaseResponse, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = defaultPurchaseListLimit
	}
	if limit > maxPurchaseListLimit {
		limit = maxPurchaseListLimit
	}
	filter := models.PurchaseListFilter{Limit: limit}

	if raw := strings.TrimSpace(q.PurchaseOrderStatus); raw != "" {
		st := models.PurchaseOrderStatus(raw)
		if !isValidPurchaseOrderStatus(st) || st == models.PurchaseOrderDraft {
			return nil, invalidf("unknown purchase_order_status %q", raw)
		}
		filter.PurchaseOrderStatus = &st
	}

	if raw := strings.TrimSpace(q.PurchaseStatus); raw != "" {
		st := models.PurchaseStatus(raw)
		if !isValidPurchaseStatus(st) {
			return nil, invalidf("unknown purchase_status %q", raw)
		}
		filter.PurchaseStatus = &st
	}

	if raw := strings.TrimSpace(q.PaymentStatus); raw != "" {
		ps := models.PaymentStatus(raw)
		if !isValidPaymentStatus(ps) {
			return nil, invalidf("unknown payment_status %q", raw)
		}
		filter.PaymentStatus = &ps
	}

	if raw := strings.TrimSpace(q.PurchaseOrderNumber); raw != "" {
		if utf8.RuneCountInString(raw) > maxPONumberLen {
			return nil, invalidf("po_number must be at most %d characters", maxPONumberLen)
		}
		filter.PurchaseOrderNumber = &raw
	}

	if raw := strings.TrimSpace(q.Search); raw != "" {
		if utf8.RuneCountInString(raw) > maxPurchaseSearchLen {
			return nil, invalidf("search must be at most %d characters", maxPurchaseSearchLen)
		}
		filter.Search = &raw
	}

	from, err := parseDay(q.FromDate)
	if err != nil {
		return nil, err
	}
	to, err := parseDay(q.ToDate)
	if err != nil {
		return nil, err
	}
	if from != nil && to != nil && from.After(*to) {
		return nil, invalidf("from_date must be on or before to_date")
	}
	filter.FromDate = from
	if to != nil {
		end := to.AddDate(0, 0, 1) // to_date is inclusive
		filter.ToDate = &end
	}

	cursor, err := decodePurchaseListCursor(q.Cursor)
	if err != nil {
		return nil, err
	}
	filter.Cursor = cursor

	items, total, err := s.repo.GetAllPurchase(ctx, filter)
	if err != nil {
		return nil, err
	}

	resp := &models.GetAllPurchaseResponse{Items: items, Total: total}
	if len(items) > limit {
		resp.Items = items[:limit]
		resp.HasMore = true
		next := encodePurchaseListCursor(items[limit-1])
		resp.NextCursor = &next
	}

	return resp, nil
}

func (s *purchaseService) CreatePurchase(ctx context.Context, userID uuid.UUID, req models.CreatePurchaseRequest) (uuid.UUID, error) {
	if req.PurchaseOrderID == uuid.Nil {
		return uuid.Nil, models.NewValidationError("purchase order is required")
	}
	if err := normalizeAndValidate(&req); err != nil {
		return uuid.Nil, err
	}
	calc, err := calculatePurchase(req)
	if err != nil {
		return uuid.Nil, err
	}
	return s.repo.CreatePurchase(ctx, userID, calc)
}

func (s *purchaseService) UpdatePurchase(ctx context.Context, userID, purchaseID uuid.UUID, req models.UpdatePurchaseRequest) error {
	if purchaseID == uuid.Nil {
		return models.NewValidationError("invalid purchase id")
	}
	// same validation and bill calculation as a new purchase; payments are untouched
	shell := models.CreatePurchaseRequest{
		SupplierBillNo:  req.SupplierBillNo,
		PurchaseDate:    req.PurchaseDate,
		DueDate:         req.DueDate,
		ExciseDuty:      req.ExciseDuty,
		SpecialDiscount: req.SpecialDiscount,
		Discount:        req.Discount,
		VatRate:         req.VatRate,
		FreightCharge:   req.FreightCharge,
		Note:            req.Note,
		Items:           req.Items,
	}
	if err := normalizeAndValidate(&shell); err != nil {
		return err
	}
	calc, err := calculatePurchase(shell)
	if err != nil {
		return err
	}
	return s.repo.UpdatePurchase(ctx, userID, purchaseID, calc)
}

func (s *purchaseService) DeletePurchase(ctx context.Context, userID, purchaseID uuid.UUID) error {
	if purchaseID == uuid.Nil {
		return models.NewValidationError("invalid purchase id")
	}
	return s.repo.DeletePurchase(ctx, userID, purchaseID)
}

func normalizeAndValidate(req *models.CreatePurchaseRequest) error {
	bad := models.NewValidationError

	req.SupplierBillNo = strings.TrimSpace(req.SupplierBillNo)
	if req.SupplierBillNo == "" || len(req.SupplierBillNo) > 50 {
		return bad("supplier bill number is required (max 50 characters)")
	}

	purchaseDate, err := time.Parse("2006-01-02", req.PurchaseDate)
	if err != nil {
		return bad("invalid purchase date")
	}
	if req.DueDate != nil {
		d := strings.TrimSpace(*req.DueDate)
		if d == "" {
			req.DueDate = nil
		} else {
			due, err := time.Parse("2006-01-02", d)
			if err != nil {
				return bad("invalid due date")
			}
			if due.Before(purchaseDate) {
				return bad("due date cannot be before the purchase date")
			}
			req.DueDate = &d
		}
	}
	if req.Note != nil {
		n := strings.TrimSpace(*req.Note)
		if n == "" {
			req.Note = nil
		} else {
			req.Note = &n
		}
	}

	for _, v := range []decimal.Decimal{req.ExciseDuty, req.SpecialDiscount, req.Discount, req.FreightCharge, req.PaidAmount} {
		if v.IsNegative() {
			return bad("amounts cannot be negative")
		}
	}
	if req.VatRate.IsNegative() || req.VatRate.GreaterThan(decimal.NewFromInt(100)) {
		return bad("VAT rate must be between 0 and 100")
	}

	// the table columns are NUMERIC(.., 2): round now so stored values equal calculated ones
	req.ExciseDuty = req.ExciseDuty.Round(2)
	req.SpecialDiscount = req.SpecialDiscount.Round(2)
	req.Discount = req.Discount.Round(2)
	req.FreightCharge = req.FreightCharge.Round(2)
	req.PaidAmount = req.PaidAmount.Round(2)
	req.VatRate = req.VatRate.Round(2)

	switch req.PaymentMethod {
	case models.PaymentMethodCash, models.PaymentMethodBank,
		models.PaymentMethodCheque, models.PaymentMethodOnline:
	case "":
		req.PaymentMethod = models.PaymentMethodCash
	default:
		return bad("invalid payment method")
	}

	items := make([]models.CreatePurchaseItemRequest, 0, len(req.Items))
	seen := map[uuid.UUID]bool{}
	for _, it := range req.Items {
		if it.ProductID == uuid.Nil {
			return bad("product is required")
		}
		if it.Qty.IsNegative() {
			return bad("quantity cannot be negative")
		}
		it.Qty = it.Qty.Round(3)   // qty column is NUMERIC(14,3)
		it.Rate = it.Rate.Round(4) // rate column is NUMERIC(14,4)
		if it.Qty.IsZero() {
			continue // not received on this purchase
		}
		if !it.Rate.IsPositive() {
			return bad("rate must be greater than zero")
		}
		if seen[it.ProductID] {
			return bad("a product appears more than once")
		}
		seen[it.ProductID] = true
		items = append(items, it)
	}
	if len(items) == 0 {
		return bad("enter a quantity for at least one item")
	}

	// lock products in a fixed order so concurrent purchases cannot deadlock
	sort.Slice(items, func(i, j int) bool {
		return items[i].ProductID.String() < items[j].ProductID.String()
	})
	req.Items = items
	return nil
}

// Discount and excise are applied once on the bill, after the gross amount.
// Item rows carry only plain values: VAT on qty x rate, and unit cost = rate.
func calculatePurchase(req models.CreatePurchaseRequest) (*models.PurchaseCalc, error) {
	hundred := decimal.NewFromInt(100)
	vatFactor := decimal.NewFromInt(1).Add(req.VatRate.Div(hundred))

	calc := &models.PurchaseCalc{Req: req}
	gross := decimal.Zero

	for _, it := range req.Items {
		lineValue := it.Qty.Mul(it.Rate).Round(4)
		gross = gross.Add(lineValue)

		calc.Items = append(calc.Items, models.PurchaseItemCalc{
			CreatePurchaseItemRequest: it,
			VatAmount:                 lineValue.Mul(req.VatRate).Div(hundred).Round(4),
			UnitCost:                  it.Rate,
			UnitCostWithVat:           it.Rate.Mul(vatFactor).Round(4),
		})
	}

	// bill totals
	calc.GrossAmount = gross.Round(2)
	taxable := calc.GrossAmount.Add(req.ExciseDuty).Sub(req.SpecialDiscount).Sub(req.Discount)
	if taxable.IsNegative() {
		return nil, models.NewValidationError("discounts are larger than the gross amount")
	}
	calc.TaxableAmount = taxable.Round(2)
	calc.VatAmount = taxable.Mul(req.VatRate).Div(hundred).Round(2)
	calc.Total = calc.TaxableAmount.Add(calc.VatAmount).Add(req.FreightCharge).Round(2)

	if req.PaidAmount.GreaterThan(calc.Total) {
		return nil, models.NewValidationError("paid amount cannot be more than the purchase total")
	}
	switch {
	case req.PaidAmount.GreaterThanOrEqual(calc.Total):
		calc.PaymentStatus = models.PaymentPaid
	case req.PaidAmount.IsPositive():
		calc.PaymentStatus = models.PaymentPartial
	default:
		calc.PaymentStatus = models.PaymentUnpaid
	}
	return calc, nil
}

func (s *purchaseService) GetPurchaseDataById(ctx context.Context, purchaseID uuid.UUID) (*models.PurchaseByIdResponse, error) {
	if purchaseID == uuid.Nil {
		return nil, models.NewValidationError("invalid purchase id")
	}
	return s.repo.GetPurchaseDataById(ctx, purchaseID)
}

// normalizeReturn validates a return request and returns its total.
func normalizeReturn(req *models.CreatePurchaseReturnRequest) (decimal.Decimal, error) {
	bad := models.NewValidationError

	date := strings.TrimSpace(req.ReturnDate)
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return decimal.Zero, bad("invalid return date")
	}
	req.ReturnDate = date

	if req.Note != nil {
		n := strings.TrimSpace(*req.Note)
		if n == "" {
			req.Note = nil
		} else {
			req.Note = &n
		}
	}

	items := make([]models.CreatePurchaseReturnItemRequest, 0, len(req.Items))
	seen := map[uuid.UUID]bool{}
	total := decimal.Zero
	for _, it := range req.Items {
		it.Qty = it.Qty.Round(3)
		it.Amount = it.Amount.Round(2)
		if it.Qty.IsNegative() || it.Amount.IsNegative() {
			return decimal.Zero, bad("quantity and amount cannot be negative")
		}
		if it.Qty.IsZero() {
			continue
		}
		if it.PurchaseItemID == uuid.Nil {
			return decimal.Zero, bad("purchase item is required")
		}
		if seen[it.PurchaseItemID] {
			return decimal.Zero, bad("an item appears more than once")
		}
		seen[it.PurchaseItemID] = true
		if it.Reason != nil {
			rs := strings.TrimSpace(*it.Reason)
			if len(rs) > 250 {
				return decimal.Zero, bad("reason is too long (max 250 characters)")
			}
			if rs == "" {
				it.Reason = nil
			} else {
				it.Reason = &rs
			}
		}
		items = append(items, it)
		total = total.Add(it.Amount)
	}
	if len(items) == 0 {
		return decimal.Zero, bad("select at least one item to return")
	}
	sort.Slice(items, func(i, j int) bool { return items[i].PurchaseItemID.String() < items[j].PurchaseItemID.String() })
	req.Items = items
	return total.Round(2), nil
}

func (s *purchaseService) CreatePurchaseReturn(ctx context.Context, userID, purchaseID uuid.UUID, req models.CreatePurchaseReturnRequest) (uuid.UUID, error) {
	if purchaseID == uuid.Nil {
		return uuid.Nil, models.NewValidationError("invalid purchase id")
	}
	total, err := normalizeReturn(&req)
	if err != nil {
		return uuid.Nil, err
	}
	return s.repo.CreatePurchaseReturn(ctx, userID, purchaseID, &models.PurchaseReturnCalc{Req: req, Total: total})
}

func (s *purchaseService) UpdatePurchaseReturn(ctx context.Context, userID, purchaseID, returnID uuid.UUID, req models.CreatePurchaseReturnRequest) error {
	if purchaseID == uuid.Nil || returnID == uuid.Nil {
		return models.NewValidationError("invalid id")
	}
	total, err := normalizeReturn(&req)
	if err != nil {
		return err
	}
	return s.repo.UpdatePurchaseReturn(ctx, userID, purchaseID, returnID, &models.PurchaseReturnCalc{Req: req, Total: total})
}

func (s *purchaseService) DeletePurchaseReturn(ctx context.Context, userID, purchaseID, returnID uuid.UUID) error {
	if purchaseID == uuid.Nil || returnID == uuid.Nil {
		return models.NewValidationError("invalid id")
	}
	return s.repo.DeletePurchaseReturn(ctx, userID, purchaseID, returnID)
}

func (s *purchaseService) UpdatePurchasePayment(ctx context.Context, userID, purchaseID, paymentID uuid.UUID, req models.UpdatePurchasePaymentRequest) error {
	bad := models.NewValidationError
	if purchaseID == uuid.Nil || paymentID == uuid.Nil {
		return bad("invalid id")
	}

	req.Amount = req.Amount.Round(2)
	if !req.Amount.IsPositive() {
		return bad("amount must be greater than zero. Delete the payment to remove it.")
	}

	// an empty date or method means "leave as it is"
	req.PaymentDate = strings.TrimSpace(req.PaymentDate)
	if req.PaymentDate != "" {
		if _, err := time.Parse("2006-01-02", req.PaymentDate); err != nil {
			return bad("invalid payment date")
		}
	}
	switch req.PaymentMethod {
	case models.PaymentMethodCash, models.PaymentMethodBank, models.PaymentMethodCheque, models.PaymentMethodOnline, "":
	default:
		return bad("invalid payment method")
	}

	if req.Note != nil {
		n := strings.TrimSpace(*req.Note)
		if n == "" {
			req.Note = nil
		} else {
			req.Note = &n
		}
	}
	return s.repo.UpdatePurchasePayment(ctx, userID, purchaseID, paymentID, &req)
}

func (s *purchaseService) DeletePurchasePayment(ctx context.Context, userID, purchaseID, paymentID uuid.UUID) error {
	if purchaseID == uuid.Nil || paymentID == uuid.Nil {
		return models.NewValidationError("invalid id")
	}
	return s.repo.DeletePurchasePayment(ctx, userID, purchaseID, paymentID)
}

func (s *purchaseService) CreatePurchasePayment(ctx context.Context, userID, purchaseID uuid.UUID, req models.CreatePurchasePaymentRequest) (uuid.UUID, error) {
	bad := models.NewValidationError
	if purchaseID == uuid.Nil {
		return uuid.Nil, bad("invalid purchase id")
	}

	req.Amount = req.Amount.Round(2)
	if !req.Amount.IsPositive() {
		return uuid.Nil, bad("amount must be greater than zero")
	}

	date := strings.TrimSpace(req.PaymentDate)
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return uuid.Nil, bad("invalid payment date")
	}
	req.PaymentDate = date

	switch req.Source {
	case models.PaymentTypeRegular:
		switch req.PaymentMethod {
		case models.PaymentMethodCash, models.PaymentMethodBank, models.PaymentMethodCheque, models.PaymentMethodOnline:
		case "":
			req.PaymentMethod = models.PaymentMethodCash
		default:
			return uuid.Nil, bad("invalid payment method")
		}
	case models.PaymentTypeAdvance:
		req.PaymentMethod = ""
	default:
		return uuid.Nil, bad("choose how to pay: manually or from advance credit")
	}

	if req.Note != nil {
		n := strings.TrimSpace(*req.Note)
		if n == "" {
			req.Note = nil
		} else {
			req.Note = &n
		}
	}
	return s.repo.CreatePurchasePayment(ctx, userID, purchaseID, &req)
}

// NEW FILE: internals/services/purchase_supplier_lists.go
//
// Also add these two lines to the PurchaseService interface:
//
//

func encodeSupplierListCursor(date string, createdAt time.Time, id uuid.UUID) string {
	b, _ := json.Marshal(models.SupplierListCursor{Date: date, CreatedAt: createdAt, ID: id})
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeSupplierListCursor(raw string) (*models.SupplierListCursor, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, invalidf("invalid cursor")
	}
	var c models.SupplierListCursor
	if err := json.Unmarshal(b, &c); err != nil || c.ID == uuid.Nil || c.CreatedAt.IsZero() {
		return nil, invalidf("invalid cursor")
	}
	if _, err := time.Parse("2006-01-02", c.Date); err != nil {
		return nil, invalidf("invalid cursor")
	}
	return &c, nil
}

func clampSupplierListLimit(limit int) int {
	if limit <= 0 {
		return defaultPurchaseListLimit
	}
	if limit > maxPurchaseListLimit {
		return maxPurchaseListLimit
	}
	return limit
}

// both dates are inclusive (the columns are DATE, so no +1 day trick is needed)
func parseSupplierDateRange(fromRaw, toRaw string) (from, to *time.Time, err error) {
	if from, err = parseDay(fromRaw); err != nil {
		return nil, nil, err
	}
	if to, err = parseDay(toRaw); err != nil {
		return nil, nil, err
	}
	if from != nil && to != nil && from.After(*to) {
		return nil, nil, invalidf("from_date must be on or before to_date")
	}
	return from, to, nil
}

func (s *purchaseService) GetSupplierPurchases(ctx context.Context, supplierID uuid.UUID, q models.SupplierPurchaseListQuery) (*models.GetSupplierPurchasesResponse, error) {
	if supplierID == uuid.Nil {
		return nil, invalidf("invalid supplier id")
	}
	limit := clampSupplierListLimit(q.Limit)
	filter := models.SupplierPurchaseListFilter{SupplierID: supplierID, Limit: limit}

	if raw := strings.TrimSpace(q.PaymentStatus); raw != "" {
		ps := models.PaymentStatus(raw)
		if !isValidPaymentStatus(ps) {
			return nil, invalidf("unknown payment_status %q", raw)
		}
		filter.PaymentStatus = &ps
	}
	if raw := strings.TrimSpace(q.PurchaseStatus); raw != "" {
		st := models.PurchaseStatus(raw)
		if !isValidPurchaseStatus(st) {
			return nil, invalidf("unknown purchase_status %q", raw)
		}
		filter.PurchaseStatus = &st
	}

	from, to, err := parseSupplierDateRange(q.FromDate, q.ToDate)
	if err != nil {
		return nil, err
	}
	filter.FromDate, filter.ToDate = from, to

	cursor, err := decodeSupplierListCursor(q.Cursor)
	if err != nil {
		return nil, err
	}
	filter.Cursor = cursor

	items, total, err := s.repo.GetSupplierPurchases(ctx, filter)
	if err != nil {
		return nil, err
	}

	resp := &models.GetSupplierPurchasesResponse{Items: items, Total: total}
	if len(items) > limit {
		resp.Items = items[:limit]
		resp.HasMore = true
		last := items[limit-1]
		next := encodeSupplierListCursor(last.PurchaseDate, last.CreatedAt, last.PurchaseID)
		resp.NextCursor = &next
	}
	return resp, nil
}

func (s *purchaseService) GetSupplierReturns(ctx context.Context, supplierID uuid.UUID, q models.SupplierReturnListQuery) (*models.GetSupplierReturnsResponse, error) {
	if supplierID == uuid.Nil {
		return nil, invalidf("invalid supplier id")
	}
	limit := clampSupplierListLimit(q.Limit)
	filter := models.SupplierReturnListFilter{SupplierID: supplierID, Limit: limit}

	from, to, err := parseSupplierDateRange(q.FromDate, q.ToDate)
	if err != nil {
		return nil, err
	}
	filter.FromDate, filter.ToDate = from, to

	cursor, err := decodeSupplierListCursor(q.Cursor)
	if err != nil {
		return nil, err
	}
	filter.Cursor = cursor

	items, total, err := s.repo.GetSupplierReturns(ctx, filter)
	if err != nil {
		return nil, err
	}

	resp := &models.GetSupplierReturnsResponse{Items: items, Total: total}
	if len(items) > limit {
		resp.Items = items[:limit]
		resp.HasMore = true
		last := items[limit-1]
		next := encodeSupplierListCursor(last.ReturnDate, last.CreatedAt, last.PurchaseReturnID)
		resp.NextCursor = &next
	}
	return resp, nil
}
