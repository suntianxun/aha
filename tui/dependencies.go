package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stephen/aha/model"
)

type depView int

const (
	depViewExternal depView = iota
	depViewInternal
)

type dependenciesModel struct {
	view     depView
	external externalDepsModel
	internal internalDepsModel
	width    int
	height   int
}

func newDependenciesModel(deps []model.ExternalDep, modules map[string]*model.Module) dependenciesModel {
	return dependenciesModel{
		external: newExternalDepsModel(deps),
		internal: newInternalDepsModel(modules),
	}
}

func (m dependenciesModel) Update(msg tea.Msg) (dependenciesModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "e":
			m.view = depViewExternal
			return m, nil
		case "i":
			m.view = depViewInternal
			return m, nil
		}

		switch m.view {
		case depViewExternal:
			m.external.width = m.width
			m.external.height = m.height - 2
			var cmd tea.Cmd
			m.external, cmd = m.external.Update(msg)
			return m, cmd
		case depViewInternal:
			m.internal.height = m.height - 2
			var cmd tea.Cmd
			m.internal, cmd = m.internal.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m dependenciesModel) View() string {
	var b strings.Builder

	extLabel := depInactiveSubStyle.Render("[e] External")
	intLabel := depInactiveSubStyle.Render("[i] Internal")
	if m.view == depViewExternal {
		extLabel = depActiveSubStyle.Render("[e] External")
	} else {
		intLabel = depActiveSubStyle.Render("[i] Internal")
	}
	b.WriteString("  " + extLabel + "  " + intLabel + "\n\n")

	switch m.view {
	case depViewExternal:
		b.WriteString(m.external.View())
	case depViewInternal:
		b.WriteString(m.internal.View())
	}

	return b.String()
}

// --- External deps sub-model ---

type sortColumn int

const (
	sortByName sortColumn = iota
	sortByVersion
	sortByUsedIn
)

type externalDepsModel struct {
	deps    []model.ExternalDep
	cursor  int
	sortCol sortColumn
	width   int
	height  int
	offset  int
}

func newExternalDepsModel(deps []model.ExternalDep) externalDepsModel {
	m := externalDepsModel{
		deps: make([]model.ExternalDep, len(deps)),
	}
	copy(m.deps, deps)
	m.sortDeps()
	return m
}

func (m *externalDepsModel) sortDeps() {
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

func (m externalDepsModel) Update(msg tea.Msg) (externalDepsModel, tea.Cmd) {
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
				vis := m.visibleRows()
				if m.cursor >= m.offset+vis {
					m.offset = m.cursor - vis + 1
				}
			}
		case "s":
			m.sortCol = (m.sortCol + 1) % 3
			m.sortDeps()
		}
	}
	return m, nil
}

func (m externalDepsModel) visibleRows() int {
	rows := m.height - 4
	if rows < 1 {
		return 1
	}
	return rows
}

func (m externalDepsModel) View() string {
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
	b.WriteString(tableHeaderStyle.Render(header) + "\n")
	b.WriteString(strings.Repeat("-", m.width) + "\n")

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

// --- Internal deps sub-model ---

type internalNodeType int

const (
	intNodeModule internalNodeType = iota
	intNodeImportsHeader
	intNodeImportedByHeader
	intNodeImportEntry
	intNodeImportedByEntry
)

type internalNode struct {
	label    string
	nodeType internalNodeType
	depth    int
}

type internalDepsModel struct {
	modules    map[string]*model.Module
	sortedKeys []string
	expanded   map[string]bool
	cursor     int
	nodes      []internalNode
	height     int
	offset     int
}

func newInternalDepsModel(modules map[string]*model.Module) internalDepsModel {
	m := internalDepsModel{
		modules:  modules,
		expanded: make(map[string]bool),
	}
	for k := range modules {
		m.sortedKeys = append(m.sortedKeys, k)
	}
	sort.Strings(m.sortedKeys)
	m.rebuildNodes()
	return m
}

func (m *internalDepsModel) rebuildNodes() {
	m.nodes = nil
	for _, key := range m.sortedKeys {
		mod := m.modules[key]
		prefix := "▸"
		if m.expanded[key] {
			prefix = "▾"
		}
		m.nodes = append(m.nodes, internalNode{
			label:    fmt.Sprintf("%s %s", prefix, mod.RelPath),
			nodeType: intNodeModule,
			depth:    0,
		})
		if m.expanded[key] {
			m.nodes = append(m.nodes, internalNode{
				label:    "imports:",
				nodeType: intNodeImportsHeader,
				depth:    1,
			})
			if len(mod.Imports) > 0 {
				sorted := make([]string, len(mod.Imports))
				copy(sorted, mod.Imports)
				sort.Strings(sorted)
				for _, imp := range sorted {
					m.nodes = append(m.nodes, internalNode{
						label:    imp,
						nodeType: intNodeImportEntry,
						depth:    2,
					})
				}
			} else {
				m.nodes = append(m.nodes, internalNode{
					label:    "(none)",
					nodeType: intNodeImportEntry,
					depth:    2,
				})
			}
			m.nodes = append(m.nodes, internalNode{
				label:    "imported by:",
				nodeType: intNodeImportedByHeader,
				depth:    1,
			})
			if len(mod.ImportedBy) > 0 {
				sorted := make([]string, len(mod.ImportedBy))
				copy(sorted, mod.ImportedBy)
				sort.Strings(sorted)
				for _, imp := range sorted {
					m.nodes = append(m.nodes, internalNode{
						label:    imp,
						nodeType: intNodeImportedByEntry,
						depth:    2,
					})
				}
			} else {
				m.nodes = append(m.nodes, internalNode{
					label:    "(none)",
					nodeType: intNodeImportedByEntry,
					depth:    2,
				})
			}
		}
	}
}

func (m *internalDepsModel) moduleKeyAtCursor() string {
	if m.cursor < 0 || m.cursor >= len(m.nodes) {
		return ""
	}
	node := m.nodes[m.cursor]
	if node.nodeType != intNodeModule {
		return ""
	}
	label := node.label
	if len(label) > 2 {
		return label[2:]
	}
	return ""
}

func (m internalDepsModel) Update(msg tea.Msg) (internalDepsModel, tea.Cmd) {
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
		case "enter", " ":
			key := m.moduleKeyAtCursor()
			if key != "" {
				m.expanded[key] = !m.expanded[key]
				m.rebuildNodes()
				if m.cursor >= len(m.nodes) {
					m.cursor = len(m.nodes) - 1
				}
			}
		}
	}
	return m, nil
}

func (m internalDepsModel) visibleRows() int {
	rows := m.height - 2
	if rows < 1 {
		return 1
	}
	return rows
}

func (m internalDepsModel) View() string {
	if len(m.nodes) == 0 {
		return "\n  No internal modules found.\n"
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
		case i == m.cursor && node.nodeType == intNodeModule:
			b.WriteString(treeSelectedStyle.Render(line))
		case i == m.cursor:
			b.WriteString(treeSelectedStyle.Render(line))
		case node.nodeType == intNodeModule:
			b.WriteString(treeNodeStyle.Render(line))
		case node.nodeType == intNodeImportsHeader || node.nodeType == intNodeImportedByHeader:
			b.WriteString(treeLabelStyle.Render(line))
		default:
			b.WriteString(treeNodeStyle.Render(line))
		}
		b.WriteString("\n")
	}

	if len(m.nodes) > visible {
		b.WriteString(fmt.Sprintf("\n  %d/%d", m.cursor+1, len(m.nodes)))
	}

	return b.String()
}

var (
	depActiveSubStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("99"))

	depInactiveSubStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241"))
)
