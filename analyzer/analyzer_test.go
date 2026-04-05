package analyzer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stephen/aha/model"
)

func TestAnalyzeIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	// pyproject.toml
	os.WriteFile(filepath.Join(tmpDir, "pyproject.toml"), []byte(`
[project]
name = "testlib"
version = "0.1.0"
description = "A test library"
requires-python = ">=3.8"
dependencies = ["requests>=2.28"]
`), 0o644)

	pkgDir := filepath.Join(tmpDir, "testlib")
	os.MkdirAll(pkgDir, 0o755)

	os.WriteFile(filepath.Join(pkgDir, "__init__.py"), []byte(`
__all__ = ["MyClass", "helper"]
`), 0o644)

	os.WriteFile(filepath.Join(pkgDir, "core.py"), []byte(`
import os
from typing import Optional
import requests

class MyClass:
    """A test class."""
    def method(self, x: int) -> str:
        return str(x)

def helper(x: int) -> bool:
    return x > 0

VERSION = "0.1.0"
`), 0o644)

	progressCh := make(chan model.AnalysisProgress, 100)
	result, err := Analyze(tmpDir, progressCh)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if result.ProjectName != "testlib" {
		t.Errorf("project name: got %q, want testlib", result.ProjectName)
	}
	if result.Stats.TotalFiles < 2 {
		t.Errorf("total files: got %d, want >= 2", result.Stats.TotalFiles)
	}
	if len(result.ExternalDeps) == 0 {
		t.Error("expected at least one external dep")
	}

	// Check progress was emitted
	close(progressCh)
	count := 0
	for range progressCh {
		count++
	}
	if count == 0 {
		t.Error("expected progress messages")
	}
}
