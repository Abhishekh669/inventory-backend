package routes

import (
	"inventory/internals/app"
	"inventory/internals/middlewares"
	"inventory/internals/rbac"

	"github.com/gin-gonic/gin"
)

func PurchaseServiceRouter(router *gin.RouterGroup, app *app.App) {
	purchaseServiceRoute := router.Group("/purchase-service")
	purchaseServiceRoute.Use(middlewares.UserMiddleware())

	// get
	purchaseServiceRoute.GET("/get-purchase/:purchase_id", app.PermissionChecker.Require(rbac.PurchaseView), app.PurchaseHandler.GetPurchaseDataById)
	purchaseServiceRoute.GET("/get-purchases", app.PermissionChecker.Require(rbac.PurchaseView), app.PurchaseHandler.GetAllPurchases)

	// post
	purchaseServiceRoute.POST("/create-purchase", app.PermissionChecker.Require(rbac.PurchaseCreate), app.PurchaseHandler.CreatePurchase)
	purchaseServiceRoute.POST("/create-purchase-return/:purchase_id", app.PermissionChecker.Require(rbac.OrderManage), app.PurchaseHandler.CreatePurchaseReturn)
	purchaseServiceRoute.POST("/create-purchase-payment/:purchase_id", app.PermissionChecker.Require(rbac.OrderManage), app.PurchaseHandler.CreatePurchasePayment)

	// put
	purchaseServiceRoute.PUT("/update-purchase/:purchase_id", app.PermissionChecker.Require(rbac.OrderManage), app.PurchaseHandler.UpdatePurchase)
	purchaseServiceRoute.PUT("/update-purchase-return/:purchase_id/:return_id", app.PermissionChecker.Require(rbac.OrderManage), app.PurchaseHandler.UpdatePurchaseReturn)
	purchaseServiceRoute.PUT("/update-purchase-payment/:purchase_id/:payment_id", app.PermissionChecker.Require(rbac.OrderManage), app.PurchaseHandler.UpdatePurchasePayment)

	// delete
	purchaseServiceRoute.DELETE("/delete-purchase/:purchase_id", app.PermissionChecker.Require(rbac.OrderManage), app.PurchaseHandler.DeletePurchase)
	purchaseServiceRoute.DELETE("/delete-purchase-return/:purchase_id/:return_id", app.PermissionChecker.Require(rbac.OrderManage), app.PurchaseHandler.DeletePurchaseReturn)
	purchaseServiceRoute.DELETE("/delete-purchase-payment/:purchase_id/:payment_id", app.PermissionChecker.Require(rbac.OrderManage), app.PurchaseHandler.DeletePurchasePayment)
}
