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
	APIAuthDisable = os.Getenv("WAF_API_AUTH_DISABLE") == "1"
}
