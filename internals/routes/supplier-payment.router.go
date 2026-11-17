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

	// get
	supplierPaymentRoute.GET(
		"/get-supplier-payments/:supplier_id",
		app.PermissionChecker.Require(rbac.PaymentCreate), // use the read permission you have
		app.SupplierPaymentHandler.GetSupplierPayments,
	)

	// post
	supplierPaymentRoute.POST(
		"/create-advance-payment",
		app.PermissionChecker.Require(rbac.PaymentCreate),
		app.SupplierPaymentHandler.CreateAdvancePaymentHandlerForSupplier,
	)

	supplierPaymentRoute.GET(
		"/get-supplier-ledger/:supplier_id",
		app.PermissionChecker.Require(rbac.PaymentCreate), // swap for your read permission
		app.SupplierPaymentHandler.GetSupplierLedger,
	)
}
