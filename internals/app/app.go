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

	return &App{
		UserRepo:    userRepo,
		UserService: userService,
		UserHandler: *userHandler,

		SettingRepo:       settingRepo,
		SettingService:    settingService,
		SettingHandler:    *settingHandler,
		PermissionChecker: *checker,
	}, nil
}
