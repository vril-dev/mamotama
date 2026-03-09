package handler

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"mamotama/internal/bypassconf"
)

type botDefensePutBody struct {
	Raw string `json:"raw"`
}

func bindBotDefensePutBody(c *gin.Context) (botDefensePutBody, bool) {
	var in botDefensePutBody
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return botDefensePutBody{}, false
	}

	return in, true
}

func GetBotDefenseRules(c *gin.Context) {
	path := GetBotDefensePath()
	raw, _ := os.ReadFile(path)
	cfg := GetBotDefenseConfig()

	c.JSON(http.StatusOK, gin.H{
		"etag":          bypassconf.ComputeETag(raw),
		"raw":           string(raw),
		"enabled":       cfg.Enabled,
		"mode":          cfg.Mode,
		"path_prefixes": cfg.PathPrefixes,
	})
}

func ValidateBotDefenseRules(c *gin.Context) {
	in, ok := bindBotDefensePutBody(c)
	if !ok {
		return
	}

	rt, err := ValidateBotDefenseRaw(in.Raw)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"ok": false, "messages": []string{err.Error()}})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":            true,
		"messages":      []string{},
		"enabled":       rt.Raw.Enabled,
		"mode":          rt.Raw.Mode,
		"path_prefixes": rt.Raw.PathPrefixes,
	})
}

func PutBotDefenseRules(c *gin.Context) {
	path := GetBotDefensePath()
	ifMatch := c.GetHeader("If-Match")
	curRaw, _ := os.ReadFile(path)
	curETag := bypassconf.ComputeETag(curRaw)
	if ifMatch != "" && ifMatch != curETag {
		c.JSON(http.StatusConflict, gin.H{"error": "conflict", "currentETag": curETag})
		return
	}

	in, ok := bindBotDefensePutBody(c)
	if !ok {
		return
	}

	rt, err := ValidateBotDefenseRaw(in.Raw)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"ok": false, "messages": []string{err.Error()}})
		return
	}

	if err := bypassconf.AtomicWriteWithBackup(path, []byte(in.Raw)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := ReloadBotDefense(); err != nil {
		_ = bypassconf.AtomicWriteWithBackup(path, curRaw)
		_ = ReloadBotDefense()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	newETag := bypassconf.ComputeETag([]byte(in.Raw))
	c.JSON(http.StatusOK, gin.H{
		"ok":            true,
		"etag":          newETag,
		"enabled":       rt.Raw.Enabled,
		"mode":          rt.Raw.Mode,
		"path_prefixes": rt.Raw.PathPrefixes,
	})
}
