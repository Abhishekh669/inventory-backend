package app

import (
	"context"
	authMiddleware "inventory/internals/auth"
	"inventory/internals/handlers"
	"inventory/internals/repository"
	"inventory/internals/services"
	"log"
)

type App struct {
	UserRepo    repository.UserRepo
	UserService services.UserService
	UserHandler handlers.UserHandler

	SettingRepo    repository.SettingRepo
	SettingService services.SettingService
	SettingHandler handlers.SettingHandler

	CategoryRepo    repository.CategoryRepo
	CategoryService services.CategoryService
	CategoryHandler handlers.CategoryHandler

	ProductRepo    repository.ProductRepo
	ProductService services.ProductService
	ProductHandler handlers.ProductHandler

	SupplierRepo    repository.SupplierRepo
	SupplierService services.SupplierService
	SupplierHandler handlers.SupplierHandler

	PurchaseOrderRepo    repository.PurchaseOrderRepository
	PurchaseOrderService services.PurchaseOrderService
	PurchaseOrderHandler handlers.PurchaseOrderHandler

	PurchaseRepo    repository.PurchaseRepository
	PurchaseService services.PurchaseService
	PurchaseHandler handlers.PurchaseHandler

	SupplierPaymentRepo    repository.SupplierPaymentRepository
	SupplierPaymentService services.SupplierPaymentService
	SupplierPaymentHandler handlers.SupplierPaymentHandler

	StockMovementRepo    repository.StockMovementRepository
	StockMovementService services.StockMovementService
	StockMovementHandler handlers.StockMovementHandler

	PermissionChecker authMiddleware.PermissionChecker
}

func New() (*App, error) {
	userRepo := repository.NewUserRepository()
	err := userRepo.EnsureAdminExists(context.Background())
	if err != nil {
		log.Println("error in ensuring admin user exists: ", err)
		return nil, err
	}
	checker := authMiddleware.NewPermissionChecker(userRepo)

	userService := services.NewUserService(userRepo)
	userHandler := handlers.NewUserHandler(userService)

	settingRepo := repository.NewSettingRepository()
	settingService := services.NewSettingService(settingRepo)
	settingHandler := handlers.NewSettingHandler(settingService)

	categoryRepo := repository.NewCategoryRepository()
	categoryService := services.NewCategoryService(categoryRepo)
	categoryHandler := handlers.NewCategoryHandler(categoryService)

	productRepo := repository.NewProductRepository()
	productService := services.NewProductService(productRepo)
	productHandler := handlers.NewProductHandler(productService)

	supplierRepo := repository.NewSupplierRepository()
	supplierService := services.NewSupplierService(supplierRepo)
	supplierHandler := handlers.NewSupplierHandler(supplierService)

	purchaseOrderRepo := repository.NewPurchaseOrderRepository()
	purchaseOrderService := services.NewPurchaseOrderService(purchaseOrderRepo)
	purchaseOrderHandler := handlers.NewPurchaseOrderHandler(purchaseOrderService)

	purchaseRepo := repository.NewPurchaseRepository()
	purchaseService := services.NewPurchaseService(purchaseRepo)
	purchaseHandler := handlers.NewPurchaseHandler(purchaseService)

	// Supplier Payment
	supplierPaymentRepo := repository.NewSupplierPaymentRepository()
	supplierPaymentService := services.NewSupplierPaymentService(supplierPaymentRepo)
	supplierPaymentHandler := handlers.NewSupplierPaymentHandler(supplierPaymentService)

	// Stock Movement
	stockMovementRepo := repository.NewStockMovementRepository()
	stockMovementService := services.NewStockMovementService(stockMovementRepo)
	stockMovementHandler := handlers.NewStockMovementHandler(stockMovementService)

	return &App{
		UserRepo:    userRepo,
		UserService: userService,
		UserHandler: *userHandler,

		SettingRepo:    settingRepo,
		SettingService: settingService,
		SettingHandler: *settingHandler,

		CategoryRepo:    categoryRepo,
		CategoryService: categoryService,
		CategoryHandler: *categoryHandler,

		ProductRepo:    productRepo,
		ProductService: productService,
		ProductHandler: *productHandler,

		SupplierRepo:    supplierRepo,
		SupplierService: supplierService,
		SupplierHandler: *supplierHandler,

		PurchaseOrderRepo:    purchaseOrderRepo,
		PurchaseOrderService: purchaseOrderService,
		PurchaseOrderHandler: *purchaseOrderHandler,

		PurchaseRepo:    purchaseRepo,
		PurchaseService: purchaseService,
		PurchaseHandler: *purchaseHandler,

		SupplierPaymentRepo:    supplierPaymentRepo,
		SupplierPaymentService: supplierPaymentService,
		SupplierPaymentHandler: *supplierPaymentHandler,

		StockMovementRepo:    stockMovementRepo,
		StockMovementService: stockMovementService,
		StockMovementHandler: *stockMovementHandler,

		PermissionChecker: *checker,
	}, nil
}
