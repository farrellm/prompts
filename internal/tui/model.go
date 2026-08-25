// Package tui presents the prompt archive: pick a project, scan its prompts,
// read a response, and export the ones worth keeping.
package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/farrellm/prompts/internal/claudelog"
	"github.com/farrellm/prompts/internal/export"
)

type stage int

const (
	stageProjects stage = iota
	stageTurns
	stageResponse
)

// chromeHeight is the number of lines the header and footer take from the body.
const chromeHeight = 4

// Model is the root Bubble Tea model.
type Model struct {
	stage stage

	projects list.Model
	turns    list.Model
	viewport viewport.Model
	spinner  spinner.Model
	filename textinput.Model
	renderer *renderer

	project   claudelog.Project
	sel       selection
	loading   bool
	exporting bool
	status    string
	width     int
	height    int
}

// New builds the model around an already-enumerated project list, so that an
// empty archive can be reported before the terminal is taken over.
func New(projects []claudelog.Project) Model {
	items := make([]list.Item, len(projects))
	for i, p := range projects {
		items[i] = projectItem{project: p}
	}

	pl := newList(items, "project", "projects")
	tl := newList(nil, "prompt", "prompts")

	fn := textinput.New()
	fn.Prompt = "Export to: "

	return Model{
		projects: pl,
		turns:    tl,
		viewport: viewport.New(),
		spinner:  spinner.New(spinner.WithSpinner(spinner.Dot)),
		filename: fn,
		renderer: newRenderer(80, true),
		sel:      selection{},
	}
}

func newList(items []list.Item, singular, plural string) list.Model {
	d := list.NewDefaultDelegate()
	l := list.New(items, d, 0, 0)
	// The stage header carries the title, so the list only supplies its status
	// bar, pagination and filter field.
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetStatusBarItemName(singular, plural)
	// The list's own quit bindings would bypass the stage stack.
	l.DisableQuitKeybindings()
	return l
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(tea.RequestBackgroundColor, m.spinner.Tick)
}

// turnsMsg carries the result of loading one project's transcripts.
type turnsMsg struct {
	project claudelog.Project
	turns   []claudelog.Turn
	err     error
}

// exportMsg carries the result of writing an export file.
type exportMsg struct {
	path string
	n    int
	err  error
}

func loadTurns(p claudelog.Project) tea.Cmd {
	return func() tea.Msg {
		turns, err := claudelog.LoadProject(p.Dir)
		return turnsMsg{project: p, turns: turns, err: err}
	}
}

func writeExport(path, project string, turns []claudelog.Turn) tea.Cmd {
	return func() tea.Msg {
		full, err := expand(path)
		if err != nil {
			return exportMsg{path: path, err: err}
		}
		if err := export.Write(full, project, turns); err != nil {
			return exportMsg{path: full, err: err}
		}
		return exportMsg{path: full, n: len(turns)}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		return m, nil

	case tea.BackgroundColorMsg:
		m.renderer.setDark(msg.IsDark())
		if m.stage == stageResponse {
			m.showResponse()
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case turnsMsg:
		return m.turnsLoaded(msg)

	case exportMsg:
		m.exporting = false
		if msg.err != nil {
			m.status = "Export failed: " + msg.err.Error()
		} else {
			m.status = fmt.Sprintf("Wrote %s to %s", plural(msg.n, "prompt"), msg.path)
		}
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}

	return m.delegate(msg)
}

func (m Model) turnsLoaded(msg turnsMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.status = "Could not read " + msg.project.Path + ": " + msg.err.Error()
		return m, nil
	}

	m.project = msg.project
	m.sel = selection{}
	items := make([]list.Item, len(msg.turns))
	for i, t := range msg.turns {
		items[i] = turnItem{turn: t, sel: m.sel}
	}

	cmd := m.turns.SetItems(items)
	m.turns.ResetFilter()
	m.turns.ResetSelected()
	m.stage = stageTurns
	m.status = ""
	if len(items) == 0 {
		m.status = "No prompts recorded in this project."
	}
	return m, cmd
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// The export field owns every key while it is open.
	if m.exporting {
		return m.handleExportKey(msg)
	}
	// So does the list's filter field.
	if m.filtering() {
		return m.delegate(msg)
	}

	// ctrl+c always quits; q quits from the lists but backs out of a response,
	// where it reads as a pager key.
	if key.Matches(msg, keys.ForceQuit) {
		return m, tea.Quit
	}
	if key.Matches(msg, keys.Quit) && m.stage != stageResponse {
		return m, tea.Quit
	}
	m.status = ""

	switch m.stage {
	case stageProjects:
		if key.Matches(msg, keys.Open) && !m.loading {
			item, ok := m.projects.SelectedItem().(projectItem)
			if !ok {
				return m, nil
			}
			m.loading = true
			return m, tea.Batch(loadTurns(item.project), m.spinner.Tick)
		}

	case stageTurns:
		switch {
		case key.Matches(msg, keys.Back):
			// An applied filter is the first thing esc should undo; only once
			// the full list is showing does esc leave the project.
			if m.turns.FilterState() != list.Unfiltered {
				return m.delegate(msg)
			}
			m.stage = stageProjects
			m.sel = selection{}
			return m, nil
		case key.Matches(msg, keys.Open):
			if _, ok := m.turns.SelectedItem().(turnItem); ok {
				m.stage = stageResponse
				m.showResponse()
			}
			return m, nil
		case key.Matches(msg, keys.Toggle):
			m.toggleCurrent()
			return m, nil
		case key.Matches(msg, keys.ToggleAll):
			m.toggleVisible()
			return m, nil
		case key.Matches(msg, keys.Export):
			return m.beginExport()
		}

	case stageResponse:
		switch {
		case key.Matches(msg, keys.Back), key.Matches(msg, keys.Quit):
			m.stage = stageTurns
			return m, nil
		case key.Matches(msg, keys.Open):
			// Enter already opened this view; swallow it rather than letting
			// the viewport treat it as a scroll.
			return m, nil
		case key.Matches(msg, keys.Toggle):
			m.toggleCurrent()
			return m, nil
		case key.Matches(msg, keys.Export):
			return m.beginExport()
		}
	}

	return m.delegate(msg)
}

func (m Model) handleExportKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Cancel):
		m.exporting = false
		m.filename.Blur()
		return m, nil
	case key.Matches(msg, keys.Confirm):
		path := strings.TrimSpace(m.filename.Value())
		if path == "" {
			return m, nil
		}
		m.filename.Blur()
		return m, writeExport(path, m.project.Path, m.selectedTurns())
	}

	var cmd tea.Cmd
	m.filename, cmd = m.filename.Update(msg)
	return m, cmd
}

// delegate forwards a message to whichever component owns the current stage.
func (m Model) delegate(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.stage {
	case stageProjects:
		m.projects, cmd = m.projects.Update(msg)
	case stageTurns:
		m.turns, cmd = m.turns.Update(msg)
	case stageResponse:
		m.viewport, cmd = m.viewport.Update(msg)
	}
	return m, cmd
}

func (m Model) beginExport() (tea.Model, tea.Cmd) {
	turns := m.selectedTurns()
	if len(turns) == 0 {
		m.status = "Nothing selected — press space to mark prompts for export."
		return m, nil
	}

	m.exporting = true
	m.filename.SetValue(defaultFilename(m.project.Path))
	m.filename.CursorEnd()
	return m, m.filename.Focus()
}

// toggleCurrent marks or unmarks the highlighted turn.
func (m *Model) toggleCurrent() {
	item, ok := m.currentItem()
	if !ok {
		return
	}
	if m.sel[item.turn.ID] {
		delete(m.sel, item.turn.ID)
	} else {
		m.sel[item.turn.ID] = true
	}
}

// toggleVisible marks every turn matching the active filter, or clears them if
// they are already all marked.
func (m *Model) toggleVisible() {
	visible := m.turns.VisibleItems()
	if len(visible) == 0 {
		return
	}

	allMarked := true
	for _, it := range visible {
		if item, ok := it.(turnItem); ok && !m.sel[item.turn.ID] {
			allMarked = false
			break
		}
	}
	for _, it := range visible {
		item, ok := it.(turnItem)
		if !ok {
			continue
		}
		if allMarked {
			delete(m.sel, item.turn.ID)
		} else {
			m.sel[item.turn.ID] = true
		}
	}
}

func (m Model) currentItem() (turnItem, bool) {
	item, ok := m.turns.SelectedItem().(turnItem)
	return item, ok
}

// selectedTurns collects the marked turns in list order.
func (m Model) selectedTurns() []claudelog.Turn {
	var turns []claudelog.Turn
	for _, it := range m.turns.Items() {
		if item, ok := it.(turnItem); ok && m.sel[item.turn.ID] {
			turns = append(turns, item.turn)
		}
	}
	return turns
}

// showResponse renders the highlighted turn into the viewport.
func (m *Model) showResponse() {
	item, ok := m.currentItem()
	if !ok {
		return
	}
	m.viewport.SetContent(m.renderer.render(item.turn))
	m.viewport.GotoTop()
}

func (m Model) filtering() bool {
	return m.stage == stageTurns && m.turns.SettingFilter() ||
		m.stage == stageProjects && m.projects.SettingFilter()
}

// layout distributes the terminal between the chrome and the body.
func (m *Model) layout() {
	body := m.height - chromeHeight
	if body < 3 {
		body = 3
	}

	m.projects.SetSize(m.width, body)
	m.turns.SetSize(m.width, body)
	m.viewport.SetWidth(m.width)
	m.viewport.SetHeight(body)
	m.filename.SetWidth(max(m.width-16, 20))

	m.renderer.resize(m.width - 4)
	if m.stage == stageResponse {
		m.showResponse()
	}
}

// defaultFilename proposes an export path named after the project and today.
func defaultFilename(project string) string {
	base := filepath.Base(project)
	if base == "." || base == string(filepath.Separator) || base == "" {
		base = "claude"
	}
	return fmt.Sprintf("claude-prompts-%s-%s.md", base, time.Now().Format("2006-01-02"))
}

// expand resolves a leading ~ and makes the path absolute.
func expand(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~"))
	}
	return filepath.Abs(path)
}
