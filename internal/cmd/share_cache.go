package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agynio/outline-cli/internal/config"
)

const (
	shareCacheFile = "cache.json"
	shareCacheTTL  = 30 * 24 * time.Hour
)

type shareCache struct {
	Bases map[string]shareCacheBase `json:"bases"`
}

type shareCacheBase struct {
	Shares map[string]shareCacheEntry `json:"shares"`
}

type shareCacheEntry struct {
	DocumentID string `json:"documentId"`
	CreatedAt  string `json:"createdAt"`
}

func loadShareCache() (shareCache, error) {
	path, err := shareCachePath()
	if err != nil {
		return shareCache{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return newShareCache(), nil
		}
		return shareCache{}, fmt.Errorf("read cache: %w", err)
	}
	if len(data) == 0 {
		return newShareCache(), nil
	}
	var cache shareCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return shareCache{}, fmt.Errorf("parse cache: %w", err)
	}
	if cache.Bases == nil {
		cache.Bases = map[string]shareCacheBase{}
	}
	return cache, nil
}

func saveShareCache(cache shareCache) error {
	path, err := shareCachePath()
	if err != nil {
		return err
	}
	if cache.Bases == nil {
		cache.Bases = map[string]shareCacheBase{}
	}
	payload, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cache: %w", err)
	}
	payload = append(payload, '\n')
	return atomicWriteFile(path, payload, 0600)
}

func cacheShareDocument(baseURL string, shareID string, documentID string) error {
	baseURL = strings.TrimSpace(baseURL)
	shareID = strings.TrimSpace(shareID)
	documentID = strings.TrimSpace(documentID)
	if baseURL == "" || shareID == "" || shareID == "<nil>" || documentID == "" || documentID == "<nil>" {
		return nil
	}
	cache, err := loadShareCache()
	if err != nil {
		return err
	}
	base := cache.Bases[baseURL]
	if base.Shares == nil {
		base.Shares = map[string]shareCacheEntry{}
	}
	base.Shares[shareID] = shareCacheEntry{DocumentID: documentID, CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	cache.Bases[baseURL] = base
	return saveShareCache(cache)
}

func lookupCachedShareDocument(baseURL string, shareID string) (string, bool, error) {
	baseURL = strings.TrimSpace(baseURL)
	shareID = strings.TrimSpace(shareID)
	if baseURL == "" || shareID == "" || shareID == "<nil>" {
		return "", false, nil
	}
	cache, err := loadShareCache()
	if err != nil {
		return "", false, err
	}
	base, ok := cache.Bases[baseURL]
	if !ok || base.Shares == nil {
		return "", false, nil
	}
	entry, ok := base.Shares[shareID]
	if !ok || strings.TrimSpace(entry.DocumentID) == "" {
		return "", false, nil
	}
	if shareCacheEntryExpired(entry, time.Now().UTC()) {
		delete(base.Shares, shareID)
		cache.Bases[baseURL] = base
		if err := saveShareCache(cache); err != nil {
			return "", false, err
		}
		return "", false, nil
	}
	return strings.TrimSpace(entry.DocumentID), true, nil
}

func shareCacheEntryExpired(entry shareCacheEntry, now time.Time) bool {
	createdAt, err := time.Parse(time.RFC3339, strings.TrimSpace(entry.CreatedAt))
	if err != nil {
		return true
	}
	return now.Sub(createdAt) > shareCacheTTL
}

func clearShareCache() error {
	path, err := shareCachePath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clear cache: %w", err)
	}
	return nil
}

func shareCachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, config.ConfigDir, shareCacheFile), nil
}

func newShareCache() shareCache {
	return shareCache{Bases: map[string]shareCacheBase{}}
}

func atomicWriteFile(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp cache: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp cache: %w", err)
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp cache: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp cache: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace cache: %w", err)
	}
	return nil
}
