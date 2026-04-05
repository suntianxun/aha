package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stephen/aha/model"
)

type tab int

const (
	tabExternal tab = iota
	tabInternal
)

type App struct {
	projectPath string
	warnings    []string
	activeTab   tab
	external    externalModel
	internal    internalModel
	width       int
	height      int
}

func NewApp(deps *model.ProjectAnalysis) App {
	return App{
		projectPath: deps.ProjectPath,
		warnings:    deps.Warnings,
		external:    newExternalModel(deps.ExternalDeps),
		internal:    newInternalModel(deps.Modules),
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
		contentHeight := a.height - 6
		a.external.width = a.width
		a.external.height = contentHeight
		a.internal.height = contentHeight
		return a, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return a, tea.Quit
		case "tab":
			if a.activeTab == tabExternal {
				a.activeTab = tabInternal
			} else {
				a.activeTab = tabExternal
			}
			return a, nil
		}

		switch a.activeTab {
		case tabExternal:
			var cmd tea.Cmd
			a.external, cmd = a.external.Update(msg)
			return a, cmd
		case tabInternal:
			var cmd tea.Cmd
			a.internal, cmd = a.internal.Update(msg)
			return a, cmd
		}
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

	extTab := inactiveTabStyle.Render("External Dependencies")
	intTab := inactiveTabStyle.Render("Internal Dependencies")
	if a.activeTab == tabExternal {
		extTab = activeTabStyle.Render("External Dependencies")
	} else {
		intTab = activeTabStyle.Render("Internal Dependencies")
	}
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, extTab, " ", intTab))
	b.WriteString("\n\n")

	switch a.activeTab {
	case tabExternal:
		b.WriteString(a.external.View())
	case tabInternal:
		b.WriteString(a.internal.View())
	}

	rendered := b.String()
	lines := strings.Count(rendered, "\n")
	for i := lines; i < a.height-2; i++ {
		b.WriteString("\n")
	}

	var helpItems []helpItem
	helpItems = append(helpItems,
		helpItem{"tab", "switch tab"},
		helpItem{"up/down", "navigate"},
	)
	if a.activeTab == tabExternal {
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
