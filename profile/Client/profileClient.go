// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

// Package profileclient to handle profiling of kubearmor telemetry events
package profileclient

import (
	"fmt"
	"log"
	"os"
	"time"

	klog "github.com/accuknox/accuknox-cli/log"
	profile "github.com/accuknox/accuknox-cli/profile"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evertras/bubble-table/table"
	pb "github.com/kubearmor/KubeArmor/protobuf"
)

// Column keys
const (
	ColumnNamespace   = "Namespace"
	ColumnPodname     = "Podname"
	ColumnProcessName = "Procname"
	ColumnResource    = "Resource"
	ColumnResult      = "Result"
	ColumnCount       = "Count"
	ColumnTimestamp   = "Timestamp"
)

// session state for switching views
type sessionState uint

// Manage Bubble Tea display state
const (
	processview sessionState = iota
	fileview
	networkview
)

var (
	styleBase = lipgloss.NewStyle().
			BorderForeground(lipgloss.Color("12")).
			Align(lipgloss.Right)
	//ColumnStyle for column color
	ColumnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00af00")).Align(lipgloss.Center).Bold(true)

	helptheme = lipgloss.AdaptiveColor{
		Light: "#000000",
		Dark:  "#ffffff",
	}
)

// Options for filter
type Options struct {
	Namespace string
	Pod       string
	GRPC      string
}

// Model for main Bubble Tea
type Model struct {
	File     table.Model
	Process  table.Model
	Network  table.Model
	tabs     tea.Model
	keys     keyMap
	quitting bool
	help     help.Model

	height int
	width  int

	state sessionState
}

// SomeData stores incoming row data
type SomeData struct {
	rows []table.Row
}

func waitForActivity() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(2 * time.Second)
		return klog.EventInfo{}
	}
}

var o1 Options

func generateColumns(Operation string) []table.Column {
	CountCol := table.NewFlexColumn(ColumnCount, "Count", 1).WithStyle(ColumnStyle).WithFiltered(true)

	Namespace := table.NewFlexColumn(ColumnNamespace, "Namespace", 2).WithStyle(ColumnStyle).WithFiltered(true)

	PodName := table.NewFlexColumn(ColumnPodname, "Podname", 4).WithStyle(ColumnStyle).WithFiltered(true)

	ProcName := table.NewFlexColumn(ColumnProcessName, "ProcessName", 3).WithStyle(ColumnStyle).WithFiltered(true)

	Resource := table.NewFlexColumn(ColumnResource, Operation, 6).WithStyle(
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("202")).
			Align(lipgloss.Center)).WithFiltered(true)

	Result := table.NewFlexColumn(ColumnResult, "Result", 1).WithStyle(ColumnStyle).WithFiltered(true)

	Timestamp := table.NewFlexColumn(ColumnTimestamp, "TimeStamp", 3).WithStyle(ColumnStyle)

	return []table.Column{
		Namespace,
		PodName,
		ProcName,
		Resource,
		Result,
		CountCol,
		Timestamp,
	}
}

// Init calls initial functions if needed
func (m Model) Init() tea.Cmd {
	go profile.GetLogs(o1.GRPC)
	return tea.Batch(
		waitForActivity(),
	)
}

// NewModel initializates new bubbletea model
func NewModel() Model {
	model := Model{
		File:    table.New(generateColumns("File")).WithBaseStyle(styleBase).WithPageSize(30).Filtered(true),
		Process: table.New(generateColumns("Process")).WithBaseStyle(styleBase).WithPageSize(30).Filtered(true),
		Network: table.New(generateColumns("Network")).WithBaseStyle(styleBase).WithPageSize(30).Filtered(true),
		tabs: &tabs{
			active: "Lip Gloss",
			items:  []string{"Process", "File", "Network"},
		},
		keys:  keys,
		help:  help.New(),
		state: processview,
	}

	return model
}

// Update Bubble Tea function to Update with incoming events
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)
	m.tabs, _ = m.tabs.Update(msg)
	cmds = append(cmds, cmd)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
		m.help.Width = msg.Width
		m.recalculateTable()
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.quitting = true
			return m, tea.Quit

		}

		switch msg.String() {

		case "tab":
			switch m.state {
			case processview:
				m.state = fileview
			case fileview:
				m.state = networkview
			case networkview:
				m.state = processview
			}

		case "u":
			m.File = m.File.WithPageSize(m.File.PageSize() - 1)
			m.Network = m.Network.WithPageSize(m.Network.PageSize() - 1)
			m.Process = m.Process.WithPageSize(m.Process.PageSize() - 1)

		case "i":
			m.File = m.File.WithPageSize(m.File.PageSize() + 1)
			m.Network = m.Network.WithPageSize(m.Network.PageSize() + 1)
			m.Process = m.Process.WithPageSize(m.Process.PageSize() + 1)
		}

		switch m.state {
		case processview:
			m.Process = m.Process.Focused(true)
			m.Process, cmd = m.Process.Update(msg)
			cmds = append(cmds, cmd)
		case fileview:
			m.File = m.File.Focused(true)
			m.File, cmd = m.File.Update(msg)
			cmds = append(cmds, cmd)

		case networkview:
			m.Network = m.Network.Focused(true)
			m.Network, cmd = m.Network.Update(msg)
			cmds = append(cmds, cmd)
		}
	case klog.EventInfo:
		profile.TelMutex.RLock()
		m.File = m.File.WithRows(generateRowsFromData(profile.Telemetry, "File")).WithColumns(generateColumns("File"))
		m.File = m.File.SortByDesc(ColumnCount).ThenSortByDesc(ColumnResource).ThenSortByDesc(ColumnProcessName).ThenSortByDesc(ColumnPodname).ThenSortByDesc(ColumnNamespace)
		m.Process = m.Process.WithRows(generateRowsFromData(profile.Telemetry, "Process")).WithColumns(generateColumns("Process"))
		m.Process = m.Process.SortByDesc(ColumnCount).ThenSortByDesc(ColumnResource).ThenSortByDesc(ColumnProcessName).ThenSortByDesc(ColumnPodname).ThenSortByDesc(ColumnNamespace)
		m.Network = m.Network.WithRows(generateRowsFromData(profile.Telemetry, "Network")).WithColumns(generateColumns("Network"))
		m.Network = m.Network.SortByDesc(ColumnCount).ThenSortByDesc(ColumnResource).ThenSortByDesc(ColumnProcessName).ThenSortByDesc(ColumnPodname).ThenSortByDesc(ColumnNamespace)
		profile.TelMutex.RUnlock()

		return m, waitForActivity()

	}

	return m, tea.Batch(cmds...)
}

func (m *Model) recalculateTable() {
	m.File = m.File.WithTargetWidth(m.width)
	m.Network = m.Network.WithTargetWidth(m.width)
	m.Process = m.Process.WithTargetWidth(m.width)
}

// View Renders Bubble Tea UI
func (m Model) View() string {
	pad := lipgloss.NewStyle().PaddingRight(1)
	RowCount := lipgloss.JoinHorizontal(lipgloss.Left, lipgloss.NewStyle().Foreground(helptheme).Render(fmt.Sprintf("Max Rows: %d", m.Process.PageSize())))
	helpKey := m.help.Styles.FullDesc.Foreground(helptheme).Padding(0, 0, 1)
	help := lipgloss.JoinHorizontal(lipgloss.Left, helpKey.Render(m.help.FullHelpView(m.keys.FullHelp())))
	var total string
	s := lipgloss.NewStyle().Height(m.height).MaxHeight(m.height)
	switch m.state {

	case processview:

		total = s.Render(lipgloss.JoinVertical(lipgloss.Top, lipgloss.JoinVertical(lipgloss.Top,
			help,
			RowCount,
			m.tabs.View(),
			lipgloss.JoinVertical(lipgloss.Center, pad.Render(m.Process.View()))),
		))
	case fileview:
		// s := lipgloss.NewStyle().MaxHeight(m.height).MaxWidth(m.width)
		total = s.Render(lipgloss.JoinVertical(lipgloss.Top, lipgloss.JoinVertical(lipgloss.Top,
			help,
			RowCount,
			m.tabs.View(),
			lipgloss.JoinVertical(lipgloss.Center, pad.Render(m.File.View()))),
		))
	case networkview:
		// s := lipgloss.NewStyle().MaxHeight(m.height).MaxWidth(m.width)
		total = s.Render(lipgloss.JoinVertical(lipgloss.Top, lipgloss.JoinVertical(lipgloss.Top,
			help,
			RowCount,
			m.tabs.View(),
			lipgloss.JoinVertical(lipgloss.Center, pad.Render(m.Network.View()))),
		))
	}
	return total

}

// Profile Row Data to display
type Profile struct {
	Namespace string
	PodName   string
	Process   string
	Resource  string
	Result    string
}

// Frequency and Timestamp data for another map
type Frequency struct {
	freq int
	time string
}

func generateRowsFromData(data []pb.Log, Operation string) []table.Row {
	var s SomeData
	m := make(map[Profile]int)
	w := make(map[Profile]*Frequency)
	for _, entry := range data {

		if (entry.Operation == Operation && entry.NamespaceName == o1.Namespace) ||
			(entry.Operation == Operation && entry.PodName == o1.Pod) ||
			(entry.Operation == Operation && len(o1.Namespace) == 0 && len(o1.Pod) == 0) {

			p := Profile{
				Namespace: entry.NamespaceName,
				PodName:   entry.PodName,
				Process:   entry.ProcessName,
				Resource:  entry.Resource,
				Result:    entry.Result,
			}
			f := &Frequency{
				time: entry.UpdatedTime,
			}
			w[p] = f
			m[p]++
			w[p].freq = m[p]
		}
	}

	for r, frequency := range w {
		row := table.NewRow(table.RowData{
			ColumnNamespace:   r.Namespace,
			ColumnPodname:     r.PodName,
			ColumnProcessName: r.Process,
			ColumnResource:    r.Resource,
			ColumnResult:      r.Result,
			ColumnCount:       frequency.freq,
			ColumnTimestamp:   frequency.time,
		})
		s.rows = append(s.rows, row)
	}
	return s.rows
}

// Start entire TUI
func Start(o Options) {
	os.Stderr = nil
	o1.Namespace = o.Namespace
	o1.Pod = o.Pod
	o1.GRPC = o.GRPC
	p := tea.NewProgram(NewModel(), tea.WithAltScreen())

	if err := p.Start(); err != nil {
		log.Fatal(err)
	}
}
