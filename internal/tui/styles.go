package tui

import "charm.land/lipgloss/v2"

var (
	accent = lipgloss.Color("62")
	subtle = lipgloss.Color("241")
	danger = lipgloss.Color("203")
	check  = lipgloss.Color("42")

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("230")).
			Background(accent).
			Padding(0, 1)

	// breadcrumbStyle labels which project the prompt list is showing.
	breadcrumbStyle = lipgloss.NewStyle().Foreground(subtle).Padding(0, 0, 0, 2)

	helpStyle = lipgloss.NewStyle().Foreground(subtle).Padding(0, 0, 0, 2)

	errStyle = lipgloss.NewStyle().Foreground(danger).Padding(1, 2)

	selectedMark = lipgloss.NewStyle().Foreground(check).SetString("●")
	unselectMark = lipgloss.NewStyle().Foreground(subtle).SetString("○")

	// promptStyle sets the prompt apart from the response in the reading view.
	promptStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(accent).
			Foreground(lipgloss.Color("252")).
			Padding(0, 0, 0, 1).
			MarginBottom(1)

	metaStyle = lipgloss.NewStyle().Foreground(subtle).MarginBottom(1)

	// housekeepingStyle marks the session commands the cursor skips over.
	housekeepingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true)

	toolStyle      = lipgloss.NewStyle().Foreground(accent)
	toolArgStyle   = lipgloss.NewStyle().Foreground(subtle)
	exportBoxStyle = lipgloss.NewStyle().Padding(1, 2)
)
