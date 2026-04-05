package cache

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/stephen/aha/model"
)

type cachedAnalysis struct {
	CachedAt time.Time              `json:"cached_at"`
	Analysis *model.ProjectAnalysis `json:"analysis"`
}

// Store manages cached analysis results.
type Store struct {
	Dir string // defaults to ~/.cache/aha
}

// DefaultDir returns the default cache directory.
func DefaultDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "aha")
}

func cacheKey(projectPath string) string {
	h := sha256.Sum256([]byte(projectPath))
	return fmt.Sprintf("%x", h[:16])
}

func (s Store) path(projectPath string) string {
	return filepath.Join(s.Dir, cacheKey(projectPath)+".json")
}

// Save writes analysis to cache.
func (s Store) Save(projectPath string, analysis *model.ProjectAnalysis, cachedAt time.Time) error {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(cachedAnalysis{CachedAt: cachedAt, Analysis: analysis})
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(projectPath), data, 0o644)
}

// Load returns cached analysis if it exists and is not stale.
// newestMod is the modification time of the newest .py file in the project.
// Returns nil, nil if cache is stale or missing.
func (s Store) Load(projectPath string, newestMod time.Time) (*model.ProjectAnalysis, error) {
	data, err := os.ReadFile(s.path(projectPath))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cached cachedAnalysis
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, nil // corrupt cache, treat as miss
	}
	if newestMod.After(cached.CachedAt) {
		return nil, nil // stale
	}
	return cached.Analysis, nil
}

// NewestPyModTime walks a project directory and returns the newest .py file mod time.
func NewestPyModTime(projectDir string) time.Time {
	skipDirs := map[string]bool{
		".git": true, "__pycache__": true, ".tox": true, "node_modules": true,
		".venv": true, "venv": true, ".eggs": true, "build": true, "dist": true,
		".mypy_cache": true, ".pytest_cache": true,
	}
	var newest time.Time
	filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if skipDirs[filepath.Base(path)] {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".py") && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
		return nil
	})
	return newest
}
