package models

import (
	"time"

	"github.com/google/uuid"
)

type GenderType string

const (
	GenderMale   GenderType = "male"
	GenderFemale GenderType = "female"
	GenderOther  GenderType = "other"
)

type RoleType string

const (
	RoleAdmin   RoleType = "admin"
	RoleManager RoleType = "manager"
	RoleCashier RoleType = "cashier"
)

type User struct {
	ID                  uuid.UUID  `db:"id" json:"id"`
	Name                string     `db:"name" json:"name"`
	Email               string     `db:"email" json:"email"`
	Password            string     `db:"password" json:"-"`
	Gender              GenderType `db:"gender" json:"gender"`
	Role                RoleType   `db:"role" json:"role"`
	Phone               string     `db:"phone" json:"phone"`
	LastPasswordResetAt int64      `db:"last_password_reset_at" json:"last_password_reset_at"`
	CreatedAt           time.Time  `db:"created_at" json:"created_at"`
	LastLoginAt         time.Time  `db:"last_login_at" json:"last_login_at"`
}

type SafeUser struct {
	ID          uuid.UUID  `db:"id" json:"id"`
	Name        string     `db:"name" json:"name"`
	Email       string     `db:"email" json:"email"`
	Role        RoleType   `db:"role" json:"role"`
	Gender      GenderType `db:"gender" json:"gender"`
	Phone       string     `db:"phone" json:"phone"`
	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
	LastLoginAt time.Time  `db:"last_login_at" json:"last_login_at"`
}

type UserLogin struct {
	Email    string `json:"email" db:"email"`
	Password string `json:"password" db:"password"`
}

type PasswordResetRequest struct {
	ID           uuid.UUID `json:"id" db:"id"`
	Email        string    `json:"email" db:"email"`
	SessionToken string    `json:"session_token" db:"session_token"`
	PinCode      string    `json:"pin_code" db:"pin_code"`
	IsUsed       bool      `json:"is_used" db:"is_used"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}
