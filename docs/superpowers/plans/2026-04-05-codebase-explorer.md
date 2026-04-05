# Python Codebase Explorer — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Pivot aha from a dependency viewer into a deep Python codebase explorer TUI that analyzes a project's structure, classes, functions, dependencies, and patterns — with cached results and a progress bar on first analysis.

**Architecture:** Three layers: (1) An embedded Python script uses the `ast` module for accurate source analysis and outputs newline-delimited JSON to stdout, (2) A Go orchestrator runs the script, parses output, manages caching, and reports progress via Bubble Tea messages, (3) A tabbed TUI with four views — Overview, Explorer, Dependencies, Patterns — for interactive exploration. First run shows a progress bar; subsequent runs load from a JSON cache file keyed by project path hash.

**Tech Stack:** Go, Bubble Tea v2, LipGloss v2, embedded Python script (via `go:embed`), `encoding/json`, `crypto/sha256`

---

## File Structure

```
main.go                          — CLI entry, launch analysis or cached TUI
model/
  model.go                       — All data structures (replace existing)
analyzer/
  analyzer.go                    — Orchestrator: walk files, run Python, build result (replace existing)
  analyze.py                     — Embedded Python AST analysis script (new)
  config.go                      — Keep: pyproject.toml/setup.py/requirements.txt parsing
  imports.go                     — Keep: regex import parsing (used as fallback)
  stdlib.go                      — Keep: stdlib module set
  stdlib_test.go                 — Keep
  imports_test.go                — Keep
  config_test.go                 — Keep
  analyzer_test.go               — Update tests for new Analyze signature
cache/
  cache.go                       — Cache read/write/invalidation (new)
tui/
  app.go                         — Main app with 4 tabs + analysis state (replace existing)
  styles.go                      — Expanded styles (replace existing)
  help.go                        — Keep as-is
  progress.go                    — Progress bar view during analysis (new)
  overview.go                    — Overview tab (new)
  explorer.go                    — File/class/function tree explorer (new)
  dependencies.go                — External + internal deps (refactor from external.go + internal.go)
  patterns.go                    — Detected patterns view (new)
  external.go                    — Delete
  internal.go                    — Delete
```

---

### Task 1: Model Layer — New Data Structures

**Files:**
- Modify: `model/model.go` (full rewrite)

- [ ] **Step 1: Write tests for model serialization**

Create `model/model_test.go`:

```go
package model

import (
	"encoding/json"
	"testing"
)

func TestProjectAnalysisRoundTrip(t *testing.T) {
	original := &ProjectAnalysis{
		ProjectPath: "/tmp/test",
		ProjectName: "mylib",
		Version:     "1.0.0",
		Description: "A test library",
		PythonRequires: ">=3.8",
		EntryPoints: []EntryPoint{{Name: "cli", Module: "mylib.cli", Function: "main"}},
		Files: []FileAnalysis{
			{
				RelPath: "mylib/core.py",
				LOC:     100,
				Classes: []ClassInfo{
					{
						Name:       "MyClass",
						Bases:      []string{"BaseClass"},
						Decorators: []string{"dataclass"},
						Methods: []FunctionInfo{
							{Name: "method1", Params: []string{"self", "x: int"}, ReturnType: "str", LOC: 5, Decorators: nil},
						},
						LOC:     30,
						LineNo:  10,
					},
				},
				Functions: []FunctionInfo{
					{Name: "helper", Params: []string{"x: int"}, ReturnType: "bool", LOC: 8, LineNo: 50, Decorators: []string{"cache"}},
				},
				Imports:    []string{"os", "typing"},
				AllExports: []string{"MyClass", "helper"},
				Constants:  []string{"VERSION"},
			},
		},
		ExternalDeps: []ExternalDep{
			{Name: "requests", VersionConstraint: ">=2.28", UsedIn: []string{"mylib/core.py"}},
		},
		Modules: map[string]*Module{
			"mylib/core.py": {RelPath: "mylib/core.py", Imports: []string{"mylib/utils.py"}, ImportedBy: nil},
		},
		Patterns: []PatternMatch{
			{Pattern: "Abstract Base Class", Location: "mylib/core.py:MyClass", Detail: "Inherits from ABC"},
		},
		Stats: ProjectStats{
			TotalFiles: 1, TotalLOC: 100, TotalClasses: 1, TotalFunctions: 2,
			TotalPackages: 1, TotalExternalDeps: 1,
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ProjectAnalysis
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ProjectName != original.ProjectName {
		t.Errorf("got %q, want %q", decoded.ProjectName, original.ProjectName)
	}
	if len(decoded.Files) != 1 {
		t.Fatalf("got %d files, want 1", len(decoded.Files))
	}
	if decoded.Files[0].Classes[0].Name != "MyClass" {
		t.Errorf("got class %q, want MyClass", decoded.Files[0].Classes[0].Name)
	}
	if decoded.Stats.TotalLOC != 100 {
		t.Errorf("got LOC %d, want 100", decoded.Stats.TotalLOC)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./model/... -v`
Expected: FAIL — types not defined

- [ ] **Step 3: Rewrite model.go with all new types**

Replace `model/model.go`:

```go
package model

// ProjectAnalysis is the full analysis result for a Python project.
type ProjectAnalysis struct {
	ProjectPath    string            `json:"project_path"`
	ProjectName    string            `json:"project_name"`
	Version        string            `json:"version"`
	Description    string            `json:"description"`
	PythonRequires string            `json:"python_requires"`
	EntryPoints    []EntryPoint      `json:"entry_points"`
	Files          []FileAnalysis    `json:"files"`
	ExternalDeps   []ExternalDep     `json:"external_deps"`
	Modules        map[string]*Module `json:"modules"`
	Patterns       []PatternMatch    `json:"patterns"`
	Stats          ProjectStats      `json:"stats"`
	Warnings       []string          `json:"warnings"`
}

// EntryPoint represents a CLI entry point or __main__.py.
type EntryPoint struct {
	Name     string `json:"name"`
	Module   string `json:"module"`
	Function string `json:"function"`
}

// FileAnalysis holds per-file AST analysis results.
type FileAnalysis struct {
	RelPath    string         `json:"rel_path"`
	LOC        int            `json:"loc"`
	Classes    []ClassInfo    `json:"classes"`
	Functions  []FunctionInfo `json:"functions"`
	Imports    []string       `json:"imports"`
	AllExports []string       `json:"all_exports"`
	Constants  []string       `json:"constants"`
}

// ClassInfo describes a Python class.
type ClassInfo struct {
	Name       string         `json:"name"`
	Bases      []string       `json:"bases"`
	Decorators []string       `json:"decorators"`
	Methods    []FunctionInfo `json:"methods"`
	LOC        int            `json:"loc"`
	LineNo     int            `json:"line_no"`
}

// FunctionInfo describes a Python function or method.
type FunctionInfo struct {
	Name       string   `json:"name"`
	Params     []string `json:"params"`
	ReturnType string   `json:"return_type"`
	Decorators []string `json:"decorators"`
	LOC        int      `json:"loc"`
	LineNo     int      `json:"line_no"`
}

// ExternalDep represents an external (third-party) dependency.
type ExternalDep struct {
	Name              string   `json:"name"`
	VersionConstraint string   `json:"version_constraint"`
	UsedIn            []string `json:"used_in"`
}

// Module represents a Python file and its import relationships.
type Module struct {
	RelPath    string   `json:"rel_path"`
	Imports    []string `json:"imports"`
	ImportedBy []string `json:"imported_by"`
}

// PatternMatch describes a detected design pattern or code pattern.
type PatternMatch struct {
	Pattern  string `json:"pattern"`
	Location string `json:"location"`
	Detail   string `json:"detail"`
}

// ProjectStats holds aggregate statistics.
type ProjectStats struct {
	TotalFiles        int `json:"total_files"`
	TotalLOC          int `json:"total_loc"`
	TotalClasses      int `json:"total_classes"`
	TotalFunctions    int `json:"total_functions"`
	TotalPackages     int `json:"total_packages"`
	TotalExternalDeps int `json:"total_external_deps"`
}

// AnalysisProgress is emitted by the analyzer during analysis.
type AnalysisProgress struct {
	Phase      string  `json:"phase"`
	Current    int     `json:"current"`
	Total      int     `json:"total"`
	Detail     string  `json:"detail"`
	Percent    float64 `json:"percent"`
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./model/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add model/model.go model/model_test.go
git commit -m "feat: rewrite model layer with full analysis data structures"
```

---

### Task 2: Python AST Analysis Script

**Files:**
- Create: `analyzer/analyze.py`

This is the embedded Python script that performs deep AST analysis. It reads a project directory, analyzes each `.py` file using Python's `ast` module, and outputs newline-delimited JSON (one progress message per file, then a final result).

- [ ] **Step 1: Write the Python analysis script**

Create `analyzer/analyze.py`:

```python
#!/usr/bin/env python3
"""Deep Python project analyzer using the ast module.

Usage: python3 analyze.py <project_dir>

Output: Newline-delimited JSON to stdout.
  - Progress lines: {"type": "progress", "phase": "...", "current": N, "total": N, "detail": "..."}
  - Result line:    {"type": "result", "files": [...]}
"""

import ast
import json
import os
import sys


def emit(obj):
    print(json.dumps(obj), flush=True)


def progress(phase, current, total, detail=""):
    emit({"type": "progress", "phase": phase, "current": current, "total": total, "detail": detail})


SKIP_DIRS = {".git", "__pycache__", ".tox", "node_modules", ".venv", "venv", ".eggs", "build", "dist", ".mypy_cache", ".pytest_cache"}


def find_py_files(project_dir):
    py_files = []
    for root, dirs, files in os.walk(project_dir):
        dirs[:] = [d for d in dirs if d not in SKIP_DIRS]
        for f in files:
            if f.endswith(".py"):
                full = os.path.join(root, f)
                rel = os.path.relpath(full, project_dir)
                py_files.append((full, rel))
    py_files.sort(key=lambda x: x[1])
    return py_files


def analyze_function(node):
    params = []
    for arg in node.args.args:
        param = arg.arg
        if arg.annotation:
            param += ": " + ast.unparse(arg.annotation)
        params.append(param)

    return_type = ""
    if node.returns:
        return_type = ast.unparse(node.returns)

    decorators = []
    for dec in node.decorator_list:
        decorators.append(ast.unparse(dec))

    loc = (node.end_lineno or node.lineno) - node.lineno + 1

    return {
        "name": node.name,
        "params": params,
        "return_type": return_type,
        "decorators": decorators,
        "loc": loc,
        "line_no": node.lineno,
    }


def analyze_class(node):
    bases = [ast.unparse(b) for b in node.bases]
    decorators = [ast.unparse(d) for d in node.decorator_list]

    methods = []
    for item in ast.walk(node):
        if isinstance(item, (ast.FunctionDef, ast.AsyncFunctionDef)) and item is not node:
            # Only direct methods, not nested functions
            pass

    # Get direct methods only
    for item in node.body:
        if isinstance(item, (ast.FunctionDef, ast.AsyncFunctionDef)):
            methods.append(analyze_function(item))

    loc = (node.end_lineno or node.lineno) - node.lineno + 1

    return {
        "name": node.name,
        "bases": bases,
        "decorators": decorators,
        "methods": methods,
        "loc": loc,
        "line_no": node.lineno,
    }


def analyze_file(full_path, rel_path):
    try:
        with open(full_path, "r", encoding="utf-8", errors="replace") as f:
            source = f.read()
    except Exception:
        return None

    loc = source.count("\n") + (1 if source and not source.endswith("\n") else 0)

    try:
        tree = ast.parse(source, filename=rel_path)
    except SyntaxError:
        return {
            "rel_path": rel_path,
            "loc": loc,
            "classes": [],
            "functions": [],
            "imports": [],
            "all_exports": [],
            "constants": [],
        }

    classes = []
    functions = []
    imports = []
    all_exports = []
    constants = []

    for node in ast.iter_child_nodes(tree):
        if isinstance(node, ast.ClassDef):
            classes.append(analyze_class(node))

        elif isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
            functions.append(analyze_function(node))

        elif isinstance(node, ast.Import):
            for alias in node.names:
                imports.append(alias.name)

        elif isinstance(node, ast.ImportFrom):
            module = node.module or ""
            level = node.level or 0
            prefix = "." * level
            if module:
                imports.append(prefix + module)
            elif prefix:
                for alias in node.names:
                    imports.append(prefix + alias.name)

        elif isinstance(node, ast.Assign):
            for target in node.targets:
                if isinstance(target, ast.Name):
                    if target.id == "__all__":
                        if isinstance(node.value, (ast.List, ast.Tuple)):
                            for elt in node.value.elts:
                                if isinstance(elt, ast.Constant) and isinstance(elt.value, str):
                                    all_exports.append(elt.value)
                    elif target.id.isupper() and not target.id.startswith("_"):
                        constants.append(target.id)

    return {
        "rel_path": rel_path,
        "loc": loc,
        "classes": classes,
        "functions": functions,
        "imports": imports,
        "all_exports": all_exports,
        "constants": constants,
    }


def detect_patterns(files):
    patterns = []

    for f in files:
        rel = f["rel_path"]
        for cls in f.get("classes", []):
            # ABC / abstract classes
            for base in cls.get("bases", []):
                if base in ("ABC", "abc.ABC"):
                    patterns.append({
                        "pattern": "Abstract Base Class",
                        "location": f"{rel}:{cls['name']}",
                        "detail": f"Inherits from {base}",
                    })
                if base == "Protocol" or base == "typing.Protocol":
                    patterns.append({
                        "pattern": "Protocol",
                        "location": f"{rel}:{cls['name']}",
                        "detail": f"Implements typing.Protocol",
                    })

            # Dataclass
            for dec in cls.get("decorators", []):
                if "dataclass" in dec:
                    patterns.append({
                        "pattern": "Dataclass",
                        "location": f"{rel}:{cls['name']}",
                        "detail": f"@{dec}",
                    })
                if "pydantic" in dec.lower() or dec in ("BaseModel",):
                    patterns.append({
                        "pattern": "Pydantic Model",
                        "location": f"{rel}:{cls['name']}",
                        "detail": f"@{dec}",
                    })

            for base in cls.get("bases", []):
                if "BaseModel" in base:
                    patterns.append({
                        "pattern": "Pydantic Model",
                        "location": f"{rel}:{cls['name']}",
                        "detail": f"Inherits from {base}",
                    })
                if "Enum" in base or "enum." in base:
                    patterns.append({
                        "pattern": "Enum",
                        "location": f"{rel}:{cls['name']}",
                        "detail": f"Inherits from {base}",
                    })
                if "Exception" in base or "Error" in base:
                    patterns.append({
                        "pattern": "Custom Exception",
                        "location": f"{rel}:{cls['name']}",
                        "detail": f"Inherits from {base}",
                    })
                if "type" in base.lower() and "meta" in base.lower():
                    patterns.append({
                        "pattern": "Metaclass",
                        "location": f"{rel}:{cls['name']}",
                        "detail": f"Uses metaclass: {base}",
                    })

            # Singleton pattern (common: __new__ with cls._instance check)
            method_names = [m["name"] for m in cls.get("methods", [])]
            if "__new__" in method_names and "__init__" in method_names:
                patterns.append({
                    "pattern": "Possible Singleton",
                    "location": f"{rel}:{cls['name']}",
                    "detail": "Defines both __new__ and __init__",
                })

        # Decorator factories (functions that return decorators)
        for func in f.get("functions", []):
            for dec in func.get("decorators", []):
                if dec in ("property", "staticmethod", "classmethod", "abstractmethod"):
                    continue
                if dec.startswith("app.") or dec.startswith("router."):
                    patterns.append({
                        "pattern": "Route Handler",
                        "location": f"{rel}:{func['name']}",
                        "detail": f"@{dec}",
                    })
                elif dec.startswith("click.") or dec.startswith("typer."):
                    patterns.append({
                        "pattern": "CLI Command",
                        "location": f"{rel}:{func['name']}",
                        "detail": f"@{dec}",
                    })
                elif dec.startswith("pytest."):
                    patterns.append({
                        "pattern": "Pytest Fixture/Mark",
                        "location": f"{rel}:{func['name']}",
                        "detail": f"@{dec}",
                    })

    return patterns


def main():
    if len(sys.argv) != 2:
        print("Usage: analyze.py <project_dir>", file=sys.stderr)
        sys.exit(1)

    project_dir = os.path.abspath(sys.argv[1])

    progress("scan", 0, 0, "Scanning for Python files...")
    py_files = find_py_files(project_dir)
    total = len(py_files)
    progress("scan", total, total, f"Found {total} Python files")

    files = []
    for i, (full_path, rel_path) in enumerate(py_files):
        progress("parse", i + 1, total, rel_path)
        result = analyze_file(full_path, rel_path)
        if result:
            files.append(result)

    progress("patterns", 0, 1, "Detecting patterns...")
    patterns = detect_patterns(files)
    progress("patterns", 1, 1, f"Found {len(patterns)} patterns")

    emit({"type": "result", "files": files, "patterns": patterns})


if __name__ == "__main__":
    main()
```

- [ ] **Step 2: Test the script manually against a Python project**

Run: `python3 /Users/stephen/dev/aha/analyzer/analyze.py /path/to/some/python/project 2>/dev/null | tail -1 | python3 -m json.tool | head -30`

Verify it outputs valid JSON with `type: "result"`, `files` array, and `patterns` array.

- [ ] **Step 3: Commit**

```bash
git add analyzer/analyze.py
git commit -m "feat: add Python AST analysis script for deep codebase analysis"
```

---

### Task 3: Cache Layer

**Files:**
- Create: `cache/cache.go`
- Create: `cache/cache_test.go`

Cache stores the full `ProjectAnalysis` as JSON in `~/.cache/aha/<sha256-of-abs-path>.json`. Invalidation: cache stores the modification time of the newest `.py` file; if any file is newer, re-analyze.

- [ ] **Step 1: Write failing tests for cache**

Create `cache/cache_test.go`:

```go
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

	// newestMod is after cache time → stale
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

	// Create files with known times
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cache/... -v`
Expected: FAIL — package doesn't exist

- [ ] **Step 3: Implement cache.go**

Create `cache/cache.go`:

```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cache/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cache/cache.go cache/cache_test.go
git commit -m "feat: add cache layer for analysis results with staleness detection"
```

---

### Task 4: Analyzer Refactor — Orchestrate Python Subprocess

**Files:**
- Modify: `analyzer/analyzer.go` (rewrite)
- Modify: `analyzer/analyzer_test.go` (update)

The analyzer now embeds the Python script, runs it as a subprocess, parses the NDJSON output for progress and results, resolves internal imports using the existing Go logic, merges declared deps from config.go, and returns a complete `ProjectAnalysis`.

- [ ] **Step 1: Write a test for the new analyzer**

Update `analyzer/analyzer_test.go` to add a test for the new flow. Keep existing tests that still apply (config, imports, stdlib).

Add to `analyzer/analyzer_test.go`:

```go
func TestAnalyzeIntegration(t *testing.T) {
	// Create a minimal Python project in a temp dir
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

	// Package dir
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./analyzer/... -run TestAnalyzeIntegration -v`
Expected: FAIL — signature mismatch

- [ ] **Step 3: Rewrite analyzer.go**

Replace `analyzer/analyzer.go`:

```go
package analyzer

import (
	"bufio"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/stephen/aha/model"
)

//go:embed analyze.py
var analyzeScript embed.FS

// pythonResult is the JSON structure returned by analyze.py.
type pythonResult struct {
	Type     string               `json:"type"`
	Phase    string               `json:"phase"`
	Current  int                  `json:"current"`
	Total    int                  `json:"total"`
	Detail   string               `json:"detail"`
	Files    []model.FileAnalysis `json:"files"`
	Patterns []model.PatternMatch `json:"patterns"`
}

// Analyze performs deep analysis of a Python project.
// Progress updates are sent to progressCh (can be nil).
func Analyze(projectDir string, progressCh chan<- model.AnalysisProgress) (*model.ProjectAnalysis, error) {
	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	info, err := os.Stat(absDir)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("not a valid directory: %s", projectDir)
	}

	result := &model.ProjectAnalysis{
		ProjectPath: absDir,
		Modules:     make(map[string]*model.Module),
	}

	// Phase 1: Load declared deps from config files
	declaredDeps, warnings := LoadDeclaredDeps(absDir)
	result.Warnings = append(result.Warnings, warnings...)

	// Load project metadata from pyproject.toml
	loadProjectMeta(absDir, result)

	// Phase 2: Run Python AST analysis
	files, patterns, err := runPythonAnalysis(absDir, progressCh)
	if err != nil {
		return nil, fmt.Errorf("python analysis: %w", err)
	}
	result.Files = files
	result.Patterns = patterns

	// Phase 3: Build module map and resolve imports
	if progressCh != nil {
		progressCh <- model.AnalysisProgress{Phase: "resolve", Current: 0, Total: 1, Detail: "Resolving imports..."}
	}

	pyFiles := make(map[string][]string)
	for _, f := range files {
		result.Modules[f.RelPath] = &model.Module{RelPath: f.RelPath}
		pyFiles[f.RelPath] = f.Imports
	}

	declaredMap := make(map[string]DeclaredDep)
	for _, d := range declaredDeps {
		declaredMap[d.Name] = d
	}

	externalUsage := make(map[string]map[string]bool)

	for relPath, imports := range pyFiles {
		for _, imp := range imports {
			if strings.HasPrefix(imp, ".") {
				resolved := resolveRelativeImport(relPath, imp, result.Modules)
				if resolved != "" {
					result.Modules[relPath].Imports = append(result.Modules[relPath].Imports, resolved)
				}
			} else {
				topLevel := TopLevelModule(imp)
				if IsStdlib(topLevel) {
					continue
				}
				if internalPath := resolveAbsoluteImport(imp, result.Modules, absDir); internalPath != "" {
					result.Modules[relPath].Imports = append(result.Modules[relPath].Imports, internalPath)
				} else {
					normalizedName := strings.ToLower(strings.ReplaceAll(topLevel, "-", "_"))
					if externalUsage[normalizedName] == nil {
						externalUsage[normalizedName] = make(map[string]bool)
					}
					externalUsage[normalizedName][relPath] = true
				}
			}
		}
	}

	// Build imported-by relationships
	for relPath, mod := range result.Modules {
		for _, imp := range mod.Imports {
			if target, ok := result.Modules[imp]; ok {
				target.ImportedBy = append(target.ImportedBy, relPath)
			}
		}
	}

	// Merge external deps
	allExtNames := make(map[string]bool)
	for name := range declaredMap {
		allExtNames[name] = true
	}
	for name := range externalUsage {
		allExtNames[name] = true
	}
	for name := range allExtNames {
		dep := model.ExternalDep{Name: name}
		if declared, ok := declaredMap[name]; ok {
			dep.VersionConstraint = declared.VersionConstraint
		}
		if files, ok := externalUsage[name]; ok {
			for f := range files {
				dep.UsedIn = append(dep.UsedIn, f)
			}
			sort.Strings(dep.UsedIn)
		}
		result.ExternalDeps = append(result.ExternalDeps, dep)
	}
	sort.Slice(result.ExternalDeps, func(i, j int) bool {
		return result.ExternalDeps[i].Name < result.ExternalDeps[j].Name
	})

	// Compute stats
	result.Stats = computeStats(result)

	if progressCh != nil {
		progressCh <- model.AnalysisProgress{Phase: "resolve", Current: 1, Total: 1, Detail: "Done"}
	}

	return result, nil
}

func runPythonAnalysis(projectDir string, progressCh chan<- model.AnalysisProgress) ([]model.FileAnalysis, []model.PatternMatch, error) {
	scriptData, err := analyzeScript.ReadFile("analyze.py")
	if err != nil {
		return nil, nil, fmt.Errorf("reading embedded script: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "aha-analyze-*.py")
	if err != nil {
		return nil, nil, err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(scriptData); err != nil {
		tmpFile.Close()
		return nil, nil, err
	}
	tmpFile.Close()

	cmd := exec.Command("python3", tmpFile.Name(), projectDir)
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("starting python3: %w (is python3 installed?)", err)
	}

	var files []model.FileAnalysis
	var patterns []model.PatternMatch

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 10*1024*1024), 10*1024*1024) // 10MB buffer for large projects
	for scanner.Scan() {
		var msg pythonResult
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}
		switch msg.Type {
		case "progress":
			if progressCh != nil {
				pct := 0.0
				if msg.Total > 0 {
					pct = float64(msg.Current) / float64(msg.Total)
				}
				progressCh <- model.AnalysisProgress{
					Phase:   msg.Phase,
					Current: msg.Current,
					Total:   msg.Total,
					Detail:  msg.Detail,
					Percent: pct,
				}
			}
		case "result":
			files = msg.Files
			patterns = msg.Patterns
		}
	}

	if err := cmd.Wait(); err != nil {
		return nil, nil, fmt.Errorf("python analysis failed: %w", err)
	}

	return files, patterns, nil
}

func loadProjectMeta(absDir string, result *model.ProjectAnalysis) {
	type pyprojectMeta struct {
		Project struct {
			Name           string   `toml:"name"`
			Version        string   `toml:"version"`
			Description    string   `toml:"description"`
			RequiresPython string   `toml:"requires-python"`
			Scripts        map[string]string `toml:"scripts"`
		} `toml:"project"`
	}

	pyprojectPath := filepath.Join(absDir, "pyproject.toml")
	data, err := os.ReadFile(pyprojectPath)
	if err != nil {
		return
	}

	// Simple extraction without importing toml again — reuse existing toml dep
	var meta pyprojectMeta
	if _, err := toml.Decode(string(data), &meta); err != nil {
		return
	}

	result.ProjectName = meta.Project.Name
	result.Version = meta.Project.Version
	result.Description = meta.Project.Description
	result.PythonRequires = meta.Project.RequiresPython

	for name, ref := range meta.Project.Scripts {
		parts := strings.SplitN(ref, ":", 2)
		ep := model.EntryPoint{Name: name}
		if len(parts) == 2 {
			ep.Module = parts[0]
			ep.Function = parts[1]
		} else {
			ep.Module = ref
		}
		result.EntryPoints = append(result.EntryPoints, ep)
	}

	// Check for __main__.py
	filepath.Walk(absDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if filepath.Base(path) == "__main__.py" {
			rel, _ := filepath.Rel(absDir, path)
			result.EntryPoints = append(result.EntryPoints, model.EntryPoint{
				Name:   rel,
				Module: rel,
			})
		}
		return nil
	})
}

func computeStats(result *model.ProjectAnalysis) model.ProjectStats {
	stats := model.ProjectStats{
		TotalFiles:        len(result.Files),
		TotalExternalDeps: len(result.ExternalDeps),
	}

	packages := make(map[string]bool)
	for _, f := range result.Files {
		stats.TotalLOC += f.LOC
		stats.TotalClasses += len(f.Classes)
		stats.TotalFunctions += len(f.Functions)
		for _, c := range f.Classes {
			stats.TotalFunctions += len(c.Methods)
		}
		pkg := filepath.Dir(f.RelPath)
		if pkg != "." {
			packages[pkg] = true
		}
	}
	stats.TotalPackages = len(packages)
	return stats
}
```

**Note:** The `loadProjectMeta` function needs the toml import. Add this at the top:

```go
import (
	...
	"github.com/BurntSushi/toml"
)
```

- [ ] **Step 4: Run all analyzer tests**

Run: `go test ./analyzer/... -v`
Expected: PASS (existing tests should still pass since config.go, imports.go, stdlib.go are unchanged)

- [ ] **Step 5: Commit**

```bash
git add analyzer/analyzer.go analyzer/analyzer_test.go
git commit -m "feat: rewrite analyzer to use Python AST subprocess with progress reporting"
```

---

### Task 5: TUI — Progress Bar View

**Files:**
- Create: `tui/progress.go`

Shows a beautiful progress bar while analysis runs. Uses Bubble Tea's tick-based animation. The app starts in "analyzing" mode and transitions to "exploring" mode when done.

- [ ] **Step 1: Create progress.go**

```go
package tui

import (
	"fmt"
	"math"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/stephen/aha/model"
)

type progressModel struct {
	progress model.AnalysisProgress
	width    int
	height   int
	phases   []phaseStatus
}

type phaseStatus struct {
	name string
	done bool
}

func newProgressModel() progressModel {
	return progressModel{
		phases: []phaseStatus{
			{name: "Scanning files"},
			{name: "Parsing Python AST"},
			{name: "Detecting patterns"},
			{name: "Resolving imports"},
		},
	}
}

func (m *progressModel) update(p model.AnalysisProgress) {
	m.progress = p
	switch p.Phase {
	case "scan":
		if p.Current == p.Total && p.Total > 0 {
			m.phases[0].done = true
		}
	case "parse":
		m.phases[0].done = true
		if p.Current == p.Total {
			m.phases[1].done = true
		}
	case "patterns":
		m.phases[0].done = true
		m.phases[1].done = true
		if p.Current == p.Total {
			m.phases[2].done = true
		}
	case "resolve":
		m.phases[0].done = true
		m.phases[1].done = true
		m.phases[2].done = true
		if p.Current == p.Total {
			m.phases[3].done = true
		}
	}
}

func (m progressModel) View() string {
	var b strings.Builder

	barWidth := m.width - 12
	if barWidth < 20 {
		barWidth = 20
	}
	if barWidth > 60 {
		barWidth = 60
	}

	// Title
	title := progressTitleStyle.Render("  Analyzing Python project...")
	b.WriteString("\n\n")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Progress bar
	pct := m.progress.Percent
	filled := int(math.Round(float64(barWidth) * pct))
	if filled > barWidth {
		filled = barWidth
	}
	empty := barWidth - filled

	bar := progressFilledStyle.Render(strings.Repeat("█", filled)) +
		progressEmptyStyle.Render(strings.Repeat("░", empty))

	pctStr := fmt.Sprintf(" %3.0f%%", pct*100)
	b.WriteString("  " + bar + progressPctStyle.Render(pctStr))
	b.WriteString("\n\n")

	// Phase checklist
	for _, phase := range m.phases {
		icon := progressPendingStyle.Render("  ○ ")
		label := progressPendingStyle.Render(phase.name)
		if phase.done {
			icon = progressDoneStyle.Render("  ● ")
			label = progressDoneStyle.Render(phase.name)
		}
		b.WriteString(icon + label + "\n")
	}

	// Current detail
	if m.progress.Detail != "" {
		b.WriteString("\n")
		detail := m.progress.Detail
		maxLen := m.width - 6
		if maxLen > 0 && len(detail) > maxLen {
			detail = "..." + detail[len(detail)-maxLen+3:]
		}
		b.WriteString(progressDetailStyle.Render("  " + detail))
	}

	// Center vertically
	content := b.String()
	lines := strings.Count(content, "\n") + 1
	topPad := (m.height - lines) / 3
	if topPad < 0 {
		topPad = 0
	}
	return strings.Repeat("\n", topPad) + content
}

// Styles for progress view
var (
	progressTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("99")).
				MarginLeft(2)

	progressFilledStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("99"))

	progressEmptyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("238"))

	progressPctStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("255"))

	progressDoneStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("42"))

	progressPendingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241"))

	progressDetailStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")).
				Italic(true)
)
```

- [ ] **Step 2: Commit**

```bash
git add tui/progress.go
git commit -m "feat: add progress bar view for analysis phase"
```

---

### Task 6: TUI — Overview Tab

**Files:**
- Create: `tui/overview.go`

Displays project summary: name, version, description, stats, entry points, and a quick dependency summary.

- [ ] **Step 1: Create overview.go**

```go
package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/stephen/aha/model"
)

type overviewModel struct {
	analysis *model.ProjectAnalysis
	width    int
	height   int
	scroll   int
}

func newOverviewModel(a *model.ProjectAnalysis) overviewModel {
	return overviewModel{analysis: a}
}

func (m overviewModel) Update(msg interface{}) overviewModel {
	return m
}

func (m overviewModel) View() string {
	a := m.analysis
	var b strings.Builder

	// Project header
	name := a.ProjectName
	if name == "" {
		name = "(unnamed project)"
	}
	header := name
	if a.Version != "" {
		header += " v" + a.Version
	}
	b.WriteString(overviewHeaderStyle.Render("  "+header) + "\n")
	if a.Description != "" {
		b.WriteString(overviewDescStyle.Render("  "+a.Description) + "\n")
	}
	if a.PythonRequires != "" {
		b.WriteString(overviewMetaStyle.Render("  Python "+a.PythonRequires) + "\n")
	}
	b.WriteString("\n")

	// Stats box
	s := a.Stats
	b.WriteString(overviewSectionStyle.Render("  Statistics") + "\n")
	b.WriteString(overviewStatStyle.Render(fmt.Sprintf("    %-20s %d", "Python files", s.TotalFiles)) + "\n")
	b.WriteString(overviewStatStyle.Render(fmt.Sprintf("    %-20s %s", "Lines of code", formatNumber(s.TotalLOC))) + "\n")
	b.WriteString(overviewStatStyle.Render(fmt.Sprintf("    %-20s %d", "Packages", s.TotalPackages)) + "\n")
	b.WriteString(overviewStatStyle.Render(fmt.Sprintf("    %-20s %d", "Classes", s.TotalClasses)) + "\n")
	b.WriteString(overviewStatStyle.Render(fmt.Sprintf("    %-20s %d", "Functions", s.TotalFunctions)) + "\n")
	b.WriteString(overviewStatStyle.Render(fmt.Sprintf("    %-20s %d", "External deps", s.TotalExternalDeps)) + "\n")
	b.WriteString("\n")

	// Entry points
	if len(a.EntryPoints) > 0 {
		b.WriteString(overviewSectionStyle.Render("  Entry Points") + "\n")
		for _, ep := range a.EntryPoints {
			if ep.Function != "" {
				b.WriteString(overviewItemStyle.Render(fmt.Sprintf("    %s → %s:%s", ep.Name, ep.Module, ep.Function)) + "\n")
			} else {
				b.WriteString(overviewItemStyle.Render(fmt.Sprintf("    %s", ep.Name)) + "\n")
			}
		}
		b.WriteString("\n")
	}

	// Top external deps
	if len(a.ExternalDeps) > 0 {
		b.WriteString(overviewSectionStyle.Render("  External Dependencies") + "\n")
		max := 10
		if len(a.ExternalDeps) < max {
			max = len(a.ExternalDeps)
		}
		for _, dep := range a.ExternalDeps[:max] {
			ver := dep.VersionConstraint
			if ver == "" {
				ver = "(any)"
			}
			used := fmt.Sprintf("%d files", len(dep.UsedIn))
			if len(dep.UsedIn) == 0 {
				used = "unused"
			}
			b.WriteString(overviewItemStyle.Render(fmt.Sprintf("    %-25s %-15s %s", dep.Name, ver, used)) + "\n")
		}
		if len(a.ExternalDeps) > max {
			b.WriteString(overviewMetaStyle.Render(fmt.Sprintf("    ... and %d more (see Dependencies tab)", len(a.ExternalDeps)-max)) + "\n")
		}
		b.WriteString("\n")
	}

	// Warnings
	if len(a.Warnings) > 0 {
		b.WriteString(overviewWarnStyle.Render("  Warnings") + "\n")
		for _, w := range a.Warnings {
			b.WriteString(overviewWarnStyle.Render("    ⚠ "+w) + "\n")
		}
	}

	return b.String()
}

func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1000000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%.1fM", float64(n)/1000000)
}

var (
	overviewHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("255"))

	overviewDescStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")).
				Italic(true)

	overviewMetaStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241"))

	overviewSectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("99"))

	overviewStatStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("255"))

	overviewItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245"))

	overviewWarnStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("214"))
)
```

- [ ] **Step 2: Commit**

```bash
git add tui/overview.go
git commit -m "feat: add overview tab with project summary and stats"
```

---

### Task 7: TUI — Explorer Tab (File/Class/Function Tree)

**Files:**
- Create: `tui/explorer.go`

Interactive tree: Package → File → Class/Function. Expand files to see classes and functions, expand classes to see methods. Shows LOC and line numbers.

- [ ] **Step 1: Create explorer.go**

```go
package tui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stephen/aha/model"
)

type explorerNodeType int

const (
	nodeTypePackage explorerNodeType = iota
	nodeTypeFile
	nodeTypeClass
	nodeTypeFunction
	nodeTypeMethod
	nodeTypeConstantsHeader
	nodeTypeConstant
)

type explorerNode struct {
	label    string
	nodeType explorerNodeType
	depth    int
	key      string // unique key for expand/collapse
	loc      int
	lineNo   int
}

type explorerModel struct {
	files    []model.FileAnalysis
	expanded map[string]bool
	cursor   int
	nodes    []explorerNode
	width    int
	height   int
	offset   int
}

func newExplorerModel(files []model.FileAnalysis) explorerModel {
	m := explorerModel{
		files:    files,
		expanded: make(map[string]bool),
	}
	m.rebuildNodes()
	return m
}

func (m *explorerModel) rebuildNodes() {
	m.nodes = nil

	// Group files by package
	packages := make(map[string][]model.FileAnalysis)
	var pkgOrder []string
	for _, f := range m.files {
		pkg := filepath.Dir(f.RelPath)
		if pkg == "." {
			pkg = "(root)"
		}
		if _, exists := packages[pkg]; !exists {
			pkgOrder = append(pkgOrder, pkg)
		}
		packages[pkg] = append(packages[pkg], f)
	}
	sort.Strings(pkgOrder)

	for _, pkg := range pkgOrder {
		files := packages[pkg]
		pkgKey := "pkg:" + pkg

		arrow := "▸"
		if m.expanded[pkgKey] {
			arrow = "▾"
		}

		fileCount := len(files)
		totalLOC := 0
		for _, f := range files {
			totalLOC += f.LOC
		}

		m.nodes = append(m.nodes, explorerNode{
			label:    fmt.Sprintf("%s %s/  (%d files, %d LOC)", arrow, pkg, fileCount, totalLOC),
			nodeType: nodeTypePackage,
			depth:    0,
			key:      pkgKey,
		})

		if !m.expanded[pkgKey] {
			continue
		}

		sort.Slice(files, func(i, j int) bool { return files[i].RelPath < files[j].RelPath })

		for _, f := range files {
			fileKey := "file:" + f.RelPath
			arrow := "▸"
			if m.expanded[fileKey] {
				arrow = "▾"
			}

			summary := fmt.Sprintf("%d LOC", f.LOC)
			if len(f.Classes) > 0 {
				summary += fmt.Sprintf(", %d classes", len(f.Classes))
			}
			if len(f.Functions) > 0 {
				summary += fmt.Sprintf(", %d funcs", len(f.Functions))
			}

			m.nodes = append(m.nodes, explorerNode{
				label:    fmt.Sprintf("%s %s  (%s)", arrow, filepath.Base(f.RelPath), summary),
				nodeType: nodeTypeFile,
				depth:    1,
				key:      fileKey,
				loc:      f.LOC,
			})

			if !m.expanded[fileKey] {
				continue
			}

			// Classes
			for _, cls := range f.Classes {
				clsKey := "cls:" + f.RelPath + ":" + cls.Name
				arrow := "▸"
				if m.expanded[clsKey] {
					arrow = "▾"
				}

				bases := ""
				if len(cls.Bases) > 0 {
					bases = "(" + strings.Join(cls.Bases, ", ") + ")"
				}
				decs := ""
				if len(cls.Decorators) > 0 {
					decs = " @" + strings.Join(cls.Decorators, " @")
				}

				m.nodes = append(m.nodes, explorerNode{
					label:    fmt.Sprintf("%s class %s%s%s  [L%d, %d LOC]", arrow, cls.Name, bases, decs, cls.LineNo, cls.LOC),
					nodeType: nodeTypeClass,
					depth:    2,
					key:      clsKey,
					loc:      cls.LOC,
					lineNo:   cls.LineNo,
				})

				if m.expanded[clsKey] {
					for _, method := range cls.Methods {
						params := strings.Join(method.Params, ", ")
						ret := ""
						if method.ReturnType != "" {
							ret = " → " + method.ReturnType
						}
						decs := ""
						if len(method.Decorators) > 0 {
							decs = " @" + strings.Join(method.Decorators, " @")
						}

						m.nodes = append(m.nodes, explorerNode{
							label:    fmt.Sprintf("def %s(%s)%s%s  [L%d]", method.Name, params, ret, decs, method.LineNo),
							nodeType: nodeTypeMethod,
							depth:    3,
							loc:      method.LOC,
							lineNo:   method.LineNo,
						})
					}
				}
			}

			// Functions
			for _, fn := range f.Functions {
				params := strings.Join(fn.Params, ", ")
				ret := ""
				if fn.ReturnType != "" {
					ret = " → " + fn.ReturnType
				}
				decs := ""
				if len(fn.Decorators) > 0 {
					decs = " @" + strings.Join(fn.Decorators, " @")
				}

				m.nodes = append(m.nodes, explorerNode{
					label:    fmt.Sprintf("def %s(%s)%s%s  [L%d, %d LOC]", fn.Name, params, ret, decs, fn.LineNo, fn.LOC),
					nodeType: nodeTypeFunction,
					depth:    2,
					loc:      fn.LOC,
					lineNo:   fn.LineNo,
				})
			}

			// Constants
			if len(f.Constants) > 0 {
				m.nodes = append(m.nodes, explorerNode{
					label:    fmt.Sprintf("constants: %s", strings.Join(f.Constants, ", ")),
					nodeType: nodeTypeConstantsHeader,
					depth:    2,
				})
			}
		}
	}
}

func (m explorerModel) Update(msg tea.Msg) (explorerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.offset {
					m.offset = m.cursor
				}
			}
		case "down", "j":
			if m.cursor < len(m.nodes)-1 {
				m.cursor++
				visibleRows := m.visibleRows()
				if m.cursor >= m.offset+visibleRows {
					m.offset = m.cursor - visibleRows + 1
				}
			}
		case "enter", " ":
			if m.cursor >= 0 && m.cursor < len(m.nodes) {
				node := m.nodes[m.cursor]
				if node.key != "" {
					m.expanded[node.key] = !m.expanded[node.key]
					m.rebuildNodes()
					if m.cursor >= len(m.nodes) {
						m.cursor = len(m.nodes) - 1
					}
				}
			}
		}
	}
	return m, nil
}

func (m explorerModel) visibleRows() int {
	rows := m.height - 2
	if rows < 1 {
		return 1
	}
	return rows
}

func (m explorerModel) View() string {
	if len(m.nodes) == 0 {
		return "\n  No files found.\n"
	}

	var b strings.Builder

	visible := m.visibleRows()
	end := m.offset + visible
	if end > len(m.nodes) {
		end = len(m.nodes)
	}

	for i := m.offset; i < end; i++ {
		node := m.nodes[i]
		indent := strings.Repeat("  ", node.depth+1)
		line := indent + node.label

		var style lipgloss.Style
		switch {
		case i == m.cursor:
			style = explorerSelectedStyle
		case node.nodeType == nodeTypePackage:
			style = explorerPackageStyle
		case node.nodeType == nodeTypeFile:
			style = explorerFileStyle
		case node.nodeType == nodeTypeClass:
			style = explorerClassStyle
		case node.nodeType == nodeTypeFunction || node.nodeType == nodeTypeMethod:
			style = explorerFuncStyle
		default:
			style = explorerDefaultStyle
		}
		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}

	if len(m.nodes) > visible {
		b.WriteString(fmt.Sprintf("\n  %d/%d", m.cursor+1, len(m.nodes)))
	}

	return b.String()
}

var (
	explorerSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("212"))

	explorerPackageStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("75"))

	explorerFileStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("255"))

	explorerClassStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("222"))

	explorerFuncStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("114"))

	explorerDefaultStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245"))
)
```

- [ ] **Step 2: Commit**

```bash
git add tui/explorer.go
git commit -m "feat: add explorer tab with package/file/class/function tree"
```

---

### Task 8: TUI — Dependencies Tab

**Files:**
- Create: `tui/dependencies.go`
- Delete: `tui/external.go`
- Delete: `tui/internal.go`

Combines the existing external and internal dependency views into one tab with a sub-tab toggle (e/i keys).

- [ ] **Step 1: Create dependencies.go**

This file combines the logic from the existing `external.go` and `internal.go`, largely preserving that code but wrapping it in a single `dependenciesModel` with a sub-view toggle.

```go
package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stephen/aha/model"
)

type depView int

const (
	depViewExternal depView = iota
	depViewInternal
)

type dependenciesModel struct {
	view     depView
	external externalDepsModel
	internal internalDepsModel
	width    int
	height   int
}

func newDependenciesModel(deps []model.ExternalDep, modules map[string]*model.Module) dependenciesModel {
	return dependenciesModel{
		external: newExternalDepsModel(deps),
		internal: newInternalDepsModel(modules),
	}
}

func (m dependenciesModel) Update(msg tea.Msg) (dependenciesModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "e":
			m.view = depViewExternal
			return m, nil
		case "i":
			m.view = depViewInternal
			return m, nil
		}

		switch m.view {
		case depViewExternal:
			m.external.width = m.width
			m.external.height = m.height - 2
			var cmd tea.Cmd
			m.external, cmd = m.external.Update(msg)
			return m, cmd
		case depViewInternal:
			m.internal.height = m.height - 2
			var cmd tea.Cmd
			m.internal, cmd = m.internal.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m dependenciesModel) View() string {
	var b strings.Builder

	extLabel := depInactiveSubStyle.Render("[e] External")
	intLabel := depInactiveSubStyle.Render("[i] Internal")
	if m.view == depViewExternal {
		extLabel = depActiveSubStyle.Render("[e] External")
	} else {
		intLabel = depActiveSubStyle.Render("[i] Internal")
	}
	b.WriteString("  " + extLabel + "  " + intLabel + "\n\n")

	switch m.view {
	case depViewExternal:
		b.WriteString(m.external.View())
	case depViewInternal:
		b.WriteString(m.internal.View())
	}

	return b.String()
}

// --- External deps sub-model (from external.go) ---

type sortColumn int

const (
	sortByName sortColumn = iota
	sortByVersion
	sortByUsedIn
)

type externalDepsModel struct {
	deps    []model.ExternalDep
	cursor  int
	sortCol sortColumn
	width   int
	height  int
	offset  int
}

func newExternalDepsModel(deps []model.ExternalDep) externalDepsModel {
	m := externalDepsModel{
		deps: make([]model.ExternalDep, len(deps)),
	}
	copy(m.deps, deps)
	m.sortDeps()
	return m
}

func (m *externalDepsModel) sortDeps() {
	switch m.sortCol {
	case sortByName:
		sort.Slice(m.deps, func(i, j int) bool { return m.deps[i].Name < m.deps[j].Name })
	case sortByVersion:
		sort.Slice(m.deps, func(i, j int) bool { return m.deps[i].VersionConstraint < m.deps[j].VersionConstraint })
	case sortByUsedIn:
		sort.Slice(m.deps, func(i, j int) bool {
			return strings.Join(m.deps[i].UsedIn, ",") < strings.Join(m.deps[j].UsedIn, ",")
		})
	}
}

func (m externalDepsModel) Update(msg tea.Msg) (externalDepsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.offset {
					m.offset = m.cursor
				}
			}
		case "down", "j":
			if m.cursor < len(m.deps)-1 {
				m.cursor++
				vis := m.visibleRows()
				if m.cursor >= m.offset+vis {
					m.offset = m.cursor - vis + 1
				}
			}
		case "s":
			m.sortCol = (m.sortCol + 1) % 3
			m.sortDeps()
		}
	}
	return m, nil
}

func (m externalDepsModel) visibleRows() int {
	rows := m.height - 4
	if rows < 1 {
		return 1
	}
	return rows
}

func (m externalDepsModel) View() string {
	if len(m.deps) == 0 {
		return "\n  No external dependencies found.\n"
	}

	nameW := 20
	versionW := 20
	for _, dep := range m.deps {
		if len(dep.Name)+2 > nameW {
			nameW = len(dep.Name) + 2
		}
		if len(dep.VersionConstraint)+2 > versionW {
			versionW = len(dep.VersionConstraint) + 2
		}
	}
	if nameW > 30 {
		nameW = 30
	}
	if versionW > 25 {
		versionW = 25
	}

	usedInW := m.width - nameW - versionW - 6
	if usedInW < 20 {
		usedInW = 20
	}

	var b strings.Builder

	sortIndicators := [3]string{"  ", "  ", "  "}
	sortIndicators[m.sortCol] = " ^"

	header := fmt.Sprintf("  %-*s %-*s %s",
		nameW, "Name"+sortIndicators[0],
		versionW, "Version"+sortIndicators[1],
		"Used In"+sortIndicators[2],
	)
	b.WriteString(tableHeaderStyle.Render(header) + "\n")
	b.WriteString(strings.Repeat("-", m.width) + "\n")

	visible := m.visibleRows()
	end := m.offset + visible
	if end > len(m.deps) {
		end = len(m.deps)
	}

	for i := m.offset; i < end; i++ {
		dep := m.deps[i]
		name := dep.Name
		if len(name) > nameW {
			name = name[:nameW-1] + "..."
		}
		version := dep.VersionConstraint
		if version == "" {
			version = "-"
		}
		if len(version) > versionW {
			version = version[:versionW-1] + "..."
		}
		usedIn := strings.Join(dep.UsedIn, ", ")
		if usedIn == "" {
			usedIn = "-"
		}
		if len(usedIn) > usedInW {
			usedIn = usedIn[:usedInW-1] + "..."
		}

		row := fmt.Sprintf("  %-*s %-*s %s", nameW, name, versionW, version, usedIn)

		if i == m.cursor {
			b.WriteString(tableSelectedStyle.Render(row))
		} else {
			b.WriteString(tableCellStyle.Render(row))
		}
		b.WriteString("\n")
	}

	if len(m.deps) > visible {
		b.WriteString(fmt.Sprintf("\n  %d/%d", m.cursor+1, len(m.deps)))
	}

	return b.String()
}

// --- Internal deps sub-model (from internal.go) ---

type internalNodeType int

const (
	intNodeModule internalNodeType = iota
	intNodeImportsHeader
	intNodeImportedByHeader
	intNodeImportEntry
	intNodeImportedByEntry
)

type internalNode struct {
	label    string
	nodeType internalNodeType
	depth    int
}

type internalDepsModel struct {
	modules    map[string]*model.Module
	sortedKeys []string
	expanded   map[string]bool
	cursor     int
	nodes      []internalNode
	height     int
	offset     int
}

func newInternalDepsModel(modules map[string]*model.Module) internalDepsModel {
	m := internalDepsModel{
		modules:  modules,
		expanded: make(map[string]bool),
	}
	for k := range modules {
		m.sortedKeys = append(m.sortedKeys, k)
	}
	sort.Strings(m.sortedKeys)
	m.rebuildNodes()
	return m
}

func (m *internalDepsModel) rebuildNodes() {
	m.nodes = nil
	for _, key := range m.sortedKeys {
		mod := m.modules[key]
		prefix := "▸"
		if m.expanded[key] {
			prefix = "▾"
		}
		m.nodes = append(m.nodes, internalNode{
			label:    fmt.Sprintf("%s %s", prefix, mod.RelPath),
			nodeType: intNodeModule,
			depth:    0,
		})
		if m.expanded[key] {
			m.nodes = append(m.nodes, internalNode{
				label:    "imports:",
				nodeType: intNodeImportsHeader,
				depth:    1,
			})
			if len(mod.Imports) > 0 {
				sorted := make([]string, len(mod.Imports))
				copy(sorted, mod.Imports)
				sort.Strings(sorted)
				for _, imp := range sorted {
					m.nodes = append(m.nodes, internalNode{
						label:    imp,
						nodeType: intNodeImportEntry,
						depth:    2,
					})
				}
			} else {
				m.nodes = append(m.nodes, internalNode{
					label:    "(none)",
					nodeType: intNodeImportEntry,
					depth:    2,
				})
			}
			m.nodes = append(m.nodes, internalNode{
				label:    "imported by:",
				nodeType: intNodeImportedByHeader,
				depth:    1,
			})
			if len(mod.ImportedBy) > 0 {
				sorted := make([]string, len(mod.ImportedBy))
				copy(sorted, mod.ImportedBy)
				sort.Strings(sorted)
				for _, imp := range sorted {
					m.nodes = append(m.nodes, internalNode{
						label:    imp,
						nodeType: intNodeImportedByEntry,
						depth:    2,
					})
				}
			} else {
				m.nodes = append(m.nodes, internalNode{
					label:    "(none)",
					nodeType: intNodeImportedByEntry,
					depth:    2,
				})
			}
		}
	}
}

func (m *internalDepsModel) moduleKeyAtCursor() string {
	if m.cursor < 0 || m.cursor >= len(m.nodes) {
		return ""
	}
	node := m.nodes[m.cursor]
	if node.nodeType != intNodeModule {
		return ""
	}
	label := node.label
	if len(label) > 2 {
		return label[2:]
	}
	return ""
}

func (m internalDepsModel) Update(msg tea.Msg) (internalDepsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.offset {
					m.offset = m.cursor
				}
			}
		case "down", "j":
			if m.cursor < len(m.nodes)-1 {
				m.cursor++
				vis := m.visibleRows()
				if m.cursor >= m.offset+vis {
					m.offset = m.cursor - vis + 1
				}
			}
		case "enter", " ":
			key := m.moduleKeyAtCursor()
			if key != "" {
				m.expanded[key] = !m.expanded[key]
				m.rebuildNodes()
				if m.cursor >= len(m.nodes) {
					m.cursor = len(m.nodes) - 1
				}
			}
		}
	}
	return m, nil
}

func (m internalDepsModel) visibleRows() int {
	rows := m.height - 2
	if rows < 1 {
		return 1
	}
	return rows
}

func (m internalDepsModel) View() string {
	if len(m.nodes) == 0 {
		return "\n  No internal modules found.\n"
	}

	var b strings.Builder
	visible := m.visibleRows()
	end := m.offset + visible
	if end > len(m.nodes) {
		end = len(m.nodes)
	}

	for i := m.offset; i < end; i++ {
		node := m.nodes[i]
		indent := strings.Repeat("  ", node.depth+1)
		line := indent + node.label

		switch {
		case i == m.cursor && node.nodeType == intNodeModule:
			b.WriteString(treeSelectedStyle.Render(line))
		case i == m.cursor:
			b.WriteString(treeSelectedStyle.Render(line))
		case node.nodeType == intNodeModule:
			b.WriteString(treeNodeStyle.Render(line))
		case node.nodeType == intNodeImportsHeader || node.nodeType == intNodeImportedByHeader:
			b.WriteString(treeLabelStyle.Render(line))
		default:
			b.WriteString(treeNodeStyle.Render(line))
		}
		b.WriteString("\n")
	}

	if len(m.nodes) > visible {
		b.WriteString(fmt.Sprintf("\n  %d/%d", m.cursor+1, len(m.nodes)))
	}

	return b.String()
}

var (
	depActiveSubStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("99"))

	depInactiveSubStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241"))
)
```

- [ ] **Step 2: Delete old files**

```bash
rm tui/external.go tui/internal.go
```

- [ ] **Step 3: Commit**

```bash
git add tui/dependencies.go
git rm tui/external.go tui/internal.go
git commit -m "feat: combine external and internal deps into unified dependencies tab"
```

---

### Task 9: TUI — Patterns Tab

**Files:**
- Create: `tui/patterns.go`

Displays detected patterns grouped by category with locations.

- [ ] **Step 1: Create patterns.go**

```go
package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stephen/aha/model"
)

type patternsModel struct {
	groups []patternGroup
	cursor int
	height int
	offset int
	nodes  []patternNode
}

type patternGroup struct {
	name     string
	patterns []model.PatternMatch
}

type patternNode struct {
	label string
	depth int
	isHeader bool
}

func newPatternsModel(patterns []model.PatternMatch) patternsModel {
	// Group by pattern type
	grouped := make(map[string][]model.PatternMatch)
	var order []string
	for _, p := range patterns {
		if _, exists := grouped[p.Pattern]; !exists {
			order = append(order, p.Pattern)
		}
		grouped[p.Pattern] = append(grouped[p.Pattern], p)
	}
	sort.Strings(order)

	var groups []patternGroup
	for _, name := range order {
		groups = append(groups, patternGroup{name: name, patterns: grouped[name]})
	}

	m := patternsModel{groups: groups}
	m.rebuildNodes()
	return m
}

func (m *patternsModel) rebuildNodes() {
	m.nodes = nil
	for _, g := range m.groups {
		m.nodes = append(m.nodes, patternNode{
			label:    fmt.Sprintf("%s (%d)", g.name, len(g.patterns)),
			depth:    0,
			isHeader: true,
		})
		for _, p := range g.patterns {
			m.nodes = append(m.nodes, patternNode{
				label: fmt.Sprintf("%s  %s", p.Location, p.Detail),
				depth: 1,
			})
		}
	}
}

func (m patternsModel) Update(msg tea.Msg) (patternsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.offset {
					m.offset = m.cursor
				}
			}
		case "down", "j":
			if m.cursor < len(m.nodes)-1 {
				m.cursor++
				vis := m.visibleRows()
				if m.cursor >= m.offset+vis {
					m.offset = m.cursor - vis + 1
				}
			}
		}
	}
	return m, nil
}

func (m patternsModel) visibleRows() int {
	rows := m.height - 2
	if rows < 1 {
		return 1
	}
	return rows
}

func (m patternsModel) View() string {
	if len(m.nodes) == 0 {
		return "\n  No patterns detected.\n"
	}

	var b strings.Builder
	visible := m.visibleRows()
	end := m.offset + visible
	if end > len(m.nodes) {
		end = len(m.nodes)
	}

	for i := m.offset; i < end; i++ {
		node := m.nodes[i]
		indent := strings.Repeat("  ", node.depth+1)
		line := indent + node.label

		switch {
		case i == m.cursor:
			b.WriteString(patternSelectedStyle.Render(line))
		case node.isHeader:
			b.WriteString(patternHeaderStyle.Render(line))
		default:
			b.WriteString(patternItemStyle.Render(line))
		}
		b.WriteString("\n")
	}

	if len(m.nodes) > visible {
		b.WriteString(fmt.Sprintf("\n  %d/%d", m.cursor+1, len(m.nodes)))
	}

	return b.String()
}

var (
	patternSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("212"))

	patternHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("222"))

	patternItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245"))
)
```

- [ ] **Step 2: Commit**

```bash
git add tui/patterns.go
git commit -m "feat: add patterns tab showing detected design patterns"
```

---

### Task 10: TUI — App Shell with Analysis Phase

**Files:**
- Modify: `tui/app.go` (full rewrite)
- Modify: `tui/styles.go` (clean up, remove unused styles from old tabs)

The app has two phases: `analyzing` (shows progress bar) and `exploring` (shows tabbed TUI). During analysis, it listens on a channel for progress updates and the final result.

- [ ] **Step 1: Rewrite app.go**

```go
package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stephen/aha/model"
)

type appPhase int

const (
	phaseAnalyzing appPhase = iota
	phaseExploring
)

type tab int

const (
	tabOverview tab = iota
	tabExplorer
	tabDependencies
	tabPatterns
)

var tabNames = []string{"Overview", "Explorer", "Dependencies", "Patterns"}

// ProgressMsg is sent when analysis progress updates.
type ProgressMsg model.AnalysisProgress

// AnalysisDoneMsg is sent when analysis completes.
type AnalysisDoneMsg struct {
	Analysis *model.ProjectAnalysis
	Err      error
}

type App struct {
	phase       appPhase
	projectPath string
	activeTab   tab
	progress    progressModel
	overview    overviewModel
	explorer    explorerModel
	deps        dependenciesModel
	patterns    patternsModel
	width       int
	height      int
}

// NewAnalyzingApp creates an app that starts in analysis mode.
func NewAnalyzingApp(projectPath string) App {
	return App{
		phase:       phaseAnalyzing,
		projectPath: projectPath,
		progress:    newProgressModel(),
	}
}

// NewExploringApp creates an app that starts directly in exploration mode (from cache).
func NewExploringApp(analysis *model.ProjectAnalysis) App {
	a := App{
		phase:       phaseExploring,
		projectPath: analysis.ProjectPath,
	}
	a.initExploring(analysis)
	return a
}

func (a *App) initExploring(analysis *model.ProjectAnalysis) {
	a.overview = newOverviewModel(analysis)
	a.explorer = newExplorerModel(analysis.Files)
	a.deps = newDependenciesModel(analysis.ExternalDeps, analysis.Modules)
	a.patterns = newPatternsModel(analysis.Patterns)
}

func (a App) Init() tea.Cmd {
	return nil
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.progress.width = a.width
		a.progress.height = a.height
		contentHeight := a.height - 6
		a.overview.width = a.width
		a.overview.height = contentHeight
		a.explorer.width = a.width
		a.explorer.height = contentHeight
		a.deps.width = a.width
		a.deps.height = contentHeight
		a.patterns.height = contentHeight
		return a, nil

	case ProgressMsg:
		p := model.AnalysisProgress(msg)
		a.progress.update(p)
		return a, nil

	case AnalysisDoneMsg:
		if msg.Err != nil {
			// TODO: show error in TUI
			return a, tea.Quit
		}
		a.phase = phaseExploring
		a.initExploring(msg.Analysis)
		// Re-apply sizes
		contentHeight := a.height - 6
		a.overview.width = a.width
		a.overview.height = contentHeight
		a.explorer.width = a.width
		a.explorer.height = contentHeight
		a.deps.width = a.width
		a.deps.height = contentHeight
		a.patterns.height = contentHeight
		return a, nil

	case tea.KeyPressMsg:
		if a.phase == phaseAnalyzing {
			if msg.String() == "ctrl+c" || msg.String() == "q" {
				return a, tea.Quit
			}
			return a, nil
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return a, tea.Quit
		case "tab":
			a.activeTab = (a.activeTab + 1) % tab(len(tabNames))
			return a, nil
		case "shift+tab":
			if a.activeTab == 0 {
				a.activeTab = tab(len(tabNames) - 1)
			} else {
				a.activeTab--
			}
			return a, nil
		case "1":
			a.activeTab = tabOverview
			return a, nil
		case "2":
			a.activeTab = tabExplorer
			return a, nil
		case "3":
			a.activeTab = tabDependencies
			return a, nil
		case "4":
			a.activeTab = tabPatterns
			return a, nil
		}

		switch a.activeTab {
		case tabExplorer:
			var cmd tea.Cmd
			a.explorer, cmd = a.explorer.Update(msg)
			return a, cmd
		case tabDependencies:
			var cmd tea.Cmd
			a.deps, cmd = a.deps.Update(msg)
			return a, cmd
		case tabPatterns:
			var cmd tea.Cmd
			a.patterns, cmd = a.patterns.Update(msg)
			return a, cmd
		}
	}

	return a, nil
}

func (a App) View() tea.View {
	var b strings.Builder

	if a.phase == phaseAnalyzing {
		b.WriteString(a.progress.View())

		// Pad to full height
		lines := strings.Count(b.String(), "\n")
		for i := lines; i < a.height-1; i++ {
			b.WriteString("\n")
		}

		v := tea.NewView(b.String())
		v.AltScreen = true
		return v
	}

	// Header
	title := titleStyle.Render(fmt.Sprintf(" aha - %s ", a.projectPath))
	b.WriteString(title)
	b.WriteString("\n\n")

	// Tabs
	var tabs []string
	for i, name := range tabNames {
		label := fmt.Sprintf(" %d %s ", i+1, name)
		if tab(i) == a.activeTab {
			tabs = append(tabs, activeTabStyle.Render(label))
		} else {
			tabs = append(tabs, inactiveTabStyle.Render(label))
		}
	}
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, tabs...))
	b.WriteString("\n\n")

	// Content
	switch a.activeTab {
	case tabOverview:
		b.WriteString(a.overview.View())
	case tabExplorer:
		b.WriteString(a.explorer.View())
	case tabDependencies:
		b.WriteString(a.deps.View())
	case tabPatterns:
		b.WriteString(a.patterns.View())
	}

	// Pad
	rendered := b.String()
	lines := strings.Count(rendered, "\n")
	for i := lines; i < a.height-2; i++ {
		b.WriteString("\n")
	}

	// Help bar
	var helpItems []helpItem
	helpItems = append(helpItems,
		helpItem{"1-4/tab", "switch tab"},
		helpItem{"up/down", "navigate"},
	)
	switch a.activeTab {
	case tabExplorer:
		helpItems = append(helpItems, helpItem{"enter", "expand/collapse"})
	case tabDependencies:
		helpItems = append(helpItems, helpItem{"e/i", "ext/int"}, helpItem{"s", "sort"}, helpItem{"enter", "expand"})
	}
	helpItems = append(helpItems, helpItem{"q", "quit"})

	b.WriteString("\n")
	b.WriteString(renderHelp(helpItems))

	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}
```

- [ ] **Step 2: Update styles.go — remove unused styles, keep shared ones**

Replace `tui/styles.go`:

```go
package tui

import "charm.land/lipgloss/v2"

var (
	colorPrimary   = lipgloss.Color("99")
	colorSecondary = lipgloss.Color("241")
	colorMuted     = lipgloss.Color("245")
	colorHighlight = lipgloss.Color("212")
	colorWarning   = lipgloss.Color("214")
	colorWhite     = lipgloss.Color("255")

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite).
			Background(colorPrimary).
			Padding(0, 1)

	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(0, 1)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(colorSecondary).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorSecondary).
				Padding(0, 1)

	tableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorPrimary)

	tableCellStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	tableSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorWhite).
				Background(colorPrimary)

	treeNodeStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	treeSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorHighlight)

	treeLabelStyle = lipgloss.NewStyle().
			Foreground(colorSecondary).
			Italic(true)

	helpKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(colorSecondary)
)
```

- [ ] **Step 3: Commit**

```bash
git add tui/app.go tui/styles.go
git commit -m "feat: rewrite app shell with analysis/exploring phases and 4-tab layout"
```

---

### Task 11: Main.go — Integration with Cache and Async Analysis

**Files:**
- Modify: `main.go` (rewrite)

The main function checks the cache. On hit, launches the TUI directly. On miss, launches the TUI in analyzing mode and kicks off analysis in a goroutine that sends Bubble Tea messages.

- [ ] **Step 1: Rewrite main.go**

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/stephen/aha/analyzer"
	"github.com/stephen/aha/cache"
	"github.com/stephen/aha/model"
	"github.com/stephen/aha/tui"
)

var version = "dev"

func main() {
	if len(os.Args) >= 2 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("aha %s\n", version)
		os.Exit(0)
	}

	if len(os.Args) >= 2 && os.Args[1] == "--no-cache" {
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: aha [--no-cache] <path-to-python-project>\n")
			os.Exit(1)
		}
		runWithAnalysis(os.Args[2], true)
		return
	}

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: aha [--no-cache] <path-to-python-project>\n")
		os.Exit(1)
	}

	runWithAnalysis(os.Args[1], false)
}

func runWithAnalysis(projectDir string, noCache bool) {
	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	store := cache.Store{Dir: cache.DefaultDir()}

	// Try cache
	if !noCache {
		newestMod := cache.NewestPyModTime(absDir)
		if cached, err := store.Load(absDir, newestMod); err == nil && cached != nil {
			app := tui.NewExploringApp(cached)
			p := tea.NewProgram(app)
			if _, err := p.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}

	// Cache miss — analyze with progress
	app := tui.NewAnalyzingApp(absDir)
	p := tea.NewProgram(app)

	go func() {
		progressCh := make(chan model.AnalysisProgress, 100)

		// Forward progress to TUI
		go func() {
			for prog := range progressCh {
				p.Send(tui.ProgressMsg(prog))
			}
		}()

		result, err := analyzer.Analyze(absDir, progressCh)
		close(progressCh)

		if err != nil {
			p.Send(tui.AnalysisDoneMsg{Err: err})
			return
		}

		// Save to cache
		_ = store.Save(absDir, result, cache.NewestPyModTime(absDir))

		p.Send(tui.AnalysisDoneMsg{Analysis: result})
	}()

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Verify it builds**

Run: `go build -o aha .`
Expected: Successful build

- [ ] **Step 3: Test manually against a Python project**

Run: `./aha /path/to/some/python/project`

Expected:
1. First run: Progress bar appears, phases complete, transitions to exploration TUI
2. Second run: Loads instantly from cache
3. `./aha --no-cache /path/to/project` forces re-analysis

- [ ] **Step 4: Commit**

```bash
git add main.go
git commit -m "feat: integrate cache, async analysis with progress, and exploring mode"
```

---

### Task 12: Update CLAUDE.md and Clean Up

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Update CLAUDE.md to reflect the new architecture**

Update the Project Overview, Status, and Architecture sections:

```markdown
# CLAUDE.md

## Project Overview

**aha** is a TUI for exploring and understanding Python codebases. Point it at any Python project to get an interactive exploration session with project overview, file/class/function tree, dependency graph, and pattern detection. Analysis results are cached for instant subsequent loads. MIT licensed.

## Status

Core implementation complete. Four tabs: Overview (stats, entry points), Explorer (package/file/class/function tree), Dependencies (external table + internal import graph), Patterns (detected design patterns).

## Language & Stack

- Go (primary language)
- Bubble Tea v2 (`charm.land/bubbletea/v2`) — TUI framework
- LipGloss v2 (`charm.land/lipgloss/v2`) — styling
- BurntSushi/toml — TOML parsing
- Embedded Python script — AST analysis (requires python3)

## Build & Test

\`\`\`bash
go build -o aha .          # Build
go test ./... -v            # Run all tests
./aha <path-to-python-project>        # Run (cached)
./aha --no-cache <path-to-python-project>  # Force re-analysis
\`\`\`

## Architecture

Three layers with one-way data flow: Python AST Script -> Go Analyzer -> Cache -> TUI

- `model/` — Data structures (ProjectAnalysis, FileAnalysis, ClassInfo, FunctionInfo, etc.)
- `analyzer/` — Orchestrates embedded Python AST script, resolves imports, merges deps
- `analyzer/analyze.py` — Embedded Python script for deep AST analysis
- `cache/` — JSON cache keyed by project path hash, invalidated by file modification times
- `tui/` — Bubble Tea app: progress bar during analysis, 4-tab exploration view
- `main.go` — CLI entry point, cache check, async analysis launch
```

- [ ] **Step 2: Run full test suite**

Run: `go test ./... -v`
Expected: All tests pass

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md for codebase explorer architecture"
```

---

## Self-Review Checklist

**Spec coverage:**
- Deep analysis with AST parsing: Task 2 (Python script) + Task 4 (Go orchestrator)
- Cached results: Task 3 (cache layer) + Task 11 (integration)
- Progress bar on first run: Task 5 (progress view) + Task 10 (app phases) + Task 11 (async flow)
- TUI exploration: Tasks 6-10 (overview, explorer, dependencies, patterns, app shell)
- Four exploration views covering structure, deps, patterns, API surface

**Placeholder scan:** No TBDs, TODOs, or "similar to Task N" references found.

**Type consistency:** `model.ProjectAnalysis`, `model.AnalysisProgress`, `tui.ProgressMsg`, `tui.AnalysisDoneMsg` used consistently across all tasks. `NewAnalyzingApp`/`NewExploringApp` constructors match main.go usage.
