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
	if err := handler.InitCountryBlock(config.CountryBlockFile); err != nil {
		log.Printf("[COUNTRY_BLOCK][INIT][ERR] %v (path=%s)", err, config.CountryBlockFile)
	} else {
		log.Printf("[COUNTRY_BLOCK][INIT] loaded %d countries", len(handler.GetBlockedCountries()))
	}
	if err := handler.InitRateLimit(config.RateLimitFile); err != nil {
		log.Printf("[RATE_LIMIT][INIT][ERR] %v (path=%s)", err, config.RateLimitFile)
	} else {
		log.Printf("[RATE_LIMIT][INIT] loaded")
	}

	log.Println("[INFO] WAF upstream target:", config.AppURL)

	r := gin.Default()

	// Never trust client-sent forwarding headers unless explicitly configured.
	if err := r.SetTrustedProxies(nil); err != nil {
		log.Fatalf("failed to configure trusted proxies: %v", err)
	}

	if len(config.APICORSOrigins) > 0 {
		r.Use(cors.New(cors.Config{
			AllowOrigins: config.APICORSOrigins,
			AllowMethods: []string{"GET", "POST", "PUT", "OPTIONS"},
			AllowHeaders: []string{"Origin", "Content-Type", "Accept", "X-API-Key"},
		}))
		log.Printf("[SECURITY] CORS enabled for origins: %s", strings.Join(config.APICORSOrigins, ","))
	} else {
		log.Println("[SECURITY] CORS disabled (same-origin only)")
	}

	api := r.Group(config.APIBasePath, middleware.APIKeyAuth())
	{
		api.GET("/", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"message": "mamotama-admin API",
				"endpoints": []string{
					config.APIBasePath + "/status",
					config.APIBasePath + "/logs",
					config.APIBasePath + "/rules",
					config.APIBasePath + "/crs-rule-sets",
					config.APIBasePath + "/bypass-rules",
					config.APIBasePath + "/cache-rules",
					config.APIBasePath + "/country-block-rules",
					config.APIBasePath + "/rate-limit-rules",
					config.APIBasePath + "/logs/read",
					config.APIBasePath + "/logs/download",
				},
			})
		})

		api.GET("/status", handler.StatusHandler)
		api.GET("/logs/read", handler.LogsRead)
		api.GET("/logs/download", handler.LogsDownload)
		api.GET("/rules", handler.RulesHandler)
		api.POST("/rules:validate", handler.ValidateRules)
		api.PUT("/rules", handler.PutRules)
		api.GET("/crs-rule-sets", handler.GetCRSRuleSets)
		api.POST("/crs-rule-sets:validate", handler.ValidateCRSRuleSets)
		api.PUT("/crs-rule-sets", handler.PutCRSRuleSets)
		api.GET("/bypass-rules", handler.GetBypassRules)
		api.POST("/bypass-rules:validate", handler.ValidateBypassRules)
		api.PUT("/bypass-rules", handler.PutBypassRules)
		api.GET("/cache-rules", handler.GetCacheRules)
		api.POST("/cache-rules:validate", handler.ValidateCacheRules)
		api.PUT("/cache-rules", handler.PutCacheRules)
		api.GET("/country-block-rules", handler.GetCountryBlockRules)
		api.POST("/country-block-rules:validate", handler.ValidateCountryBlockRules)
		api.PUT("/country-block-rules", handler.PutCountryBlockRules)
		api.GET("/rate-limit-rules", handler.GetRateLimitRules)
		api.POST("/rate-limit-rules:validate", handler.ValidateRateLimitRules)
		api.PUT("/rate-limit-rules", handler.PutRateLimitRules)
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
