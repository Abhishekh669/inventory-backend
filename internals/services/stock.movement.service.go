// NEW FILE: internals/services/stock_movement.go
package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"inventory/internals/models"
	"inventory/internals/repository"
	"strings"
	"time"

	"github.com/google/uuid"
)

type StockMovementService interface {
	GetAllStockMovements(ctx context.Context, q models.StockMovementListQuery) (*models.GetAllStockMovementsResponse, error)
}

type stockMovementService struct {
	repo repository.StockMovementRepository
}

func NewStockMovementService(repo repository.StockMovementRepository) StockMovementService {
	return &stockMovementService{repo: repo}
}

const (
	defaultStockMovementLimit = 20
	maxStockMovementLimit     = 100
)

func isValidStockMovementType(t models.StockMovementType) bool {
	switch t {
	case models.StockMovementOpening, models.StockMovementPurchase, models.StockMovementSale,
		models.StockMovementPurchaseReturn, models.StockMovementSaleReturn, models.StockMovementAdjustment:
		return true
	}
	return false
}

func encodeStockMovementCursor(it models.StockMovementItem) string {
	b, _ := json.Marshal(models.StockMovementCursor{Date: it.MovementDate, CreatedAt: it.CreatedAt, ID: it.ID})
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeStockMovementCursor(raw string) (*models.StockMovementCursor, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, invalidf("invalid cursor")
	}
	var c models.StockMovementCursor
	if err := json.Unmarshal(b, &c); err != nil || c.ID == uuid.Nil || c.CreatedAt.IsZero() {
		return nil, invalidf("invalid cursor")
	}
	if _, err := time.Parse("2006-01-02", c.Date); err != nil {
		return nil, invalidf("invalid cursor")
	}
	return &c, nil
}

func (s *stockMovementService) GetAllStockMovements(ctx context.Context, q models.StockMovementListQuery) (*models.GetAllStockMovementsResponse, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = defaultStockMovementLimit
	}
	if limit > maxStockMovementLimit {
		limit = maxStockMovementLimit
	}
	filter := models.StockMovementListFilter{Limit: limit}

	if raw := strings.TrimSpace(q.ProductID); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil || id == uuid.Nil {
			return nil, invalidf("invalid product_id")
		}
		filter.ProductID = &id
	}

	if raw := strings.TrimSpace(q.Type); raw != "" {
		t := models.StockMovementType(raw)
		if !isValidStockMovementType(t) {
			return nil, invalidf("unknown type %q", raw)
		}
		filter.Type = &t
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
	filter.FromDate, filter.ToDate = from, to // movement_date is a DATE, so both ends are inclusive

	cursor, err := decodeStockMovementCursor(q.Cursor)
	if err != nil {
		return nil, err
	}
	filter.Cursor = cursor

	items, total, err := s.repo.GetAllStockMovements(ctx, filter)
	if err != nil {
		return nil, err
	}

	resp := &models.GetAllStockMovementsResponse{Items: items, Total: total}
	if len(items) > limit {
		resp.Items = items[:limit]
		resp.HasMore = true
		next := encodeStockMovementCursor(items[limit-1])
		resp.NextCursor = &next
	}
	return resp, nil
}
