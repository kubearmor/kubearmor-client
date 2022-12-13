// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package profileclient

import (
	"github.com/charmbracelet/bubbles/key"
)

type keyMap struct {
	Quit  key.Binding
	Help  key.Binding
	Tab   key.Binding
	Arrow key.Binding
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Quit, k.Tab, k.Arrow}}
}

var keys = keyMap{
	Quit: key.NewBinding(
		key.WithKeys("q", "esc", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Arrow: key.NewBinding(
		key.WithKeys(""),
		key.WithHelp("Arrow Keys or h j k l", "scrolling through the table"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("Tab", "Change Operation"),
	),
}
