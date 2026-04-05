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
