package handler

import (
	"github.com/gin-gonic/gin"
	"io"
	"mamotama/internal/config"
	"net/http"
	"os"
)

func GetBypassHandler(c *gin.Context) {
	data, err := os.ReadFile(config.BypassFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read bypass file"})
		return
	}
	c.String(http.StatusOK, string(data))
}

func SaveBypassHandler(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	err = os.WriteFile(config.BypassFile, body, 0644)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write bypass file"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "bypass rules updated"})
}
