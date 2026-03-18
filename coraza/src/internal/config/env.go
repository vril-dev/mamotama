package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

var (
	AppURL           string
	RulesFile        string
	BypassFile       string
	CountryBlockFile string
	RateLimitFile    string
	BotDefenseFile   string
	SemanticFile     string
	LogFile          string
	StrictOverride   bool
	APIBasePath      string
	APIKeyPrimary    string
	APIKeySecondary  string
	APIAuthDisable   bool
	APICORSOrigins   []string
	CRSEnable        bool
	CRSSetupFile     string
	CRSRulesDir      string
	CRSDisabledFile  string

	AllowInsecureDefaults bool

	FPTunerMode             string
	FPTunerEndpoint         string
	FPTunerAPIKey           string
	FPTunerModel            string
	FPTunerTimeout          time.Duration
	FPTunerMockResponseFile string
	FPTunerRequireApproval  bool
	FPTunerApprovalTTL      time.Duration
	FPTunerAuditFile        string

	DBEnabled bool
	DBPath    string
)

func LoadEnv() {
	_ = godotenv.Load()

	AppURL = os.Getenv("WAF_APP_URL")
	RulesFile = os.Getenv("WAF_RULES_FILE")
	BypassFile = os.Getenv("WAF_BYPASS_FILE")
	CountryBlockFile = strings.TrimSpace(os.Getenv("WAF_COUNTRY_BLOCK_FILE"))
	if CountryBlockFile == "" {
		CountryBlockFile = "conf/country-block.conf"
	}
	RateLimitFile = strings.TrimSpace(os.Getenv("WAF_RATE_LIMIT_FILE"))
	if RateLimitFile == "" {
		RateLimitFile = "conf/rate-limit.conf"
	}
	BotDefenseFile = strings.TrimSpace(os.Getenv("WAF_BOT_DEFENSE_FILE"))
	if BotDefenseFile == "" {
		BotDefenseFile = "conf/bot-defense.conf"
	}
	SemanticFile = strings.TrimSpace(os.Getenv("WAF_SEMANTIC_FILE"))
	if SemanticFile == "" {
		SemanticFile = "conf/semantic.conf"
	}
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

	FPTunerMode = strings.ToLower(strings.TrimSpace(os.Getenv("WAF_FP_TUNER_MODE")))
	if FPTunerMode == "" {
		FPTunerMode = "mock"
	}
	FPTunerEndpoint = strings.TrimSpace(os.Getenv("WAF_FP_TUNER_ENDPOINT"))
	FPTunerAPIKey = strings.TrimSpace(os.Getenv("WAF_FP_TUNER_API_KEY"))
	FPTunerModel = strings.TrimSpace(os.Getenv("WAF_FP_TUNER_MODEL"))
	FPTunerMockResponseFile = strings.TrimSpace(os.Getenv("WAF_FP_TUNER_MOCK_RESPONSE_FILE"))
	if FPTunerMockResponseFile == "" {
		FPTunerMockResponseFile = "conf/fp-tuner-mock-response.json"
	}
	timeoutSec := parseIntDefault(os.Getenv("WAF_FP_TUNER_TIMEOUT_SEC"), 15)
	if timeoutSec < 1 || timeoutSec > 300 {
		timeoutSec = 15
	}
	FPTunerTimeout = time.Duration(timeoutSec) * time.Second
	FPTunerRequireApproval = !isFalsy(os.Getenv("WAF_FP_TUNER_REQUIRE_APPROVAL"))
	approvalTTLSec := parseIntDefault(os.Getenv("WAF_FP_TUNER_APPROVAL_TTL_SEC"), 600)
	if approvalTTLSec < 10 || approvalTTLSec > 86400 {
		approvalTTLSec = 600
	}
	FPTunerApprovalTTL = time.Duration(approvalTTLSec) * time.Second
	FPTunerAuditFile = strings.TrimSpace(os.Getenv("WAF_FP_TUNER_AUDIT_FILE"))
	if FPTunerAuditFile == "" {
		FPTunerAuditFile = "logs/coraza/fp-tuner-audit.ndjson"
	}
	DBEnabled = isTruthy(os.Getenv("WAF_DB_ENABLED"))
	DBPath = strings.TrimSpace(os.Getenv("WAF_DB_PATH"))
	if DBPath == "" {
		DBPath = "logs/coraza/mamotama.db"
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

func parseIntDefault(v string, d int) int {
	s := strings.TrimSpace(v)
	if s == "" {
		return d
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return d
	}
	return n
}
