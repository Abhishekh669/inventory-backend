// NEW FILE: internals/handlers/stock_movement.go
package handlers

import (
	"errors"
	"inventory/internals/helpers"
	"inventory/internals/models"
	"inventory/internals/services"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type StockMovementHandler struct {
	stockMovementService services.StockMovementService
}

func NewStockMovementHandler(stockMovementService services.StockMovementService) *StockMovementHandler {
	return &StockMovementHandler{stockMovementService: stockMovementService}
}

func (h *StockMovementHandler) GetAllStockMovements(c *gin.Context) {
	limit := 0
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			helpers.Error(c, http.StatusBadRequest, "limit must be a positive number")
			return
		}
		limit = n
	}

	resp, err := h.stockMovementService.GetAllStockMovements(c.Request.Context(), models.StockMovementListQuery{
		ProductID: c.Query("product_id"),
		Type:      c.Query("type"),
		FromDate:  c.Query("from_date"),
		ToDate:    c.Query("to_date"),
		Cursor:    c.Query("cursor"),
		Limit:     limit,
	})
	if err != nil {
		if errors.Is(err, services.ErrInvalidInput) {
			helpers.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		log.Println("get stock movements:", err)
		helpers.Error(c, http.StatusInternalServerError, "could not load the stock movements")
		return
	}

	helpers.Success(c, http.StatusOK, "stock movements fetched successfully", resp)
}
