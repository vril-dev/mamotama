package handler

import (
	"reflect"
	"testing"

	"mamotama/internal/config"
)

func TestEnsureEditableRulePath(t *testing.T) {
	restore := saveRuleConfig()
	defer restore()

	config.CRSEnable = false
	config.RulesFile = "rules/a.conf, rules/b.conf"

	path, err := ensureEditableRulePath("rules/a.conf")
	if err != nil {
		t.Fatalf("ensureEditableRulePath returned error: %v", err)
	}
	if path != "rules/a.conf" {
		t.Fatalf("path=%q want=%q", path, "rules/a.conf")
	}

	if _, err := ensureEditableRulePath("rules/c.conf"); err == nil {
		t.Fatal("ensureEditableRulePath should reject paths outside configured rules")
	}
}

func TestValidateRaw_StrictOverride(t *testing.T) {
	restore := saveRuleConfig()
	defer restore()

	config.StrictOverride = false
	if _, err := validateRaw("/foo rules/missing.conf\n"); err != nil {
		t.Fatalf("validateRaw should allow missing extra rule when strict=false: %v", err)
	}

	config.StrictOverride = true
	if _, err := validateRaw("/foo rules/missing.conf\n"); err == nil {
		t.Fatal("validateRaw should fail when strict=true and extra rule is missing")
	}
}

func TestBaseRuleFilesFromConfig(t *testing.T) {
	restore := saveRuleConfig()
	defer restore()

	config.RulesFile = "rules/a.conf, ./rules/a.conf ,rules/b.conf, rules/b.conf"
	got := baseRuleFilesFromConfig()
	want := []string{"rules/a.conf", "rules/b.conf"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("baseRuleFilesFromConfig=%v want=%v", got, want)
	}
}

func saveRuleConfig() func() {
	oldRulesFile := config.RulesFile
	oldCRSEnable := config.CRSEnable
	oldCRSSetup := config.CRSSetupFile
	oldCRSRulesDir := config.CRSRulesDir
	oldCRSDisabled := config.CRSDisabledFile
	oldStrict := config.StrictOverride
	return func() {
		config.RulesFile = oldRulesFile
		config.CRSEnable = oldCRSEnable
		config.CRSSetupFile = oldCRSSetup
		config.CRSRulesDir = oldCRSRulesDir
		config.CRSDisabledFile = oldCRSDisabled
		config.StrictOverride = oldStrict
	}
}
