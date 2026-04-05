package tui

import "charm.land/lipgloss/v2"

var (
	colorPrimary   = lipgloss.Color("99")
	colorSecondary = lipgloss.Color("241")
	colorMuted     = lipgloss.Color("245")
	colorHighlight = lipgloss.Color("212")
	colorWhite     = lipgloss.Color("255")

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite).
			Background(colorPrimary).
			Padding(0, 1)

	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(0, 1)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(colorSecondary).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorSecondary).
				Padding(0, 1)

	tableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorPrimary)

	tableCellStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	tableSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorWhite).
				Background(colorPrimary)

	treeNodeStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	treeSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorHighlight)

	treeLabelStyle = lipgloss.NewStyle().
			Foreground(colorSecondary).
			Italic(true)

	helpKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(colorSecondary)
)
