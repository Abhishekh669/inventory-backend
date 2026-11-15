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
	GetAllPurchases(ctx context.Context, q models.PurchaseListQuery) (*models.GetAllPurchaseResponse, error)

	CreatePurchase(ctx context.Context, userID uuid.UUID, req models.CreatePurchaseRequest) (uuid.UUID, error)
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
	if err := normalizeAndValidate(&req); err != nil {
		return uuid.Nil, err
	}
	calc, err := calculatePurchase(req)
	if err != nil {
		return uuid.Nil, err
	}
	return s.repo.CreatePurchase(ctx, userID, calc)
}

func normalizeAndValidate(req *models.CreatePurchaseRequest) error {
	bad := models.NewValidationError

	req.SupplierBillNo = strings.TrimSpace(req.SupplierBillNo)
	if req.SupplierBillNo == "" || len(req.SupplierBillNo) > 50 {
		return bad("supplier bill number is required (max 50 characters)")
	}
	if req.PurchaseOrderID == uuid.Nil {
		return bad("purchase order is required")
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
