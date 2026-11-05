package handlers

import (
	"errors"
	"fmt"
	"inventory/internals/helpers"
	"inventory/internals/models"
	"inventory/internals/repository"
	"inventory/internals/services"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type SupplierHandler struct {
	supplierService services.SupplierService
}

func NewSupplierHandler(supplierService services.SupplierService) *SupplierHandler {
	return &SupplierHandler{supplierService: supplierService}
}

// handleSupplierError maps service/repository/database errors to HTTP responses.
func handleSupplierError(c *gin.Context, err error) {
	var pgErr *pgconn.PgError

	switch {
	case errors.Is(err, repository.ErrSupplierNotFound):
		helpers.Error(c, http.StatusNotFound, "supplier not found")
	case errors.Is(err, services.ErrInvalidSupplier):
		helpers.Error(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, repository.ErrSupplierInUse):
		helpers.Error(c, http.StatusConflict, "supplier is in use and cannot be deleted, deactivate it instead")
	case errors.As(err, &pgErr) && pgErr.Code == "23505":
		helpers.Error(c, http.StatusConflict, "a supplier with this name already exists")
	case errors.As(err, &pgErr) && pgErr.Code == "23503":
		helpers.Error(c, http.StatusConflict, "supplier is in use and cannot be deleted, deactivate it instead")
	case errors.As(err, &pgErr) && pgErr.Code == "22001":
		helpers.Error(c, http.StatusBadRequest, "one of the fields is too long")
	default:
		helpers.Error(c, http.StatusInternalServerError, err.Error())
	}
}

func parseSupplierID(c *gin.Context) (uuid.UUID, bool) {
	id, err := helpers.StringToUUID(c.Param("id"))
	if err != nil {
		helpers.Error(c, http.StatusBadRequest, "invalid supplier ID")
		return uuid.Nil, false
	}
	return id, true
}

func (h *SupplierHandler) GetAllSuppliers(c *gin.Context) {
	suppliers, err := h.supplierService.GetSuppliers(c.Request.Context())
	if err != nil {
		handleSupplierError(c, err)
		return
	}

	helpers.Success(c, http.StatusOK, "suppliers fetched successfully", suppliers)
}
func (h *SupplierHandler) GetSupplier(c *gin.Context) {
	id, ok := parseSupplierID(c)
	if !ok {
		return
	}

	supplier, err := h.supplierService.GetSupplierById(c.Request.Context(), id)
	if err != nil {
		handleSupplierError(c, err)
		return
	}

	helpers.Success(c, http.StatusOK, "supplier fetched successfully", supplier)
}

func (h *SupplierHandler) CreateSuppliers(c *gin.Context) {
	var createSuppliersData []models.CreateSupplierInput
	if err := c.ShouldBindJSON(&createSuppliersData); err != nil {
		fmt.Println("Error binding JSON in create suppliers:", err)
		helpers.Error(c, http.StatusBadRequest, "invalid payload")
		return
	}

	suppliers, err := h.supplierService.CreateSuppliers(c.Request.Context(), createSuppliersData)
	if err != nil {
		handleSupplierError(c, err)
		return
	}

	helpers.Success(c, http.StatusCreated, "suppliers created successfully", suppliers)
}

func (h *SupplierHandler) UpdateSupplier(c *gin.Context) {
	id, ok := parseSupplierID(c)
	if !ok {
		return
	}

	var updateSupplierData models.UpdateSupplierInput
	if err := c.ShouldBindJSON(&updateSupplierData); err != nil {
		fmt.Println("Error binding JSON in update supplier:", err)
		helpers.Error(c, http.StatusBadRequest, "invalid payload")
		return
	}

	supplier, err := h.supplierService.UpdateSupplier(c.Request.Context(), id, updateSupplierData)
	if err != nil {
		handleSupplierError(c, err)
		return
	}

	helpers.Success(c, http.StatusOK, "supplier updated successfully", supplier)
}

func (h *SupplierHandler) DeleteSuppliers(c *gin.Context) {
	var deleteSuppliersIds models.DeleteSuppliers
	if err := c.ShouldBindJSON(&deleteSuppliersIds); err != nil {
		helpers.Error(c, http.StatusBadRequest, "invalid payload")
		return
	}

	deleteSuppliersIdsUUID := make([]uuid.UUID, 0, len(deleteSuppliersIds.IDs))
	for _, id := range deleteSuppliersIds.IDs {
		parsedUUID, err := helpers.StringToUUID(id)
		if err != nil {
			helpers.Error(c, http.StatusBadRequest, "invalid supplier ID")
			return
		}
		deleteSuppliersIdsUUID = append(deleteSuppliersIdsUUID, parsedUUID)
	}

	deleted, err := h.supplierService.DeleteSuppliers(c.Request.Context(), deleteSuppliersIdsUUID)
	if err != nil {
		handleSupplierError(c, err)
		return
	}

	helpers.Success(c, http.StatusOK, "suppliers deleted successfully", gin.H{"deleted": deleted})
}
