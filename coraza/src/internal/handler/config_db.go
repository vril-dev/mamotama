package handler

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mamotama/internal/config"
)

const (
	dbConfigKeyBypassRaw       = "bypass.raw"
	dbConfigKeyCountryBlockRaw = "country_block.raw"
	dbConfigKeyRateLimitRaw    = "rate_limit.raw"
	dbConfigKeyBotDefenseRaw   = "bot_defense.raw"
	dbConfigKeySemanticRaw     = "semantic.raw"
)

type dbConfigMirrorSpec struct {
	key     string
	path    string
	ensure  func(string) error
	context string
}

func InitDBConfigMirrors() error {
	store := getLogsStatsStore()
	if store == nil {
		return nil
	}

	specs := []dbConfigMirrorSpec{
		{
			key:     dbConfigKeyBypassRaw,
			path:    strings.TrimSpace(config.BypassFile),
			ensure:  func(path string) error { return ensureTextFile(path, []byte("# <path> [extra-rule.conf]\n")) },
			context: "bypass",
		},
		{
			key:     dbConfigKeyCountryBlockRaw,
			path:    strings.TrimSpace(config.CountryBlockFile),
			ensure:  ensureCountryBlockFile,
			context: "country block",
		},
		{
			key:     dbConfigKeyRateLimitRaw,
			path:    strings.TrimSpace(config.RateLimitFile),
			ensure:  ensureRateLimitFile,
			context: "rate limit",
		},
		{
			key:     dbConfigKeyBotDefenseRaw,
			path:    strings.TrimSpace(config.BotDefenseFile),
			ensure:  ensureBotDefenseFile,
			context: "bot defense",
		},
		{
			key:     dbConfigKeySemanticRaw,
			path:    strings.TrimSpace(config.SemanticFile),
			ensure:  ensureSemanticFile,
			context: "semantic",
		},
	}

	for _, spec := range specs {
		if spec.path == "" {
			continue
		}
		if err := spec.ensure(spec.path); err != nil {
			return fmt.Errorf("ensure %s file: %w", spec.context, err)
		}
		if err := syncConfigBlobWithFile(store, spec.key, spec.path); err != nil {
			return fmt.Errorf("sync %s config blob: %w", spec.context, err)
		}
	}

	return nil
}

func readConfigBlobOrFile(key, path string) ([]byte, error) {
	store := getLogsStatsStore()
	if store == nil {
		raw, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			return []byte{}, nil
		}
		return raw, err
	}

	raw, ok, err := store.GetConfigBlob(key)
	if err != nil {
		return nil, err
	}
	if ok {
		return []byte(raw), nil
	}

	fileRaw, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		fileRaw = []byte{}
	}
	if err := store.PutConfigBlob(key, string(fileRaw), time.Now().UTC()); err != nil {
		return nil, err
	}
	return fileRaw, nil
}

func putConfigBlobIfEnabled(key, raw string) error {
	store := getLogsStatsStore()
	if store == nil {
		return nil
	}
	return store.PutConfigBlob(key, raw, time.Now().UTC())
}

func rollbackConfigBlobIfEnabled(key, raw string) {
	store := getLogsStatsStore()
	if store == nil {
		return
	}
	_ = store.PutConfigBlob(key, raw, time.Now().UTC())
}

func syncConfigBlobWithFile(store *wafEventStore, key, path string) error {
	raw, ok, err := store.GetConfigBlob(key)
	if err != nil {
		return err
	}
	if ok {
		return os.WriteFile(path, []byte(raw), 0o644)
	}

	fileRaw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return store.PutConfigBlob(key, string(fileRaw), time.Now().UTC())
}

func ensureTextFile(path string, defaultContent []byte) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, defaultContent, 0o644)
}
