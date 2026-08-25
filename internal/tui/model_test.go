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

// loadedWithMarkers builds a list with session commands interleaved between
// prompts, including one at each end:
//
//	0 /clear      (marker)
//	1 first prompt
//	2 /model      (marker)
//	3 /plugin     (marker)
//	4 second prompt
//	5 /clear      (marker)
func loadedWithMarkers(t *testing.T) Model {
	t.Helper()
	project := claudelog.Project{Path: "/home/u/proj", Dir: "/tmp/proj", Sessions: 1}
	m := New([]claudelog.Project{project})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = next.(Model)

	marker := func(id, cmd string) claudelog.Turn {
		return claudelog.Turn{ID: id, Prompt: cmd, Command: true}
	}
	prompt := func(id, text string) claudelog.Turn {
		return claudelog.Turn{ID: id, Prompt: text, Blocks: []claudelog.Block{{Text: "ok"}}}
	}
	turns := []claudelog.Turn{
		marker("a", "/clear"),
		prompt("b", "first prompt"),
		marker("c", "/model"),
		marker("d", "/plugin"),
		prompt("e", "second prompt"),
		marker("f", "/clear"),
	}
	next, cmd := m.Update(turnsMsg{project: project, turns: turns})
	m = next.(Model)
	if cmd != nil {
		runCmd(cmd)
	}
	return m
}

func selected(t *testing.T, m Model) string {
	t.Helper()
	item, ok := m.turns.SelectedItem().(turnItem)
	if !ok {
		t.Fatal("no item selected")
	}
	return item.turn.Prompt
}

func TestCursorSkipsSessionCommands(t *testing.T) {
	m := loadedWithMarkers(t)

	// A list opening on a marker starts below it.
	if got := selected(t, m); got != "first prompt" {
		t.Errorf("opened on %q, want the first real prompt", got)
	}

	// Moving down passes over the two markers in between.
	m = send(t, m, "down")
	if got := selected(t, m); got != "second prompt" {
		t.Errorf("down landed on %q, want to skip /model and /plugin", got)
	}

	// The trailing marker is not reachable, so down stays put.
	m = send(t, m, "down")
	if got := selected(t, m); got != "second prompt" {
		t.Errorf("down landed on %q, want to stay off the trailing marker", got)
	}

	// Coming back up skips them again, and stops above the leading marker.
	m = send(t, m, "up")
	if got := selected(t, m); got != "first prompt" {
		t.Errorf("up landed on %q, want to skip back over the markers", got)
	}
	m = send(t, m, "up")
	if got := selected(t, m); got != "first prompt" {
		t.Errorf("up landed on %q, want to stay off the leading marker", got)
	}
}

// A marker has no response and nothing worth exporting, so the keys that act on
// a prompt must not act on one — even when a filter strands the cursor there.
func TestSessionCommandsAreInert(t *testing.T) {
	m := loadedWithMarkers(t)

	// Select-all covers the prompts only, never the markers.
	m = send(t, m, "a")
	if got := len(m.selectedTurns()); got != 2 {
		t.Errorf("select-all marked %d turns, want only the 2 prompts", got)
	}
	for _, turn := range m.selectedTurns() {
		if turn.Housekeeping() {
			t.Errorf("select-all marked the session command %q", turn.Prompt)
		}
	}
	m = send(t, m, "a")

	// Filter down to markers alone: the cursor has nowhere else to go.
	m = send(t, m, "/", "c", "l", "e", "a", "r", "enter")
	if got := selected(t, m); got != "/clear" {
		t.Fatalf("filter left the cursor on %q, want a stranded marker", got)
	}

	// Stranded there, none of the prompt keys may do anything.
	m = send(t, m, "space")
	if got := len(m.selectedTurns()); got != 0 {
		t.Errorf("space selected a session command (%d marked)", got)
	}
	m = send(t, m, "enter")
	if m.stage == stageResponse {
		t.Error("enter opened a response for a session command")
	}
	m = send(t, m, "a")
	if got := len(m.selectedTurns()); got != 0 {
		t.Errorf("select-all marked %d session commands, want none", got)
	}
	m = send(t, m, "e")
	if m.exporting {
		t.Error("export opened with only session commands in view")
	}
}
