# aha

<p>
  <img src=".github/logo-dark.svg" width="540" alt="aha — Aha! Now I know this library...">
</p>

A terminal UI for viewing dependencies of a Python project.

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lip Gloss](https://github.com/charmbracelet/lipgloss).

## Install

### Homebrew

```bash
brew install suntianxun/tap/aha
```

### Go

```bash
go install github.com/suntianxun/aha@latest
```

## Usage

```bash
aha <path-to-python-project>
```

Two tabs are available — press `Tab` to switch between them.

**External Dependencies** shows a table of third-party packages with version constraints and which files import them. **Internal Dependencies** shows an interactive tree of module-to-module imports within the project.

### Keybindings

| Key | Action |
|---|---|
| `Tab` | Switch between External / Internal tabs |
| `j` / `↑` | Move up |
| `k` / `↓` | Move down |
| `Enter` / `Space` | Expand / collapse module (Internal tab) |
| `s` | Cycle sort column (External tab) |
| `q` / `Ctrl+C` | Quit |

## How it works

aha parses your Python project in three ways:

1. **Config files** — reads `pyproject.toml`, `setup.py`, or `requirements.txt` for declared dependencies and version constraints
2. **Source scanning** — scans all `.py` files for `import` and `from ... import` statements
3. **Cross-referencing** — matches source imports against declared deps and Python's stdlib to classify each as external, internal, or stdlib

No Python runtime required. aha is a single Go binary.

## License

[MIT](LICENSE)
