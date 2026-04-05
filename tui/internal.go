package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/stephen/aha/model"
)

type treeNodeType int

const (
	nodeModule treeNodeType = iota
	nodeImportsHeader
	nodeImportedByHeader
	nodeImportEntry
	nodeImportedByEntry
)

type treeNode struct {
	label    string
	nodeType treeNodeType
	depth    int
}

type internalModel struct {
	modules    map[string]*model.Module
	sortedKeys []string
	expanded   map[string]bool
	cursor     int
	nodes      []treeNode
	height     int
	offset     int
}

func newInternalModel(modules map[string]*model.Module) internalModel {
	m := internalModel{
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

func (m *internalModel) rebuildNodes() {
	m.nodes = nil
	for _, key := range m.sortedKeys {
		mod := m.modules[key]
		prefix := ">"
		if m.expanded[key] {
			prefix = "v"
		}
		m.nodes = append(m.nodes, treeNode{
			label:    fmt.Sprintf("%s %s", prefix, mod.RelPath),
			nodeType: nodeModule,
			depth:    0,
		})
		if m.expanded[key] {
			m.nodes = append(m.nodes, treeNode{
				label:    "imports:",
				nodeType: nodeImportsHeader,
				depth:    1,
			})
			if len(mod.Imports) > 0 {
				sorted := make([]string, len(mod.Imports))
				copy(sorted, mod.Imports)
				sort.Strings(sorted)
				for _, imp := range sorted {
					m.nodes = append(m.nodes, treeNode{
						label:    imp,
						nodeType: nodeImportEntry,
						depth:    2,
					})
				}
			} else {
				m.nodes = append(m.nodes, treeNode{
					label:    "(none)",
					nodeType: nodeImportEntry,
					depth:    2,
				})
			}

			m.nodes = append(m.nodes, treeNode{
				label:    "imported by:",
				nodeType: nodeImportedByHeader,
				depth:    1,
			})
			if len(mod.ImportedBy) > 0 {
				sorted := make([]string, len(mod.ImportedBy))
				copy(sorted, mod.ImportedBy)
				sort.Strings(sorted)
				for _, imp := range sorted {
					m.nodes = append(m.nodes, treeNode{
						label:    imp,
						nodeType: nodeImportedByEntry,
						depth:    2,
					})
				}
			} else {
				m.nodes = append(m.nodes, treeNode{
					label:    "(none)",
					nodeType: nodeImportedByEntry,
					depth:    2,
				})
			}
		}
	}
}

func (m *internalModel) moduleKeyAtCursor() string {
	if m.cursor < 0 || m.cursor >= len(m.nodes) {
		return ""
	}
	node := m.nodes[m.cursor]
	if node.nodeType != nodeModule {
		return ""
	}
	label := node.label
	if len(label) > 2 {
		return label[2:]
	}
	return ""
}

func (m internalModel) Update(msg tea.Msg) (internalModel, tea.Cmd) {
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
				visibleRows := m.visibleRows()
				if m.cursor >= m.offset+visibleRows {
					m.offset = m.cursor - visibleRows + 1
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

func (m internalModel) visibleRows() int {
	rows := m.height - 2
	if rows < 1 {
		return 1
	}
	return rows
}

func (m internalModel) View() string {
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
		case i == m.cursor && node.nodeType == nodeModule:
			b.WriteString(treeSelectedStyle.Render(line))
		case i == m.cursor:
			b.WriteString(treeSelectedStyle.Render(line))
		case node.nodeType == nodeModule:
			b.WriteString(treeNodeStyle.Render(line))
		case node.nodeType == nodeImportsHeader || node.nodeType == nodeImportedByHeader:
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
