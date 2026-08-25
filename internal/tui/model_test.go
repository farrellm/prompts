package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/farrellm/prompts/internal/claudelog"
)

func press(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	default:
		r := []rune(s)[0]
		return tea.KeyPressMsg{Code: r, Text: s}
	}
}

// runCmd executes a command, giving up on the timer-driven ones — the cursor
// blink and the spinner tick — which never settle and which nothing here needs.
func runCmd(cmd tea.Cmd) tea.Msg {
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case msg := <-done:
		return msg
	case <-time.After(100 * time.Millisecond):
		return nil
	}
}

// send applies a sequence of keys, draining the commands each one returns so
// that list filtering — which filters asynchronously — settles before the next
// key is delivered.
func send(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	for _, k := range keys {
		next, cmd := m.Update(press(k))
		m = next.(Model)

		pending := []tea.Cmd{cmd}
		for i := 0; len(pending) > 0 && i < 50; i++ {
			cmd, pending = pending[0], pending[1:]
			if cmd == nil {
				continue
			}
			msg := runCmd(cmd)
			if msg == nil {
				continue
			}
			// A batch fans out into the commands it holds.
			if batch, ok := msg.(tea.BatchMsg); ok {
				pending = append(pending, batch...)
				continue
			}
			next, cmd = m.Update(msg)
			m = next.(Model)
			pending = append(pending, cmd)
		}
	}
	return m
}

func loaded(t *testing.T) Model {
	t.Helper()
	project := claudelog.Project{Path: "/home/u/proj", Dir: "/tmp/proj", Sessions: 1}
	m := New([]claudelog.Project{project})

	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(Model)

	turns := []claudelog.Turn{
		{ID: "1", Prompt: "refactor the parser", Time: time.Now()},
		{ID: "2", Prompt: "add a README", Time: time.Now().Add(-time.Hour)},
		{ID: "3", Prompt: "refactor the exporter", Time: time.Now().Add(-2 * time.Hour)},
	}
	next, cmd := m.Update(turnsMsg{project: project, turns: turns})
	m = next.(Model)
	if cmd != nil {
		runCmd(cmd)
	}
	if m.stage != stageTurns {
		t.Fatalf("stage = %v, want stageTurns", m.stage)
	}
	return m
}

// esc has two jobs in the prompt list, and the filter has to win: backing out
// of the project while a filter is applied loses the user's place.
func TestEscapeClearsFilterBeforeLeavingProject(t *testing.T) {
	m := loaded(t)

	m = send(t, m, "/", "r", "e", "f", "a", "c", "t", "o", "r", "enter")
	if m.turns.FilterState() == list.Unfiltered {
		t.Fatalf("filter did not apply")
	}

	m = send(t, m, "esc")
	if m.stage != stageTurns {
		t.Errorf("esc left the project while a filter was applied")
	}
	if m.turns.FilterState() != list.Unfiltered {
		t.Errorf("esc did not clear the applied filter")
	}

	m = send(t, m, "esc")
	if m.stage != stageProjects {
		t.Errorf("esc on an unfiltered list should return to the projects")
	}
}

func TestSelectionAndExportGuard(t *testing.T) {
	m := loaded(t)

	// Exporting with nothing marked explains itself rather than writing a file.
	m = send(t, m, "e")
	if m.exporting {
		t.Error("export opened with nothing selected")
	}
	if !strings.Contains(m.status, "Nothing selected") {
		t.Errorf("status = %q, want an explanation", m.status)
	}

	m = send(t, m, "space")
	if len(m.selectedTurns()) != 1 {
		t.Fatalf("space selected %d turns, want 1", len(m.selectedTurns()))
	}
	m = send(t, m, "space")
	if len(m.selectedTurns()) != 0 {
		t.Errorf("space did not deselect")
	}

	// Select-all is scoped to the filter, and toggles off when all are marked.
	m = send(t, m, "/", "r", "e", "f", "a", "c", "t", "o", "r", "enter", "a")
	if got := len(m.selectedTurns()); got != 2 {
		t.Errorf("select-all marked %d turns, want the 2 matching the filter", got)
	}
	m = send(t, m, "a")
	if got := len(m.selectedTurns()); got != 0 {
		t.Errorf("select-all again should clear, got %d", got)
	}

	// Selection survives clearing the filter, and export then opens.
	m = send(t, m, "a", "esc", "e")
	if !m.exporting {
		t.Fatal("export did not open with turns selected")
	}
	if !strings.HasSuffix(m.filename.Value(), ".md") || !strings.Contains(m.filename.Value(), "proj") {
		t.Errorf("default filename = %q, want a project-named .md", m.filename.Value())
	}

	// Escape abandons the export without writing.
	m = send(t, m, "esc")
	if m.exporting {
		t.Error("esc did not cancel the export")
	}
}

// Leaving a project must not carry its selection into the next one.
func TestSelectionResetsOnLeavingProject(t *testing.T) {
	m := send(t, loaded(t), "space", "esc")
	if m.stage != stageProjects {
		t.Fatalf("stage = %v, want stageProjects", m.stage)
	}
	if len(m.sel) != 0 {
		t.Errorf("selection survived leaving the project: %v", m.sel)
	}
}

func TestEnterOpensAndLeavesResponse(t *testing.T) {
	m := send(t, loaded(t), "enter")
	if m.stage != stageResponse {
		t.Fatalf("enter did not open the response")
	}
	if !strings.Contains(m.viewport.GetContent(), "refactor the parser") {
		t.Errorf("response view is not showing the selected prompt")
	}

	// Space still marks the turn being read.
	m = send(t, m, "space")
	if len(m.selectedTurns()) != 1 {
		t.Errorf("space in the response view did not select the turn")
	}

	if m = send(t, m, "q"); m.stage != stageTurns {
		t.Errorf("q should back out of a response, not quit")
	}
}
