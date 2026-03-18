package handler

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"mamotama/internal/bypassconf"
	"mamotama/internal/config"
)

type bypassPutBody struct {
	Raw string `json:"raw"`
}

func bindBypassPutBody(c *gin.Context) (bypassPutBody, bool) {
	var in bypassPutBody
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return bypassPutBody{}, false
	}

	return in, true
}

func GetBypassRules(c *gin.Context) {
	path := config.BypassFile
	raw, err := readConfigBlobOrFile(dbConfigKeyBypassRaw, path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"etag": bypassconf.ComputeETag(raw),
		"raw":  string(raw),
	})
}

func ValidateBypassRules(c *gin.Context) {
	in, ok := bindBypassPutBody(c)
	if !ok {
		return
	}

	if _, err := validateRaw(in.Raw); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"ok": false, "messages": []string{err.Error()}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "messages": []string{}})
}

func PutBypassRules(c *gin.Context) {
	path := config.BypassFile
	ifMatch := c.GetHeader("If-Match")
	curRaw, err := readConfigBlobOrFile(dbConfigKeyBypassRaw, path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	curETag := bypassconf.ComputeETag(curRaw)
	if ifMatch != "" && ifMatch != curETag {
		c.JSON(http.StatusConflict, gin.H{"error": "conflict", "currentETag": curETag})
		return
	}

	in, ok := bindBypassPutBody(c)
	if !ok {
		return
	}

	if _, err := validateRaw(in.Raw); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"ok": false, "messages": []string{err.Error()}})
		return
	}

	if err := putConfigBlobIfEnabled(dbConfigKeyBypassRaw, in.Raw); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := bypassconf.AtomicWriteWithBackup(path, []byte(in.Raw)); err != nil {
		rollbackConfigBlobIfEnabled(dbConfigKeyBypassRaw, string(curRaw))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := bypassconf.Reload(); err != nil {
		_ = bypassconf.AtomicWriteWithBackup(path, curRaw)
		_ = bypassconf.Reload()
		rollbackConfigBlobIfEnabled(dbConfigKeyBypassRaw, string(curRaw))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	newETag := bypassconf.ComputeETag([]byte(in.Raw))
	c.JSON(http.StatusOK, gin.H{"ok": true, "etag": newETag})
}

func validateRaw(s string) (int, error) {
	es, err := bypassconf.Parse(s)
	if err != nil {
		return 0, err
	}
	for _, e := range es {
		if e.ExtraRule == "" {
			continue
		}
		if _, statErr := os.Stat(e.ExtraRule); statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) && !config.StrictOverride {
				continue
			}
			return 0, fmt.Errorf("extra rule not found: %s", e.ExtraRule)
		}
	}

	return len(es), nil
}
