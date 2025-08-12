package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"mamotama/internal/config"
)

func APIKeyAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if config.APIAuthDisable {
			c.Next()
			return
		}
		key := strings.TrimSpace(c.GetHeader("X-API-Key"))

		if config.APIKeyPrimary == "" && config.APIKeySecondary == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		if key == config.APIKeyPrimary || (config.APIKeySecondary != "" && key == config.APIKeySecondary) {
			c.Next()
			return
		}
		c.AbortWithStatus(http.StatusUnauthorized)
	}
}
