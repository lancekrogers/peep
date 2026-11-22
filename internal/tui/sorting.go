package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/lancekrogers/peep/internal/browser"
)

func (m *Model) sortLabel() string {
	if m.filter != "" && !browser.IsGlob(m.filter) && !m.sortExplicit {
		return "Relevance"
	}
	field := m.sortField
	if field == "" {
		field = browser.SortName
	}
	direction := " ↑"
	if m.sortDescending {
		direction = " ↓"
	}
	return string(field) + direction
}
func (m *Model) sortEntries() {
	if m.filter == "" || browser.IsGlob(m.filter) || m.sortExplicit {
		browser.Sort(m.entries, m.sortField, m.sortDescending)
	}
}
func (m *Model) sortKey(key string) tea.Cmd {
	switch key {
	case "esc", "S", "enter":
		m.sortMenu = false
		return nil
	case "n":
		m.sortField = browser.SortName
		m.sortDescending = false
	case "m":
		m.sortField = browser.SortModified
		m.sortDescending = true
	case "c":
		m.sortField = browser.SortCreated
		m.sortDescending = true
	case "s":
		m.sortField = browser.SortSize
		m.sortDescending = true
	case "t":
		m.sortField = browser.SortType
		m.sortDescending = false
	case "r":
		m.sortDescending = !m.sortDescending
	default:
		return nil
	}
	m.sortExplicit = true
	m.sortMenu = false
	return m.refilter()
}
