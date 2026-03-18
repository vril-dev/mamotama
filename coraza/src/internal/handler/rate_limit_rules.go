package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"mamotama/internal/bypassconf"
)

type rateLimitPutBody struct {
	Raw string `json:"raw"`
}

func bindRateLimitPutBody(c *gin.Context) (rateLimitPutBody, bool) {
	var in rateLimitPutBody
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return rateLimitPutBody{}, false
	}

	return in, true
}

func GetRateLimitRules(c *gin.Context) {
	path := GetRateLimitPath()
	raw, err := readConfigBlobOrFile(dbConfigKeyRateLimitRaw, path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	cfg := GetRateLimitConfig()
	c.JSON(http.StatusOK, gin.H{
		"etag":    bypassconf.ComputeETag(raw),
		"raw":     string(raw),
		"enabled": cfg.Enabled,
		"rules":   len(cfg.Rules),
	})
}

func ValidateRateLimitRules(c *gin.Context) {
	in, ok := bindRateLimitPutBody(c)
	if !ok {
		return
	}

	rt, err := ValidateRateLimitRaw(in.Raw)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"ok": false, "messages": []string{err.Error()}})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":       true,
		"messages": []string{},
		"enabled":  rt.Raw.Enabled,
		"rules":    len(rt.Raw.Rules),
	})
}

func PutRateLimitRules(c *gin.Context) {
	path := GetRateLimitPath()
	ifMatch := c.GetHeader("If-Match")
	curRaw, err := readConfigBlobOrFile(dbConfigKeyRateLimitRaw, path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	curETag := bypassconf.ComputeETag(curRaw)
	if ifMatch != "" && ifMatch != curETag {
		c.JSON(http.StatusConflict, gin.H{"error": "conflict", "currentETag": curETag})
		return
	}

	in, ok := bindRateLimitPutBody(c)
	if !ok {
		return
	}

	rt, err := ValidateRateLimitRaw(in.Raw)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"ok": false, "messages": []string{err.Error()}})
		return
	}

	if err := putConfigBlobIfEnabled(dbConfigKeyRateLimitRaw, in.Raw); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := bypassconf.AtomicWriteWithBackup(path, []byte(in.Raw)); err != nil {
		rollbackConfigBlobIfEnabled(dbConfigKeyRateLimitRaw, string(curRaw))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := ReloadRateLimit(); err != nil {
		_ = bypassconf.AtomicWriteWithBackup(path, curRaw)
		_ = ReloadRateLimit()
		rollbackConfigBlobIfEnabled(dbConfigKeyRateLimitRaw, string(curRaw))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	newETag := bypassconf.ComputeETag([]byte(in.Raw))
	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"etag":    newETag,
		"enabled": rt.Raw.Enabled,
		"rules":   len(rt.Raw.Rules),
	})
}
