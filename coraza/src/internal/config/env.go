package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

var (
	AppURL         string
	RulesFile      string
	BypassFile     string
	LogFile        string
	StrictOverride bool
	AdminBasePath  string
)

func LoadEnv() {
	_ = godotenv.Load()

	AppURL = os.Getenv("WAF_APP_URL")
	RulesFile = os.Getenv("WAF_RULES_FILE")
	BypassFile = os.Getenv("WAF_BYPASS_FILE")
	LogFile = os.Getenv("WAF_LOG_FILE")
	StrictOverride = os.Getenv("WAF_STRICT_OVERRIDE") == "true"
	AdminBasePath = os.Getenv("WAF_ADMIN_BASEPATH")
	if !strings.HasPrefix(AdminBasePath, "/") {
		AdminBasePath = "/" + AdminBasePath
	}
	if AdminBasePath == "/" {
		log.Fatal("WAF_ADMIN_BASEPATH cannot be root path '/'")
	}
}
