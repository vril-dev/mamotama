package bypassconf

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
)

var (
	confPath string
	mu       sync.RWMutex
	entries  []Entry
	watcher  *fsnotify.Watcher
)

func Init(path string) error {
	confPath = path
	if err := reload(); err != nil {
		return err
	}

	return startWatch()
}

func GetPath() string { return confPath }

func Get() []Entry {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Entry, len(entries))
	copy(out, entries)

	return out
}

func Match(reqPath string) MatchResult {
	p := normalize(reqPath)
	mu.RLock()
	defer mu.RUnlock()
	bypassHit := false
	for _, e := range entries {
		matched := false
		if eqLoosely(p, normalize(e.Path)) {
			matched = true
		} else if strings.HasSuffix(e.Path, "/") {
			pp := normalize(e.Path)
			if strings.HasPrefix(p, pp) {
				matched = true
			}
		}
		if !matched {
			continue
		}
		if e.ExtraRule != "" {
			return MatchResult{Action: ACTION_RULE, ExtraRule: e.ExtraRule}
		}
		bypassHit = true
	}
	if bypassHit {
		return MatchResult{Action: ACTION_BYPASS}
	}

	return MatchResult{Action: ACTION_NONE}
}

func reload() error {
	b, err := os.ReadFile(confPath)
	if err != nil {
		return err
	}

	es, err := Parse(string(b))
	if err != nil {
		return err
	}

	mu.Lock()
	entries = es
	mu.Unlock()
	log.Printf("[BYPASS][RELOAD] path=%s entries=%d", confPath, len(es))

	return nil
}

func Parse(s string) ([]Entry, error) {
	sc := bufio.NewScanner(strings.NewReader(s))
	var out []Entry
	lineNo := 0

	for sc.Scan() {
		lineNo++
		line := sc.Text()
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		if len(parts) > 2 {
			return nil, fmt.Errorf("line %d: expected '<path>' or '<path> <rule.conf>'", lineNo)
		}
		if !strings.HasPrefix(parts[0], "/") {
			return nil, fmt.Errorf("line %d: path must start with '/'", lineNo)
		}

		e := Entry{Path: normalize(parts[0])}
		if len(parts) == 2 {
			rule := strings.TrimSpace(parts[1])
			if !strings.HasSuffix(strings.ToLower(rule), ".conf") {
				return nil, fmt.Errorf("line %d: extra rule must be .conf file", lineNo)
			}
			e.ExtraRule = rule
		}

		out = append(out, e)
	}

	return out, sc.Err()
}

func startWatch() error {
	if watcher != nil {
		return nil
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	watcher = w

	dir := filepath.Dir(confPath)
	if err := watcher.Add(dir); err != nil {
		return err
	}

	go func() {
		for {
			select {
			case ev, ok := <-watcher.Events:
				if !ok {
					return
				}

				if filepath.Clean(ev.Name) == filepath.Clean(confPath) ||
					filepath.Base(ev.Name) == filepath.Base(confPath) {
					_ = reload()
				}
			case err := <-watcher.Errors:
				log.Printf("[BYPASS][WATCH][ERR] %v", err)
			}
		}
	}()

	return nil
}

func Reload() error { return reload() }
