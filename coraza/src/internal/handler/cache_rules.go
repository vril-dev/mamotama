package handler

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"mamotama/internal/cacheconf"
)

const cacheConfPath = "conf/cache.conf"

type crPutBody struct {
	RawMode bool                `json:"rawMode"`
	Raw     string              `json:"raw"`
	Rules   []cacheconf.RuleDTO `json:"rules"`
}

func GetCacheRules(c *gin.Context) {
	raw, _ := os.ReadFile(cacheConfPath)
	rs := cacheconf.Get()
	dto := cacheconf.RulesDTO{
		ETag:  cacheconf.ComputeETag(raw),
		Raw:   string(raw),
		Rules: cacheconf.ToDTO(rs),
	}

	c.JSON(http.StatusOK, dto)
}

func ValidateCacheRules(c *gin.Context) {
	var in crPutBody
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if in.RawMode {
		if _, err := cacheconf.LoadFromBytes([]byte(in.Raw)); err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"ok": false, "messages": []string{err.Error()}})
			return
		}

		c.JSON(http.StatusOK, gin.H{"ok": true, "messages": []string{}})
		return
	}

	if _, errs := cacheconf.FromDTO(in.Rules); len(errs) > 0 {
		msgs := make([]string, 0, len(errs))
		for _, e := range errs {
			msgs = append(msgs, e.Error())
		}

		c.JSON(http.StatusUnprocessableEntity, gin.H{"ok": false, "messages": msgs})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "messages": []string{}})
}

func PutCacheRules(c *gin.Context) {
	ifMatch := c.GetHeader("If-Match")
	curRaw, _ := os.ReadFile(cacheConfPath)
	curETag := cacheconf.ComputeETag(curRaw)
	if ifMatch != "" && ifMatch != curETag {
		c.JSON(http.StatusConflict, gin.H{"error": "conflict", "currentETag": curETag})
		return
	}

	var in crPutBody
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var outBytes []byte
	if in.RawMode {
		if _, err := cacheconf.LoadFromBytes([]byte(in.Raw)); err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"ok": false, "messages": []string{err.Error()}})
			return
		}

		outBytes = []byte(in.Raw)
	} else {
		rs, errs := cacheconf.FromDTO(in.Rules)
		if len(errs) > 0 {
			msgs := make([]string, 0, len(errs))
			for _, e := range errs {
				msgs = append(msgs, e.Error())
			}
			c.JSON(http.StatusUnprocessableEntity, gin.H{"ok": false, "messages": msgs})
			return
		}

		header := []string{
			"# cache.conf - Mamotama Cache Rules",
			"# Top-down evaluation; first match wins.",
			"# Syntax: ALLOW|DENY prefix=...|regex=...|exact=... methods=GET,HEAD ttl=<sec> vary=Header,Header",
			"",
		}
		lines := cacheconf.RulesetToLines(rs)
		out := append(header, lines...)
		outBytes = []byte(strings.Join(out, "\n") + "\n")
	}

	if err := cacheconf.AtomicWriteWithBackup(cacheConfPath, outBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	newETag := cacheconf.ComputeETag(outBytes)
	c.JSON(http.StatusOK, gin.H{"ok": true, "etag": newETag})
}
