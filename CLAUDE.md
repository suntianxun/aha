# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

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

```bash
go build -o aha .          # Build
go test ./... -v            # Run all tests
./aha <path-to-python-project>        # Run (cached)
./aha --no-cache <path-to-python-project>  # Force re-analysis
```

## Architecture

Three layers with one-way data flow: Python AST Script -> Go Analyzer -> Cache -> TUI

- `model/` — Data structures (ProjectAnalysis, FileAnalysis, ClassInfo, FunctionInfo, etc.)
- `analyzer/` — Orchestrates embedded Python AST script, resolves imports, merges deps
- `analyzer/analyze.py` — Embedded Python script for deep AST analysis
- `cache/` — JSON cache keyed by project path hash, invalidated by file modification times
- `tui/` — Bubble Tea app: progress bar during analysis, 4-tab exploration view
- `main.go` — CLI entry point, cache check, async analysis launch
