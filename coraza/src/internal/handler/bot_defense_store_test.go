package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateBotDefenseRaw(t *testing.T) {
	raw := `{
  "enabled": true,
  "mode": "suspicious",
  "path_prefixes": ["api"],
  "exempt_cidrs": ["127.0.0.1/32"],
  "suspicious_user_agents": ["curl"],
  "challenge_cookie_name": "__bot_ok",
  "challenge_secret": "test-secret-12345",
  "challenge_ttl_seconds": 1800,
  "challenge_status_code": 429
}`
	rt, err := ValidateBotDefenseRaw(raw)
	if err != nil {
		t.Fatalf("ValidateBotDefenseRaw() unexpected error: %v", err)
	}
	if rt == nil || !rt.Raw.Enabled {
		t.Fatalf("runtime config should be enabled: %#v", rt)
	}
	if rt.Raw.Mode != "suspicious" {
		t.Fatalf("mode=%q want=suspicious", rt.Raw.Mode)
	}
	if len(rt.Raw.PathPrefixes) != 1 || rt.Raw.PathPrefixes[0] != "/api" {
		t.Fatalf("path_prefixes=%v want=[/api]", rt.Raw.PathPrefixes)
	}
}

func TestValidateBotDefenseRaw_InvalidCookieName(t *testing.T) {
	raw := `{
  "enabled": true,
  "mode": "suspicious",
  "path_prefixes": ["/"],
  "challenge_cookie_name": "bad cookie",
  "challenge_ttl_seconds": 10,
  "challenge_status_code": 429
}`
	if _, err := ValidateBotDefenseRaw(raw); err == nil {
		t.Fatal("expected invalid cookie name error")
	}
}

func TestEvaluateBotDefense_ChallengeThenPass(t *testing.T) {
	raw := `{
  "enabled": true,
  "mode": "suspicious",
  "path_prefixes": ["/"],
  "suspicious_user_agents": ["curl"],
  "challenge_cookie_name": "__mamotama_bot_ok",
  "challenge_secret": "test-bot-defense-secret-12345",
  "challenge_ttl_seconds": 3600,
  "challenge_status_code": 429
}`
	rt, err := ValidateBotDefenseRaw(raw)
	if err != nil {
		t.Fatalf("ValidateBotDefenseRaw() unexpected error: %v", err)
	}

	botDefenseMu.Lock()
	prev := botDefenseRuntime
	botDefenseRuntime = rt
	botDefenseMu.Unlock()
	defer func() {
		botDefenseMu.Lock()
		botDefenseRuntime = prev
		botDefenseMu.Unlock()
	}()

	now := time.Unix(1_700_000_000, 0).UTC()

	req1 := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
	req1.Header.Set("User-Agent", "curl/8.0")
	d1 := EvaluateBotDefense(req1, "10.0.0.1", now)
	if d1.Allowed {
		t.Fatalf("first request should require challenge: %+v", d1)
	}
	if d1.Token == "" {
		t.Fatal("challenge token should not be empty")
	}

	req2 := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
	req2.Header.Set("User-Agent", "curl/8.0")
	req2.AddCookie(&http.Cookie{Name: d1.CookieName, Value: d1.Token})
	d2 := EvaluateBotDefense(req2, "10.0.0.1", now.Add(1*time.Second))
	if !d2.Allowed {
		t.Fatalf("request with valid challenge cookie should pass: %+v", d2)
	}
}

func TestSyncBotDefenseStorage_SeedsDBFromFileWhenMissingBlob(t *testing.T) {
	restore := saveBotDefenseStateForTest()
	defer restore()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "bot-defense.conf")
	raw := `{
  "enabled": true,
  "mode": "suspicious",
  "path_prefixes": ["/"],
  "suspicious_user_agents": ["curl"],
  "challenge_cookie_name": "__bot_ok",
  "challenge_secret": "test-secret-12345",
  "challenge_ttl_seconds": 1800,
  "challenge_status_code": 429
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write bot-defense file: %v", err)
	}
	if err := InitBotDefense(path); err != nil {
		t.Fatalf("init bot-defense: %v", err)
	}

	dbPath := filepath.Join(tmp, "mamotama.db")
	if err := InitLogsStatsStoreWithBackend("db", "sqlite", dbPath, "", 30); err != nil {
		t.Fatalf("init sqlite store: %v", err)
	}
	t.Cleanup(func() {
		_ = InitLogsStatsStoreWithBackend("file", "", "", "", 0)
	})

	if err := SyncBotDefenseStorage(); err != nil {
		t.Fatalf("sync bot-defense storage: %v", err)
	}

	store := getLogsStatsStore()
	if store == nil {
		t.Fatal("expected sqlite store")
	}
	gotRaw, _, found, err := store.GetConfigBlob(botDefenseConfigBlobKey)
	if err != nil {
		t.Fatalf("get config blob: %v", err)
	}
	if !found {
		t.Fatal("expected bot-defense config blob to be seeded")
	}
	if strings.TrimSpace(string(gotRaw)) != strings.TrimSpace(raw) {
		t.Fatalf("seeded blob mismatch:\n got=%s\nwant=%s", string(gotRaw), raw)
	}
}

func TestSyncBotDefenseStorage_RestoresFileAndRuntimeFromDB(t *testing.T) {
	restore := saveBotDefenseStateForTest()
	defer restore()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "bot-defense.conf")
	fileRaw := `{
  "enabled": false,
  "mode": "suspicious",
  "path_prefixes": ["/"]
}`
	if err := os.WriteFile(path, []byte(fileRaw), 0o644); err != nil {
		t.Fatalf("write bot-defense file: %v", err)
	}
	if err := InitBotDefense(path); err != nil {
		t.Fatalf("init bot-defense: %v", err)
	}

	dbPath := filepath.Join(tmp, "mamotama.db")
	if err := InitLogsStatsStoreWithBackend("db", "sqlite", dbPath, "", 30); err != nil {
		t.Fatalf("init sqlite store: %v", err)
	}
	t.Cleanup(func() {
		_ = InitLogsStatsStoreWithBackend("file", "", "", "", 0)
	})

	store := getLogsStatsStore()
	if store == nil {
		t.Fatal("expected sqlite store")
	}
	dbRaw := `{
  "enabled": true,
  "mode": "always",
  "path_prefixes": ["/api"],
  "challenge_cookie_name": "__bot_ok",
  "challenge_secret": "test-secret-12345",
  "challenge_ttl_seconds": 1800,
  "challenge_status_code": 429
}`
	if err := store.UpsertConfigBlob(botDefenseConfigBlobKey, []byte(dbRaw), "", time.Now().UTC()); err != nil {
		t.Fatalf("upsert config blob: %v", err)
	}

	if err := SyncBotDefenseStorage(); err != nil {
		t.Fatalf("sync bot-defense storage: %v", err)
	}

	gotFileRaw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read bot-defense file: %v", err)
	}
	if strings.TrimSpace(string(gotFileRaw)) != strings.TrimSpace(dbRaw) {
		t.Fatalf("file should be restored from db blob:\n got=%s\nwant=%s", string(gotFileRaw), dbRaw)
	}

	cfg := GetBotDefenseConfig()
	if !cfg.Enabled || cfg.Mode != "always" {
		t.Fatalf("runtime config mismatch: enabled=%v mode=%q", cfg.Enabled, cfg.Mode)
	}
}

func saveBotDefenseStateForTest() func() {
	botDefenseMu.RLock()
	oldPath := botDefensePath
	oldRuntime := botDefenseRuntime
	botDefenseMu.RUnlock()

	return func() {
		botDefenseMu.Lock()
		botDefensePath = oldPath
		botDefenseRuntime = oldRuntime
		botDefenseMu.Unlock()
	}
}
