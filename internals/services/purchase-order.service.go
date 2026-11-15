package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"inventory/internals/models"
	"inventory/internals/repository"
)

// ErrInvalidInput is wrapped by every validation error so the handler can answer 400.
var ErrInvalidInput = errors.New("invalid input")

const (
	defaultPurchaseOrderLimit = 10
	maxPurchaseOrderLimit     = 30
	maxPurchaseOrderItems     = 100
	maxPurchaseOrderTitleLen  = 150
	maxPurchaseOrderNoteLen   = 2000
	maxPurchaseOrderSearchLen = 100
	maxDeletePurchaseOrders   = 50
)

var (
	maxPurchaseOrderQty  = decimal.RequireFromString("99999999999")
	maxPurchaseOrderCost = decimal.RequireFromString("9999999999")
)

// Nepal has no daylight saving, so a fixed offset is enough and needs no tzdata.
var nepalTime = time.FixedZone("NPT", 5*60*60+45*60)

type PurchaseOrderService interface {
	GetPurchaseOrdersBySupplier(ctx context.Context, q models.PurchaseOrderListQuery) (*models.PurchaseOrderListResponse, error)
	GetPurchaseOrderItems(ctx context.Context, purchaseOrderID uuid.UUID) ([]models.PurchaseOrderItemDetail, error)
	GetPurchaseOrder(ctx context.Context, id uuid.UUID) (*models.PurchaseOrderDetail, error)
	CreatePurchaseOrder(ctx context.Context, in models.PurchaseOrderInput, createdBy uuid.UUID) (*models.PurchaseOrderDetail, error)
	UpdatePurchaseOrder(ctx context.Context, id uuid.UUID, in models.PurchaseOrderInput) (*models.PurchaseOrderDetail, error)
	DeletePurchaseOrders(ctx context.Context, ids []uuid.UUID) error
}

type purchaseOrderService struct {
	repo repository.PurchaseOrderRepository
}

func NewPurchaseOrderService(repo repository.PurchaseOrderRepository) PurchaseOrderService {
	return &purchaseOrderService{repo: repo}
}

// ---------- reads ----------

func (s *purchaseOrderService) GetPurchaseOrdersBySupplier(
	ctx context.Context,
	q models.PurchaseOrderListQuery,
) (*models.PurchaseOrderListResponse, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = defaultPurchaseOrderLimit
	}
	if limit > maxPurchaseOrderLimit {
		limit = maxPurchaseOrderLimit
	}

	filter := models.PurchaseOrderListFilter{SupplierID: q.SupplierID, Limit: limit}

	if raw := strings.TrimSpace(q.Status); raw != "" {
		status := models.PurchaseOrderStatus(raw)
		if !isValidPurchaseOrderStatus(status) {
			return nil, invalidf("unknown status %q", raw)
		}
		filter.Status = &status
	}

	// add to the const block

	if raw := strings.TrimSpace(q.Search); raw != "" {
		if utf8.RuneCountInString(raw) > maxPurchaseOrderSearchLen {
			return nil, invalidf("search must be at most %d characters", maxPurchaseOrderSearchLen)
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

	cursor, err := decodePurchaseOrderCursor(q.Cursor)
	if err != nil {
		return nil, err
	}
	filter.Cursor = cursor

	orders, total, err := s.repo.GetBySupplier(ctx, filter)
	if err != nil {
		return nil, err
	}

	resp := &models.PurchaseOrderListResponse{PurchaseOrders: orders, Total: total}
	if len(orders) > limit {
		resp.PurchaseOrders = orders[:limit]
		resp.HasMore = true
		next := encodePurchaseOrderCursor(orders[limit-1])
		resp.NextCursor = &next
	}

	return resp, nil
}

func (s *purchaseOrderService) GetPurchaseOrderItems(
	ctx context.Context,
	purchaseOrderID uuid.UUID,
) ([]models.PurchaseOrderItemDetail, error) {
	return s.repo.GetItems(ctx, purchaseOrderID)
}

func (s *purchaseOrderService) GetPurchaseOrder(ctx context.Context, id uuid.UUID) (*models.PurchaseOrderDetail, error) {
	return s.repo.GetByID(ctx, id)
}

// ---------- writes ----------

func (s *purchaseOrderService) CreatePurchaseOrder(
	ctx context.Context,
	in models.PurchaseOrderInput,
	createdBy uuid.UUID,
) (*models.PurchaseOrderDetail, error) {
	if in.SupplierID == uuid.Nil {
		return nil, invalidf("supplier is required")
	}

	write, err := buildPurchaseOrderWrite(in, true)
	if err != nil {
		return nil, err
	}

	id, err := s.repo.Create(ctx, *write, createdBy)
	if err != nil {
		return nil, err
	}

	return s.repo.GetByID(ctx, id)
}

func (s *purchaseOrderService) UpdatePurchaseOrder(
	ctx context.Context,
	id uuid.UUID,
	in models.PurchaseOrderInput,
) (*models.PurchaseOrderDetail, error) {
	write, err := buildPurchaseOrderWrite(in, false)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, id, *write); err != nil {
		return nil, err
	}

	return s.repo.GetByID(ctx, id)
}

func (s *purchaseOrderService) DeletePurchaseOrders(ctx context.Context, ids []uuid.UUID) error {
	unique := make([]uuid.UUID, 0, len(ids))
	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			return invalidf("invalid purchase order ID")
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}

	if len(unique) == 0 {
		return invalidf("select at least one purchase order")
	}
	if len(unique) > maxDeletePurchaseOrders {
		return invalidf("you can delete at most %d purchase orders at once", maxDeletePurchaseOrders)
	}

	return s.repo.Delete(ctx, unique)
}

// ---------- validation ----------

func buildPurchaseOrderWrite(in models.PurchaseOrderInput, isCreate bool) (*models.PurchaseOrderWrite, error) {
	out := &models.PurchaseOrderWrite{SupplierID: in.SupplierID}

	title, err := cleanText(in.Title, maxPurchaseOrderTitleLen, "title")
	if err != nil {
		return nil, err
	}
	out.Title = title

	note, err := cleanText(in.Note, maxPurchaseOrderNoteLen, "note")
	if err != nil {
		return nil, err
	}
	out.Note = note

	orderDate := todayNepal()
	if strings.TrimSpace(in.OrderDate) != "" {
		orderDate, err = parseDateOnly(in.OrderDate, "order date")
		if err != nil {
			return nil, err
		}
	}
	out.OrderDate = orderDate

	if in.ExpectedDate != nil && strings.TrimSpace(*in.ExpectedDate) != "" {
		expected, err := parseDateOnly(*in.ExpectedDate, "expected date")
		if err != nil {
			return nil, err
		}
		if expected.Before(orderDate) {
			return nil, invalidf("expected date can't be before the order date")
		}
		out.ExpectedDate = &expected
	}

	if isCreate {
		status := models.PurchaseOrderDraft
		if in.Status != nil {
			status = *in.Status
		}
		if status != models.PurchaseOrderDraft && status != models.PurchaseOrderSent {
			return nil, invalidf("a new purchase order can only be draft or sent")
		}
		out.Status = &status
	} else if in.Status != nil {
		status := *in.Status
		if status != models.PurchaseOrderDraft && status != models.PurchaseOrderSent && status != models.PurchaseOrderCancelled {
			return nil, invalidf("status can only be changed to draft, sent or cancelled")
		}
		out.Status = &status
	}

	if len(in.Items) == 0 {
		return nil, invalidf("add at least one product")
	}
	if len(in.Items) > maxPurchaseOrderItems {
		return nil, invalidf("a purchase order can have at most %d products", maxPurchaseOrderItems)
	}

	seenProducts := make(map[uuid.UUID]struct{}, len(in.Items))
	seenItems := make(map[uuid.UUID]struct{}, len(in.Items))

	for i, item := range in.Items {
		line := i + 1

		if item.ProductID == uuid.Nil {
			return nil, invalidf("item %d: product is required", line)
		}
		if _, ok := seenProducts[item.ProductID]; ok {
			return nil, invalidf("item %d: the same product is added more than once", line)
		}
		seenProducts[item.ProductID] = struct{}{}

		if item.QtyOrdered.Sign() <= 0 {
			return nil, invalidf("item %d: quantity must be more than 0", line)
		}
		if item.QtyOrdered.GreaterThan(maxPurchaseOrderQty) {
			return nil, invalidf("item %d: quantity is too large", line)
		}
		if !item.QtyOrdered.Equal(item.QtyOrdered.Round(3)) {
			return nil, invalidf("item %d: quantity can have at most 3 decimal places", line)
		}

		if item.ExpectedUnitCost != nil {
			cost := *item.ExpectedUnitCost
			if cost.Sign() < 0 {
				return nil, invalidf("item %d: expected cost can't be negative", line)
			}
			if cost.GreaterThan(maxPurchaseOrderCost) {
				return nil, invalidf("item %d: expected cost is too large", line)
			}
			if !cost.Equal(cost.Round(4)) {
				return nil, invalidf("item %d: expected cost can have at most 4 decimal places", line)
			}
		}

		if item.ID != nil {
			if *item.ID == uuid.Nil {
				return nil, invalidf("item %d: invalid item ID", line)
			}
			if _, ok := seenItems[*item.ID]; ok {
				return nil, invalidf("item %d: the same item is sent more than once", line)
			}
			seenItems[*item.ID] = struct{}{}
		}

		out.Items = append(out.Items, models.PurchaseOrderItemWrite{
			ID:               item.ID,
			ProductID:        item.ProductID,
			QtyOrdered:       item.QtyOrdered,
			ExpectedUnitCost: item.ExpectedUnitCost,
		})
	}

	return out, nil
}

func invalidf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidInput, fmt.Sprintf(format, args...))
}

// cleanText trims the value and turns an empty string into nil.
func cleanText(value *string, maxLen int, label string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(trimmed) > maxLen {
		return nil, invalidf("%s must be at most %d characters", label, maxLen)
	}
	return &trimmed, nil
}

func isValidPurchaseOrderStatus(status models.PurchaseOrderStatus) bool {
	switch status {
	case models.PurchaseOrderDraft,
		models.PurchaseOrderSent,
		models.PurchaseOrderPartial,
		models.PurchaseOrderReceived,
		models.PurchaseOrderCancelled:
		return true
	}
	return false
}

// ---------- dates and cursor ----------

// todayNepal is today's date in Nepal, stored as UTC midnight so the civil date is kept.
func todayNepal() time.Time {
	now := time.Now().In(nepalTime)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

func parseDateOnly(value, label string) (time.Time, error) {
	day, err := time.Parse("2006-01-02", strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, invalidf("%s must look like YYYY-MM-DD", label)
	}
	return day, nil
}

// parseDay is used for the created_at filters, where the day starts at midnight in Nepal.
func parseDay(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	day, err := time.ParseInLocation("2006-01-02", value, nepalTime)
	if err != nil {
		return nil, invalidf("dates must look like YYYY-MM-DD")
	}
	return &day, nil
}

func encodePurchaseOrderCursor(po models.PurchaseOrderSummary) string {
	raw, _ := json.Marshal(models.PurchaseOrderCursor{CreatedAt: po.CreatedAt.UTC(), ID: po.ID})
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodePurchaseOrderCursor(value string) (*models.PurchaseOrderCursor, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}

	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, invalidf("invalid cursor")
	}

	var cursor models.PurchaseOrderCursor
	if err := json.Unmarshal(raw, &cursor); err != nil || cursor.ID == uuid.Nil || cursor.CreatedAt.IsZero() {
		return nil, invalidf("invalid cursor")
	}

	return &cursor, nil
}
