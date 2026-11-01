package helpers

import (
	"errors"
	"fmt"
	"inventory/internals/config"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserId              string `json:"user_id"`
	Email               string `json:"email"`
	LastPasswordResetAt int64  `json:"last_password_reset_at"`
	TokenType           string `json:"token_type"`
	jwt.RegisteredClaims
}

type JwtDataType struct {
	UserId              string
	Email               string
	LastPasswordResetAt int64
}

func GenerateJWTToken(jwtData *JwtDataType) (string, error) {
	if jwtData == nil || jwtData.Email == "" || jwtData.UserId == "" {
		return "", fmt.Errorf("invalid user data")
	}

	secret := config.AppConfig.JWTToken
	if secret == "" {
		return "", fmt.Errorf("invalid jwt secret")
	}

	now := time.Now()

	claims := &Claims{
		UserId:              jwtData.UserId,
		Email:               jwtData.Email,
		LastPasswordResetAt: jwtData.LastPasswordResetAt,
		TokenType:           "access",

		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(7 * 24 * time.Hour)),
			Issuer:    "golang-backend",
			Subject:   jwtData.UserId,
			Audience:  []string{"user"},
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ParseJwtToken(tokenString string) (*Claims, error) {
	if tokenString == "" {
		return nil, fmt.Errorf("empty token")
	}

	secret := config.AppConfig.JWTToken
	if secret == "" {
		return nil, errors.New("jwt secret not configured")
	}

	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return []byte(secret), nil
		},
		jwt.WithIssuer("golang-backend"),
		jwt.WithAudience("user"),
		jwt.WithLeeway(2*time.Minute),
	)

	if err != nil {
		switch {
		case errors.Is(err, jwt.ErrTokenExpired):
			return nil, errors.New("token expired")
		case errors.Is(err, jwt.ErrTokenMalformed):
			return nil, errors.New("malformed token")
		case errors.Is(err, jwt.ErrTokenNotValidYet):
			return nil, errors.New("token not valid yet")
		case errors.Is(err, jwt.ErrTokenSignatureInvalid):
			return nil, errors.New("invalid token signature")
		default:
			return nil, errors.New("invalid token")
		}
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	if claims.UserId == "" || claims.Email == "" {
		return nil, errors.New("token missing required claims")
	}

	if claims.TokenType != "access" {
		return nil, errors.New("invalid token type")
	}

	return claims, nil
}
