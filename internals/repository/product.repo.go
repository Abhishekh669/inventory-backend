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
