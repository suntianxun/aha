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
