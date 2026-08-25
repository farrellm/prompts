package tui

import (
	"fmt"
	"io"

	"charm.land/bubbles/v2/list"
)

// turnDelegate renders prompts the way the default delegate does, and renders
// housekeeping commands as a single dim line.
//
// The list reserves Height() rows per item, so a one-line marker leaves a row
// unfilled at the bottom of a page. That is only ever cosmetic: the list joins
// whatever each delegate writes, so nothing overlaps or is lost.
type turnDelegate struct {
	list.DefaultDelegate
}

func newTurnDelegate() turnDelegate {
	return turnDelegate{DefaultDelegate: list.NewDefaultDelegate()}
}

func (d turnDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	turn, ok := item.(turnItem)
	if !ok || !turn.turn.Housekeeping() {
		d.DefaultDelegate.Render(w, m, index, item)
		return
	}

	width := m.Width() - 4
	if width < 8 {
		width = 8
	}
	fmt.Fprint(w, housekeepingStyle.Render("  "+truncate(firstLine(turn.turn.Prompt), width)))
}
