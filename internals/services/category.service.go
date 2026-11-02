package services

import (
	"context"
	"inventory/internals/models"
	"inventory/internals/repository"

	"github.com/google/uuid"
)

type CategoryService interface {
	GetAllCategories(ctx context.Context) ([]models.Category, error)
	GetCategoryByID(ctx context.Context, id uuid.UUID) (*models.Category, error)
	CreateCategory(ctx context.Context, category *models.CreateCategory) error
	UpdateCategory(ctx context.Context, category *models.UpdateCategory) error
	DeleteCategory(ctx context.Context, ids []uuid.UUID) error
}

type categoryService struct {
	categoryRepo repository.CategoryRepo
}

func (s *categoryService) GetAllCategories(ctx context.Context) ([]models.Category, error) {
	return s.categoryRepo.GetAllCategories(ctx)
}

func (s *categoryService) GetCategoryByID(ctx context.Context, id uuid.UUID) (*models.Category, error) {
	return s.categoryRepo.GetCategoryByID(ctx, id)
}

func (s *categoryService) CreateCategory(ctx context.Context, category *models.CreateCategory) error {
	return s.categoryRepo.CreateCategory(ctx, category.Name, category.Slug)
}

func (s *categoryService) UpdateCategory(ctx context.Context, category *models.UpdateCategory) error {
	return s.categoryRepo.UpdateCategory(ctx, category.ID, category.Name, category.Slug)
}

func (s *categoryService) DeleteCategory(ctx context.Context, ids []uuid.UUID) error {
	return s.categoryRepo.DeleteCategory(ctx, ids)
}

func NewCategoryService(categoryRepo repository.CategoryRepo) CategoryService {
	return &categoryService{categoryRepo: categoryRepo}
}
