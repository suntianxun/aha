package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/stephen/aha/analyzer"
	"github.com/stephen/aha/tui"
)

var version = "dev"

func main() {
	if len(os.Args) >= 2 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("aha %s\n", version)
		os.Exit(0)
	}

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: aha <path-to-python-project>\n")
		os.Exit(1)
	}

	projectDir := os.Args[1]

	deps, err := analyzer.Analyze(projectDir, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	app := tui.NewExploringApp(deps)
	p := tea.NewProgram(app)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
