package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"inventory/internals/helpers"
	"inventory/internals/models"
	"inventory/internals/repository"
	"inventory/internals/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// PurchaseOrderHandler handles purchase order HTTP requests.
type PurchaseOrderHandler struct {
	purchaseOrderService services.PurchaseOrderService
}

func NewPurchaseOrderHandler(
	purchaseOrderService services.PurchaseOrderService,
) *PurchaseOrderHandler {
	return &PurchaseOrderHandler{
		purchaseOrderService: purchaseOrderService,
	}
}

// handlePurchaseOrderError maps service and database errors to HTTP responses.
func handlePurchaseOrderError(c *gin.Context, err error) {
	var pgErr *pgconn.PgError

	switch {
	case errors.Is(err, services.ErrInvalidInput):
		helpers.Error(c, http.StatusBadRequest, err.Error())

	case errors.Is(err, repository.ErrPurchaseOrderNotFound):
		helpers.Error(c, http.StatusNotFound, "purchase order not found")

	case errors.Is(err, repository.ErrPurchaseOrderItemNotFound):
		helpers.Error(c, http.StatusNotFound, "purchase order item not found")

	case errors.As(err, &pgErr) && pgErr.Code == "23505":
		helpers.Error(c, http.StatusConflict, "purchase order already exists")

	case errors.As(err, &pgErr) && pgErr.Code == "23503":
		helpers.Error(c, http.StatusConflict, "related record does not exist or is in use")

	case errors.As(err, &pgErr) && pgErr.Code == "22001":
		helpers.Error(c, http.StatusBadRequest, "provided value is too long")

	default:
		fmt.Println("Purchase order handler error:", err)
		helpers.Error(c, http.StatusInternalServerError, "internal server error")
	}
}

// GET /purchase-order-service/get-purchase-orders-by-supplier/:supplier_id
// Query parameters: limit, cursor, status, from_date, to_date
func (h *PurchaseOrderHandler) GetPurchaseOrdersBySupplier(c *gin.Context) {
	supplierID, err := uuid.Parse(c.Param("supplier_id"))
	if err != nil {
		helpers.Error(c, http.StatusBadRequest, "invalid supplier ID")
		return
	}

	// Validate pagination limit.
	limit := 0

	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			helpers.Error(c, http.StatusBadRequest, "limit must be a positive number")
			return
		}

		limit = n
	}

	resp, err := h.purchaseOrderService.GetPurchaseOrdersBySupplier(
		c.Request.Context(),
		models.PurchaseOrderListQuery{
			SupplierID: supplierID,
			Status:     c.Query("status"),
			FromDate:   c.Query("from_date"),
			ToDate:     c.Query("to_date"),
			Search:     c.Query("search"),
			Cursor:     c.Query("cursor"),
			Limit:      limit,
		},
	)

	if err != nil {
		handlePurchaseOrderError(c, err)
		return
	}

	helpers.Success(
		c,
		http.StatusOK,
		"purchase orders fetched successfully",
		resp,
	)
}

// GET /purchase-order-service/get-purchase-order-items/:purchase_order_id
func (h *PurchaseOrderHandler) GetPurchaseOrderItems(c *gin.Context) {
	purchaseOrderID, err := uuid.Parse(c.Param("purchase_order_id"))
	if err != nil {
		helpers.Error(c, http.StatusBadRequest, "invalid purchase order ID")
		return
	}

	items, err := h.purchaseOrderService.GetPurchaseOrderItems(
		c.Request.Context(),
		purchaseOrderID,
	)

	if err != nil {
		handlePurchaseOrderError(c, err)
		return
	}

	helpers.Success(
		c,
		http.StatusOK,
		"purchase order items fetched successfully",
		gin.H{
			"items": items,
		},
	)
}

// GET /purchase-order-service/get-purchase-order/:id
func (h *PurchaseOrderHandler) GetPurchaseOrder(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		helpers.Error(c, http.StatusBadRequest, "invalid purchase order ID")
		return
	}

	purchaseOrder, err := h.purchaseOrderService.GetPurchaseOrder(c.Request.Context(), id)
	if err != nil {
		handlePurchaseOrderError(c, err)
		return
	}

	helpers.Success(
		c,
		http.StatusOK,
		"purchase order fetched successfully",
		purchaseOrder,
	)
}

// POST /purchase-order-service/create-purchase-order
func (h *PurchaseOrderHandler) CreatePurchaseOrder(c *gin.Context) {
	user, ok := helpers.GetUserContext(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req models.PurchaseOrderInput
	if err := c.ShouldBindJSON(&req); err != nil {
		helpers.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	purchaseOrder, err := h.purchaseOrderService.CreatePurchaseOrder(c.Request.Context(), req, user.UserID)
	if err != nil {
		handlePurchaseOrderError(c, err)
		return
	}

	helpers.Success(
		c,
		http.StatusCreated,
		"purchase order created successfully",
		purchaseOrder,
	)
}

// PUT /purchase-order-service/update-purchase-order/:id
func (h *PurchaseOrderHandler) UpdatePurchaseOrder(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		helpers.Error(c, http.StatusBadRequest, "invalid purchase order ID")
		return
	}

	var req models.PurchaseOrderInput
	if err := c.ShouldBindJSON(&req); err != nil {
		helpers.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	purchaseOrder, err := h.purchaseOrderService.UpdatePurchaseOrder(c.Request.Context(), id, req)
	if err != nil {
		handlePurchaseOrderError(c, err)
		return
	}

	helpers.Success(
		c,
		http.StatusOK,
		"purchase order updated successfully",
		purchaseOrder,
	)
}

// POST /purchase-order-service/delete-purchase-orders
// Body: { "ids": ["..."] }
func (h *PurchaseOrderHandler) DeletePurchaseOrders(c *gin.Context) {
	var req models.DeletePurchaseOrdersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helpers.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.purchaseOrderService.DeletePurchaseOrders(c.Request.Context(), req.IDs); err != nil {
		handlePurchaseOrderError(c, err)
		return
	}

	helpers.Success(
		c,
		http.StatusOK,
		"purchase orders deleted successfully",
		nil,
	)
}
