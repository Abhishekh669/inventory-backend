package services

import (
	"context"
	"time"

	"inventory/internals/models"
	"inventory/internals/repository"

	"github.com/google/uuid"
)

type ProductService interface {
	GetProductsByCategory(ctx context.Context, slug string, limit, page int, toDate, fromDate *time.Time, name *string, currentStock *int, greaterThan *bool) (*models.ProductsListTypeResponse, error)
	GetProducts(ctx context.Context, limit, page int, toDate, fromDate *time.Time, name *string, currentStock *int, greaterThan *bool) (*models.ProductsListTypeResponse, error)
	CreateNewProduct(ctx context.Context, newProducts *[]models.CreateProductRequestType) error
	UpdateNewProduct(ctx context.Context, updatedProduct *models.UpdateProductRequestType) error
	DeleteNewProducts(ctx context.Context, ids []uuid.UUID) error
}

type productService struct {
	productRepo repository.ProductRepo
}

func NewProductService(productRepo repository.ProductRepo) ProductService {
	return &productService{productRepo: productRepo}
}

func (s *productService) GetProducts(ctx context.Context, limit, page int, toDate, fromDate *time.Time, name *string, currentStock *int, greaterThan *bool) (*models.ProductsListTypeResponse, error) {
	return s.productRepo.GetProducts(ctx, limit, page, toDate, fromDate, name, currentStock, greaterThan)
}

func (s *productService) CreateNewProduct(ctx context.Context, newProducts *[]models.CreateProductRequestType) error {
	return s.productRepo.CreateNewProduct(ctx, newProducts)
}

func (s *productService) UpdateNewProduct(ctx context.Context, updatedProduct *models.UpdateProductRequestType) error {
	return s.productRepo.UpdateNewProduct(ctx, updatedProduct)
}

func (s *productService) DeleteNewProducts(ctx context.Context, ids []uuid.UUID) error {
	return s.productRepo.DeleteNewProducts(ctx, ids)
}

func (s *productService) GetProductsByCategory(ctx context.Context, slug string, limit, page int, toDate, fromDate *time.Time, name *string, currentStock *int, greaterThan *bool) (*models.ProductsListTypeResponse, error) {
	return s.productRepo.GetProductsByCategory(ctx, slug, limit, page, toDate, fromDate, name, currentStock, greaterThan)
}
