// NEW FILE: internals/handlers/supplier_bank_account.go
package handlers

import (
	"errors"
	"inventory/internals/helpers"
	"inventory/internals/models"
	"inventory/internals/services"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type SupplierBankAccountHandler struct {
	service services.SupplierBankAccountService
}

func NewSupplierBankAccountHandler(service services.SupplierBankAccountService) *SupplierBankAccountHandler {
	return &SupplierBankAccountHandler{service: service}
}

func bankSupplierIDParam(c *gin.Context) (uuid.UUID, bool) {
	id, err := helpers.StringToUUID(c.Param("supplier_id"))
	if err != nil || id == uuid.Nil {
		helpers.Error(c, http.StatusBadRequest, "invalid supplier id")
		return uuid.Nil, false
	}
	return id, true
}

func bankAccountIDParam(c *gin.Context) (uuid.UUID, bool) {
	id, err := helpers.StringToUUID(c.Param("bank_account_id"))
	if err != nil || id == uuid.Nil {
		helpers.Error(c, http.StatusBadRequest, "invalid bank account id")
		return uuid.Nil, false
	}
	return id, true
}

func respondSupplierBankError(c *gin.Context, err error, action string) {
	var ve *models.ValidationError
	switch {
	case errors.Is(err, models.ErrNotFound):
		helpers.Error(c, http.StatusNotFound, "bank account not found")
	case errors.As(err, &ve):
		helpers.Error(c, http.StatusBadRequest, ve.Message)
	default:
		log.Println(action+":", err)
		helpers.Error(c, http.StatusInternalServerError, "could not "+action)
	}
}

// GET /supplier-bank-service/get-supplier-bank-accounts/:supplier_id
func (h *SupplierBankAccountHandler) GetSupplierBankAccounts(c *gin.Context) {
	supplierID, ok := bankSupplierIDParam(c)
	if !ok {
		return
	}
	accounts, err := h.service.GetSupplierBankAccounts(c.Request.Context(), supplierID)
	if err != nil {
		respondSupplierBankError(c, err, "load the bank accounts")
		return
	}
	helpers.Success(c, http.StatusOK, "bank accounts fetched", gin.H{"accounts": accounts})
}

// POST /supplier-bank-service/create-supplier-bank-account/:supplier_id
func (h *SupplierBankAccountHandler) CreateSupplierBankAccount(c *gin.Context) {
	supplierID, ok := bankSupplierIDParam(c)
	if !ok {
		return
	}
	var req models.CreateSupplierBankAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helpers.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	id, err := h.service.CreateSupplierBankAccount(c.Request.Context(), supplierID, req)
	if err != nil {
		respondSupplierBankError(c, err, "create the bank account")
		return
	}
	helpers.Success(c, http.StatusCreated, "bank account created", gin.H{"bank_account_id": id})
}

// PUT /supplier-bank-service/update-supplier-bank-account/:supplier_id/:bank_account_id
func (h *SupplierBankAccountHandler) UpdateSupplierBankAccount(c *gin.Context) {
	supplierID, ok := bankSupplierIDParam(c)
	if !ok {
		return
	}
	accountID, ok := bankAccountIDParam(c)
	if !ok {
		return
	}
	var req models.UpdateSupplierBankAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		helpers.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.service.UpdateSupplierBankAccount(c.Request.Context(), supplierID, accountID, req); err != nil {
		respondSupplierBankError(c, err, "update the bank account")
		return
	}
	helpers.Success(c, http.StatusOK, "bank account updated", gin.H{"bank_account_id": accountID})
}

// DELETE /supplier-bank-service/delete-supplier-bank-account/:supplier_id/:bank_account_id
func (h *SupplierBankAccountHandler) DeleteSupplierBankAccount(c *gin.Context) {
	supplierID, ok := bankSupplierIDParam(c)
	if !ok {
		return
	}
	accountID, ok := bankAccountIDParam(c)
	if !ok {
		return
	}
	if err := h.service.DeleteSupplierBankAccount(c.Request.Context(), supplierID, accountID); err != nil {
		respondSupplierBankError(c, err, "delete the bank account")
		return
	}
	helpers.Success(c, http.StatusOK, "bank account deleted", gin.H{"bank_account_id": accountID})
}
