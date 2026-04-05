package tui

import "strings"

type helpItem struct {
	key  string
	desc string
}

func renderHelp(items []helpItem) string {
	var parts []string
	for _, item := range items {
		parts = append(parts, helpKeyStyle.Render(item.key)+helpDescStyle.Render(": "+item.desc))
	}
	return strings.Join(parts, helpDescStyle.Render("  |  "))
}
