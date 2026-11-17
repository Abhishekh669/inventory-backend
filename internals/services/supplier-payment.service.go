package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"inventory/internals/models"
	"inventory/internals/repository"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

const (
	defaultSupplierPaymentLimit = 10
	maxSupplierPaymentLimit     = 30
)

type SupplierPaymentService interface {
	CreateAdvancePaymentForSupplierService(
		ctx context.Context,
		data *models.CreateAdvancePaymentType,
		userId uuid.UUID,
	) error

	GetSupplierPaymentsService(
		ctx context.Context,
		q models.SupplierPaymentListQuery,
	) (*models.SupplierPaymentListResponse, error)

	GetSupplierLedgerService(ctx context.Context, q models.SupplierLedgerQuery) (*models.SupplierLedgerResponse, error)
}

type supplierPaymentService struct {
	repo repository.SupplierPaymentRepository
}

func NewSupplierPaymentService(repo repository.SupplierPaymentRepository) SupplierPaymentService {
	return &supplierPaymentService{repo: repo}
}

// ---------- reads ----------

func (s *supplierPaymentService) GetSupplierPaymentsService(
	ctx context.Context,
	q models.SupplierPaymentListQuery,
) (*models.SupplierPaymentListResponse, error) {
	if q.SupplierID == uuid.Nil {
		return nil, invalidf("supplier is required")
	}

	limit := q.Limit
	if limit <= 0 {
		limit = defaultSupplierPaymentLimit
	}
	if limit > maxSupplierPaymentLimit {
		limit = maxSupplierPaymentLimit
	}

	filter := models.SupplierPaymentListFilter{SupplierID: q.SupplierID, Limit: limit}

	if raw := strings.TrimSpace(q.Status); raw != "" {
		status := models.SupplierPaymentStatus(raw)
		switch status {
		case models.SupplierPaymentUnallocated,
			models.SupplierPaymentPartial,
			models.SupplierPaymentAllocated:
			filter.Status = &status
		default:
			return nil, invalidf("unknown status %q", raw)
		}
	}

	from, err := optionalDay(q.FromDate, "from date")
	if err != nil {
		return nil, err
	}
	to, err := optionalDay(q.ToDate, "to date")
	if err != nil {
		return nil, err
	}
	if from != nil && to != nil && from.After(*to) {
		return nil, invalidf("from date must be on or before to date")
	}
	filter.FromDate = from
	filter.ToDate = to

	cursor, err := decodeSupplierPaymentCursor(q.Cursor)
	if err != nil {
		return nil, err
	}
	filter.Cursor = cursor

	payments, total, err := s.repo.GetSupplierPayments(ctx, filter)
	if err != nil {
		return nil, err
	}

	summary, err := s.repo.GetSupplierFinancialSummary(ctx, q.SupplierID)
	if err != nil {
		return nil, err
	}
	summary.Calculate()

	resp := &models.SupplierPaymentListResponse{
		Payments: payments,
		Total:    total,
		Summary:  *summary,
	}
	if len(payments) > limit {
		resp.Payments = payments[:limit]
		resp.HasMore = true
		next := encodeSupplierPaymentCursor(payments[limit-1])
		resp.NextCursor = &next
	}

	return resp, nil
}

// ---------- writes ----------

func (s *supplierPaymentService) CreateAdvancePaymentForSupplierService(
	ctx context.Context,
	data *models.CreateAdvancePaymentType,
	userId uuid.UUID,
) error {

	if data.SupplierID == uuid.Nil {
		return errors.New("supplier is required")
	}

	if data.Amount.LessThanOrEqual(decimal.Zero) {
		return errors.New("payment amount must be greater than zero")
	}

	switch data.PaymentMethod {
	case models.PaymentMethodCash,
		models.PaymentMethodBank,
		models.PaymentMethodCheque,
		models.PaymentMethodOnline:
	default:
		return errors.New("invalid payment method")
	}

	paymentData := &models.SupplierPaymentCreationType{
		SupplierID:    data.SupplierID,
		Amount:        data.Amount,
		PaymentMethod: data.PaymentMethod,
		PaymentType:   models.PaymentTypeAdvance,
		Note:          data.Note,
	}

	_, err := s.repo.CreateAdvancePaymentForSupplier(ctx, paymentData, userId)
	return err
}

// ---------- helpers ----------

// optionalDay parses YYYY-MM-DD; an empty value means "no filter".
func optionalDay(value, label string) (*time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	day, err := parseDateOnly(value, label)
	if err != nil {
		return nil, err
	}
	return &day, nil
}

func encodeSupplierPaymentCursor(p models.SupplierIDPaymentResponse) string {
	raw, _ := json.Marshal(models.SupplierPaymentCursor{
		PaymentDate: p.PaymentDate.Format("2006-01-02"),
		CreatedAt:   p.CreatedAt.UTC(),
		ID:          p.ID,
	})
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeSupplierPaymentCursor(value string) (*models.SupplierPaymentCursor, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}

	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, invalidf("invalid cursor")
	}

	var c models.SupplierPaymentCursor
	if err := json.Unmarshal(raw, &c); err != nil || c.ID == uuid.Nil || c.CreatedAt.IsZero() {
		return nil, invalidf("invalid cursor")
	}
	if _, err := time.Parse("2006-01-02", c.PaymentDate); err != nil {
		return nil, invalidf("invalid cursor")
	}

	return &c, nil
}

const (
	defaultSupplierLedgerLimit = 20
	maxSupplierLedgerLimit     = 100
)

func (s *supplierPaymentService) GetSupplierLedgerService(
	ctx context.Context,
	q models.SupplierLedgerQuery,
) (*models.SupplierLedgerResponse, error) {
	if q.SupplierID == uuid.Nil {
		return nil, invalidf("supplier is required")
	}

	limit := q.Limit
	if limit <= 0 {
		limit = defaultSupplierLedgerLimit
	}
	if limit > maxSupplierLedgerLimit {
		limit = maxSupplierLedgerLimit
	}

	filter := models.SupplierLedgerFilter{SupplierID: q.SupplierID, Limit: limit}

	from, err := optionalDay(q.FromDate, "from date")
	if err != nil {
		return nil, err
	}
	to, err := optionalDay(q.ToDate, "to date")
	if err != nil {
		return nil, err
	}
	if from != nil && to != nil && from.After(*to) {
		return nil, invalidf("from date must be on or before to date")
	}
	filter.FromDate = from
	filter.ToDate = to

	cursor, err := decodeSupplierLedgerCursor(q.Cursor)
	if err != nil {
		return nil, err
	}
	filter.Cursor = cursor

	res, err := s.repo.GetSupplierLedger(ctx, filter)
	if err != nil {
		return nil, err
	}

	if len(res.Entries) > limit {
		res.Entries = res.Entries[:limit]
		res.HasMore = true
		next := encodeSupplierLedgerCursor(res.Entries[limit-1])
		res.NextCursor = &next
	}

	return res, nil
}

func encodeSupplierLedgerCursor(e models.SupplierLedgerEntry) string {
	raw, _ := json.Marshal(models.SupplierLedgerCursor{
		EntryDate: e.EntryDate.Format("2006-01-02"),
		CreatedAt: e.CreatedAt.UTC(),
		Rank:      e.SortRank,
		ID:        e.ID,
	})
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeSupplierLedgerCursor(value string) (*models.SupplierLedgerCursor, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}

	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, invalidf("invalid cursor")
	}

	var c models.SupplierLedgerCursor
	if err := json.Unmarshal(raw, &c); err != nil ||
		c.ID == uuid.Nil || c.CreatedAt.IsZero() || c.Rank < 1 || c.Rank > 4 {
		return nil, invalidf("invalid cursor")
	}
	if _, err := time.Parse("2006-01-02", c.EntryDate); err != nil {
		return nil, invalidf("invalid cursor")
	}

	return &c, nil
}
