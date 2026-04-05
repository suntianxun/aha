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
