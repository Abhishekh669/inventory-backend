package routes

import (
	"inventory/internals/app"
	"inventory/internals/middlewares"
	"inventory/internals/rbac"

	"github.com/gin-gonic/gin"
)

func SupplierServiceRouter(router *gin.RouterGroup, app *app.App) {
	supplierServiceRoute := router.Group("/supplier-service")
	supplierServiceRoute.Use(middlewares.UserMiddleware())

	// get
	supplierServiceRoute.GET("/get-suppliers", app.PermissionChecker.Require(rbac.SupplierView), app.SupplierHandler.GetAllSuppliers)
	supplierServiceRoute.GET("/get-supplier/:id", app.PermissionChecker.Require(rbac.SupplierView), app.SupplierHandler.GetSupplier)

	// post
	supplierServiceRoute.POST("/create-suppliers", app.PermissionChecker.Require(rbac.SupplierCreate), app.SupplierHandler.CreateSuppliers)

	// put
	supplierServiceRoute.PUT("/update-supplier/:id", app.PermissionChecker.Require(rbac.SupplierUpdate), app.SupplierHandler.UpdateSupplier)

	// delete
	supplierServiceRoute.POST("/delete-suppliers", app.PermissionChecker.Require(rbac.SupplierDelete), app.SupplierHandler.DeleteSuppliers)
}
