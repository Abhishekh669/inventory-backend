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
)

type SupplierPaymentHandler struct {
	supplierPaymentService services.SupplierPaymentService
}

func NewSupplierPaymentHandler(service services.SupplierPaymentService) *SupplierPaymentHandler {
	return &SupplierPaymentHandler{
		supplierPaymentService: service,
	}
}

// GET /supplier-payment-service/get-supplier-payments/:supplier_id
// Query parameters: limit, cursor, status, from_date, to_date
func (h *SupplierPaymentHandler) GetSupplierPayments(c *gin.Context) {
	supplierID, err := uuid.Parse(c.Param("supplier_id"))
	if err != nil {
		helpers.Error(c, http.StatusBadRequest, "invalid supplier ID")
		return
	}

	limit := 0
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			helpers.Error(c, http.StatusBadRequest, "limit must be a positive number")
			return
		}
		limit = n
	}

	resp, err := h.supplierPaymentService.GetSupplierPaymentsService(
		c.Request.Context(),
		models.SupplierPaymentListQuery{
			SupplierID: supplierID,
			Status:     c.Query("status"),
			FromDate:   c.Query("from_date"),
			ToDate:     c.Query("to_date"),
			Cursor:     c.Query("cursor"),
			Limit:      limit,
		},
	)
	if err != nil {
		if errors.Is(err, services.ErrInvalidInput) {
			helpers.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		fmt.Println("Supplier payment handler error:", err)
		helpers.Error(c, http.StatusInternalServerError, "internal server error")
		return
	}

	helpers.Success(c, http.StatusOK, "supplier payments fetched successfully", resp)
}

func (h *SupplierPaymentHandler) CreateAdvancePaymentHandlerForSupplier(c *gin.Context) {
	user, ok := helpers.GetUserContext(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req models.CreateAdvancePaymentType

	if err := c.ShouldBindJSON(&req); err != nil {
		helpers.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	err := h.supplierPaymentService.CreateAdvancePaymentForSupplierService(
		c.Request.Context(),
		&req,
		user.UserID,
	)
	if err != nil {
		var ve *models.ValidationError
		if errors.As(err, &ve) {
			helpers.Error(c, http.StatusBadRequest, ve.Message)
			return
		}
		helpers.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	helpers.Success(
		c,
		http.StatusCreated,
		"advance payment created successfully",
		nil,
	)
}

// GET /supplier-payment-service/get-supplier-ledger/:supplier_id
// Query parameters: limit, cursor, from_date, to_date
func (h *SupplierPaymentHandler) GetSupplierLedger(c *gin.Context) {
	supplierID, err := uuid.Parse(c.Param("supplier_id"))
	if err != nil {
		helpers.Error(c, http.StatusBadRequest, "invalid supplier ID")
		return
	}

	limit := 0
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			helpers.Error(c, http.StatusBadRequest, "limit must be a positive number")
			return
		}
		limit = n
	}

	resp, err := h.supplierPaymentService.GetSupplierLedgerService(
		c.Request.Context(),
		models.SupplierLedgerQuery{
			SupplierID: supplierID,
			FromDate:   c.Query("from_date"),
			ToDate:     c.Query("to_date"),
			Cursor:     c.Query("cursor"),
			Limit:      limit,
		},
	)
	if err != nil {
		if errors.Is(err, services.ErrInvalidInput) {
			helpers.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, repository.ErrSupplierNotFound) {
			helpers.Error(c, http.StatusNotFound, "supplier not found")
			return
		}
		fmt.Println("Supplier ledger handler error:", err)
		helpers.Error(c, http.StatusInternalServerError, "internal server error")
		return
	}

	helpers.Success(c, http.StatusOK, "supplier ledger fetched successfully", resp)
}
