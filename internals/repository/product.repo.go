package repository

import (
	"inventory/internals/database"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ProductRepo interface {
}

type productRepo struct {
	pool *pgxpool.Pool
}

// func (r *productRepo) CreateNewProduct(ctx context.Context, newProduct *models.CreateProductRequestType) error {
// 	return nil
// }

// func (r *productRepo) UpdateNewProduct(ctx context.Context, updatedProduct *models.UpdateProductRequestType) error {

// }

// func (r *productRepo) DeleteNewProducts(ctx context.Context, ids []uuid.UUID) error {

// }

// func (r *productRepo) GetProducts(ctx context.Context, limit int, page int, toDate *time.Time, fromDate *time.Time, name *string, currentStock *int, greaterThan *bool) (*models.ProductsListTypeResponse, error) {

// }

func NewProductRepository() ProductRepo {
	pool, err := database.GetPostgresPool()
	if err != nil {
		log.Println("Failed to initialize pool pool:", err)
		return nil
	}

	return &productRepo{
		pool: pool,
	}
}
