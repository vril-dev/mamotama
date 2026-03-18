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
)

func InitDBConfigMirrors() error {
	store := getLogsStatsStore()
	if store == nil {
		return nil
	}

	if path := strings.TrimSpace(config.BypassFile); path != "" {
		if err := ensureTextFile(path, []byte("# <path> [extra-rule.conf]\n")); err != nil {
			return fmt.Errorf("ensure bypass file: %w", err)
		}
		if err := syncConfigBlobWithFile(store, dbConfigKeyBypassRaw, path); err != nil {
			return fmt.Errorf("sync bypass config blob: %w", err)
		}
	}

	if path := strings.TrimSpace(config.CountryBlockFile); path != "" {
		if err := ensureCountryBlockFile(path); err != nil {
			return fmt.Errorf("ensure country block file: %w", err)
		}
		if err := syncConfigBlobWithFile(store, dbConfigKeyCountryBlockRaw, path); err != nil {
			return fmt.Errorf("sync country block config blob: %w", err)
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
