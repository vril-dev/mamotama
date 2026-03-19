package handler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateRateLimitRaw(t *testing.T) {
	raw := `{
  "enabled": true,
  "allowlist_ips": ["127.0.0.1/32"],
  "allowlist_countries": ["JP"],
  "default_policy": {
    "enabled": true,
    "limit": 100,
    "window_seconds": 60,
    "burst": 10,
    "key_by": "ip",
    "action": {"status": 429, "retry_after_seconds": 60}
  },
  "rules": [
    {
      "name": "login",
      "match_type": "prefix",
      "match_value": "/login",
      "methods": ["POST"],
      "policy": {
        "enabled": true,
        "limit": 5,
        "window_seconds": 60,
        "burst": 0,
        "key_by": "ip_country",
        "action": {"status": 429, "retry_after_seconds": 60}
      }
    }
  ]
}`

	rt, err := ValidateRateLimitRaw(raw)
	if err != nil {
		t.Fatalf("ValidateRateLimitRaw() unexpected error: %v", err)
	}
	if rt == nil || !rt.Raw.Enabled {
		t.Fatalf("runtime config should be enabled: %#v", rt)
	}
	if got := len(rt.Rules); got != 1 {
		t.Fatalf("len(rt.Rules)=%d want=1", got)
	}
}

func TestEvaluateRateLimit_BlocksAfterLimit(t *testing.T) {
	raw := `{
  "enabled": true,
  "allowlist_ips": [],
  "allowlist_countries": [],
  "default_policy": {
    "enabled": true,
    "limit": 2,
    "window_seconds": 60,
    "burst": 0,
    "key_by": "ip",
    "action": {"status": 429, "retry_after_seconds": 30}
  },
  "rules": []
}`
	rt, err := ValidateRateLimitRaw(raw)
	if err != nil {
		t.Fatalf("ValidateRateLimitRaw() unexpected error: %v", err)
	}

	rateLimitMu.Lock()
	prevRuntime := rateLimitRuntime
	rateLimitRuntime = rt
	rateLimitMu.Unlock()
	defer func() {
		rateLimitMu.Lock()
		rateLimitRuntime = prevRuntime
		rateLimitMu.Unlock()
	}()

	rateCounterMu.Lock()
	rateCounters = map[string]rateCounter{}
	rateCounterSweep = 0
	rateCounterMu.Unlock()

	now := time.Unix(1_700_000_000, 0).UTC()
	d1 := EvaluateRateLimit("GET", "/items", "10.0.0.1", "JP", now)
	d2 := EvaluateRateLimit("GET", "/items", "10.0.0.1", "JP", now.Add(1*time.Second))
	d3 := EvaluateRateLimit("GET", "/items", "10.0.0.1", "JP", now.Add(2*time.Second))

	if !d1.Allowed || !d2.Allowed {
		t.Fatalf("first two requests should be allowed: d1=%+v d2=%+v", d1, d2)
	}
	if d3.Allowed {
		t.Fatalf("third request should be blocked: d3=%+v", d3)
	}
	if d3.Status != 429 {
		t.Fatalf("blocked status=%d want=429", d3.Status)
	}
}

func TestSyncRateLimitStorage_SeedsDBFromFileWhenMissingBlob(t *testing.T) {
	restore := saveRateLimitStateForTest()
	defer restore()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "rate-limit.conf")
	raw := rateLimitRawForTest(77)
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write rate-limit file: %v", err)
	}
	if err := InitRateLimit(path); err != nil {
		t.Fatalf("init rate-limit: %v", err)
	}

	dbPath := filepath.Join(tmp, "mamotama.db")
	if err := InitLogsStatsStoreWithBackend("db", "sqlite", dbPath, "", 30); err != nil {
		t.Fatalf("init sqlite store: %v", err)
	}
	t.Cleanup(func() {
		_ = InitLogsStatsStoreWithBackend("file", "", "", "", 0)
	})

	if err := SyncRateLimitStorage(); err != nil {
		t.Fatalf("sync rate-limit storage: %v", err)
	}

	store := getLogsStatsStore()
	if store == nil {
		t.Fatal("expected sqlite store")
	}
	gotRaw, _, found, err := store.GetConfigBlob(rateLimitConfigBlobKey)
	if err != nil {
		t.Fatalf("get config blob: %v", err)
	}
	if !found {
		t.Fatal("expected rate-limit config blob to be seeded")
	}
	if strings.TrimSpace(string(gotRaw)) != strings.TrimSpace(raw) {
		t.Fatalf("seeded blob mismatch:\n got=%s\nwant=%s", string(gotRaw), raw)
	}
}

func TestSyncRateLimitStorage_RestoresFileAndRuntimeFromDB(t *testing.T) {
	restore := saveRateLimitStateForTest()
	defer restore()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "rate-limit.conf")
	fileRaw := rateLimitRawForTest(120)
	if err := os.WriteFile(path, []byte(fileRaw), 0o644); err != nil {
		t.Fatalf("write rate-limit file: %v", err)
	}
	if err := InitRateLimit(path); err != nil {
		t.Fatalf("init rate-limit: %v", err)
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
	dbRaw := rateLimitRawForTest(9)
	if err := store.UpsertConfigBlob(rateLimitConfigBlobKey, []byte(dbRaw), "", time.Now().UTC()); err != nil {
		t.Fatalf("upsert config blob: %v", err)
	}

	if err := SyncRateLimitStorage(); err != nil {
		t.Fatalf("sync rate-limit storage: %v", err)
	}

	gotFileRaw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read rate-limit file: %v", err)
	}
	if strings.TrimSpace(string(gotFileRaw)) != strings.TrimSpace(dbRaw) {
		t.Fatalf("file should be restored from db blob:\n got=%s\nwant=%s", string(gotFileRaw), dbRaw)
	}

	cfg := GetRateLimitConfig()
	if cfg.DefaultPolicy.Limit != 9 {
		t.Fatalf("runtime default_policy.limit=%d want=9", cfg.DefaultPolicy.Limit)
	}
}

func saveRateLimitStateForTest() func() {
	rateLimitMu.RLock()
	oldPath := rateLimitPath
	oldRuntime := rateLimitRuntime
	rateLimitMu.RUnlock()

	rateCounterMu.Lock()
	oldCounters := make(map[string]rateCounter, len(rateCounters))
	for k, v := range rateCounters {
		oldCounters[k] = v
	}
	oldSweep := rateCounterSweep
	rateCounterMu.Unlock()

	return func() {
		rateLimitMu.Lock()
		rateLimitPath = oldPath
		rateLimitRuntime = oldRuntime
		rateLimitMu.Unlock()

		rateCounterMu.Lock()
		rateCounters = oldCounters
		rateCounterSweep = oldSweep
		rateCounterMu.Unlock()
	}
}

func rateLimitRawForTest(limit int) string {
	return fmt.Sprintf(`{
  "enabled": true,
  "allowlist_ips": [],
  "allowlist_countries": [],
  "default_policy": {
    "enabled": true,
    "limit": %d,
    "window_seconds": 60,
    "burst": 0,
    "key_by": "ip",
    "action": {"status": 429, "retry_after_seconds": 60}
  },
  "rules": []
}`, limit)
}
