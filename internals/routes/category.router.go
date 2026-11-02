package routes

import (
	"inventory/internals/app"
	"inventory/internals/middlewares"
	"inventory/internals/rbac"

	"github.com/gin-gonic/gin"
)

func CategoryServiceRouter(router *gin.RouterGroup, app *app.App) {
	categoryServiceRoute := router.Group("/category-service")
	categoryServiceRoute.Use(middlewares.UserMiddleware())

	categoryServiceRoute.GET("/get-categories", app.PermissionChecker.Require(rbac.CategoryView), app.CategoryHandler.GetAllCategories)
	categoryServiceRoute.POST("/create-category", app.PermissionChecker.Require(rbac.CategoryCreate), app.CategoryHandler.CreateCategory)
	categoryServiceRoute.PUT("/update-category", app.PermissionChecker.Require(rbac.CategoryUpdate), app.CategoryHandler.UpdateCategory)
	categoryServiceRoute.POST("/delete-categories", app.PermissionChecker.Require(rbac.CategoryDelete), app.CategoryHandler.DeleteCategory)
}
