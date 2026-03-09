package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestValidateSemanticRaw(t *testing.T) {
	raw := `{
  "enabled": true,
  "mode": "challenge",
  "exempt_path_prefixes": ["/healthz"],
  "log_threshold": 2,
  "challenge_threshold": 4,
  "block_threshold": 8,
  "max_inspect_body": 8192
}`
	rt, err := ValidateSemanticRaw(raw)
	if err != nil {
		t.Fatalf("ValidateSemanticRaw() unexpected error: %v", err)
	}
	if rt == nil || !rt.Raw.Enabled {
		t.Fatalf("runtime config should be enabled: %#v", rt)
	}
	if rt.Raw.Mode != "challenge" {
		t.Fatalf("mode=%q want=challenge", rt.Raw.Mode)
	}
}

func TestEvaluateSemantic_BlockAction(t *testing.T) {
	raw := `{
  "enabled": true,
  "mode": "block",
  "exempt_path_prefixes": [],
  "log_threshold": 1,
  "challenge_threshold": 2,
  "block_threshold": 3,
  "max_inspect_body": 16384
}`
	rt, err := ValidateSemanticRaw(raw)
	if err != nil {
		t.Fatalf("ValidateSemanticRaw() unexpected error: %v", err)
	}

	semanticMu.Lock()
	prev := semanticRuntime
	semanticRuntime = rt
	semanticMu.Unlock()
	defer func() {
		semanticMu.Lock()
		semanticRuntime = prev
		semanticMu.Unlock()
	}()

	req := httptest.NewRequest(http.MethodGet, "http://example.test/?q=union+select+password+from+users", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	eval := EvaluateSemantic(req)
	if eval.Action != semanticActionBlock {
		t.Fatalf("expected block action, got=%+v", eval)
	}
	if eval.Score < 3 {
		t.Fatalf("expected score >= 3, got=%d", eval.Score)
	}
}

func TestEvaluateSemantic_ChallengeCookiePass(t *testing.T) {
	raw := `{
  "enabled": true,
  "mode": "challenge",
  "exempt_path_prefixes": [],
  "log_threshold": 1,
  "challenge_threshold": 2,
  "block_threshold": 10,
  "max_inspect_body": 16384
}`
	rt, err := ValidateSemanticRaw(raw)
	if err != nil {
		t.Fatalf("ValidateSemanticRaw() unexpected error: %v", err)
	}

	semanticMu.Lock()
	prev := semanticRuntime
	semanticRuntime = rt
	semanticMu.Unlock()
	defer func() {
		semanticMu.Lock()
		semanticRuntime = prev
		semanticMu.Unlock()
	}()

	now := time.Unix(1_700_000_000, 0).UTC()
	req1 := httptest.NewRequest(http.MethodGet, "http://example.test/?q=union+select+1", nil)
	req1.Header.Set("User-Agent", "curl/8.0")
	eval := EvaluateSemantic(req1)
	if eval.Action != semanticActionChallenge {
		t.Fatalf("expected challenge action, got=%+v", eval)
	}
	if HasValidSemanticChallengeCookie(req1, "10.0.0.1", now) {
		t.Fatal("request without cookie should not pass challenge")
	}

	token := issueSemanticChallengeToken(rt, "10.0.0.1", "curl/8.0", now)
	req2 := httptest.NewRequest(http.MethodGet, "http://example.test/?q=union+select+1", nil)
	req2.Header.Set("User-Agent", "curl/8.0")
	req2.AddCookie(&http.Cookie{Name: rt.challengeCookieName, Value: token})
	if !HasValidSemanticChallengeCookie(req2, "10.0.0.1", now.Add(1*time.Second)) {
		t.Fatal("request with valid cookie should pass challenge")
	}
}
