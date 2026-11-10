package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"inventory/internals/models"
	"inventory/internals/repository"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type ProductService interface {
	GetProductsByCategory(ctx context.Context, slug string, limit, page int, toDate, fromDate *time.Time, name *string, currentStock *int, greaterThan *bool) (*models.ProductsListTypeResponse, error)
	GetProducts(ctx context.Context, limit, page int, toDate, fromDate *time.Time, name *string, currentStock *int, greaterThan *bool) (*models.ProductsListTypeResponse, error)
	CreateNewProduct(ctx context.Context, newProducts *[]models.CreateProductRequestType, userId uuid.UUID) error
	UpdateNewProduct(ctx context.Context, updatedProduct *models.UpdateProductRequestType) error
	DeleteNewProducts(ctx context.Context, ids []uuid.UUID) error
	UpdateProductPrice(ctx context.Context, in *models.UpdateProdcutPrice) (*models.ProductPriceResult, error)
}

type productService struct {
	productRepo repository.ProductRepo
}

var ErrInvalidProductPrice = errors.New("invalid product price")

func (s *productService) UpdateProductPrice(ctx context.Context, in *models.UpdateProdcutPrice) (*models.ProductPriceResult, error) {
	if in.ID == uuid.Nil {
		return nil, fmt.Errorf("%w: product id is required", ErrInvalidProductPrice)
	}
	if in.OldPrice.IsNegative() {
		return nil, fmt.Errorf("%w: old price cannot be negative", ErrInvalidProductPrice)
	}
	if !in.NewPrice.IsPositive() {
		return nil, fmt.Errorf("%w: new price must be greater than 0", ErrInvalidProductPrice)
	}
	if in.NewPrice.GreaterThan(decimal.NewFromInt(100_000_000)) {
		return nil, fmt.Errorf("%w: new price is too large", ErrInvalidProductPrice)
	}
	if in.CurrentVat.IsNegative() || in.CurrentVat.GreaterThan(decimal.NewFromInt(100)) {
		return nil, fmt.Errorf("%w: VAT must be between 0 and 100", ErrInvalidProductPrice)
	}
	if in.CurrentMargin.IsNegative() || in.CurrentMargin.GreaterThan(decimal.NewFromInt(1000)) {
		return nil, fmt.Errorf("%w: margin must be between 0 and 1000", ErrInvalidProductPrice)
	}

	one := decimal.NewFromInt(1)
	hundred := decimal.NewFromInt(100)

	costPrice := in.NewPrice.Round(4)                                                // new unit cost
	costWithVat := costPrice.Mul(one.Add(in.CurrentVat.Div(hundred))).Round(4)       // cost + VAT
	sellingPrice := costWithVat.Mul(one.Add(in.CurrentMargin.Div(hundred))).Round(2) // cost with VAT + margin

	return s.productRepo.UpdateProductPrice(ctx, &models.ProductPriceUpdate{
		ID:            in.ID,
		OldCost:       in.OldPrice,
		VatRate:       in.CurrentVat.Round(2),
		MarginPercent: in.CurrentMargin.Round(2),
		CostPrice:     costPrice,
		CostWithVat:   costWithVat,
		SellingPrice:  sellingPrice,
	})
}

func NewProductService(productRepo repository.ProductRepo) ProductService {
	return &productService{productRepo: productRepo}
}

func (s *productService) GetProducts(ctx context.Context, limit, page int, toDate, fromDate *time.Time, name *string, currentStock *int, greaterThan *bool) (*models.ProductsListTypeResponse, error) {
	return s.productRepo.GetProducts(ctx, limit, page, toDate, fromDate, name, currentStock, greaterThan)
}

func (s *productService) CreateNewProduct(ctx context.Context, newProducts *[]models.CreateProductRequestType, userId uuid.UUID) error {
	return s.productRepo.CreateNewProduct(ctx, newProducts, userId)
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
