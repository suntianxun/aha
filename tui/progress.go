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
			icon := progressDoneStyle.Render("  ● ")
			label := progressDoneStyle.Render(phase.name)
			b.WriteString(icon + label + "\n")
			continue
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
