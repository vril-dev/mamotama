package handler

import (
	"bufio"
	"log"
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

func LogsHandler(c *gin.Context) {
	file, err := os.Open(config.LogFile)
	if err != nil {
		log.Printf("[WAF] Failed to read log: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read log file"})
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lines := []string{}
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > 100 {
			lines = lines[1:] // 最新100行だけ保持
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"log_tail": lines,
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
