package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"mamotama/internal/bypassconf"
	"mamotama/internal/config"
	"mamotama/internal/waf"
)

func StatusHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":                  "running",
		"rules_file":              config.RulesFile,
		"bypass_file":             config.BypassFile,
		"country_block_file":      config.CountryBlockFile,
		"blocked_countries":       GetBlockedCountries(),
		"log_file":                config.LogFile,
		"strict_mode":             config.StrictOverride,
		"api_base":                config.APIBasePath,
		"crs_enabled":             config.CRSEnable,
		"crs_setup_file":          config.CRSSetupFile,
		"crs_rules_dir":           config.CRSRulesDir,
		"crs_disabled_file":       config.CRSDisabledFile,
		"allow_insecure_defaults": config.AllowInsecureDefaults,
	})
}

func RulesHandler(c *gin.Context) {
	files := configuredRuleFiles()
	result := make(map[string]string)
	out := make([]gin.H, 0, len(files))

	for _, path := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			result[path] = "[読込失敗] " + err.Error()
			out = append(out, gin.H{
				"path":  path,
				"raw":   "",
				"etag":  "",
				"error": err.Error(),
			})
			continue
		}
		result[path] = string(content)
		out = append(out, gin.H{
			"path": path,
			"raw":  string(content),
			"etag": bypassconf.ComputeETag(content),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"rules": result,
		"files": out,
	})
}

type rulesPutBody struct {
	Path string `json:"path"`
	Raw  string `json:"raw"`
}

func ValidateRules(c *gin.Context) {
	var in rulesPutBody
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	target, err := ensureEditableRulePath(in.Path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := waf.ValidateWithRuleOverride(target, []byte(in.Raw)); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"ok": false, "messages": []string{err.Error()}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "messages": []string{}})
}

func PutRules(c *gin.Context) {
	var in rulesPutBody
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	target, err := ensureEditableRulePath(in.Path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	curRaw, err := os.ReadFile(target)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	curETag := bypassconf.ComputeETag(curRaw)
	if ifMatch := c.GetHeader("If-Match"); ifMatch != "" && ifMatch != curETag {
		c.JSON(http.StatusConflict, gin.H{"error": "conflict", "currentETag": curETag})
		return
	}

	if err := waf.ValidateWithRuleOverride(target, []byte(in.Raw)); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"ok": false, "messages": []string{err.Error()}})
		return
	}

	if err := bypassconf.AtomicWriteWithBackup(target, []byte(in.Raw)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := waf.ReloadBaseWAF(); err != nil {
		_ = bypassconf.AtomicWriteWithBackup(target, curRaw)
		_ = waf.ReloadBaseWAF()
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("reload failed and rollback applied: %v", err),
		})
		return
	}

	newETag := bypassconf.ComputeETag([]byte(in.Raw))
	c.JSON(http.StatusOK, gin.H{
		"ok":            true,
		"etag":          newETag,
		"hot_reloaded":  true,
		"reloaded_file": target,
	})
}

func configuredRuleFiles() []string {
	files, err := waf.PrepareInitialRuleFiles()
	if err == nil {
		return files
	}

	parts := strings.Split(config.RulesFile, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

func ensureEditableRulePath(path string) (string, error) {
	target := filepath.Clean(strings.TrimSpace(path))
	if target == "" {
		return "", fmt.Errorf("path is empty")
	}
	for _, p := range configuredRuleFiles() {
		if filepath.Clean(p) == target {
			return p, nil
		}
	}
	return "", fmt.Errorf("path is not editable: %s", path)
}
