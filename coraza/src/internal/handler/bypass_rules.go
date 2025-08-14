package handler

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"mamotama/internal/bypassconf"
	"mamotama/internal/config"
)

type bypassPutBody struct {
	Raw string `json:"raw"`
}

func GetBypassRules(c *gin.Context) {
	path := config.BypassFile
	raw, _ := os.ReadFile(path)
	c.JSON(http.StatusOK, gin.H{
		"etag": bypassconf.ComputeETag(raw),
		"raw":  string(raw),
	})
}

func ValidateBypassRules(c *gin.Context) {
	var in bypassPutBody
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if _, err := os.ReadFile(config.BypassFile); err != nil {
		//
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
	curRaw, _ := os.ReadFile(path)
	curETag := bypassconf.ComputeETag(curRaw)
	if ifMatch != "" && ifMatch != curETag {
		c.JSON(http.StatusConflict, gin.H{"error": "conflict", "currentETag": curETag})
		return
	}

	var in bypassPutBody
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if _, err := validateRaw(in.Raw); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"ok": false, "messages": []string{err.Error()}})
		return
	}

	if err := bypassconf.AtomicWriteWithBackup(path, []byte(in.Raw)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_ = bypassconf.Reload()

	newETag := bypassconf.ComputeETag([]byte(in.Raw))
	c.JSON(http.StatusOK, gin.H{"ok": true, "etag": newETag})
}

func validateRaw(s string) (int, error) {
	es, err := func() ([]bypassconf.Entry, error) {
		tmp := s // no-op
		_ = tmp

		return nil, nil
	}()

	if err != nil {
		return 0, err
	}

	return len(es), nil
}
