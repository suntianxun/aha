package tui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stephen/aha/model"
)

type explorerNodeType int

const (
	nodeTypePackage explorerNodeType = iota
	nodeTypeFile
	nodeTypeClass
	nodeTypeFunction
	nodeTypeMethod
	nodeTypeConstantsHeader
	nodeTypeConstant
)

type explorerNode struct {
	label    string
	nodeType explorerNodeType
	depth    int
	key      string // unique key for expand/collapse
	loc      int
	lineNo   int
}

type explorerModel struct {
	files    []model.FileAnalysis
	expanded map[string]bool
	cursor   int
	nodes    []explorerNode
	width    int
	height   int
	offset   int
}

func newExplorerModel(files []model.FileAnalysis) explorerModel {
	m := explorerModel{
		files:    files,
		expanded: make(map[string]bool),
	}
	m.rebuildNodes()
	return m
}

func (m *explorerModel) rebuildNodes() {
	m.nodes = nil

	// Group files by package
	packages := make(map[string][]model.FileAnalysis)
	var pkgOrder []string
	for _, f := range m.files {
		pkg := filepath.Dir(f.RelPath)
		if pkg == "." {
			pkg = "(root)"
		}
		if _, exists := packages[pkg]; !exists {
			pkgOrder = append(pkgOrder, pkg)
		}
		packages[pkg] = append(packages[pkg], f)
	}
	sort.Strings(pkgOrder)

	for _, pkg := range pkgOrder {
		files := packages[pkg]
		pkgKey := "pkg:" + pkg

		arrow := "▸"
		if m.expanded[pkgKey] {
			arrow = "▾"
		}

		fileCount := len(files)
		totalLOC := 0
		for _, f := range files {
			totalLOC += f.LOC
		}

		m.nodes = append(m.nodes, explorerNode{
			label:    fmt.Sprintf("%s %s/  (%d files, %d LOC)", arrow, pkg, fileCount, totalLOC),
			nodeType: nodeTypePackage,
			depth:    0,
			key:      pkgKey,
		})

		if !m.expanded[pkgKey] {
			continue
		}

		sort.Slice(files, func(i, j int) bool { return files[i].RelPath < files[j].RelPath })

		for _, f := range files {
			fileKey := "file:" + f.RelPath
			arrow := "▸"
			if m.expanded[fileKey] {
				arrow = "▾"
			}

			summary := fmt.Sprintf("%d LOC", f.LOC)
			if len(f.Classes) > 0 {
				summary += fmt.Sprintf(", %d classes", len(f.Classes))
			}
			if len(f.Functions) > 0 {
				summary += fmt.Sprintf(", %d funcs", len(f.Functions))
			}

			m.nodes = append(m.nodes, explorerNode{
				label:    fmt.Sprintf("%s %s  (%s)", arrow, filepath.Base(f.RelPath), summary),
				nodeType: nodeTypeFile,
				depth:    1,
				key:      fileKey,
				loc:      f.LOC,
			})

			if !m.expanded[fileKey] {
				continue
			}

			// Classes
			for _, cls := range f.Classes {
				clsKey := "cls:" + f.RelPath + ":" + cls.Name
				arrow := "▸"
				if m.expanded[clsKey] {
					arrow = "▾"
				}

				bases := ""
				if len(cls.Bases) > 0 {
					bases = "(" + strings.Join(cls.Bases, ", ") + ")"
				}
				decs := ""
				if len(cls.Decorators) > 0 {
					decs = " @" + strings.Join(cls.Decorators, " @")
				}

				m.nodes = append(m.nodes, explorerNode{
					label:    fmt.Sprintf("%s class %s%s%s  [L%d, %d LOC]", arrow, cls.Name, bases, decs, cls.LineNo, cls.LOC),
					nodeType: nodeTypeClass,
					depth:    2,
					key:      clsKey,
					loc:      cls.LOC,
					lineNo:   cls.LineNo,
				})

				if m.expanded[clsKey] {
					for _, method := range cls.Methods {
						params := strings.Join(method.Params, ", ")
						ret := ""
						if method.ReturnType != "" {
							ret = " → " + method.ReturnType
						}
						decs := ""
						if len(method.Decorators) > 0 {
							decs = " @" + strings.Join(method.Decorators, " @")
						}

						m.nodes = append(m.nodes, explorerNode{
							label:    fmt.Sprintf("def %s(%s)%s%s  [L%d]", method.Name, params, ret, decs, method.LineNo),
							nodeType: nodeTypeMethod,
							depth:    3,
							loc:      method.LOC,
							lineNo:   method.LineNo,
						})
					}
				}
			}

			// Functions
			for _, fn := range f.Functions {
				params := strings.Join(fn.Params, ", ")
				ret := ""
				if fn.ReturnType != "" {
					ret = " → " + fn.ReturnType
				}
				decs := ""
				if len(fn.Decorators) > 0 {
					decs = " @" + strings.Join(fn.Decorators, " @")
				}

				m.nodes = append(m.nodes, explorerNode{
					label:    fmt.Sprintf("def %s(%s)%s%s  [L%d, %d LOC]", fn.Name, params, ret, decs, fn.LineNo, fn.LOC),
					nodeType: nodeTypeFunction,
					depth:    2,
					loc:      fn.LOC,
					lineNo:   fn.LineNo,
				})
			}

			// Constants
			if len(f.Constants) > 0 {
				m.nodes = append(m.nodes, explorerNode{
					label:    fmt.Sprintf("constants: %s", strings.Join(f.Constants, ", ")),
					nodeType: nodeTypeConstantsHeader,
					depth:    2,
				})
			}
		}
	}
}

func (m explorerModel) Update(msg tea.Msg) (explorerModel, tea.Cmd) {
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
			if m.cursor >= 0 && m.cursor < len(m.nodes) {
				node := m.nodes[m.cursor]
				if node.key != "" {
					m.expanded[node.key] = !m.expanded[node.key]
					m.rebuildNodes()
					if m.cursor >= len(m.nodes) {
						m.cursor = len(m.nodes) - 1
					}
				}
			}
		}
	}
	return m, nil
}

func (m explorerModel) visibleRows() int {
	rows := m.height - 2
	if rows < 1 {
		return 1
	}
	return rows
}

func (m explorerModel) View() string {
	if len(m.nodes) == 0 {
		return "\n  No files found.\n"
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

		var style lipgloss.Style
		switch {
		case i == m.cursor:
			style = explorerSelectedStyle
		case node.nodeType == nodeTypePackage:
			style = explorerPackageStyle
		case node.nodeType == nodeTypeFile:
			style = explorerFileStyle
		case node.nodeType == nodeTypeClass:
			style = explorerClassStyle
		case node.nodeType == nodeTypeFunction || node.nodeType == nodeTypeMethod:
			style = explorerFuncStyle
		default:
			style = explorerDefaultStyle
		}
		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}

	if len(m.nodes) > visible {
		b.WriteString(fmt.Sprintf("\n  %d/%d", m.cursor+1, len(m.nodes)))
	}

	return b.String()
}

var (
	explorerSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("212"))

	explorerPackageStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("75"))

	explorerFileStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("255"))

	explorerClassStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("222"))

	explorerFuncStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("114"))

	explorerDefaultStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245"))
)
