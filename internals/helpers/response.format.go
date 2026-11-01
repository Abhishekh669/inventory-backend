package helpers

import "github.com/gin-gonic/gin"

func Success(c *gin.Context, status int, message string, data interface{}) {
	c.JSON(status, gin.H{
		"success": true,
		"message": message,
		"payload": data,
	})
}

func Error(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{
		"success": false,
		"error":   message,
	})
}
