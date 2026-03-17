package handler

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"mamotama/internal/config"
)

func TestDecodeFPTunerProviderResponseWrapped(t *testing.T) {
	raw := []byte(`{"proposal":{"id":"fp-1","summary":"ok","rule_line":"SecRule REQUEST_URI \"@beginsWith /search\" \"id:190123,phase:1,pass,nolog,ctl:ruleRemoveTargetById=100004;ARGS:q,msg:'mamotama fp_tuner scoped exclusion'\""}}`)

	proposal, err := decodeFPTunerProviderResponse(raw)
	if err != nil {
		t.Fatalf("decodeFPTunerProviderResponse wrapped error: %v", err)
	}
	if proposal.ID != "fp-1" {
		t.Fatalf("proposal.ID=%q want=fp-1", proposal.ID)
	}
}

func TestDecodeFPTunerProviderResponseDirect(t *testing.T) {
	raw := []byte(`{"id":"fp-2","summary":"ok","rule_line":"SecRule REQUEST_URI \"@beginsWith /search\" \"id:190123,phase:1,pass,nolog,ctl:ruleRemoveTargetById=100004;ARGS:q,msg:'mamotama fp_tuner scoped exclusion'\""}`)

	proposal, err := decodeFPTunerProviderResponse(raw)
	if err != nil {
		t.Fatalf("decodeFPTunerProviderResponse direct error: %v", err)
	}
	if proposal.ID != "fp-2" {
		t.Fatalf("proposal.ID=%q want=fp-2", proposal.ID)
	}
}

func TestBuildFPTunerRuleLine(t *testing.T) {
	line := buildFPTunerRuleLine(fpTunerEventInput{
		Path:            "/search",
		RuleID:          100004,
		MatchedVariable: "ARGS:q",
	})

	if !strings.Contains(line, "ctl:ruleRemoveTargetById=100004;ARGS:q") {
		t.Fatalf("rule line missing ctl fragment: %s", line)
	}
	if !strings.HasPrefix(line, `SecRule REQUEST_URI "@beginsWith /search"`) {
		t.Fatalf("unexpected rule line prefix: %s", line)
	}
}

func TestValidateFPTunerRuleLine(t *testing.T) {
	good := `SecRule REQUEST_URI "@beginsWith /search" "id:190123,phase:1,pass,nolog,ctl:ruleRemoveTargetById=100004;ARGS:q,msg:'mamotama fp_tuner scoped exclusion'"`
	if err := validateFPTunerRuleLine(good); err != nil {
		t.Fatalf("validateFPTunerRuleLine good returned err: %v", err)
	}

	bad := `SecAction "id:1,phase:1,pass"`
	if err := validateFPTunerRuleLine(bad); err == nil {
		t.Fatal("validateFPTunerRuleLine should reject unsafe line")
	}
}

func TestMaskSensitiveText(t *testing.T) {
	in := "Authorization=Bearer abc.def.ghi token=supersecret1234567890123456 email=a@example.com ip=10.1.2.3"
	out := maskSensitiveText(in)
	if strings.Contains(out, "supersecret1234567890123456") {
		t.Fatalf("token should be masked: %s", out)
	}
	if strings.Contains(out, "a@example.com") {
		t.Fatalf("email should be masked: %s", out)
	}
	if strings.Contains(out, "10.1.2.3") {
		t.Fatalf("ip should be masked: %s", out)
	}
}

func TestDecodeJSONBodyStrictRejectsUnknownFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"event":{"path":"/search"},"unknown":"x"}`)
	req := httptest.NewRequest("POST", "/mamotama-api/fp-tuner/propose", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	var payload fpTunerProposeBody
	err := decodeJSONBodyStrict(c, &payload)
	if err == nil {
		t.Fatal("decodeJSONBodyStrict should reject unknown fields")
	}
}

func TestDecodeJSONBodyStrictSingleObject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	obj := fpTunerApplyBody{
		Proposal: fpTunerProposal{
			ID:         "fp-1",
			RuleLine:   `SecRule REQUEST_URI "@beginsWith /search" "id:190123,phase:1,pass,nolog,ctl:ruleRemoveTargetById=100004;ARGS:q,msg:'mamotama fp_tuner scoped exclusion'"`,
			TargetPath: "rules/mamotama.conf",
		},
	}
	raw, _ := json.Marshal(obj)
	req := httptest.NewRequest("POST", "/mamotama-api/fp-tuner/apply", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	var payload fpTunerApplyBody
	if err := decodeJSONBodyStrict(c, &payload); err != nil {
		t.Fatalf("decodeJSONBodyStrict returned error: %v", err)
	}
	if payload.Proposal.ID != "fp-1" {
		t.Fatalf("proposal id mismatch: %q", payload.Proposal.ID)
	}
}

func TestProposalHashStable(t *testing.T) {
	p := fpTunerProposal{
		ID:         "fp-1",
		TargetPath: "rules/mamotama.conf",
		RuleLine:   `SecRule REQUEST_URI "@beginsWith /search" "id:190123,phase:1,pass,nolog,ctl:ruleRemoveTargetById=100004;ARGS:q,msg:'mamotama fp_tuner scoped exclusion'"`,
	}
	h1 := proposalHash(p)
	h2 := proposalHash(p)
	if h1 == "" || h1 != h2 {
		t.Fatalf("proposalHash should be stable and non-empty: %q %q", h1, h2)
	}
}

func TestApprovalTokenLifecycle(t *testing.T) {
	prevTTL := config.FPTunerApprovalTTL
	config.FPTunerApprovalTTL = 60 * time.Second
	defer func() { config.FPTunerApprovalTTL = prevTTL }()

	fpApprovalMu.Lock()
	fpApprovalStore = map[string]fpApprovalEntry{}
	fpApprovalMu.Unlock()

	proposal := fpTunerProposal{
		ID:         "fp-1",
		TargetPath: "rules/mamotama.conf",
		RuleLine:   `SecRule REQUEST_URI "@beginsWith /search" "id:190123,phase:1,pass,nolog,ctl:ruleRemoveTargetById=100004;ARGS:q,msg:'mamotama fp_tuner scoped exclusion'"`,
	}
	token, err := issueFPTunerApprovalToken(proposal)
	if err != nil {
		t.Fatalf("issueFPTunerApprovalToken error: %v", err)
	}
	if token == "" {
		t.Fatal("approval token should not be empty")
	}

	if err := consumeFPTunerApprovalToken(token, proposal); err != nil {
		t.Fatalf("consumeFPTunerApprovalToken first call error: %v", err)
	}
	if err := consumeFPTunerApprovalToken(token, proposal); err == nil {
		t.Fatal("consumeFPTunerApprovalToken should reject reused token")
	}
}

func TestApprovalTokenProposalMismatch(t *testing.T) {
	prevTTL := config.FPTunerApprovalTTL
	config.FPTunerApprovalTTL = 60 * time.Second
	defer func() { config.FPTunerApprovalTTL = prevTTL }()

	fpApprovalMu.Lock()
	fpApprovalStore = map[string]fpApprovalEntry{}
	fpApprovalMu.Unlock()

	p1 := fpTunerProposal{
		ID:         "fp-1",
		TargetPath: "rules/mamotama.conf",
		RuleLine:   `SecRule REQUEST_URI "@beginsWith /search" "id:190123,phase:1,pass,nolog,ctl:ruleRemoveTargetById=100004;ARGS:q,msg:'mamotama fp_tuner scoped exclusion'"`,
	}
	p2 := fpTunerProposal{
		ID:         "fp-2",
		TargetPath: "rules/mamotama.conf",
		RuleLine:   `SecRule REQUEST_URI "@beginsWith /users" "id:190124,phase:1,pass,nolog,ctl:ruleRemoveTargetById=100004;ARGS:q,msg:'mamotama fp_tuner scoped exclusion'"`,
	}

	token, err := issueFPTunerApprovalToken(p1)
	if err != nil {
		t.Fatalf("issueFPTunerApprovalToken error: %v", err)
	}
	if err := consumeFPTunerApprovalToken(token, p2); err == nil {
		t.Fatal("consumeFPTunerApprovalToken should reject proposal mismatch")
	}
}
