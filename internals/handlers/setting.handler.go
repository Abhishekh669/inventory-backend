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

type SettingHandler struct {
	settingService services.SettingService
}

func NewSettingHandler(settingService services.SettingService) *SettingHandler {
	return &SettingHandler{settingService: settingService}
}

// handleUnitError maps repository/database errors to HTTP responses.
func handleUnitError(c *gin.Context, err error) {
	var pgErr *pgconn.PgError

	switch {
	case errors.Is(err, repository.ErrUnitNotFound):
		helpers.Error(c, http.StatusNotFound, "unit not found")
	case errors.Is(err, repository.ErrUnitExists):
		helpers.Error(c, http.StatusConflict, "a unit with this name already exists")
	case errors.Is(err, repository.ErrUnitInUse):
		helpers.Error(c, http.StatusConflict, "unit is in use and cannot be deleted")
	case errors.As(err, &pgErr) && pgErr.Code == "22001":
		helpers.Error(c, http.StatusBadRequest, "unit name is too long")
	default:
		helpers.Error(c, http.StatusInternalServerError, err.Error())
	}
}

// ---------- setting ----------

func (h *SettingHandler) GetSetting(c *gin.Context) {
	info, err := h.settingService.GetSetting(c)
	if err != nil {
		helpers.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	helpers.Success(c, http.StatusOK, "setting fetched successfully", info)
}

func (h *SettingHandler) UpdateSetting(c *gin.Context) {
	errMessage := "invalid payload"
	var updateSettingData models.UpdateSettingRequest
	if err := c.ShouldBindJSON(&updateSettingData); err != nil {
		fmt.Println("Error binding JSON in update setting:", err)
		helpers.Error(c, http.StatusBadRequest, errMessage)
		return
	}

	err := h.settingService.UpdateSetting(c, &updateSettingData)
	if err != nil {
		helpers.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	helpers.Success(c, http.StatusOK, "setting updated successfully", updateSettingData)
}

// ---------- units ----------

func (h *SettingHandler) GetAllUnits(c *gin.Context) {
	units, err := h.settingService.GetAllUnits(c.Request.Context())
	if err != nil {
		helpers.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	helpers.Success(c, http.StatusOK, "units fetched successfully", units)
}

func (h *SettingHandler) CreateUnit(c *gin.Context) {
	var data []models.CreateUnit
	if err := c.ShouldBindJSON(&data); err != nil {
		fmt.Println("Error binding JSON in create unit:", err)
		helpers.Error(c, http.StatusBadRequest, "invalid payload")
		return
	}

	if err := h.settingService.CreateUnit(c.Request.Context(), data); err != nil {
		handleUnitError(c, err)
		return
	}

	helpers.Success(c, http.StatusCreated, "units created successfully", data)
}

func (h *SettingHandler) UpdateUnit(c *gin.Context) {
	var data models.UpdateUnit
	if err := c.ShouldBindJSON(&data); err != nil {
		fmt.Println("Error binding JSON in update unit:", err)
		helpers.Error(c, http.StatusBadRequest, "invalid payload")
		return
	}

	if err := h.settingService.UpdateUnit(c.Request.Context(), &data); err != nil {
		handleUnitError(c, err)
		return
	}

	helpers.Success(c, http.StatusOK, "unit updated successfully", data)
}

func (h *SettingHandler) DeleteUnits(c *gin.Context) {
	var data models.DeleteUnits
	if err := c.ShouldBindJSON(&data); err != nil {
		helpers.Error(c, http.StatusBadRequest, "invalid payload")
		return
	}

	ids := make([]uuid.UUID, 0, len(data.IDs))
	for _, id := range data.IDs {
		parsed, err := helpers.StringToUUID(id)
		if err != nil {
			helpers.Error(c, http.StatusBadRequest, "invalid unit ID")
			return
		}
		ids = append(ids, parsed)
	}

	if err := h.settingService.DeleteUnits(c.Request.Context(), ids); err != nil {
		handleUnitError(c, err)
		return
	}

	helpers.Success(c, http.StatusOK, "units deleted successfully", nil)
}
