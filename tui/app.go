package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stephen/aha/model"
)

type App struct {
	projectPath  string
	warnings     []string
	dependencies dependenciesModel
	width        int
	height       int
}

func NewApp(deps *model.ProjectAnalysis) App {
	return App{
		projectPath:  deps.ProjectPath,
		warnings:     deps.Warnings,
		dependencies: newDependenciesModel(deps.ExternalDeps, deps.Modules),
	}
}

func (a App) Init() tea.Cmd {
	return nil
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.dependencies.width = a.width
		a.dependencies.height = a.height - 6
		return a, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return a, tea.Quit
		}

		var cmd tea.Cmd
		a.dependencies, cmd = a.dependencies.Update(msg)
		return a, cmd
	}

	return a, nil
}

func (a App) View() tea.View {
	var b strings.Builder

	title := titleStyle.Render(fmt.Sprintf(" aha - %s ", a.projectPath))
	b.WriteString(title)
	if len(a.warnings) > 0 {
		b.WriteString("  ")
		b.WriteString(warningStyle.Render(strings.Join(a.warnings, " | ")))
	}
	b.WriteString("\n\n")

	depsTab := activeTabStyle.Render("Dependencies")
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, depsTab))
	b.WriteString("\n\n")

	b.WriteString(a.dependencies.View())

	rendered := b.String()
	lines := strings.Count(rendered, "\n")
	for i := lines; i < a.height-2; i++ {
		b.WriteString("\n")
	}

	var helpItems []helpItem
	helpItems = append(helpItems,
		helpItem{"e/i", "switch view"},
		helpItem{"up/down", "navigate"},
	)
	if a.dependencies.view == depViewExternal {
		helpItems = append(helpItems, helpItem{"s", "sort"})
	} else {
		helpItems = append(helpItems, helpItem{"enter", "expand/collapse"})
	}
	helpItems = append(helpItems, helpItem{"q", "quit"})

	b.WriteString("\n")
	b.WriteString(renderHelp(helpItems))

	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}
