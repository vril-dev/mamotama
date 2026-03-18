package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestLogsStatsSQLiteStoreAggregatesAndIngestsIncrementally(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Now().UTC()
	entries := []map[string]any{
		{
			"ts":      now.Add(-10 * time.Minute).Format(time.RFC3339Nano),
			"event":   "waf_block",
			"rule_id": 942100,
			"path":    "/login",
			"country": "jp",
			"status":  403,
			"req_id":  "req-1",
		},
		{
			"ts":      now.Add(-2 * time.Hour).Format(time.RFC3339Nano),
			"event":   "waf_block",
			"rule_id": "920350",
			"path":    "/admin",
			"country": "US",
			"status":  403,
			"req_id":  "req-2",
		},
		{
			"ts":    now.Add(-5 * time.Minute).Format(time.RFC3339Nano),
			"event": "waf_hit_allow",
			"path":  "/allow",
		},
	}

	tmp := t.TempDir()
	logPath := filepath.Join(tmp, "waf-events.ndjson")
	writeNDJSONFile(t, logPath, entries)

	restoreLogPath := setWAFLogPathForTest(t, logPath)
	defer restoreLogPath()

	dbPath := filepath.Join(tmp, "mamotama.db")
	if err := InitLogsStatsStore(true, dbPath); err != nil {
		t.Fatalf("init sqlite store: %v", err)
	}
	t.Cleanup(func() {
		_ = InitLogsStatsStore(false, "")
	})

	first := callLogsStats(t, "/mamotama-api/logs/stats?hours=6")
	if first.ScannedLines != len(entries) {
		t.Fatalf("first scanned_lines=%d want=%d", first.ScannedLines, len(entries))
	}
	if first.WAFBlock.TotalInScan != 2 {
		t.Fatalf("first total_in_scan=%d want=2", first.WAFBlock.TotalInScan)
	}
	if first.WAFBlock.Last1h != 1 {
		t.Fatalf("first last_1h=%d want=1", first.WAFBlock.Last1h)
	}
	if first.WAFBlock.Last24h != 2 {
		t.Fatalf("first last_24h=%d want=2", first.WAFBlock.Last24h)
	}

	appendNDJSONLine(t, logPath, map[string]any{
		"ts":      now.Add(-2 * time.Minute).Format(time.RFC3339Nano),
		"event":   "waf_block",
		"rule_id": 942100,
		"path":    "/login",
		"country": "JP",
		"status":  403,
		"req_id":  "req-3",
	})

	second := callLogsStats(t, "/mamotama-api/logs/stats?hours=6")
	if second.ScannedLines != 1 {
		t.Fatalf("second scanned_lines=%d want=1", second.ScannedLines)
	}
	if second.WAFBlock.TotalInScan != 3 {
		t.Fatalf("second total_in_scan=%d want=3", second.WAFBlock.TotalInScan)
	}
	if second.WAFBlock.Last1h != 2 {
		t.Fatalf("second last_1h=%d want=2", second.WAFBlock.Last1h)
	}
	if second.WAFBlock.Last24h != 3 {
		t.Fatalf("second last_24h=%d want=3", second.WAFBlock.Last24h)
	}
}

func callLogsStats(t *testing.T, path string) logsStatsResp {
	t.Helper()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, path, nil)

	LogsStats(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var out logsStatsResp
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

func appendNDJSONLine(t *testing.T, path string, entry map[string]any) {
	t.Helper()

	line, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal ndjson entry: %v", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		t.Fatalf("open append file: %v", err)
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		t.Fatalf("append ndjson entry: %v", err)
	}
}
