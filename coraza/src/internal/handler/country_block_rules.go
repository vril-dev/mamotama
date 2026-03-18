package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"mamotama/internal/bypassconf"
)

type countryBlockPutBody struct {
	Raw string `json:"raw"`
}

func bindCountryBlockPutBody(c *gin.Context) (countryBlockPutBody, bool) {
	var in countryBlockPutBody
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return countryBlockPutBody{}, false
	}

	return in, true
}

func GetCountryBlockRules(c *gin.Context) {
	path := GetCountryBlockPath()
	raw, err := readConfigBlobOrFile(dbConfigKeyCountryBlockRaw, path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"etag":    bypassconf.ComputeETag(raw),
		"raw":     string(raw),
		"blocked": GetBlockedCountries(),
	})
}

func ValidateCountryBlockRules(c *gin.Context) {
	in, ok := bindCountryBlockPutBody(c)
	if !ok {
		return
	}

	codes, err := ParseCountryBlockRaw(in.Raw)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"ok": false, "messages": []string{err.Error()}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "messages": []string{}, "blocked": codes})
}

func PutCountryBlockRules(c *gin.Context) {
	path := GetCountryBlockPath()
	ifMatch := c.GetHeader("If-Match")
	curRaw, err := readConfigBlobOrFile(dbConfigKeyCountryBlockRaw, path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	curETag := bypassconf.ComputeETag(curRaw)
	if ifMatch != "" && ifMatch != curETag {
		c.JSON(http.StatusConflict, gin.H{"error": "conflict", "currentETag": curETag})
		return
	}

	in, ok := bindCountryBlockPutBody(c)
	if !ok {
		return
	}

	codes, err := ParseCountryBlockRaw(in.Raw)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"ok": false, "messages": []string{err.Error()}})
		return
	}

	if err := putConfigBlobIfEnabled(dbConfigKeyCountryBlockRaw, in.Raw); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := bypassconf.AtomicWriteWithBackup(path, []byte(in.Raw)); err != nil {
		rollbackConfigBlobIfEnabled(dbConfigKeyCountryBlockRaw, string(curRaw))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := ReloadCountryBlock(); err != nil {
		_ = bypassconf.AtomicWriteWithBackup(path, curRaw)
		_ = ReloadCountryBlock()
		rollbackConfigBlobIfEnabled(dbConfigKeyCountryBlockRaw, string(curRaw))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	newETag := bypassconf.ComputeETag([]byte(in.Raw))
	c.JSON(http.StatusOK, gin.H{"ok": true, "etag": newETag, "blocked": codes})
}
