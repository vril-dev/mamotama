package handler

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"

	"github.com/gin-gonic/gin"
	"mamotama/internal/bypassconf"
	"mamotama/internal/config"
	"mamotama/internal/crsselection"
	"mamotama/internal/waf"
)

type crsRuleSetItem struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Enabled bool   `json:"enabled"`
}

type crsRuleSetPutBody struct {
	Enabled []string `json:"enabled"`
}

func GetCRSRuleSets(c *gin.Context) {
	if !config.CRSEnable {
		raw, _ := os.ReadFile(config.CRSDisabledFile)
		c.JSON(http.StatusOK, gin.H{
			"crs_enabled":    false,
			"disabled_file":  config.CRSDisabledFile,
			"etag":           bypassconf.ComputeETag(raw),
			"rules":          []crsRuleSetItem{},
			"enabled_rules":  []string{},
			"total_rules":    0,
			"enabled_count":  0,
			"disabled_count": 0,
		})
		return
	}

	crsFiles, err := waf.DiscoverCRSRuleFiles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	raw, _ := os.ReadFile(config.CRSDisabledFile)
	disabledSet, err := crsselection.LoadDisabledFile(config.CRSDisabledFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	items := make([]crsRuleSetItem, 0, len(crsFiles))
	enabled := make([]string, 0, len(crsFiles))
	for _, p := range crsFiles {
		name := crsselection.NormalizeName(p)
		_, off := disabledSet[name]
		items = append(items, crsRuleSetItem{
			Name:    name,
			Path:    p,
			Enabled: !off,
		})
		if !off {
			enabled = append(enabled, name)
		}
	}
	sort.Strings(enabled)

	c.JSON(http.StatusOK, gin.H{
		"crs_enabled":    config.CRSEnable,
		"disabled_file":  config.CRSDisabledFile,
		"etag":           bypassconf.ComputeETag(raw),
		"rules":          items,
		"enabled_rules":  enabled,
		"total_rules":    len(items),
		"enabled_count":  len(enabled),
		"disabled_count": len(items) - len(enabled),
	})
}

func ValidateCRSRuleSets(c *gin.Context) {
	if !config.CRSEnable {
		c.JSON(http.StatusConflict, gin.H{"error": "CRS is disabled (WAF_CRS_ENABLE=false)"})
		return
	}

	var in crsRuleSetPutBody
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := waf.ValidateWithCRSSelection(in.Enabled); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"ok": false, "messages": []string{err.Error()}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "messages": []string{}})
}

func PutCRSRuleSets(c *gin.Context) {
	if !config.CRSEnable {
		c.JSON(http.StatusConflict, gin.H{"error": "CRS is disabled (WAF_CRS_ENABLE=false)"})
		return
	}

	var in crsRuleSetPutBody
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	curRaw, hadFile, err := readFileMaybe(config.CRSDisabledFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	curETag := bypassconf.ComputeETag(curRaw)
	if ifMatch := c.GetHeader("If-Match"); ifMatch != "" && ifMatch != curETag {
		c.JSON(http.StatusConflict, gin.H{"error": "conflict", "currentETag": curETag})
		return
	}

	crsFiles, err := waf.DiscoverCRSRuleFiles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	disabledNames, err := crsselection.BuildDisabledFromEnabled(crsFiles, in.Enabled)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"ok": false, "messages": []string{err.Error()}})
		return
	}

	if err := waf.ValidateWithCRSSelection(in.Enabled); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"ok": false, "messages": []string{err.Error()}})
		return
	}

	if err := os.MkdirAll(filepath.Dir(config.CRSDisabledFile), 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	nextRaw := crsselection.SerializeDisabled(disabledNames)
	if err := bypassconf.AtomicWriteWithBackup(config.CRSDisabledFile, nextRaw); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := waf.ReloadBaseWAF(); err != nil {
		rollbackErr := rollbackCRSDisabledFile(config.CRSDisabledFile, hadFile, curRaw)
		_ = waf.ReloadBaseWAF()
		msg := fmt.Sprintf("reload failed and rollback applied: %v", err)
		if rollbackErr != nil {
			msg = fmt.Sprintf("%s (rollback error: %v)", msg, rollbackErr)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":             true,
		"etag":           bypassconf.ComputeETag(nextRaw),
		"hot_reloaded":   true,
		"disabled_count": len(disabledNames),
	})
}

func readFileMaybe(path string) ([]byte, bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []byte{}, false, nil
		}
		return nil, false, err
	}
	return b, true, nil
}

func rollbackCRSDisabledFile(path string, hadFile bool, previous []byte) error {
	if hadFile {
		return bypassconf.AtomicWriteWithBackup(path, previous)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
