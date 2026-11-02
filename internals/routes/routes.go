package routes

import (
	"inventory/internals/app"

	"github.com/gin-gonic/gin"
)

func SetUpRoutes(app *gin.Engine, appConfig *app.App) {
	apiGroup := app.Group("/api/v1")
	UserServiceRouter(apiGroup, appConfig)
	SettingServiceRouter(apiGroup, appConfig)
	CategoryServiceRouter(apiGroup, appConfig)

}
