package handlers

import (
	"inventory/internals/helpers"
	"inventory/internals/models"
	"inventory/internals/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

type SupplierPaymentHandler struct {
	supplierPaymentService services.SupplierPaymentService
}

func NewSupplierPaymentHandler(service services.SupplierPaymentService) *SupplierPaymentHandler {
	return &SupplierPaymentHandler{
		supplierPaymentService: service,
	}
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
