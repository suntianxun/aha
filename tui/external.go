package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/stephen/aha/model"
)

type sortColumn int

const (
	sortByName sortColumn = iota
	sortByVersion
	sortByUsedIn
)

type externalModel struct {
	deps    []model.ExternalDep
	cursor  int
	sortCol sortColumn
	width   int
	height  int
	offset  int
}

func newExternalModel(deps []model.ExternalDep) externalModel {
	m := externalModel{
		deps: make([]model.ExternalDep, len(deps)),
	}
	copy(m.deps, deps)
	m.sortDeps()
	return m
}

func (m *externalModel) sortDeps() {
	switch m.sortCol {
	case sortByName:
		sort.Slice(m.deps, func(i, j int) bool { return m.deps[i].Name < m.deps[j].Name })
	case sortByVersion:
		sort.Slice(m.deps, func(i, j int) bool { return m.deps[i].VersionConstraint < m.deps[j].VersionConstraint })
	case sortByUsedIn:
		sort.Slice(m.deps, func(i, j int) bool {
			return strings.Join(m.deps[i].UsedIn, ",") < strings.Join(m.deps[j].UsedIn, ",")
		})
	}
}

func (m externalModel) Update(msg tea.Msg) (externalModel, tea.Cmd) {
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
			if m.cursor < len(m.deps)-1 {
				m.cursor++
				visibleRows := m.visibleRows()
				if m.cursor >= m.offset+visibleRows {
					m.offset = m.cursor - visibleRows + 1
				}
			}
		case "s":
			m.sortCol = (m.sortCol + 1) % 3
			m.sortDeps()
		}
	}
	return m, nil
}

func (m externalModel) visibleRows() int {
	rows := m.height - 4
	if rows < 1 {
		return 1
	}
	return rows
}

func (m externalModel) View() string {
	if len(m.deps) == 0 {
		return "\n  No external dependencies found.\n"
	}

	nameW := 20
	versionW := 20
	for _, dep := range m.deps {
		if len(dep.Name)+2 > nameW {
			nameW = len(dep.Name) + 2
		}
		if len(dep.VersionConstraint)+2 > versionW {
			versionW = len(dep.VersionConstraint) + 2
		}
	}
	if nameW > 30 {
		nameW = 30
	}
	if versionW > 25 {
		versionW = 25
	}

	usedInW := m.width - nameW - versionW - 6
	if usedInW < 20 {
		usedInW = 20
	}

	var b strings.Builder

	sortIndicators := [3]string{"  ", "  ", "  "}
	sortIndicators[m.sortCol] = " ^"

	header := fmt.Sprintf("  %-*s %-*s %s",
		nameW, "Name"+sortIndicators[0],
		versionW, "Version"+sortIndicators[1],
		"Used In"+sortIndicators[2],
	)
	b.WriteString(tableHeaderStyle.Render(header))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("-", m.width))
	b.WriteString("\n")

	visible := m.visibleRows()
	end := m.offset + visible
	if end > len(m.deps) {
		end = len(m.deps)
	}

	for i := m.offset; i < end; i++ {
		dep := m.deps[i]

		name := dep.Name
		if len(name) > nameW {
			name = name[:nameW-1] + "..."
		}

		version := dep.VersionConstraint
		if version == "" {
			version = "-"
		}
		if len(version) > versionW {
			version = version[:versionW-1] + "..."
		}

		usedIn := strings.Join(dep.UsedIn, ", ")
		if usedIn == "" {
			usedIn = "-"
		}
		if len(usedIn) > usedInW {
			usedIn = usedIn[:usedInW-1] + "..."
		}

		row := fmt.Sprintf("  %-*s %-*s %s", nameW, name, versionW, version, usedIn)

		if i == m.cursor {
			b.WriteString(tableSelectedStyle.Render(row))
		} else {
			b.WriteString(tableCellStyle.Render(row))
		}
		b.WriteString("\n")
	}

	if len(m.deps) > visible {
		b.WriteString(fmt.Sprintf("\n  %d/%d", m.cursor+1, len(m.deps)))
	}

	return b.String()
}
