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

	config.BypassFile = bypassPath
	config.CountryBlockFile = countryBlockPath
	config.RateLimitFile = rateLimitPath
	config.BotDefenseFile = botDefensePath
	config.SemanticFile = semanticPath

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
}

func saveDBMirrorConfig() func() {
	oldBypass := config.BypassFile
	oldCountryBlock := config.CountryBlockFile
	oldRateLimit := config.RateLimitFile
	oldBotDefense := config.BotDefenseFile
	oldSemantic := config.SemanticFile
	return func() {
		config.BypassFile = oldBypass
		config.CountryBlockFile = oldCountryBlock
		config.RateLimitFile = oldRateLimit
		config.BotDefenseFile = oldBotDefense
		config.SemanticFile = oldSemantic
	}
}
