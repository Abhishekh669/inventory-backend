package middleware

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"inventory/internals/models"
	"inventory/internals/rbac"
	"inventory/internals/repository"
)

// Context keys. The JWT middleware sets the first two, this file sets the third.
const (
	CtxUserEmail           = "user_email"
	CtxLastPasswordResetAt = "last_password_reset_at"
	CtxCurrentUser         = "current_user"
)

var (
	ErrUnauthorized   = errors.New("unauthorized")             // 401: log in again
	ErrSessionExpired = errors.New("session expired")          // 401: password was reset
	ErrForbidden      = errors.New("insufficient permissions") // 403: role not allowed
)

type PermissionChecker struct {
	users repository.UserRepo
}

func NewPermissionChecker(users repository.UserRepo) *PermissionChecker {
	return &PermissionChecker{users: users}
}

// Check validates the session and the permission. It returns the current user.
func (p *PermissionChecker) Check(c *gin.Context, action rbac.Permission) (*models.User, error) {
	email := c.GetString(CtxUserEmail)
	if email == "" {
		return nil, ErrUnauthorized
	}

	// Token's password-reset stamp. Missing or wrong type becomes 0.
	var tokenStamp int64
	if v, ok := c.Get(CtxLastPasswordResetAt); ok {
		if ts, ok := v.(int64); ok {
			tokenStamp = ts
		} else {
			log.Printf("auth: last_password_reset_at has type %T, expected int64", v)
		}
	}

	user, err := p.users.GetUserByEmail(c.Request.Context(), email)
	if err != nil || user == nil {
		return nil, ErrUnauthorized
	}

	// 1. Session check first, so an old token gets 401 and not 403.
	if user.LastPasswordResetAt != tokenStamp {
		log.Printf("auth: stale session for user id=%s", user.ID)
		return nil, ErrSessionExpired
	}

	// 2. Then the permission.
	if user.Role == "" || !rbac.HasPermission(&user.Role, action) {
		log.Printf("auth: user id=%s role=%s denied %s", user.ID, user.Role, action)
		return nil, ErrForbidden
	}

	return user, nil
}

// Require is the route middleware: r.POST("/x", checker.Require(rbac.SaleCreate), handler)
func (p *PermissionChecker) Require(action rbac.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := p.Check(c, action)
		if err != nil {
			abortWithAuthError(c, err)
			return
		}
		c.Set(CtxCurrentUser, user) // handlers read this for created_by
		c.Next()
	}
}

// RequireAny allows the request if the user has at least one of the permissions.
func (p *PermissionChecker) RequireAny(actions ...rbac.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		var lastErr error
		for _, a := range actions {
			user, err := p.Check(c, a)
			if err == nil {
				c.Set(CtxCurrentUser, user)
				c.Next()
				return
			}
			lastErr = err
			if !errors.Is(err, ErrForbidden) {
				break // session problem, no point trying others
			}
		}
		abortWithAuthError(c, lastErr)
	}
}

func abortWithAuthError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		c.AbortWithStatusJSON(http.StatusForbidden,
			gin.H{"error": "you do not have permission for this action"})
	case errors.Is(err, ErrUnauthorized), errors.Is(err, ErrSessionExpired):
		c.AbortWithStatusJSON(http.StatusUnauthorized,
			gin.H{"error": "please log in again"})
	default:
		c.AbortWithStatusJSON(http.StatusInternalServerError,
			gin.H{"error": "something went wrong"})
	}
}

// CurrentUser is a helper for handlers.
func CurrentUser(c *gin.Context) *models.User {
	u, _ := c.Get(CtxCurrentUser)
	user, _ := u.(*models.User)
	return user
}

// HasPermissionCheck keeps your old function signature for code that still calls it.
func (p *PermissionChecker) HasPermissionCheck(c *gin.Context, action rbac.Permission) (*models.RoleType, error) {
	user, err := p.Check(c, action)
	if err != nil {
		return nil, err
	}
	return &user.Role, nil
}

var _ = context.Background // keep the import if you add ctx helpers later
