package waf

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/corazawaf/coraza/v3"
	"github.com/corazawaf/coraza/v3/debuglog"
	"github.com/corazawaf/coraza/v3/types"

	"mamotama/internal/bypassconf"
	"mamotama/internal/config"
)

var WAF coraza.WAF
var overrideMu sync.RWMutex
var overrideWAFs = map[string]coraza.WAF{}

func buildWAF(files []string) (coraza.WAF, error) {
	cfg := coraza.NewWAFConfig().
		WithDebugLogger(debuglog.Default().WithLevel(debuglog.LevelInfo)).
		WithErrorCallback(func(m types.MatchedRule) {
			log.Printf("[WAF] Blocked: URI=%s, MSG=%s", m.URI(), m.MatchedDatas())
		})

	for _, file := range files {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}
		cfg = cfg.WithDirectivesFromFile(file)
		log.Printf("[WAF] Loaded rules from: %s", file)
	}

	return coraza.NewWAF(cfg)
}

func splitRuleFiles(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}

	return out
}

func InitWAF() {
	base, err := buildWAF(splitRuleFiles(config.RulesFile))
	if err != nil {
		log.Fatalf("failed to initialize WAF: %v", err)
	}
	WAF = base

	if err := bypassconf.Init(config.BypassFile); err != nil {
		log.Printf("[BYPASS][INIT][ERR] %v (path=%s)", err, config.BypassFile)
	} else {
		log.Printf("[BYPASS][INIT] watching %s", bypassconf.GetPath())
	}
}

func GetWAFForExtraRule(extraRule string) (coraza.WAF, error) {
	rule := strings.TrimSpace(extraRule)
	if rule == "" {
		return WAF, nil
	}

	overrideMu.RLock()
	if w, ok := overrideWAFs[rule]; ok {
		overrideMu.RUnlock()
		return w, nil
	}
	overrideMu.RUnlock()

	w, err := buildWAF([]string{rule})
	if err != nil {
		return nil, fmt.Errorf("failed to load extra rule %q: %w", rule, err)
	}

	overrideMu.Lock()
	if existing, ok := overrideWAFs[rule]; ok {
		overrideMu.Unlock()
		return existing, nil
	}
	overrideWAFs[rule] = w
	overrideMu.Unlock()
	log.Printf("[BYPASS][RULE] loaded extra rules from: %s", rule)

	return w, nil
}
