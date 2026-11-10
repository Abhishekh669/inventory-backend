package routes

import (
	"inventory/internals/app"
	"inventory/internals/middlewares"
	"inventory/internals/rbac"

	"github.com/gin-gonic/gin"
)

func PurchaseOrderServiceRouter(router *gin.RouterGroup, app *app.App) {
	purchaseOrderServiceRoute := router.Group("/purchase-order-service")
	purchaseOrderServiceRoute.Use(middlewares.UserMiddleware())

	// get
	purchaseOrderServiceRoute.GET("/get-purchase-orders-by-supplier/:supplier_id", app.PermissionChecker.Require(rbac.PurchaseOrderView), app.PurchaseOrderHandler.GetPurchaseOrdersBySupplier)
	purchaseOrderServiceRoute.GET("/get-purchase-order/:id", app.PermissionChecker.Require(rbac.PurchaseOrderView), app.PurchaseOrderHandler.GetPurchaseOrder)
	purchaseOrderServiceRoute.GET("/get-purchase-order-items/:purchase_order_id", app.PermissionChecker.Require(rbac.PurchaseOrderView), app.PurchaseOrderHandler.GetPurchaseOrderItems)

	// post
	purchaseOrderServiceRoute.POST("/create-purchase-order", app.PermissionChecker.Require(rbac.PurchaseOrderCreate), app.PurchaseOrderHandler.CreatePurchaseOrder)

	// put
	purchaseOrderServiceRoute.PUT("/update-purchase-order/:id", app.PermissionChecker.Require(rbac.PurchaseOrderUpdate), app.PurchaseOrderHandler.UpdatePurchaseOrder)

	// delete
	purchaseOrderServiceRoute.POST("/delete-purchase-orders", app.PermissionChecker.Require(rbac.PurchaseOrderDelete), app.PurchaseOrderHandler.DeletePurchaseOrders)

}
