package handler

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	logDirCoraza    = "logs/coraza"
	logDirOpenresty = "logs/openresty"

	logFiles = map[string]string{
		"waf":    filepath.Join(logDirCoraza, "waf-events.ndjson"),
		"accerr": filepath.Join(logDirOpenresty, "access-error.ndjson"),
		"intr":   filepath.Join(logDirOpenresty, "interesting.ndjson"),
	}

	readChunkSize   = int64(64 * 1024)
	maxLinesPerRead = 200
	maxBytesPerRead = int64(512 * 1024)
)

type logLine map[string]any

type lineIndex struct {
	Offsets []int64
	Size    int64
	ModTime time.Time
}

var (
	idxMu  sync.RWMutex
	fileIx = map[string]*lineIndex{}
)

type readResp struct {
	Lines      []logLine `json:"lines"`
	NextCursor *int64    `json:"next_cursor,omitempty"`
	HasMore    bool      `json:"has_more"`
	HasPrev    bool      `json:"has_prev"`
	HasNext    bool      `json:"has_next"`
}

func LogsRead(c *gin.Context) {
	src := c.Query("src")
	path, ok := logFiles[src]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid src"})
		return
	}

	tail := clampInt(mustAtoiDefault(c.Query("tail"), 30), 1, maxLinesPerRead)
	dir := c.DefaultQuery("dir", "")
	var cursor *int64
	if v := c.Query("cursor"); v != "" {
		off := mustAtoi64Default(v, 0)
		cursor = &off
	}

	lines, nextCur, hasPrev, hasNext, err := readByLine(path, tail, cursor, dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusOK, readResp{Lines: nil, NextCursor: nil, HasMore: false})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := readResp{
		Lines:      lines,
		NextCursor: nextCur,
		HasPrev:    hasPrev,
		HasNext:    hasNext,
	}

	if dir == "prev" {
		resp.HasMore = hasPrev
	} else {
		resp.HasMore = hasNext
	}

	c.JSON(http.StatusOK, resp)
}

func LogsDownload(c *gin.Context) {
	src := c.Query("src")
	path, ok := logFiles[src]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid src"})
		return
	}

	fromStr := c.Query("from")
	toStr := c.Query("to")
	var (
		from time.Time
		to   time.Time
		err  error
	)

	if fromStr != "" {
		from, err = time.Parse(time.RFC3339, fromStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid from"})
			return
		}
	}

	if toStr != "" {
		to, err = time.Parse(time.RFC3339, toStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to"})
			return
		}
	}

	if toStr == "" {
		to = time.Now().Add(1 * time.Second)
	}

	c.Header("Content-Type", "application/x-ndjson")
	filename := fmt.Sprintf("%s-%s.ndjson.gz", src, time.Now().Format("20060102"))
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Header("Content-Encoding", "gzip")

	f, err := os.Open(path)
	if err != nil {
		gw := gzip.NewWriter(c.Writer)
		_ = gw.Close()
		return
	}
	defer f.Close()

	gw := gzip.NewWriter(c.Writer)
	defer gw.Close()

	br := bufio.NewReaderSize(f, 64*1024)
	for {
		b, err := br.ReadBytes('\n')
		if len(b) > 0 {
			var m map[string]any
			if json.Unmarshal(b, &m) == nil {
				if ts, ok := m["ts"].(string); ok && tsInRange(ts, from, to) {
					if _, err := gw.Write(b); err != nil {
						break
					}
				}
			}
		}

		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			break
		}
	}
}

func buildOrUpdateIndex(path string) (*lineIndex, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	idxMu.Lock()
	defer idxMu.Unlock()

	li := fileIx[path]
	if li == nil || fi.Size() < li.Size || fi.ModTime().After(li.ModTime) {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		br := bufio.NewReaderSize(f, 128*1024)
		var offs []int64
		offs = append(offs, 0)
		var pos int64
		for {
			b, err := br.ReadBytes('\n')
			if len(b) > 0 {
				pos += int64(len(b))
				offs = append(offs, pos)
			}

			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}

				return nil, err
			}
		}

		li = &lineIndex{Offsets: offs, Size: fi.Size(), ModTime: fi.ModTime()}
		fileIx[path] = li

		return li, nil
	}

	if fi.Size() > li.Size {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		if _, err := f.Seek(li.Size, io.SeekStart); err != nil {
			return nil, err
		}

		br := bufio.NewReaderSize(f, 128*1024)
		pos := li.Size
		for {
			b, err := br.ReadBytes('\n')
			if len(b) > 0 {
				pos += int64(len(b))
				li.Offsets = append(li.Offsets, pos)
			}

			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}

				return nil, err
			}
		}

		li.Size = fi.Size()
		li.ModTime = fi.ModTime()
	}

	return li, nil
}

func readByLine(path string, tail int, cursor *int64, dir string) ([]logLine, *int64, bool, bool, error) {
	li, err := buildOrUpdateIndex(path)
	if err != nil {
		return nil, nil, false, false, err
	}

	totalMarks := len(li.Offsets)
	if totalMarks == 0 {
		z := int64(0)
		return nil, &z, false, false, nil
	}
	totalLines := totalMarks - 1

	var cur int
	if cursor == nil {
		if tail > totalLines {
			cur = 0
		} else {
			cur = totalLines - tail
		}
	} else {
		cur = int(*cursor)
		if cur < 0 {
			cur = 0
		}
		if cur > totalLines {
			cur = totalLines
		}
	}

	var start, end int
	switch dir {
	case "prev":
		start, end = maxInt(cur-tail, 0), cur
	case "next", "":
		start, end = cur, minInt(cur+tail, totalLines)
	default:
		return nil, nil, false, false, fmt.Errorf("invalid dir")
	}

	if start >= end {
		nextCur := int64(end)
		return []logLine{}, &nextCur, start > 0, end < totalLines, nil
	}

	byteStart := li.Offsets[start]
	byteEnd := li.Offsets[end]
	size := byteEnd - byteStart

	f, err := os.Open(path)
	if err != nil {
		return nil, nil, false, false, err
	}
	defer f.Close()

	if _, err := f.Seek(byteStart, io.SeekStart); err != nil {
		return nil, nil, false, false, err
	}

	buf := make([]byte, size)
	if _, err := io.ReadFull(f, buf); err != nil {
		return nil, nil, false, false, err
	}

	br := bufio.NewReaderSize(bytes.NewReader(buf), 64*1024)
	out := make([]logLine, 0, tail)
	for {
		b, err := br.ReadBytes('\n')
		if len(b) > 0 {
			var m map[string]any
			if json.Unmarshal(trimLastNewline(b), &m) == nil {
				out = append(out, m)
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, nil, false, false, err
		}
	}

	var nextCur int64
	if dir == "prev" {
		nextCur = int64(start)
	} else {
		nextCur = int64(end)
	}
	hasPrev := start > 0
	hasNext := end < totalLines
	return out, &nextCur, hasPrev, hasNext, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}

	if v > hi {
		return hi
	}

	return v
}

func clamp64(v, lo, hi int64) int64 {
	if v < lo {
		return lo
	}

	if v > hi {
		return hi
	}

	return v
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}

	return b
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}

	return b
}

func mustAtoiDefault(s string, d int) int {
	if s == "" {
		return d
	}

	i, _ := strconv.Atoi(s)
	if i == 0 {
		return d
	}

	return i
}

func mustAtoi64Default(s string, d int64) int64 {
	if s == "" {
		return d
	}

	i, _ := strconv.ParseInt(s, 10, 64)
	if i == 0 {
		return d
	}

	return i
}

func tsInRange(ts string, from, to time.Time) bool {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return false
	}

	if !from.IsZero() && t.Before(from) {
		return false
	}

	if !to.IsZero() && !t.Before(to) {
		return false
	}

	return true
}

func bytesSplitKeep(b []byte, sep byte) [][]byte {
	var out [][]byte
	start := 0
	for i, c := range b {
		if c == sep {
			out = append(out, b[start:i+1])
			start = i + 1
		}
	}

	if start < len(b) {
		out = append(out, b[start:])
	}

	return out
}

func trimLastNewline(b []byte) []byte {
	if len(b) > 0 && b[len(b)-1] == '\n' {
		return b[:len(b)-1]
	}

	return b
}
