package helpers

import (
	"fmt"
	"inventory/internals/models"

	"github.com/gin-gonic/gin"
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

type UserContext struct {
	UserID              uuid.UUID `json:"user_id"`
	Email               string    `json:"email"`
	LastPasswordResetAt int64     `json:"last_password_reset_at"`
}

func GetUserContext(c *gin.Context) (*UserContext, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return nil, false
	}

	email, exists := c.Get("user_email")
	if !exists {
		return nil, false
	}

	lastPasswordResetAt, exists := c.Get("last_password_reset_at")
	if !exists {
		return nil, false
	}

	uid, ok := userID.(string)

	if !ok {
		return nil, false
	}

	userUUID, err := StringToUUID(uid)
	if err != nil {
		return nil, false
	}

	userEmail, ok := email.(string)
	if !ok {
		return nil, false
	}

	resetAt, ok := lastPasswordResetAt.(int64)

	if !ok {
		return nil, false
	}

	return &UserContext{
		UserID:              userUUID,
		Email:               userEmail,
		LastPasswordResetAt: resetAt,
	}, true
}
