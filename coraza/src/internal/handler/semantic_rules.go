package handler

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"mamotama/internal/bypassconf"
)

type semanticPutBody struct {
	Raw string `json:"raw"`
}

func bindSemanticPutBody(c *gin.Context) (semanticPutBody, bool) {
	var in semanticPutBody
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return semanticPutBody{}, false
	}

	return in, true
}

func GetSemanticRules(c *gin.Context) {
	path := GetSemanticPath()
	raw, _ := os.ReadFile(path)
	cfg := GetSemanticConfig()
	stats := GetSemanticStats()

	c.JSON(http.StatusOK, gin.H{
		"etag":                 bypassconf.ComputeETag(raw),
		"raw":                  string(raw),
		"enabled":              cfg.Enabled,
		"mode":                 cfg.Mode,
		"exempt_path_prefixes": cfg.ExemptPathPrefixes,
		"log_threshold":        cfg.LogThreshold,
		"challenge_threshold":  cfg.ChallengeThreshold,
		"block_threshold":      cfg.BlockThreshold,
		"max_inspect_body":     cfg.MaxInspectBody,
		"stats":                stats,
	})
}

func ValidateSemanticRules(c *gin.Context) {
	in, ok := bindSemanticPutBody(c)
	if !ok {
		return
	}

	rt, err := ValidateSemanticRaw(in.Raw)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"ok": false, "messages": []string{err.Error()}})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":                   true,
		"messages":             []string{},
		"enabled":              rt.Raw.Enabled,
		"mode":                 rt.Raw.Mode,
		"exempt_path_prefixes": rt.Raw.ExemptPathPrefixes,
		"log_threshold":        rt.Raw.LogThreshold,
		"challenge_threshold":  rt.Raw.ChallengeThreshold,
		"block_threshold":      rt.Raw.BlockThreshold,
		"max_inspect_body":     rt.Raw.MaxInspectBody,
	})
}

func PutSemanticRules(c *gin.Context) {
	path := GetSemanticPath()
	ifMatch := c.GetHeader("If-Match")
	curRaw, _ := os.ReadFile(path)
	curETag := bypassconf.ComputeETag(curRaw)
	if ifMatch != "" && ifMatch != curETag {
		c.JSON(http.StatusConflict, gin.H{"error": "conflict", "currentETag": curETag})
		return
	}

	in, ok := bindSemanticPutBody(c)
	if !ok {
		return
	}

	rt, err := ValidateSemanticRaw(in.Raw)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"ok": false, "messages": []string{err.Error()}})
		return
	}

	if err := bypassconf.AtomicWriteWithBackup(path, []byte(in.Raw)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := ReloadSemantic(); err != nil {
		_ = bypassconf.AtomicWriteWithBackup(path, curRaw)
		_ = ReloadSemantic()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	newETag := bypassconf.ComputeETag([]byte(in.Raw))
	c.JSON(http.StatusOK, gin.H{
		"ok":                   true,
		"etag":                 newETag,
		"enabled":              rt.Raw.Enabled,
		"mode":                 rt.Raw.Mode,
		"exempt_path_prefixes": rt.Raw.ExemptPathPrefixes,
		"log_threshold":        rt.Raw.LogThreshold,
		"challenge_threshold":  rt.Raw.ChallengeThreshold,
		"block_threshold":      rt.Raw.BlockThreshold,
		"max_inspect_body":     rt.Raw.MaxInspectBody,
	})
}
