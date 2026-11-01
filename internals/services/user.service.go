package services

import (
	"context"
	"errors"
	"inventory/internals/helpers"
	"inventory/internals/models"
	"inventory/internals/repository"

	"github.com/google/uuid"
)

type UserService interface {
	//get
	GetUserById(ctx context.Context, userId uuid.UUID) (*models.User, error)
	//post
	LoginUserService(email, password string, ctx context.Context) (*string, error)
}

type userService struct {
	repo repository.UserRepo
}

func (s *userService) GetUserById(ctx context.Context, userId uuid.UUID) (*models.User, error) {
	user, err := s.repo.GetUserById(ctx, userId)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (s *userService) LoginUserService(email, password string, ctx context.Context) (*string, error) {
	failedMessage := "incorrect credentials"
	dbUser, err := s.repo.GetUserByEmail(ctx, email)

	if dbUser == nil || err != nil {
		return nil, errors.New(failedMessage)
	}

	status, err := helpers.CheckPasswordHash(password, dbUser.Password)
	if err != nil || !status {
		return nil, errors.New(failedMessage)
	}

	jwtData := helpers.JwtDataType{
		UserId:              dbUser.ID.String(),
		Email:               dbUser.Email,
		LastPasswordResetAt: dbUser.LastPasswordResetAt,
	}

	jwtToken, err := helpers.GenerateJWTToken(&jwtData)
	if err != nil {
		return nil, errors.New(failedMessage)
	}

	return &jwtToken, nil
}

func NewUserService(repo repository.UserRepo) UserService {
	return &userService{
		repo: repo,
	}
}
