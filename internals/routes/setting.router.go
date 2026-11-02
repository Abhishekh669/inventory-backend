package routes

import (
	"inventory/internals/app"
	"inventory/internals/middlewares"
	"inventory/internals/rbac"

	"github.com/gin-gonic/gin"
)

func SettingServiceRouter(router *gin.RouterGroup, app *app.App) {
	settingServiceRoute := router.Group("/setting-service")
	settingServiceRoute.Use(middlewares.UserMiddleware())
	settingServiceRoute.GET("/get-setting", app.PermissionChecker.Require(rbac.GetSetting), (app.SettingHandler.GetSetting))
	//post
	settingServiceRoute.PUT("/update-setting", app.PermissionChecker.Require(rbac.UpdateSetting), app.SettingHandler.UpdateSetting)

	//units
	settingServiceRoute.GET("/get-units", app.SettingHandler.GetAllUnits)
	settingServiceRoute.POST("/create-units", app.SettingHandler.CreateUnit)
	settingServiceRoute.PUT("/update-unit", app.SettingHandler.UpdateUnit)
	settingServiceRoute.POST("/delete-units", app.SettingHandler.DeleteUnits)

}
