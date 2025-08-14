package bypassconf

import (
	"bufio"
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
	for _, e := range entries {
		if eqLoosely(p, normalize(e.Path)) {
			if e.ExtraRule != "" {
				return MatchResult{Action: ACTION_RULE, ExtraRule: e.ExtraRule}
			}

			return MatchResult{Action: ACTION_BYPASS}
		}

		if strings.HasSuffix(e.Path, "/") {
			pp := normalize(e.Path)
			if strings.HasPrefix(p, pp) {
				if e.ExtraRule != "" {
					return MatchResult{Action: ACTION_RULE, ExtraRule: e.ExtraRule}
				}

				return MatchResult{Action: ACTION_BYPASS}
			}
		}
	}

	return MatchResult{Action: ACTION_NONE}
}

func reload() error {
	b, err := os.ReadFile(confPath)
	if err != nil {
		return err
	}

	es, err := parse(string(b))
	if err != nil {
		return err
	}

	mu.Lock()
	entries = es
	mu.Unlock()
	log.Printf("[BYPASS][RELOAD] path=%s entries=%d", confPath, len(es))

	return nil
}

func parse(s string) ([]Entry, error) {
	sc := bufio.NewScanner(strings.NewReader(s))
	var out []Entry

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		e := Entry{Path: parts[0]}
		if len(parts) >= 2 {
			for _, p := range parts[1:] {
				if strings.HasPrefix(p, "rules/") || strings.HasSuffix(p, ".conf") {
					e.ExtraRule = p
					break
				}
			}
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
