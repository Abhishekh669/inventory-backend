package routes

import (
	"inventory/internals/app"
	"inventory/internals/middlewares"
	"inventory/internals/rbac"

	"github.com/gin-gonic/gin"
)

func ProductServiceRouter(router *gin.RouterGroup, app *app.App) {
	productServiceRoute := router.Group("/product-service")
	productServiceRoute.Use(middlewares.UserMiddleware())

	// get
	productServiceRoute.GET("/get-products", app.PermissionChecker.Require(rbac.ProductView), app.ProductHandler.GetProducts)

	// post
	productServiceRoute.POST("/create-products", app.PermissionChecker.Require(rbac.ProductCreate), app.ProductHandler.CreateProduct)

	// put
	productServiceRoute.PUT("/update-product", app.PermissionChecker.Require(rbac.ProductUpdate), app.ProductHandler.UpdateProduct)

	// delete (POST with ids in body, same as categories and units)
	productServiceRoute.POST("/delete-products", app.PermissionChecker.Require(rbac.ProductDelete), app.ProductHandler.DeleteProducts)

	productServiceRoute.GET("/get-products/category/:slug", app.PermissionChecker.Require(rbac.ProductView), app.ProductHandler.GetProductsByCategory)
	productServiceRoute.PUT("/update-product-price", app.PermissionChecker.Require(rbac.ProductUpdate), app.ProductHandler.UpdateProductPrice)
}
