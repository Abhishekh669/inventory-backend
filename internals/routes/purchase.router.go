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
	purchaseServiceRoute.GET("/get-purchases", app.PermissionChecker.Require(rbac.PurchaseView), app.PurchaseHandler.GetAllPurchases)

	// post
	purchaseServiceRoute.POST("/create-purchase", app.PermissionChecker.Require(rbac.PurchaseCreate), app.PurchaseHandler.CreatePurchase)
}
