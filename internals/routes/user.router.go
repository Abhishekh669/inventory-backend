package routes

import (
	"inventory/internals/app"

	"github.com/gin-gonic/gin"
)

func UserServiceRouter(router *gin.RouterGroup, app *app.App) {
	userServiceRoute := router.Group("/user-service")
	userServiceRoute.GET("/get-user-from-token", app.UserHandler.GetUserFromTokenHandler)
	//post
	userServiceRoute.POST("/login", app.UserHandler.LoginUserHandler)
}
