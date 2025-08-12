package handler

import (
	"log"

	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"mamotama/internal/cacheconf"
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

		proxy.ModifyResponse = func(res *http.Response) error {
			rs := cacheconf.Get()
			if rs == nil || res == nil || res.Request == nil {
				return nil
			}

			method := res.Request.Method
			if method != http.MethodGet && method != http.MethodHead {
				return nil
			}

			path := res.Request.URL.Path
			if rule, allow := rs.Match(method, path); allow {
				ttl := rule.TTL
				if ttl <= 0 {
					ttl = 600
				}

				h := res.Header
				h.Set("X-Mamotama-Cacheable", "1")
				h.Set("X-Accel-Expires", strconv.Itoa(ttl))
				if len(rule.Vary) > 0 {
					h.Set("Vary", strings.Join(rule.Vary, ", "))
				}

			}

			return nil
		}
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
