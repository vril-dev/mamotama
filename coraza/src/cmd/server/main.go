package main

import (
	"log"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"mamotama/internal/cacheconf"
	"mamotama/internal/config"
	"mamotama/internal/handler"
	"mamotama/internal/middleware"
	"mamotama/internal/waf"
)

func main() {
	config.LoadEnv()
	waf.InitWAF()

	log.Println("[INFO] WAF upstream target:", config.AppURL)

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "OPTIONS"},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept", "X-API-Key"},
	}))

	api := r.Group(config.APIBasePath, middleware.APIKeyAuth())
	{
		api.GET("/", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"message": "mamotama-admin API",
				"endpoints": []string{
					config.APIBasePath + "/status",
					config.APIBasePath + "/logs",
					config.APIBasePath + "/rules",
					config.APIBasePath + "/bypass",
				},
			})
		})

		api.GET("/status", handler.StatusHandler)
		api.GET("/logs", handler.LogsHandler)
		api.GET("/rules", handler.RulesHandler)
		api.GET("/bypass", handler.GetBypassHandler)
		api.POST("/bypass", handler.SaveBypassHandler)
	}

	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, config.APIBasePath) {
			c.AbortWithStatus(404)
			return
		}

		handler.ProxyHandler(c)
	})

	const cacheConfPath = "conf/cache.conf"
	stopWatch, err := cacheconf.Watch(cacheConfPath, func(rs *cacheconf.Ruleset) {
		//
	})
	if err != nil {
		log.Printf("[CACHE] watch disabled: %v", err)
	} else {
		defer stopWatch()
	}

	r.Run(":9090")
}
