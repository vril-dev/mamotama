package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"mamotama/internal/bypassconf"
	"mamotama/internal/cacheconf"
	"mamotama/internal/config"
	"mamotama/internal/waf"
)

type ctxKey string

const (
	ctxKeyReqID   ctxKey = "req_id"
	ctxKeyWafHit  ctxKey = "waf_hit"
	ctxKeyWafRule ctxKey = "waf_rules"
	ctxKeyIP      ctxKey = "client_ip"
)

var targetURL *url.URL
var proxy *httputil.ReverseProxy

func ensureProxy() {
	if proxy != nil {
		return
	}
	u, err := url.Parse(config.AppURL)
	if err != nil {
		log.Fatalf("Invalid WAF_APP_URL: %v", err)
	}
	targetURL = u
	proxy = httputil.NewSingleHostReverseProxy(targetURL)

	proxy.ModifyResponse = func(res *http.Response) error {
		if res != nil && res.Request != nil {
			ctx := res.Request.Context()
			if hit, _ := ctx.Value(ctxKeyWafHit).(bool); hit {
				if res.Header != nil {
					res.Header.Set("X-WAF-Hit", "1")
					if rid, _ := ctx.Value(ctxKeyWafRule).(string); rid != "" {
						res.Header.Set("X-WAF-RuleIDs", rid)
					}
				}

				reqID, _ := ctx.Value(ctxKeyReqID).(string)
				ip, _ := ctx.Value(ctxKeyIP).(string)
				path := res.Request.URL.Path
				status := res.StatusCode
				emitJSONLog(map[string]any{
					"ts":      time.Now().UTC().Format(time.RFC3339Nano),
					"service": "coraza",
					"level":   "INFO",
					"event":   "waf_hit_allow",
					"req_id":  reqID, "ip": ip, "path": path,
					"rules":  res.Header.Get("X-WAF-RuleIDs"),
					"status": status,
				})
			}
		}

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

func ProxyHandler(c *gin.Context) {
	ensureProxy()

	reqID := c.Request.Header.Get("X-Request-ID")
	if reqID == "" {
		reqID = genReqID()
		c.Request.Header.Set("X-Request-ID", reqID)
	}
	c.Writer.Header().Set("X-Request-ID", reqID)

	reqPath := c.Request.URL.Path
	switch mr := bypassconf.Match(reqPath); mr.Action {
	case bypassconf.ACTION_BYPASS:
		log.Printf("[BYPASS][HIT] %s -> skip WAF", reqPath)
		proxy.ServeHTTP(c.Writer, c.Request)
		return
	case bypassconf.ACTION_RULE:
		log.Printf("[BYPASS][RULE] %s extra=%s (not applied yet)", reqPath, mr.ExtraRule)
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

	wafHit := false
	ruleIDs := make([]string, 0, 4)
	for _, mr := range tx.MatchedRules() {
		wafHit = true
		// Rule().ID() on v3; fallback to mr.RuleID if your type differs
		if mr.Rule() != nil {
			ruleIDs = append(ruleIDs, strconv.Itoa(mr.Rule().ID()))
		}
	}

	ctx := context.WithValue(c.Request.Context(), ctxKeyReqID, reqID)
	ctx = context.WithValue(ctx, ctxKeyIP, c.ClientIP())
	ctx = context.WithValue(ctx, ctxKeyWafHit, wafHit)
	ctx = context.WithValue(ctx, ctxKeyWafRule, strings.Join(unique(ruleIDs), ","))
	c.Request = c.Request.WithContext(ctx)

	if it := tx.Interruption(); it != nil {
		evt := map[string]any{
			"ts":      time.Now().UTC().Format(time.RFC3339Nano),
			"service": "coraza",
			"level":   "WARN",
			"event":   "waf_block",
			"req_id":  reqID, "ip": c.ClientIP(), "path": c.Request.URL.Path,
			"rule_id": it.RuleID, "status": it.Status,
		}
		emitJSONLog(evt)
		_ = appendEventToFile(evt)
		c.AbortWithStatus(it.Status)
		return
	}

	proxy.ServeHTTP(c.Writer, c.Request)
}

func genReqID() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}

func unique(in []string) []string {
	m := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := m[s]; !ok && s != "" {
			m[s] = struct{}{}
			out = append(out, s)
		}
	}

	return out
}

func emitJSONLog(obj map[string]any) {
	if b, err := json.Marshal(obj); err == nil {
		log.Println(string(b))
	}
}

func appendEventToFile(obj map[string]any) error {
	path := os.Getenv("WAF_EVENTS_FILE")
	if path == "" {
		path = "/app/logs/coraza/waf-events.ndjson"
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	b, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))

	return err
}
