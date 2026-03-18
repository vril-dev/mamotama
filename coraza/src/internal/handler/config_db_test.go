package handler

import (
	"os"
	"path/filepath"
	"testing"

	"mamotama/internal/config"
)

func TestReadConfigBlobOrFile_FileFallbackWhenStoreDisabled(t *testing.T) {
	if err := InitLogsStatsStore(false, "", 0); err != nil {
		t.Fatalf("disable sqlite store: %v", err)
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "bypass.conf")
	if err := os.WriteFile(path, []byte("/healthz\n"), 0o644); err != nil {
		t.Fatalf("write bypass file: %v", err)
	}

	raw, err := readConfigBlobOrFile(dbConfigKeyBypassRaw, path)
	if err != nil {
		t.Fatalf("read config from file fallback: %v", err)
	}
	if got := string(raw); got != "/healthz\n" {
		t.Fatalf("raw=%q want=%q", got, "/healthz\n")
	}

	missingPath := filepath.Join(tmp, "missing.conf")
	raw, err = readConfigBlobOrFile(dbConfigKeyBypassRaw, missingPath)
	if err != nil {
		t.Fatalf("read missing config file fallback: %v", err)
	}
	if len(raw) != 0 {
		t.Fatalf("raw bytes=%d want=0", len(raw))
	}
}

func TestReadConfigBlobOrFile_SeedsBlobAndPrefersDB(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "bypass.conf")
	if err := os.WriteFile(path, []byte("/v1\n"), 0o644); err != nil {
		t.Fatalf("write bypass file: %v", err)
	}

	dbPath := filepath.Join(tmp, "mamotama.db")
	if err := InitLogsStatsStore(true, dbPath, 30); err != nil {
		t.Fatalf("init sqlite store: %v", err)
	}
	t.Cleanup(func() {
		_ = InitLogsStatsStore(false, "", 0)
	})

	raw, err := readConfigBlobOrFile(dbConfigKeyBypassRaw, path)
	if err != nil {
		t.Fatalf("seed config blob from file: %v", err)
	}
	if got := string(raw); got != "/v1\n" {
		t.Fatalf("raw=%q want=%q", got, "/v1\n")
	}

	if err := os.WriteFile(path, []byte("/v2\n"), 0o644); err != nil {
		t.Fatalf("rewrite bypass file: %v", err)
	}

	raw, err = readConfigBlobOrFile(dbConfigKeyBypassRaw, path)
	if err != nil {
		t.Fatalf("read config from blob: %v", err)
	}
	if got := string(raw); got != "/v1\n" {
		t.Fatalf("raw=%q want=%q", got, "/v1\n")
	}

	store := getLogsStatsStore()
	if store == nil {
		t.Fatal("sqlite store is nil")
	}
	blob, ok, err := store.GetConfigBlob(dbConfigKeyBypassRaw)
	if err != nil {
		t.Fatalf("get config blob: %v", err)
	}
	if !ok {
		t.Fatal("config blob was not created")
	}
	if blob != "/v1\n" {
		t.Fatalf("blob=%q want=%q", blob, "/v1\n")
	}
}

func TestInitDBConfigMirrors_SyncsDBBlobToConfigFiles(t *testing.T) {
	restore := saveDBMirrorConfig()
	defer restore()

	tmp := t.TempDir()
	bypassPath := filepath.Join(tmp, "bypass.conf")
	countryBlockPath := filepath.Join(tmp, "country-block.conf")
	rateLimitPath := filepath.Join(tmp, "rate-limit.conf")
	botDefensePath := filepath.Join(tmp, "bot-defense.conf")
	semanticPath := filepath.Join(tmp, "semantic.conf")
	crsDisabledPath := filepath.Join(tmp, "crs-disabled.conf")
	cachePath := filepath.Join(tmp, "cache.conf")
	rulePath := filepath.Join(tmp, "rules", "custom.conf")
	if err := os.WriteFile(bypassPath, []byte("/seed\n"), 0o644); err != nil {
		t.Fatalf("write bypass file: %v", err)
	}
	if err := os.WriteFile(countryBlockPath, []byte("JP\nUS\n"), 0o644); err != nil {
		t.Fatalf("write country block file: %v", err)
	}
	if err := os.WriteFile(rateLimitPath, []byte("{\"enabled\":false}\n"), 0o644); err != nil {
		t.Fatalf("write rate limit file: %v", err)
	}
	if err := os.WriteFile(botDefensePath, []byte("{\"enabled\":false,\"mode\":\"log_only\"}\n"), 0o644); err != nil {
		t.Fatalf("write bot defense file: %v", err)
	}
	if err := os.WriteFile(semanticPath, []byte("{\"enabled\":false,\"mode\":\"log_only\"}\n"), 0o644); err != nil {
		t.Fatalf("write semantic file: %v", err)
	}
	if err := os.WriteFile(crsDisabledPath, []byte("REQUEST-920-PROTOCOL-ENFORCEMENT.conf\n"), 0o644); err != nil {
		t.Fatalf("write crs disabled file: %v", err)
	}
	if err := os.WriteFile(cachePath, []byte("ALLOW prefix=/static methods=GET ttl=120\n"), 0o644); err != nil {
		t.Fatalf("write cache file: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(rulePath), 0o755); err != nil {
		t.Fatalf("mkdir rule dir: %v", err)
	}
	if err := os.WriteFile(rulePath, []byte("SecRuleEngine On\n"), 0o644); err != nil {
		t.Fatalf("write rule file: %v", err)
	}

	config.BypassFile = bypassPath
	config.CountryBlockFile = countryBlockPath
	config.RateLimitFile = rateLimitPath
	config.BotDefenseFile = botDefensePath
	config.SemanticFile = semanticPath
	config.CRSDisabledFile = crsDisabledPath
	config.RulesFile = rulePath
	cacheConfPath = cachePath

	dbPath := filepath.Join(tmp, "mamotama.db")
	if err := InitLogsStatsStore(true, dbPath, 30); err != nil {
		t.Fatalf("init sqlite store: %v", err)
	}
	t.Cleanup(func() {
		_ = InitLogsStatsStore(false, "", 0)
	})

	if err := InitDBConfigMirrors(); err != nil {
		t.Fatalf("first mirror init: %v", err)
	}

	if err := os.WriteFile(bypassPath, []byte("/changed\n"), 0o644); err != nil {
		t.Fatalf("rewrite bypass file: %v", err)
	}
	if err := os.WriteFile(countryBlockPath, []byte("CA\n"), 0o644); err != nil {
		t.Fatalf("rewrite country block file: %v", err)
	}
	if err := os.WriteFile(rateLimitPath, []byte("{\"enabled\":true}\n"), 0o644); err != nil {
		t.Fatalf("rewrite rate limit file: %v", err)
	}
	if err := os.WriteFile(botDefensePath, []byte("{\"enabled\":true}\n"), 0o644); err != nil {
		t.Fatalf("rewrite bot defense file: %v", err)
	}
	if err := os.WriteFile(semanticPath, []byte("{\"enabled\":true}\n"), 0o644); err != nil {
		t.Fatalf("rewrite semantic file: %v", err)
	}
	if err := os.WriteFile(crsDisabledPath, []byte("REQUEST-933-APPLICATION-ATTACK-PHP.conf\n"), 0o644); err != nil {
		t.Fatalf("rewrite crs disabled file: %v", err)
	}
	if err := os.WriteFile(cachePath, []byte("DENY prefix=/admin methods=GET ttl=60\n"), 0o644); err != nil {
		t.Fatalf("rewrite cache file: %v", err)
	}
	if err := os.WriteFile(rulePath, []byte("SecRuleEngine DetectionOnly\n"), 0o644); err != nil {
		t.Fatalf("rewrite rule file: %v", err)
	}

	if err := InitDBConfigMirrors(); err != nil {
		t.Fatalf("second mirror init: %v", err)
	}

	bypassRaw, err := os.ReadFile(bypassPath)
	if err != nil {
		t.Fatalf("read bypass file: %v", err)
	}
	if got := string(bypassRaw); got != "/seed\n" {
		t.Fatalf("bypass raw=%q want=%q", got, "/seed\n")
	}

	countryRaw, err := os.ReadFile(countryBlockPath)
	if err != nil {
		t.Fatalf("read country block file: %v", err)
	}
	if got := string(countryRaw); got != "JP\nUS\n" {
		t.Fatalf("country block raw=%q want=%q", got, "JP\nUS\n")
	}

	rateLimitRaw, err := os.ReadFile(rateLimitPath)
	if err != nil {
		t.Fatalf("read rate limit file: %v", err)
	}
	if got := string(rateLimitRaw); got != "{\"enabled\":false}\n" {
		t.Fatalf("rate limit raw=%q want=%q", got, "{\"enabled\":false}\n")
	}

	botDefenseRaw, err := os.ReadFile(botDefensePath)
	if err != nil {
		t.Fatalf("read bot defense file: %v", err)
	}
	if got := string(botDefenseRaw); got != "{\"enabled\":false,\"mode\":\"log_only\"}\n" {
		t.Fatalf("bot defense raw=%q want=%q", got, "{\"enabled\":false,\"mode\":\"log_only\"}\n")
	}

	semanticRaw, err := os.ReadFile(semanticPath)
	if err != nil {
		t.Fatalf("read semantic file: %v", err)
	}
	if got := string(semanticRaw); got != "{\"enabled\":false,\"mode\":\"log_only\"}\n" {
		t.Fatalf("semantic raw=%q want=%q", got, "{\"enabled\":false,\"mode\":\"log_only\"}\n")
	}

	crsDisabledRaw, err := os.ReadFile(crsDisabledPath)
	if err != nil {
		t.Fatalf("read crs disabled file: %v", err)
	}
	if got := string(crsDisabledRaw); got != "REQUEST-920-PROTOCOL-ENFORCEMENT.conf\n" {
		t.Fatalf("crs disabled raw=%q want=%q", got, "REQUEST-920-PROTOCOL-ENFORCEMENT.conf\n")
	}

	cacheRaw, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("read cache file: %v", err)
	}
	if got := string(cacheRaw); got != "ALLOW prefix=/static methods=GET ttl=120\n" {
		t.Fatalf("cache raw=%q want=%q", got, "ALLOW prefix=/static methods=GET ttl=120\n")
	}

	ruleRaw, err := os.ReadFile(rulePath)
	if err != nil {
		t.Fatalf("read rule file: %v", err)
	}
	if got := string(ruleRaw); got != "SecRuleEngine On\n" {
		t.Fatalf("rule raw=%q want=%q", got, "SecRuleEngine On\n")
	}
}

func TestInitDBConfigMirrors_RestoresBlobAfterStoreReopen(t *testing.T) {
	restore := saveDBMirrorConfig()
	defer restore()

	tmp := t.TempDir()
	bypassPath := filepath.Join(tmp, "bypass.conf")
	countryBlockPath := filepath.Join(tmp, "country-block.conf")
	rateLimitPath := filepath.Join(tmp, "rate-limit.conf")
	botDefensePath := filepath.Join(tmp, "bot-defense.conf")
	semanticPath := filepath.Join(tmp, "semantic.conf")
	crsDisabledPath := filepath.Join(tmp, "crs-disabled.conf")
	cachePath := filepath.Join(tmp, "cache.conf")
	rulePath := filepath.Join(tmp, "rules", "custom.conf")

	if err := os.MkdirAll(filepath.Dir(rulePath), 0o755); err != nil {
		t.Fatalf("mkdir rule dir: %v", err)
	}

	fileSeeds := map[string]string{
		bypassPath:       "/seed\n",
		countryBlockPath: "JP\n",
		rateLimitPath:    "{\"enabled\":false}\n",
		botDefensePath:   "{\"enabled\":false,\"mode\":\"log_only\"}\n",
		semanticPath:     "{\"enabled\":false,\"mode\":\"log_only\"}\n",
		crsDisabledPath:  "REQUEST-920-PROTOCOL-ENFORCEMENT.conf\n",
		cachePath:        "ALLOW prefix=/seed methods=GET ttl=60\n",
		rulePath:         "SecRuleEngine On\n",
	}
	for path, raw := range fileSeeds {
		if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
			t.Fatalf("write seed file %s: %v", path, err)
		}
	}

	config.BypassFile = bypassPath
	config.CountryBlockFile = countryBlockPath
	config.RateLimitFile = rateLimitPath
	config.BotDefenseFile = botDefensePath
	config.SemanticFile = semanticPath
	config.CRSDisabledFile = crsDisabledPath
	config.RulesFile = rulePath
	cacheConfPath = cachePath

	dbPath := filepath.Join(tmp, "mamotama.db")
	if err := InitLogsStatsStore(true, dbPath, 30); err != nil {
		t.Fatalf("init sqlite store: %v", err)
	}
	t.Cleanup(func() {
		_ = InitLogsStatsStore(false, "", 0)
	})

	if err := InitDBConfigMirrors(); err != nil {
		t.Fatalf("seed mirrors: %v", err)
	}

	dbRaw := map[string]string{
		dbConfigKeyBypassRaw:         "/from-db\n",
		dbConfigKeyCountryBlockRaw:   "US\nUNKNOWN\n",
		dbConfigKeyRateLimitRaw:      "{\"enabled\":true}\n",
		dbConfigKeyBotDefenseRaw:     "{\"enabled\":true,\"mode\":\"suspicious\"}\n",
		dbConfigKeySemanticRaw:       "{\"enabled\":true,\"mode\":\"blocking\"}\n",
		dbConfigKeyCRSDisabledRaw:    "REQUEST-933-APPLICATION-ATTACK-PHP.conf\n",
		dbConfigKeyCacheRaw:          "DENY prefix=/admin methods=GET ttl=30\n",
		dbConfigKeyRuleRaw(rulePath): `SecRule REQUEST_URI "@beginsWith /admin" "id:100001,phase:1,deny,status:403,msg:'block admin'"` + "\n",
	}
	for key, raw := range dbRaw {
		if err := putConfigBlobIfEnabled(key, raw); err != nil {
			t.Fatalf("put blob %s: %v", key, err)
		}
	}

	if err := InitLogsStatsStore(false, "", 0); err != nil {
		t.Fatalf("close sqlite store: %v", err)
	}

	fileDrift := map[string]string{
		bypassPath:       "/from-file\n",
		countryBlockPath: "CA\n",
		rateLimitPath:    "{\"enabled\":false,\"rules\":[]}\n",
		botDefensePath:   "{\"enabled\":false,\"mode\":\"log_only\"}\n",
		semanticPath:     "{\"enabled\":false,\"mode\":\"log_only\"}\n",
		crsDisabledPath:  "REQUEST-942-APPLICATION-ATTACK-SQLI.conf\n",
		cachePath:        "ALLOW prefix=/drift methods=GET ttl=10\n",
		rulePath:         "SecRuleEngine DetectionOnly\n",
	}
	for path, raw := range fileDrift {
		if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
			t.Fatalf("write drift file %s: %v", path, err)
		}
	}

	if err := InitLogsStatsStore(true, dbPath, 30); err != nil {
		t.Fatalf("reopen sqlite store: %v", err)
	}
	if err := InitDBConfigMirrors(); err != nil {
		t.Fatalf("restore from db blobs: %v", err)
	}

	wantByPath := map[string]string{
		bypassPath:       dbRaw[dbConfigKeyBypassRaw],
		countryBlockPath: dbRaw[dbConfigKeyCountryBlockRaw],
		rateLimitPath:    dbRaw[dbConfigKeyRateLimitRaw],
		botDefensePath:   dbRaw[dbConfigKeyBotDefenseRaw],
		semanticPath:     dbRaw[dbConfigKeySemanticRaw],
		crsDisabledPath:  dbRaw[dbConfigKeyCRSDisabledRaw],
		cachePath:        dbRaw[dbConfigKeyCacheRaw],
		rulePath:         dbRaw[dbConfigKeyRuleRaw(rulePath)],
	}
	for path, want := range wantByPath {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read restored file %s: %v", path, err)
		}
		if string(got) != want {
			t.Fatalf("restored raw mismatch path=%s got=%q want=%q", path, string(got), want)
		}
	}
}

func saveDBMirrorConfig() func() {
	oldBypass := config.BypassFile
	oldCountryBlock := config.CountryBlockFile
	oldRateLimit := config.RateLimitFile
	oldBotDefense := config.BotDefenseFile
	oldSemantic := config.SemanticFile
	oldCRSDisabled := config.CRSDisabledFile
	oldRulesFile := config.RulesFile
	oldCacheConfPath := cacheConfPath
	return func() {
		config.BypassFile = oldBypass
		config.CountryBlockFile = oldCountryBlock
		config.RateLimitFile = oldRateLimit
		config.BotDefenseFile = oldBotDefense
		config.SemanticFile = oldSemantic
		config.CRSDisabledFile = oldCRSDisabled
		config.RulesFile = oldRulesFile
		cacheConfPath = oldCacheConfPath
	}
}
