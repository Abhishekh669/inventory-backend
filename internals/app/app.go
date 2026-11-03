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

		PermissionChecker: *checker,
	}, nil
}
