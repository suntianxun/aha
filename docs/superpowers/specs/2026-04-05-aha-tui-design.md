# aha — Python Dependency Viewer TUI

## Overview

aha is a single-binary Go TUI that shows the dependencies of a Python project. Given a path to a local Python project directory, it parses source files and config to present two views: external dependencies (as a detailed table) and internal module dependencies (as an interactive tree).

## Usage

```
aha <path-to-python-project>
```

The argument is a local directory path. The tool runs analysis at startup, then presents an interactive TUI.

## Architecture

Three layers with one-way data flow: Analyzer -> Model -> TUI.

### Analyzer

Pure Go package. Runs once at startup and produces the full dependency model.

**External dependency detection** from two sources:

1. Config files — parses `pyproject.toml` (`[project.dependencies]` section), `setup.py` (`install_requires`), and `requirements.txt` for declared dependencies with version constraints.
2. Source scanning — scans all `.py` files for `import X` and `from X import Y` statements. Matches top-level import names against declared dependencies and a hardcoded Python 3.x stdlib module list.

Cross-references both sources to populate the table with: dependency name, version constraint (from config), and which `.py` files import it.

**Internal dependency detection:**

- For each `.py` file, extracts imports and determines if they refer to other modules within the project by matching against the project's own module/package structure.
- Builds a bidirectional graph: "module A imports module B" and "module B is imported by module A."
- Resolves relative imports (`from . import X`, `from ..utils import Y`) using the package directory structure.

**Explicitly out of scope:** Dynamic imports (`__import__()`, `importlib.import_module()`), conditional imports inside functions, and `try/except` import fallbacks.

### Model

Data structures representing the dependency graph:

- `Module` — a Python file with its absolute and relative paths, and its import/imported-by relationships.
- `ExternalDep` — a declared or discovered external dependency with name, version constraint, and list of files that import it.
- `ImportGraph` — the full bidirectional internal dependency graph.

### TUI

Built with Bubble Tea, LipGloss, Bubbles, and Glamour.

## TUI Layout

**Top bar:** App title ("aha") and the project path being analyzed. Warnings (e.g., "No dependency config found") appear here when applicable.

**Tab bar:** Two tabs switched with the Tab key.

**Footer:** Key hints — `tab: switch tab | up/down: navigate | enter: expand | q: quit`.

### External Dependencies Tab

A scrollable table (Bubbles `table` component) with three columns:

| Name | Version Constraint | Used In |
|---|---|---|
| requests | >=2.28.0 | client.py, api.py |
| pydantic | ^2.0 | models.py |

- Arrow keys to navigate rows.
- `s` key cycles sort column (name alphabetical by default).
- Dependency found in source but not declared in config: version shows "—".
- Dependency declared but never imported: "Used In" shows "—".

### Internal Dependencies Tab

An expandable/collapsible tree view:

```
v myproject/
  v main.py
    imports: utils.py, config.py
    imported by: (none)
  > utils.py
  > config.py
```

- Selecting a module expands it to show "imports" and "imported by" as child nodes.
- Arrow keys navigate, Enter/Space toggles expand/collapse.

## Key Bindings

| Key | Action |
|---|---|
| Tab | Switch between External/Internal tabs |
| Up/Down | Navigate rows or tree nodes |
| Enter/Space | Expand/collapse tree node (Internal tab) |
| s | Cycle sort column (External tab) |
| q / Ctrl+C | Quit |

## Error Handling

- **Invalid path:** Print error to stderr and exit (no TUI).
- **No Python files found:** Print error to stderr and exit.
- **No config file found:** Launch TUI, show "—" for all version constraints, note "No dependency config found" in the top bar.
- **Malformed config files:** Best-effort parse, skip unreadable parts, show warning in top bar.
- **Empty project (no deps found):** Launch TUI with empty tables and "No dependencies found" message.

## Tech Stack

- **Go** — primary language
- **Bubble Tea** — TUI framework
- **LipGloss** — styling (colors, borders, layout)
- **Bubbles** — table, tree, and other components
- **Glamour** — help text rendering
- **Huh** — available if interactive forms are needed later
- **Log** — structured logging for debug output

## Project Structure

```
aha/
├── main.go              # CLI entry, arg parsing, launches TUI
├── analyzer/
│   ├── analyzer.go      # Orchestrates analysis
│   ├── imports.go        # Python import statement parser
│   ├── config.go        # pyproject.toml / setup.py / requirements.txt parser
│   └── stdlib.go        # Python stdlib module list
├── model/
│   └── model.go         # Data structures (Module, ExternalDep, ImportGraph)
├── tui/
│   ├── app.go           # Root Bubble Tea model, tab switching
│   ├── external.go      # External dependencies table view
│   ├── internal.go      # Internal dependencies tree view
│   ├── styles.go        # LipGloss styles
│   └── help.go          # Footer/help bar
├── go.mod
└── go.sum
```
