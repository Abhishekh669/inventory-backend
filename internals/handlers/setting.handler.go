package handlers

import (
	"fmt"
	"inventory/internals/helpers"
	"inventory/internals/models"
	"inventory/internals/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

type SettingHandler struct {
	settingService services.SettingService
}

func NewSettingHandler(settingService services.SettingService) *SettingHandler {
	return &SettingHandler{settingService: settingService}
}

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
		fmt.Println("Error binding JSON in registration :", err)
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
