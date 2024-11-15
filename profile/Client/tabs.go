// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package profileclient

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	activeTabBorder = lipgloss.Border{
		Top:         "─",
		Bottom:      " ",
		Left:        "│",
		Right:       "│",
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "┘",
		BottomRight: "└",
	}

	tabBorder = lipgloss.Border{
		Top:         "─",
		Bottom:      "─",
		Left:        "│",
		Right:       "│",
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "┴",
		BottomRight: "┴",
	}
)

type tabs struct {
	width int

	active string
	cursor int
	items  []string
}

func (m tabs) Init() tea.Cmd {
	return nil
}

func (m tabs) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			} else if m.cursor == len(m.items)-1 {
				m.cursor = 0
			}
			return m, nil
		}
	}
	return m, nil
}

func (m tabs) View() string {
	tab := lipgloss.NewStyle().
		Border(tabBorder, true).
		Padding(0, 4).Bold(true)

	activeTab := tab.Copy().Border(activeTabBorder, true)

	tabGap := tab.Copy().
		BorderTop(false).
		BorderLeft(false).
		BorderRight(false)

	out := []string{}
	cursor := " "
	for i, item := range m.items {
		if m.cursor == i {
			cursor = activeTab.Render(item)
			m.active = item
		}
	}

	for _, item := range m.items {
		if item == m.active {
			out = append(out, cursor)
		} else {
			out = append(out, tab.Render(item))
		}
	}

	row := lipgloss.JoinHorizontal(lipgloss.Top, out...)
	gap := tabGap.Render(strings.Repeat(" ", max(0, m.width-lipgloss.Width(row)-2)))
	row = lipgloss.JoinHorizontal(lipgloss.Bottom, row, gap)
	return row
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
