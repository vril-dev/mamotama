package handler

import (
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
