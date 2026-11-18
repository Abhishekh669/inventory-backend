// NEW FILE: internals/routes/stock_movement.go
package routes

import (
	"inventory/internals/app"
	"inventory/internals/middlewares"
	"inventory/internals/rbac"

	"github.com/gin-gonic/gin"
)

func StockMovementServiceRouter(router *gin.RouterGroup, app *app.App) {
	stockMovementRoute := router.Group("/stock-movement-service")
	stockMovementRoute.Use(middlewares.UserMiddleware())

	// get
	// rbac.StockMovementView is new: add it to your rbac package (or reuse an existing permission such as rbac.ProductView)
	stockMovementRoute.GET("/get-stock-movements", app.PermissionChecker.Require(rbac.StockView), app.StockMovementHandler.GetAllStockMovements)
}

/*
WIRING (do the same as you did for PurchaseHandler):

1. internals/app: add a field to the App struct
       StockMovementHandler *handlers.StockMovementHandler

   and build it where the other handlers are built
       stockMovementRepo := repository.NewStockMovementRepository()
       stockMovementService := services.NewStockMovementService(stockMovementRepo)
       StockMovementHandler: handlers.NewStockMovementHandler(stockMovementService),

2. where the other routers are registered (next to PurchaseServiceRouter):
       routes.StockMovementServiceRouter(v1, app)

3. internals/database schema, inside the "stock_movements" Schema string, after the existing indexes:

       CREATE INDEX IF NOT EXISTS idx_stock_mov_date
           ON stock_movements(movement_date DESC, created_at DESC, id DESC);

       CREATE INDEX IF NOT EXISTS idx_stock_mov_product_date
           ON stock_movements(product_id, movement_date DESC, created_at DESC, id DESC);
*/
