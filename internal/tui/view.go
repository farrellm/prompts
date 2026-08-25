package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func (m Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	return v
}

func (m Model) render() string {
	if m.width == 0 {
		// The first window size has not arrived yet.
		return ""
	}

	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString("\n")
	b.WriteString(m.body())
	b.WriteString("\n")
	b.WriteString(m.footer())
	return b.String()
}

func (m Model) header() string {
	switch m.stage {
	case stageProjects:
		return titleStyle.Render("Claude Code prompts") + "\n"
	default:
		crumb := m.project.Path
		if n := len(m.sel); n > 0 {
			crumb += fmt.Sprintf("   %s selected", plural(n, "prompt"))
		}
		return titleStyle.Render("Claude Code prompts") + "\n" + breadcrumbStyle.Render(crumb)
	}
}

func (m Model) body() string {
	if m.loading {
		return "\n" + helpStyle.Render(m.spinner.View()+" Reading transcripts…")
	}
	switch m.stage {
	case stageProjects:
		return m.projects.View()
	case stageTurns:
		return m.turns.View()
	default:
		return m.viewport.View()
	}
}

func (m Model) footer() string {
	if m.exporting {
		return exportBoxStyle.Render(m.filename.View() + "\n" +
			helpStyle.Render("enter  write   esc  cancel"))
	}
	if m.status != "" {
		return helpStyle.Render(truncateLine(m.status, m.width-4))
	}
	return helpStyle.Render(m.help())
}

func (m Model) help() string {
	switch m.stage {
	case stageProjects:
		return join("↑/↓  move", "/  filter", "enter  open", "q  quit")
	case stageTurns:
		return join("↑/↓  move", "/  filter", "enter  read", "space  select", "a  all", "e  export", "esc  back")
	default:
		return join("↑/↓  scroll", "space  select", "e  export", "esc/q  back")
	}
}

func join(parts ...string) string {
	return strings.Join(parts, lipgloss.NewStyle().Foreground(subtle).Render("  ·  "))
}

func truncateLine(s string, width int) string {
	if width < 8 {
		width = 8
	}
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	return string(r[:width-1]) + "…"
}
