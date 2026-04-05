package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stephen/aha/model"
)

type patternsModel struct {
	groups []patternGroup
	cursor int
	height int
	offset int
	nodes  []patternNode
}

type patternGroup struct {
	name     string
	patterns []model.PatternMatch
}

type patternNode struct {
	label    string
	depth    int
	isHeader bool
}

func newPatternsModel(patterns []model.PatternMatch) patternsModel {
	// Group by pattern type
	grouped := make(map[string][]model.PatternMatch)
	var order []string
	for _, p := range patterns {
		if _, exists := grouped[p.Pattern]; !exists {
			order = append(order, p.Pattern)
		}
		grouped[p.Pattern] = append(grouped[p.Pattern], p)
	}
	sort.Strings(order)

	var groups []patternGroup
	for _, name := range order {
		groups = append(groups, patternGroup{name: name, patterns: grouped[name]})
	}

	m := patternsModel{groups: groups}
	m.rebuildNodes()
	return m
}

func (m *patternsModel) rebuildNodes() {
	m.nodes = nil
	for _, g := range m.groups {
		m.nodes = append(m.nodes, patternNode{
			label:    fmt.Sprintf("%s (%d)", g.name, len(g.patterns)),
			depth:    0,
			isHeader: true,
		})
		for _, p := range g.patterns {
			m.nodes = append(m.nodes, patternNode{
				label: fmt.Sprintf("%s  %s", p.Location, p.Detail),
				depth: 1,
			})
		}
	}
}

func (m patternsModel) Update(msg tea.Msg) (patternsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.offset {
					m.offset = m.cursor
				}
			}
		case "down", "j":
			if m.cursor < len(m.nodes)-1 {
				m.cursor++
				vis := m.visibleRows()
				if m.cursor >= m.offset+vis {
					m.offset = m.cursor - vis + 1
				}
			}
		}
	}
	return m, nil
}

func (m patternsModel) visibleRows() int {
	rows := m.height - 2
	if rows < 1 {
		return 1
	}
	return rows
}

func (m patternsModel) View() string {
	if len(m.nodes) == 0 {
		return "\n  No patterns detected.\n"
	}

	var b strings.Builder
	visible := m.visibleRows()
	end := m.offset + visible
	if end > len(m.nodes) {
		end = len(m.nodes)
	}

	for i := m.offset; i < end; i++ {
		node := m.nodes[i]
		indent := strings.Repeat("  ", node.depth+1)
		line := indent + node.label

		switch {
		case i == m.cursor:
			b.WriteString(patternSelectedStyle.Render(line))
		case node.isHeader:
			b.WriteString(patternHeaderStyle.Render(line))
		default:
			b.WriteString(patternItemStyle.Render(line))
		}
		b.WriteString("\n")
	}

	if len(m.nodes) > visible {
		b.WriteString(fmt.Sprintf("\n  %d/%d", m.cursor+1, len(m.nodes)))
	}

	return b.String()
}

var (
	patternSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("212"))

	patternHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("222"))

	patternItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245"))
)
