package main

import (
	"log"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"mamotama/internal/config"
	"mamotama/internal/handler"
	"mamotama/internal/waf"
)

func main() {
	config.LoadEnv()
	waf.InitWAF()

	log.Println("[INFO] WAF upstream target:", config.AppURL)

	r := gin.Default()
	r.Use(cors.Default())

	admin := r.Group(config.AdminBasePath)
	{
		admin.GET("/", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"message": "mamotama-admin API",
				"endpoints": []string{
					config.AdminBasePath + "/status",
					config.AdminBasePath + "/logs",
				},
			})
		})

		admin.GET("/status", handler.StatusHandler)
		admin.GET("/logs", handler.LogsHandler)
		admin.GET("/rules", handler.RulesHandler)
		admin.GET("/bypass", handler.GetBypassHandler)
		admin.POST("/bypass", handler.SaveBypassHandler)
	}

	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, config.AdminBasePath) {
			c.AbortWithStatus(404)
			return
		}

		handler.ProxyHandler(c)
	})

	r.Run(":9090")
}
