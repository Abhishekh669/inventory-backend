package repository

import (
	"context"
	"errors"
	"inventory/internals/database"
	"inventory/internals/models"
	"log"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrCategoryNotFound = errors.New("category not found")

type CategoryRepo interface {
	GetAllCategories(ctx context.Context) ([]models.Category, error)
	GetCategoryByID(ctx context.Context, id uuid.UUID) (*models.Category, error)
	CreateCategory(ctx context.Context, name string, slug string) error
	UpdateCategory(ctx context.Context, id uuid.UUID, name string, slug string) error
	DeleteCategory(ctx context.Context, ids []uuid.UUID) error
}

type categoryRepo struct {
	pool *pgxpool.Pool
}

func (r *categoryRepo) GetAllCategories(ctx context.Context) ([]models.Category, error) {
	const query = `
		SELECT id, name, slug, created_at
		FROM categories
		ORDER BY name ASC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// pgx.CollectRows maps columns to struct fields using the `db` tags
	categories, err := pgx.CollectRows(rows, pgx.RowToStructByName[models.Category])
	if err != nil {
		return nil, err
	}

	return categories, nil
}

func (r *categoryRepo) GetCategoryByID(ctx context.Context, id uuid.UUID) (*models.Category, error) {
	const query = `
		SELECT id, name, slug, created_at
		FROM categories
		WHERE id = $1
	`

	rows, err := r.pool.Query(ctx, query, id)
	if err != nil {
		return nil, err
	}

	category, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[models.Category])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCategoryNotFound
		}
		return nil, err
	}

	return &category, nil
}

func (r *categoryRepo) CreateCategory(ctx context.Context, name string, slug string) error {
	const query = `
		INSERT INTO categories (name, slug)
		VALUES ($1, $2)
	`

	_, err := r.pool.Exec(ctx, query, name, slug)
	return err
}

func (r *categoryRepo) UpdateCategory(ctx context.Context, id uuid.UUID, name string, slug string) error {
	const query = `
		UPDATE categories
		SET name = $1, slug = $2
		WHERE id = $3
	`

	tag, err := r.pool.Exec(ctx, query, name, slug, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrCategoryNotFound
	}

	return nil
}

func (r *categoryRepo) DeleteCategory(ctx context.Context, ids []uuid.UUID) error {
	const query = `DELETE FROM categories WHERE id = ANY($1)`

	tag, err := r.pool.Exec(ctx, query, ids)
	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return ErrCategoryNotFound
	}

	return nil
}

func NewCategoryRepository() CategoryRepo {
	pool, err := database.GetPostgresPool()
	if err != nil {
		log.Println("Failed to initialize pool:", err)
		return nil
	}

	return &categoryRepo{
		pool: pool,
	}
}
