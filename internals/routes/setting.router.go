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
}
