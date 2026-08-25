package tui

import "charm.land/bubbles/v2/key"

type keyMap struct {
	Open      key.Binding
	Back      key.Binding
	Toggle    key.Binding
	ToggleAll key.Binding
	Export    key.Binding
	Quit      key.Binding
	ForceQuit key.Binding
	Confirm   key.Binding
	Cancel    key.Binding
}

var keys = keyMap{
	Open:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
	Back:      key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	Toggle:    key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "select")),
	ToggleAll: key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "select all")),
	Export:    key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "export")),
	Quit:      key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
	ForceQuit: key.NewBinding(key.WithKeys("ctrl+c")),
	Confirm:   key.NewBinding(key.WithKeys("enter")),
	Cancel:    key.NewBinding(key.WithKeys("esc", "ctrl+c")),
}
