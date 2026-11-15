package routes

import (
	"inventory/internals/app"
	"inventory/internals/middlewares"
	"inventory/internals/rbac"

	"github.com/gin-gonic/gin"
)

func SupplierPaymentServiceRouter(router *gin.RouterGroup, app *app.App) {
	supplierPaymentRoute := router.Group("/supplier-payment-service")
	supplierPaymentRoute.Use(middlewares.UserMiddleware())

	// post
	supplierPaymentRoute.POST(
		"/create-advance-payment",
		app.PermissionChecker.Require(rbac.PaymentCreate),
		app.SupplierPaymentHandler.CreateAdvancePaymentHandlerForSupplier,
	)

}
