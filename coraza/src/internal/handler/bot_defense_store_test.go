package handler

import (
	"net/http"
	"net/http/httptest"
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
