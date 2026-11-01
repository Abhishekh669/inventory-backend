package middlewares

import (
	"inventory/internals/helpers"
	"net/http"

	"github.com/gin-gonic/gin"
)

func UserMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, err := helpers.ExtractTokenFromHeader(c)
		if err != nil || tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "missing or invalid authorization header",
			})
			c.Abort()
			return
		}

		claims, err := helpers.ParseJwtToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "invalid or expired token",
			})
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserId)
		c.Set("user_email", claims.Email)
		c.Set("last_password_reset_at", claims.LastPasswordResetAt)

		// 4️⃣ Continue to next middleware or route handler
		c.Next()

	}
}
