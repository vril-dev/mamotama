package handler

import (
	"log"
	"net/http/httputil"
	"net/url"

	"github.com/gin-gonic/gin"
	"mamotama/internal/config"
	"mamotama/internal/waf"
)

var targetURL *url.URL
var proxy *httputil.ReverseProxy

func ProxyHandler(c *gin.Context) {
	if proxy == nil {
		targetURL, err := url.Parse(config.AppURL)
		if err != nil {
			log.Fatalf("Invalid WAF_APP_URL: %v", err)
		}
		proxy = httputil.NewSingleHostReverseProxy(targetURL)
	}

	tx := waf.WAF.NewTransaction()
	defer func() {
		tx.ProcessLogging()
		tx.Close()
	}()

	tx.ProcessURI(c.Request.URL.String(), c.Request.Method, c.Request.Proto)
	tx.AddRequestHeader("Host", c.Request.Host)
	if err := tx.ProcessRequestHeaders(); err != nil {
		log.Println("Header error:", err)
	}
	if _, err := tx.ProcessRequestBody(); err != nil {
		log.Println("Body error:", err)
	}

	if it := tx.Interruption(); it != nil {
		log.Printf("[WAF] Blocked: rule ID %d, status=%d path=%s", it.RuleID, it.Status, c.Request.URL.Path)
		c.AbortWithStatus(it.Status)
		return
	}

	proxy.ServeHTTP(c.Writer, c.Request)
}
