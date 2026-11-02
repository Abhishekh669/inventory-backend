package models

import (
	"time"

	"github.com/google/uuid"
)

type Category struct {
	ID        uuid.UUID `db:"id" json:"id"`
	Name      string    `db:"name" json:"name"`
	Slug      string    `db:"slug" json:"slug"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type CreateCategory struct {
	Name string `db:"name" json:"name"`
	Slug string `db:"slug" json:"slug"`
}

type UpdateCategory struct {
	ID   uuid.UUID `db:"id" json:"id"`
	Name string    `db:"name" json:"name"`
	Slug string    `db:"slug" json:"slug"`
}

type DeleteCategories struct {
	IDs []string `json:"ids"`
}
