package tui

import (
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// Bubble Tea v2 reports the space bar as "space": Key.String falls through to
// Keystroke for it rather than returning " ". A binding on " " silently never
// matches, so the key name is pinned here.
func TestBindingsMatchRealKeys(t *testing.T) {
	for _, tc := range []struct {
		name    string
		msg     tea.KeyPressMsg
		binding key.Binding
	}{
		{"space", tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}, keys.Toggle},
		{"enter", tea.KeyPressMsg{Code: tea.KeyEnter}, keys.Open},
		{"esc", tea.KeyPressMsg{Code: tea.KeyEscape}, keys.Back},
		{"a", tea.KeyPressMsg{Code: 'a', Text: "a"}, keys.ToggleAll},
		{"e", tea.KeyPressMsg{Code: 'e', Text: "e"}, keys.Export},
		{"q", tea.KeyPressMsg{Code: 'q', Text: "q"}, keys.Quit},
		{"ctrl+c", tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}, keys.ForceQuit},
	} {
		if !key.Matches(tc.msg, tc.binding) {
			t.Errorf("%s (String()=%q) does not match its binding %v",
				tc.name, tc.msg.String(), tc.binding.Keys())
		}
	}
}
