// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

// profileclient to handle profiling of kubearmor telemetry events
package profileclient

import (
	"log"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evertras/bubble-table/table"
	pb "github.com/kubearmor/KubeArmor/protobuf"
	klog "github.com/kubearmor/kubearmor-client/log"
	profile "github.com/kubearmor/kubearmor-client/profile"
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
		BorderForeground(lipgloss.Color("#57f8c8")).
		Align(lipgloss.Right)
)

// Options for filter
type Options struct {
	Namespace string
	Pod       string
}

func waitForActivity() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(1 * time.Second)
		return klog.EventInfo{}
	}
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

// Row Data
type SomeData struct {
	rows []table.Row
}

var o1 Options

// Bubble tea Model initialization
func NewModel() Model {

	return Model{
		File:    table.New(generateColumns("File")).BorderRounded().WithBaseStyle(styleBase).WithPageSize(10).Filtered(true),
		Process: table.New(generateColumns("Process")).BorderRounded().WithBaseStyle(styleBase).WithPageSize(10),
		Network: table.New(generateColumns("Network")).BorderRounded().WithBaseStyle(styleBase).WithPageSize(10),
		tabs: &tabs{
			height: 3,
			active: "Lip Gloss",
			items:  []string{"Process", "File", "Network"},
		},
		keys:  keys,
		help:  help.New(),
		state: processview,
	}
}

func generateColumns(Operation string) []table.Column {
	CountCol := table.NewColumn(ColumnCount, "Count", 10).WithStyle(
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("#09ff00")).
			Align(lipgloss.Center))

	Namespace := table.NewColumn(ColumnNamespace, "Namespace", 20).WithStyle(
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("#09ff00")).
			Align(lipgloss.Center))

	PodName := table.NewColumn(ColumnPodname, "Podname", 40).WithStyle(
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("#09ff00")).
			Align(lipgloss.Center))

	ProcName := table.NewColumn(ColumnProcessName, "ProcessName", 30).WithStyle(
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("#09ff00")).
			Align(lipgloss.Center))

	Resource := table.NewColumn(ColumnResource, Operation, 60).WithStyle(
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("#57f8c8")).
			Align(lipgloss.Center))

	Result := table.NewColumn(ColumnResult, "Result", 10).WithStyle(
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("#09ff00")).
			Align(lipgloss.Center))

	Timestamp := table.NewColumn(ColumnTimestamp, "TimeStamp", 30).WithStyle(
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("#09ff00")).
			Align(lipgloss.Center))

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

// Initial functions to be called
func (m Model) Init() tea.Cmd {
	go profile.GetLogs()
	return tea.Batch(
		waitForActivity(),
	)
}

// Bubble Tea function to Update with incoming events
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
		msg.Height -= 2
		msg.Width -= 4
		m.help.Width = msg.Width
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.quitting = true
			return m, tea.Quit
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll

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
		m.File = m.File.SortByDesc(ColumnCount)
		m.Process = m.Process.WithRows(generateRowsFromData(profile.Telemetry, "Process")).WithColumns(generateColumns("Process"))
		m.Process = m.Process.SortByDesc(ColumnCount)
		m.Network = m.Network.WithRows(generateRowsFromData(profile.Telemetry, "Network")).WithColumns(generateColumns("Network"))
		m.Network = m.Network.SortByDesc(ColumnCount)
		profile.TelMutex.RUnlock()

		return m, waitForActivity()

	}

	return m, tea.Batch(cmds...)
}

// Render Bubble Tea UI
func (m Model) View() string {
	pad := lipgloss.NewStyle().Padding(1)

	helpKey := m.help.Styles.FullKey.Foreground(lipgloss.Color("#57f8c8")).PaddingLeft(1)
	help := lipgloss.JoinHorizontal(lipgloss.Left, helpKey.Render(m.help.FullHelpView(m.keys.FullHelp())))
	var total string
	switch m.state {

	case processview:
		s := lipgloss.NewStyle().MaxHeight(m.height).MaxWidth(m.width)
		total = s.Render(lipgloss.JoinVertical(lipgloss.Top, lipgloss.JoinVertical(lipgloss.Top,
			m.tabs.View(),
			lipgloss.JoinVertical(lipgloss.Center, pad.Render(m.Process.View()))),
			help,
		))
	case fileview:
		s := lipgloss.NewStyle().MaxHeight(m.height).MaxWidth(m.width)
		total = s.Render(lipgloss.JoinVertical(lipgloss.Top, lipgloss.JoinVertical(lipgloss.Top,
			m.tabs.View(),
			lipgloss.JoinVertical(lipgloss.Center, pad.Render(m.File.View()))),
			help,
		))
	case networkview:
		s := lipgloss.NewStyle().MaxHeight(m.height).MaxWidth(m.width)
		total = s.Render(lipgloss.JoinVertical(lipgloss.Top, lipgloss.JoinVertical(lipgloss.Top,
			m.tabs.View(),
			lipgloss.JoinVertical(lipgloss.Center, pad.Render(m.Network.View()))),
			help,
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

func Start(o Options) {
	os.Stderr = nil
	o1.Namespace = o.Namespace
	o1.Pod = o.Pod
	p := tea.NewProgram(NewModel(), tea.WithAltScreen())

	if err := p.Start(); err != nil {
		log.Fatal(err)
	}
}
