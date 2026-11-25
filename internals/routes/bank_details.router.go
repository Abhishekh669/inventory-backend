// NEW FILE: internals/routes/supplier_bank_account.go
package routes

import (
	"inventory/internals/app"
	"inventory/internals/middlewares"
	"inventory/internals/rbac"

	"github.com/gin-gonic/gin"
)

func SupplierBankAccountServiceRouter(router *gin.RouterGroup, app *app.App) {
	bankRoute := router.Group("/supplier-bank-service")
	bankRoute.Use(middlewares.UserMiddleware())

	// Use the permissions you already use for suppliers. rbac.SupplierView / rbac.SupplierManage
	// are placeholders: rename them, or add them to your rbac package.

	// get
	bankRoute.GET("/get-supplier-bank-accounts/:supplier_id", app.PermissionChecker.Require(rbac.SupplierView), app.SupplierBankAccountHandler.GetSupplierBankAccounts)

	// post
	bankRoute.POST("/create-supplier-bank-account/:supplier_id", app.PermissionChecker.Require(rbac.SupplierCreate), app.SupplierBankAccountHandler.CreateSupplierBankAccount)

	// put
	bankRoute.PUT("/update-supplier-bank-account/:supplier_id/:bank_account_id", app.PermissionChecker.Require(rbac.SupplierUpdate), app.SupplierBankAccountHandler.UpdateSupplierBankAccount)

	// delete
	bankRoute.DELETE("/delete-supplier-bank-account/:supplier_id/:bank_account_id", app.PermissionChecker.Require(rbac.SupplierDelete), app.SupplierBankAccountHandler.DeleteSupplierBankAccount)
}

// Register it next to the other routers:
//     routes.SupplierBankAccountServiceRouter(v1, app)
