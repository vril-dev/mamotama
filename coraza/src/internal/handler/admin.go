package handler

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"mamotama/internal/config"
)

func StatusHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":      "running",
		"rules_file":  config.RulesFile,
		"bypass_file": config.BypassFile,
		"log_file":    config.LogFile,
		"strict_mode": config.StrictOverride,
		"api_base":    config.APIBasePath,
	})
}

func RulesHandler(c *gin.Context) {
	files := strings.Split(config.RulesFile, ",")
	result := make(map[string]string)

	for _, path := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			result[path] = "[読込失敗]"
			continue
		}
		result[path] = string(content)
	}

	c.JSON(http.StatusOK, gin.H{"rules": result})
}
