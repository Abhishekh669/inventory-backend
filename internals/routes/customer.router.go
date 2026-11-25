// NEW FILE: internals/routes/customer.go
package routes

import (
	"inventory/internals/app"
	"inventory/internals/middlewares"
	"inventory/internals/rbac"

	"github.com/gin-gonic/gin"
)

func CustomerServiceRouter(router *gin.RouterGroup, app *app.App) {
	customerRoute := router.Group("/customer-service")
	customerRoute.Use(middlewares.UserMiddleware())

	// rbac.CustomerView / rbac.CustomerManage are placeholders: use the permissions you already have
	// for customers, or add these two to your rbac package and roles.
	// Cashiers sell to customers, so give them CustomerView and CustomerManage (to add a customer at the counter).

	// get
	customerRoute.GET("/get-customers", app.PermissionChecker.Require(rbac.CustomerView), app.CustomerHandler.GetAllCustomers)

	// post
	customerRoute.POST("/create-customer", app.PermissionChecker.Require(rbac.CustomerManage), app.CustomerHandler.CreateCustomer)

	// put
	customerRoute.PUT("/update-customer/:customer_id", app.PermissionChecker.Require(rbac.CustomerManage), app.CustomerHandler.UpdateCustomer)

	// delete
	customerRoute.DELETE("/delete-customer/:customer_id", app.PermissionChecker.Require(rbac.CustomerManage), app.CustomerHandler.DeleteCustomer)
}

// Register it next to the other routers:
//     routes.CustomerServiceRouter(v1, app)