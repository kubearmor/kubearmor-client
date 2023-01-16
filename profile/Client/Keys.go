// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package profileclient

import (
	"github.com/charmbracelet/bubbles/key"
)

type keyMap struct {
	Quit   key.Binding
	Help   key.Binding
	Tab    key.Binding
	Arrow  key.Binding
	MaxRow key.Binding
	Filter key.Binding
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Quit, k.Tab, k.Arrow, k.MaxRow, k.Filter}}
}

var keys = keyMap{
	Quit: key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("", "(ctrl + c)quit"),
	),
	Arrow: key.NewBinding(
		key.WithKeys(""),
		key.WithHelp("", "(arrow keys or h j k l) scrolling through the table"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("", "(Tab) Change Operation"),
	),
	MaxRow: key.NewBinding(
		key.WithKeys(""),
		key.WithHelp("", "(i)increase or (u)decrease max rows per page"),
	),
	Filter: key.NewBinding(
		key.WithKeys(""),
		key.WithHelp("", "(/) To filter the tables, Press <Esc> to clear filter"),
	),
}
