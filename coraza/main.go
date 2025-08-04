package main

import (
	"bufio"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/corazawaf/coraza/v3"
	"github.com/corazawaf/coraza/v3/debuglog"
	"github.com/corazawaf/coraza/v3/types"
)

const (
	appURLEnv         = "APP_URL"
	logOutputEnv      = "WAF_LOG_FILE"
	bypassFileEnv     = "WAF_BYPASS_FILE"
	rulesFileEnv      = "WAF_RULES_FILE"
	strictOverrideEnv = "WAF_STRICT_OVERRIDE"
	defaultBypass     = "conf/waf.bypass"
	defaultRulesFile  = "rules/mamotama.conf"
)

type wafLog struct {
	RuleID      int           `json:"rule_id,omitempty"`
	Severity    string        `json:"severity,omitempty"`
	Message     string        `json:"message,omitempty"`
	File        string        `json:"file,omitempty"`
	MatchedData []matchedData `json:"matched_data,omitempty"`
}

type matchedData struct {
	Variable string `json:"variable"`
	Key      string `json:"key"`
	Value    string `json:"value"`
	Message  string `json:"message"`
}

type wafBypassRule struct {
	pattern       string
	isWildcard    bool
	isExact       bool
	isDir         bool
	subdirMatch   bool
	extraRuleFile string
}

func loadBypassRules(file string) []wafBypassRule {
	f, err := os.Open(file)
	if err != nil {
		log.Printf("[WAF] no bypass file found: %v", err)
		return nil
	}
	defer f.Close()

	var rules []wafBypassRule
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		pattern := parts[0]
		rule := wafBypassRule{pattern: pattern}

		if strings.HasSuffix(pattern, "/") {
			if strings.HasPrefix(pattern, "/") {
				rule.isDir = true
			} else {
				rule.subdirMatch = true
			}
		} else if strings.HasPrefix(pattern, "*") {
			rule.isWildcard = true
		} else if strings.HasPrefix(pattern, "/") {
			rule.isExact = true
		}

		if len(parts) > 1 {
			rule.extraRuleFile = parts[1]
		}

		rules = append(rules, rule)
	}
	return rules
}

func shouldBypass(r *http.Request, rules []wafBypassRule) (bool, string) {
	path := r.URL.Path
	for _, rule := range rules {
		switch {
		case rule.isExact:
			if path == rule.pattern {
				if rule.extraRuleFile != "" {
					return false, rule.extraRuleFile
				}
				return true, ""
			}
		case rule.isDir:
			trimmed := strings.TrimSuffix(rule.pattern, "/")
			if strings.HasPrefix(path, trimmed) {
				if rule.extraRuleFile != "" {
					return false, rule.extraRuleFile
				}
				return true, ""
			}
		case rule.subdirMatch:
			if strings.Contains(path, "/"+strings.TrimSuffix(rule.pattern, "/")) {
				if rule.extraRuleFile != "" {
					return false, rule.extraRuleFile
				}
				return true, ""
			}
		case rule.isWildcard:
			if match, _ := filepath.Match(rule.pattern, filepath.Base(path)); match {
				if rule.extraRuleFile != "" {
					return false, rule.extraRuleFile
				}
				return true, ""
			}
		default:
			if filepath.Base(path) == rule.pattern {
				if rule.extraRuleFile != "" {
					return false, rule.extraRuleFile
				}
				return true, ""
			}
		}
	}
	return false, ""
}

func main() {
	if logPath := os.Getenv(logOutputEnv); logPath != "" {
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			log.Fatalf("failed to open log file: %v", err)
		}
		log.SetOutput(logFile)
	}

	errorCallback := func(m types.MatchedRule) {
		logData := wafLog{
			RuleID:   m.Rule().ID(),
			Severity: m.Rule().Severity().String(),
			Message:  m.Message(),
			File:     m.Rule().File(),
		}
		for _, md := range m.MatchedDatas() {
			logData.MatchedData = append(logData.MatchedData, matchedData{
				Variable: md.Variable().Name(),
				Key:      md.Key(),
				Value:    md.Value(),
				Message:  md.Message(),
			})
		}
		logJSON, _ := json.Marshal(logData)
		log.Printf("[WAF] %s", logJSON)
	}

	appURL := os.Getenv(appURLEnv)
	if appURL == "" {
		log.Fatal("APP_URL not set")
	}

	bypassPath := os.Getenv(bypassFileEnv)
	if bypassPath == "" {
		bypassPath = defaultBypass
	}

	rulesFile := os.Getenv(rulesFileEnv)
	if rulesFile == "" {
		rulesFile = defaultRulesFile
	}

	bypassRules := loadBypassRules(bypassPath)

	cfg := coraza.NewWAFConfig().
		WithDebugLogger(debuglog.Default().WithLevel(debuglog.LevelInfo)).
		WithErrorCallback(errorCallback)

	ruleFiles := strings.Split(rulesFile, ",")
	for _, file := range ruleFiles {
		file = strings.TrimSpace(file)
		cfg = cfg.WithDirectivesFromFile(file)
	}

	waf, err := coraza.NewWAF(cfg)
	if err != nil {
		log.Fatalf("failed to initialize WAF: %v", err)
	}

	target, _ := url.Parse(appURL)
	proxy := httputil.NewSingleHostReverseProxy(target)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		strictOverride := os.Getenv(strictOverrideEnv) == "true"
		bypass, overrideRule := shouldBypass(r, bypassRules)
		if overrideRule != "" {
			cfg2 := coraza.NewWAFConfig().
				WithDebugLogger(debuglog.Default().WithLevel(debuglog.LevelInfo)).
				WithErrorCallback(errorCallback)

			var waf2 coraza.WAF
			waf2, err = coraza.NewWAF(cfg2.WithDirectivesFromFile(overrideRule))
			if err != nil {
				if strictOverride {
					log.Fatalf("[WAF] override rule required but failed to load: '%s': %v", overrideRule, err)
				}
				log.Printf("[WAF] failed to load override rule '%s': %v", overrideRule, err)
			} else {
				tx := waf2.NewTransaction()
				tx.ProcessURI(r.URL.String(), r.Method, r.Proto)
				tx.AddRequestHeader("Host", r.Host)
				defer func() {
					tx.ProcessLogging()
					tx.Close()
				}()
				if err := tx.ProcessRequestHeaders(); err != nil {
					log.Println("Header error:", err)
				}
				if _, err := tx.ProcessRequestBody(); err != nil {
					log.Println("Body error:", err)
				}
				if it := tx.Interruption(); it != nil {
					log.Printf("[WAF] Blocked (override): rule ID %d, status=%d path=%s", it.RuleID, it.Status, r.URL.Path)
					http.Error(w, http.StatusText(it.Status), it.Status)
					return
				}
				proxy.ServeHTTP(w, r)
				return
			}
		}

		if bypass {
			log.Printf("[WAF] bypassed: %s", r.URL.Path)
			proxy.ServeHTTP(w, r)
			return
		}

		tx := waf.NewTransaction()
		tx.ProcessURI(r.URL.String(), r.Method, r.Proto)
		tx.AddRequestHeader("Host", r.Host)
		defer func() {
			tx.ProcessLogging()
			tx.Close()
		}()
		if err := tx.ProcessRequestHeaders(); err != nil {
			log.Println("Header error:", err)
		}
		if _, err := tx.ProcessRequestBody(); err != nil {
			log.Println("Body error:", err)
		}
		if it := tx.Interruption(); it != nil {
			log.Printf("[WAF] Blocked: rule ID %d, status=%d method=%s host=%s path=%s query=%s user-agent=%q",
				it.RuleID,
				it.Status,
				r.Method,
				r.Host,
				r.URL.Path,
				r.URL.RawQuery,
				r.UserAgent(),
			)
			http.Error(w, http.StatusText(it.Status), it.Status)
			return
		}
		proxy.ServeHTTP(w, r)
	})

	log.Println("Coraza WAF running on :9090")
	log.Fatal(http.ListenAndServe(":9090", nil))
}
