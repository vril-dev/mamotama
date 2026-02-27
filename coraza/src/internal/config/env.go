package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

var (
	AppURL          string
	RulesFile       string
	BypassFile      string
	LogFile         string
	StrictOverride  bool
	APIBasePath     string
	APIKeyPrimary   string
	APIKeySecondary string
	APIAuthDisable  bool
	APICORSOrigins  []string
	CRSEnable       bool
	CRSSetupFile    string
	CRSRulesDir     string
	CRSDisabledFile string

	AllowInsecureDefaults bool
)

func LoadEnv() {
	_ = godotenv.Load()

	AppURL = os.Getenv("WAF_APP_URL")
	RulesFile = os.Getenv("WAF_RULES_FILE")
	BypassFile = os.Getenv("WAF_BYPASS_FILE")
	LogFile = os.Getenv("WAF_LOG_FILE")
	StrictOverride = os.Getenv("WAF_STRICT_OVERRIDE") == "true"

	APIBasePath = os.Getenv("WAF_API_BASEPATH")
	if APIBasePath == "" {
		APIBasePath = "/mamotama-api"
	}
	if !strings.HasPrefix(APIBasePath, "/") {
		APIBasePath = "/" + APIBasePath
	}
	if APIBasePath == "/" {
		log.Fatal("WAF_API_BASEPATH cannot be root path '/'")
	}

	APIKeyPrimary = strings.TrimSpace(os.Getenv("WAF_API_KEY_PRIMARY"))
	APIKeySecondary = strings.TrimSpace(os.Getenv("WAF_API_KEY_SECONDARY"))
	APIAuthDisable = isTruthy(os.Getenv("WAF_API_AUTH_DISABLE"))
	APICORSOrigins = parseCSV(os.Getenv("WAF_API_CORS_ALLOWED_ORIGINS"))

	CRSEnable = !isFalsy(os.Getenv("WAF_CRS_ENABLE"))
	CRSSetupFile = strings.TrimSpace(os.Getenv("WAF_CRS_SETUP_FILE"))
	if CRSSetupFile == "" {
		CRSSetupFile = "rules/crs/crs-setup.conf"
	}
	CRSRulesDir = strings.TrimSpace(os.Getenv("WAF_CRS_RULES_DIR"))
	if CRSRulesDir == "" {
		CRSRulesDir = "rules/crs/rules"
	}
	CRSDisabledFile = strings.TrimSpace(os.Getenv("WAF_CRS_DISABLED_FILE"))
	if CRSDisabledFile == "" {
		CRSDisabledFile = "conf/crs-disabled.conf"
	}

	AllowInsecureDefaults = isTruthy(os.Getenv("WAF_ALLOW_INSECURE_DEFAULTS"))
	enforceSecureDefaults()
}

func enforceSecureDefaults() {
	if AllowInsecureDefaults {
		log.Println("[SECURITY][WARN] WAF_ALLOW_INSECURE_DEFAULTS enabled; weak bootstrap settings are allowed")
		return
	}

	if APIAuthDisable {
		log.Fatal("[SECURITY] WAF_API_AUTH_DISABLE is enabled; set WAF_ALLOW_INSECURE_DEFAULTS=1 only for local testing")
	}
	if isWeakAPIKey(APIKeyPrimary) {
		log.Fatal("[SECURITY] WAF_API_KEY_PRIMARY is weak; set a random key with 16+ chars")
	}
	if APIKeySecondary != "" && isWeakAPIKey(APIKeySecondary) {
		log.Fatal("[SECURITY] WAF_API_KEY_SECONDARY is weak; set a random key with 16+ chars or leave it empty")
	}
}

func isWeakAPIKey(v string) bool {
	trimmed := strings.TrimSpace(v)
	s := strings.ToLower(trimmed)
	if s == "" || len(trimmed) < 16 {
		return true
	}

	weak := map[string]struct{}{
		"change-me":                        {},
		"changeme":                         {},
		"replace-with-long-random-api-key": {},
		"replace-me":                       {},
		"example":                          {},
		"test":                             {},
	}
	_, ok := weak[s]
	return ok
}

func isTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func isFalsy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "0", "false", "no", "off":
		return true
	default:
		return false
	}
}

func parseCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s == "" {
			continue
		}
		out = append(out, s)
	}

	return out
}
