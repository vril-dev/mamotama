package handler

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const (
	logStatsStoreSourceWAF = "waf"
)

var (
	logStatsStoreMu sync.RWMutex
	logStatsStore   *wafEventStore
)

type wafEventStore struct {
	db *sql.DB
	mu sync.Mutex
}

type logIngestState struct {
	Offset    int64
	Size      int64
	ModTimeNS int64
}

type logSyncResult struct {
	ScannedLines int
}

func InitLogsStatsStore(enabled bool, dbPath string) error {
	logStatsStoreMu.Lock()
	defer logStatsStoreMu.Unlock()

	if logStatsStore != nil {
		_ = logStatsStore.Close()
		logStatsStore = nil
	}

	if !enabled {
		return nil
	}

	store, err := openWAFEventStore(dbPath)
	if err != nil {
		return err
	}
	logStatsStore = store
	return nil
}

func getLogsStatsStore() *wafEventStore {
	logStatsStoreMu.RLock()
	defer logStatsStoreMu.RUnlock()
	return logStatsStore
}

func openWAFEventStore(dbPath string) (*wafEventStore, error) {
	p := strings.TrimSpace(dbPath)
	if p == "" {
		return nil, fmt.Errorf("db path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir db dir: %w", err)
	}

	db, err := sql.Open("sqlite", p)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)

	stmts := []string{
		`PRAGMA journal_mode = WAL;`,
		`PRAGMA synchronous = NORMAL;`,
		`CREATE TABLE IF NOT EXISTS waf_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			event TEXT NOT NULL,
			ts_unix INTEGER NOT NULL,
			ts TEXT NOT NULL,
			rule_id TEXT NOT NULL,
			path TEXT NOT NULL,
			country TEXT NOT NULL,
			status INTEGER NOT NULL,
			req_id TEXT,
			line_hash TEXT NOT NULL UNIQUE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_waf_events_ts_unix ON waf_events(ts_unix);`,
		`CREATE INDEX IF NOT EXISTS idx_waf_events_rule_id ON waf_events(rule_id);`,
		`CREATE INDEX IF NOT EXISTS idx_waf_events_path ON waf_events(path);`,
		`CREATE INDEX IF NOT EXISTS idx_waf_events_country ON waf_events(country);`,
		`CREATE TABLE IF NOT EXISTS ingest_state (
			source TEXT PRIMARY KEY,
			offset INTEGER NOT NULL,
			size INTEGER NOT NULL,
			mod_time_ns INTEGER NOT NULL
		);`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("init sqlite schema: %w", err)
		}
	}

	return &wafEventStore{db: db}, nil
}

func (s *wafEventStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *wafEventStore) BuildLogsStats(logPath string, rangeHours int, now time.Time) (logsStatsResp, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now = now.UTC()
	seriesStart, seriesEnd := statsHourlyRange(now, rangeHours)
	emptySeries := buildHourlySeries(seriesStart, seriesEnd, map[int64]int{})
	base := logsStatsResp{
		GeneratedAt:  now.Format(time.RFC3339Nano),
		ScannedLines: 0,
		RangeHours:   rangeHours,
		WAFBlock: wafBlockStats{
			TopRuleIDs24h:   []statsBucket{},
			TopPaths24h:     []statsBucket{},
			TopCountries24h: []statsBucket{},
			SeriesHourly:    emptySeries,
		},
	}

	syncResult, err := s.syncWAFEvents(logPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return base, nil
		}
		return logsStatsResp{}, err
	}
	base.ScannedLines = syncResult.ScannedLines

	since1hUnix := now.Add(-1 * time.Hour).Unix()
	since24hUnix := now.Add(-24 * time.Hour).Unix()
	seriesStartUnix := seriesStart.Unix()
	seriesEndUnix := seriesEnd.Unix()

	base.WAFBlock.TotalInScan, err = s.queryCount(`SELECT COUNT(*) FROM waf_events WHERE event = 'waf_block'`)
	if err != nil {
		return logsStatsResp{}, err
	}
	base.WAFBlock.Last1h, err = s.queryCount(`SELECT COUNT(*) FROM waf_events WHERE event = 'waf_block' AND ts_unix >= ?`, since1hUnix)
	if err != nil {
		return logsStatsResp{}, err
	}
	base.WAFBlock.Last24h, err = s.queryCount(`SELECT COUNT(*) FROM waf_events WHERE event = 'waf_block' AND ts_unix >= ?`, since24hUnix)
	if err != nil {
		return logsStatsResp{}, err
	}

	base.WAFBlock.TopRuleIDs24h, err = s.queryTopBuckets("rule_id", since24hUnix, statsTopN)
	if err != nil {
		return logsStatsResp{}, err
	}
	base.WAFBlock.TopPaths24h, err = s.queryTopBuckets("path", since24hUnix, statsTopN)
	if err != nil {
		return logsStatsResp{}, err
	}
	base.WAFBlock.TopCountries24h, err = s.queryTopBuckets("country", since24hUnix, statsTopN)
	if err != nil {
		return logsStatsResp{}, err
	}
	seriesCounts, err := s.querySeriesCounts(seriesStartUnix, seriesEndUnix)
	if err != nil {
		return logsStatsResp{}, err
	}
	base.WAFBlock.SeriesHourly = buildHourlySeries(seriesStart, seriesEnd, seriesCounts)

	oldest, newest, err := s.queryMinMaxTS()
	if err != nil {
		return logsStatsResp{}, err
	}
	if oldest != 0 && newest != 0 {
		base.OldestScannedTS = time.Unix(oldest, 0).UTC().Format(time.RFC3339Nano)
		base.NewestScannedTS = time.Unix(newest, 0).UTC().Format(time.RFC3339Nano)
	}

	return base, nil
}

func (s *wafEventStore) syncWAFEvents(logPath string) (logSyncResult, error) {
	fi, err := os.Stat(logPath)
	if err != nil {
		return logSyncResult{}, err
	}

	state, err := s.loadIngestState(logStatsStoreSourceWAF)
	if err != nil {
		return logSyncResult{}, err
	}

	offset := state.Offset
	if offset < 0 || offset > fi.Size() {
		offset = 0
	}
	currentMod := fi.ModTime().UTC().UnixNano()
	if fi.Size() < offset || (state.ModTimeNS != 0 && state.ModTimeNS != currentMod && fi.Size() <= offset) {
		offset = 0
	}

	f, err := os.Open(logPath)
	if err != nil {
		return logSyncResult{}, err
	}
	defer f.Close()

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return logSyncResult{}, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return logSyncResult{}, err
	}

	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO waf_events (
		event, ts_unix, ts, rule_id, path, country, status, req_id, line_hash
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return logSyncResult{}, err
	}
	defer stmt.Close()

	reader := bufio.NewReaderSize(f, 64*1024)
	currentOffset := offset
	scannedLines := 0

	for {
		line, readErr := reader.ReadBytes('\n')
		if len(line) > 0 {
			scannedLines++
			currentOffset += int64(len(line))
			if err := ingestWAFEventLine(stmt, line); err != nil {
				_ = tx.Rollback()
				return logSyncResult{}, err
			}
		}

		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			_ = tx.Rollback()
			return logSyncResult{}, readErr
		}
	}

	finalSize := fi.Size()
	finalMod := currentMod
	if finalInfo, statErr := os.Stat(logPath); statErr == nil {
		finalSize = finalInfo.Size()
		finalMod = finalInfo.ModTime().UTC().UnixNano()
	}
	if currentOffset > finalSize {
		finalSize = currentOffset
	}

	nextState := logIngestState{
		Offset:    currentOffset,
		Size:      finalSize,
		ModTimeNS: finalMod,
	}
	if err := saveIngestState(tx, logStatsStoreSourceWAF, nextState); err != nil {
		_ = tx.Rollback()
		return logSyncResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return logSyncResult{}, err
	}

	return logSyncResult{ScannedLines: scannedLines}, nil
}

func ingestWAFEventLine(stmt *sql.Stmt, rawLine []byte) error {
	line := bytes.TrimSpace(rawLine)
	if len(line) == 0 {
		return nil
	}

	var m map[string]any
	if err := json.Unmarshal(line, &m); err != nil {
		return nil
	}

	if strings.TrimSpace(logFieldString(m["event"])) != "waf_block" {
		return nil
	}

	ts, ok := parseLogTS(m["ts"])
	if !ok {
		return nil
	}
	ts = ts.UTC()

	ruleID := normalizeStatsRuleID(m["rule_id"])
	pathKey := normalizeStatsPath(m["path"])
	country := normalizeCountryFromAny(m["country"])
	status := anyToInt(m["status"])
	reqID := strings.TrimSpace(anyToString(m["req_id"]))

	hash := sha256.Sum256(line)
	lineHash := hex.EncodeToString(hash[:])

	_, err := stmt.Exec(
		"waf_block",
		ts.Unix(),
		ts.Format(time.RFC3339Nano),
		ruleID,
		pathKey,
		country,
		status,
		reqID,
		lineHash,
	)
	return err
}

func (s *wafEventStore) loadIngestState(source string) (logIngestState, error) {
	var st logIngestState
	row := s.db.QueryRow(`SELECT offset, size, mod_time_ns FROM ingest_state WHERE source = ?`, source)
	switch err := row.Scan(&st.Offset, &st.Size, &st.ModTimeNS); {
	case errors.Is(err, sql.ErrNoRows):
		return logIngestState{}, nil
	case err != nil:
		return logIngestState{}, err
	default:
		return st, nil
	}
}

func saveIngestState(tx *sql.Tx, source string, st logIngestState) error {
	_, err := tx.Exec(
		`INSERT INTO ingest_state (source, offset, size, mod_time_ns)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(source) DO UPDATE SET
			offset = excluded.offset,
			size = excluded.size,
			mod_time_ns = excluded.mod_time_ns`,
		source,
		st.Offset,
		st.Size,
		st.ModTimeNS,
	)
	return err
}

func (s *wafEventStore) queryCount(query string, args ...any) (int, error) {
	var n int
	if err := s.db.QueryRow(query, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (s *wafEventStore) queryTopBuckets(column string, sinceUnix int64, n int) ([]statsBucket, error) {
	if n <= 0 {
		return []statsBucket{}, nil
	}

	switch column {
	case "rule_id", "path", "country":
	default:
		return nil, fmt.Errorf("invalid bucket column: %s", column)
	}

	q := fmt.Sprintf(
		`SELECT %s AS key, COUNT(*) AS cnt
		   FROM waf_events
		  WHERE event = 'waf_block' AND ts_unix >= ?
		  GROUP BY %s
		  ORDER BY cnt DESC, key ASC
		  LIMIT ?`,
		column,
		column,
	)
	rows, err := s.db.Query(q, sinceUnix, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]statsBucket, 0, n)
	for rows.Next() {
		var b statsBucket
		if err := rows.Scan(&b.Key, &b.Count); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *wafEventStore) querySeriesCounts(startUnix, endUnix int64) (map[int64]int, error) {
	rows, err := s.db.Query(
		`SELECT (ts_unix / 3600) * 3600 AS bucket, COUNT(*) AS cnt
		   FROM waf_events
		  WHERE event = 'waf_block' AND ts_unix >= ? AND ts_unix < ?
		  GROUP BY bucket`,
		startUnix,
		endUnix,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[int64]int{}
	for rows.Next() {
		var bucket int64
		var count int
		if err := rows.Scan(&bucket, &count); err != nil {
			return nil, err
		}
		out[bucket] = count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *wafEventStore) queryMinMaxTS() (int64, int64, error) {
	var minTS sql.NullInt64
	var maxTS sql.NullInt64
	if err := s.db.QueryRow(
		`SELECT MIN(ts_unix), MAX(ts_unix) FROM waf_events WHERE event = 'waf_block'`,
	).Scan(&minTS, &maxTS); err != nil {
		return 0, 0, err
	}
	if !minTS.Valid || !maxTS.Valid {
		return 0, 0, nil
	}
	return minTS.Int64, maxTS.Int64, nil
}
