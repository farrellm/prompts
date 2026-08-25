package tui

import (
	"fmt"
	"strings"

	"github.com/farrellm/prompts/internal/claudelog"
)

// selection tracks which turns are marked for export. It is shared by reference
// with the list items so that toggling redraws without rebuilding the list.
type selection map[string]bool

type projectItem struct {
	project claudelog.Project
}

func (i projectItem) Title() string { return i.project.Path }

func (i projectItem) Description() string {
	return fmt.Sprintf("%s · last active %s",
		plural(i.project.Sessions, "session"),
		i.project.Modified.Format("Jan 2"))
}

func (i projectItem) FilterValue() string { return i.project.Path }

type turnItem struct {
	turn claudelog.Turn
	sel  selection
}

func (i turnItem) Title() string {
	mark := unselectMark
	if i.sel[i.turn.ID] {
		mark = selectedMark
	}
	return mark.String() + " " + firstLine(i.turn.Prompt)
}

func (i turnItem) Description() string {
	return fmt.Sprintf("   %s · %s",
		i.turn.SessionTitle,
		i.turn.Time.Format("Jan 2 15:04"))
}

// FilterValue searches the whole prompt, not just the line shown, so a filter
// can reach text that scrolled off the title.
func (i turnItem) FilterValue() string {
	return i.turn.Prompt + " " + i.turn.SessionTitle
}

// firstLine reduces a prompt to the single line shown in the list.
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return "(empty prompt)"
}

func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
