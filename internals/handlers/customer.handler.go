// NEW FILE: internals/handlers/customer.go
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
	"github.com/google/uuid"
)

type CustomerHandler struct {
	service services.CustomerService
}

func NewCustomerHandler(service services.CustomerService) *CustomerHandler {
	return &CustomerHandler{service: service}
}

func customerIDParam(c *gin.Context) (uuid.UUID, bool) {
	id, err := helpers.StringToUUID(c.Param("customer_id"))
	if err != nil || id == uuid.Nil {
		helpers.Error(c, http.StatusBadRequest, "invalid customer id")
		return uuid.Nil, false
	}
	return id, true
}

func respondCustomerError(c *gin.Context, err error, action string) {
	var ve *models.ValidationError
	switch {
	case errors.Is(err, models.ErrNotFound):
		helpers.Error(c, http.StatusNotFound, "customer not found")
	case errors.As(err, &ve):
		helpers.Error(c, http.StatusBadRequest, ve.Message)
	default:
		log.Println(action+":", err)
		helpers.Error(c, http.StatusInternalServerError, "could not "+action)
	}
}

// GET /customer-service/get-customers
// Query parameters (all optional): search, balance_op (gt|gte|lt|lte|eq), balance_amount, limit, cursor
func (h *CustomerHandler) GetAllCustomers(c *gin.Context) {
	limit := 0
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			helpers.Error(c, http.StatusBadRequest, "limit must be a positive number")
			return
		}
		limit = n
	}

	resp, err := h.service.GetAllCustomers(c.Request.Context(), models.CustomerListQuery{
		Search:        c.Query("search"),
		BalanceOp:     c.Query("balance_op"),
		BalanceAmount: c.Query("balance_amount"),
		Cursor:        c.Query("cursor"),
		Limit:         limit,
	})
	if err != nil {
		respondCustomerError(c, err, "load the customers")
		return
	}
	helpers.Success(c, http.StatusOK, "customers fetched successfully", resp)
}

// POST /customer-service/create-customer
func (h *CustomerHandler) CreateCustomer(c *gin.Context) {
	var req models.CreateCustomerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helpers.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	customer, err := h.service.CreateCustomer(c.Request.Context(), req)
	if err != nil {
		respondCustomerError(c, err, "create the customer")
		return
	}
	// the full customer comes back so a sale screen can select it straight away
	helpers.Success(c, http.StatusCreated, "customer created", gin.H{"customer": customer})
}

// PUT /customer-service/update-customer/:customer_id
func (h *CustomerHandler) UpdateCustomer(c *gin.Context) {
	customerID, ok := customerIDParam(c)
	if !ok {
		return
	}
	var req models.UpdateCustomerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helpers.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	customer, err := h.service.UpdateCustomer(c.Request.Context(), customerID, req)
	if err != nil {
		respondCustomerError(c, err, "update the customer")
		return
	}
	helpers.Success(c, http.StatusOK, "customer updated", gin.H{"customer": customer})
}

// DELETE /customer-service/delete-customer/:customer_id
func (h *CustomerHandler) DeleteCustomer(c *gin.Context) {
	customerID, ok := customerIDParam(c)
	if !ok {
		return
	}
	if err := h.service.DeleteCustomer(c.Request.Context(), customerID); err != nil {
		respondCustomerError(c, err, "delete the customer")
		return
	}
	helpers.Success(c, http.StatusOK, "customer deleted", gin.H{"customer_id": customerID})
}