package helpers

import (
	"fmt"
	"inventory/internals/models"

	"github.com/google/uuid"
)

func StringToUUID(idStr string) (uuid.UUID, error) {
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid UUID: %w", err)
	}
	return id, nil
}

func ConvertToSafeUser(u *models.User) *models.SafeUser {
	if u == nil {
		return nil
	}

	return &models.SafeUser{
		ID:          u.ID,
		Name:        u.Name,
		Email:       u.Email,
		Gender:      u.Gender,
		Role:        u.Role,
		Phone:       u.Phone,
		LastLoginAt: u.LastLoginAt,
		CreatedAt:   u.CreatedAt,
	}
}
