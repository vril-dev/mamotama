package waf

import (
	"log"
	"strings"

	"github.com/corazawaf/coraza/v3"
	"github.com/corazawaf/coraza/v3/debuglog"
	"github.com/corazawaf/coraza/v3/types"
	"mamotama/internal/config"
)

var WAF coraza.WAF

func InitWAF() {
	cfg := coraza.NewWAFConfig().
		WithDebugLogger(debuglog.Default().WithLevel(debuglog.LevelInfo)).
		WithErrorCallback(func(m types.MatchedRule) {
			log.Printf("[WAF] Blocked: URI=%s, MSG=%s", m.URI(), m.MatchedDatas())
		})

	files := strings.Split(config.RulesFile, ",")
	for _, file := range files {
		file = strings.TrimSpace(file)
		cfg = cfg.WithDirectivesFromFile(file)
		log.Printf("[WAF] Loaded rules from: %s", file)
	}

	var err error
	WAF, err = coraza.NewWAF(cfg)
	if err != nil {
		log.Fatalf("failed to initialize WAF: %v", err)
	}
}
