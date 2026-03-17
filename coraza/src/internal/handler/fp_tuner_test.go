package handler

import (
	"strings"
	"testing"
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
