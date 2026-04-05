package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stephen/aha/model"
)

func TestCacheRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	c := Store{Dir: tmpDir}

	analysis := &model.ProjectAnalysis{
		ProjectPath: "/tmp/testproject",
		ProjectName: "testproject",
		Stats:       model.ProjectStats{TotalFiles: 5, TotalLOC: 500},
	}

	if err := c.Save("/tmp/testproject", analysis, time.Now()); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := c.Load("/tmp/testproject", time.Now().Add(-time.Second))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.ProjectName != "testproject" {
		t.Errorf("got %q, want testproject", loaded.ProjectName)
	}
	if loaded.Stats.TotalFiles != 5 {
		t.Errorf("got %d files, want 5", loaded.Stats.TotalFiles)
	}
}

func TestCacheInvalidation(t *testing.T) {
	tmpDir := t.TempDir()
	c := Store{Dir: tmpDir}

	analysis := &model.ProjectAnalysis{
		ProjectPath: "/tmp/testproject",
		ProjectName: "testproject",
	}

	cacheTime := time.Now().Add(-time.Hour)
	if err := c.Save("/tmp/testproject", analysis, cacheTime); err != nil {
		t.Fatalf("save: %v", err)
	}

	// newestMod is after cache time -> stale
	loaded, err := c.Load("/tmp/testproject", time.Now())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded != nil {
		t.Error("expected nil (stale cache), got result")
	}
}

func TestCacheKeyDeterministic(t *testing.T) {
	k1 := cacheKey("/tmp/project")
	k2 := cacheKey("/tmp/project")
	if k1 != k2 {
		t.Errorf("cache keys differ: %s vs %s", k1, k2)
	}
}

func TestNewestModTime(t *testing.T) {
	tmpDir := t.TempDir()
	sub := filepath.Join(tmpDir, "pkg")
	os.MkdirAll(sub, 0o755)

	f1 := filepath.Join(tmpDir, "a.py")
	f2 := filepath.Join(sub, "b.py")
	os.WriteFile(f1, []byte("pass"), 0o644)
	os.WriteFile(f2, []byte("pass"), 0o644)

	now := time.Now()
	os.Chtimes(f1, now.Add(-time.Hour), now.Add(-time.Hour))
	os.Chtimes(f2, now, now)

	newest := NewestPyModTime(tmpDir)
	if newest.Before(now.Add(-time.Second)) {
		t.Errorf("newest mod time too old: %v", newest)
	}
}
