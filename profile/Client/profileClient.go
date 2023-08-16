// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

// Package profileclient to handle profiling of kubearmor telemetry events
package profileclient

import (
	"bytes"
	"fmt"
	"github.com/accuknox/auto-policy-discovery/src/common"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evertras/bubble-table/table"
	pb "github.com/kubearmor/KubeArmor/protobuf"
	klog "github.com/kubearmor/kubearmor-client/log"
	profile "github.com/kubearmor/kubearmor-client/profile"
	log "github.com/sirupsen/logrus"
	"os"
	"strings"
	"time"
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

var errbuf bytes.Buffer

// session state for switching views
type sessionState uint

// Manage Bubble Tea display state
const (
	processview sessionState = iota
	fileview
	syscallview
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
	Container string
}

// Model for main Bubble Tea
type Model struct {
	File     table.Model
	Process  table.Model
	Network  table.Model
	Syscall  table.Model
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

	PodName := table.NewFlexColumn(ColumnPodname, "ContainerName", 4).WithStyle(ColumnStyle).WithFiltered(true)

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
		Syscall: table.New(generateColumns("Syscall")).WithBaseStyle(styleBase).WithPageSize(30).Filtered(true),
		tabs: &tabs{
			active: "Lip Gloss",
			items:  []string{"Process", "File", "Network", "Syscall"},
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
				m.state = syscallview
			case syscallview:
				m.state = processview
			}

		case "u":
			m.File = m.File.WithPageSize(m.File.PageSize() - 1)
			m.Network = m.Network.WithPageSize(m.Network.PageSize() - 1)
			m.Process = m.Process.WithPageSize(m.Process.PageSize() - 1)
			m.Syscall = m.Syscall.WithPageSize(m.Syscall.PageSize() - 1)

		case "i":
			m.File = m.File.WithPageSize(m.File.PageSize() + 1)
			m.Network = m.Network.WithPageSize(m.Network.PageSize() + 1)
			m.Process = m.Process.WithPageSize(m.Process.PageSize() + 1)
			m.Syscall = m.Syscall.WithPageSize(m.Syscall.PageSize() + 1)

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

		case syscallview:
			m.Syscall = m.Syscall.Focused(true)
			m.Syscall, cmd = m.Syscall.Update(msg)
			cmds = append(cmds, cmd)
		}
	case klog.EventInfo:
		profile.TelMutex.RLock()
		m.File = m.File.WithRows(generateRowsFromData(profile.Telemetry, "File")).WithColumns(generateColumns("File"))
		m.File = m.File.SortByAsc(ColumnNamespace).ThenSortByAsc(ColumnPodname).ThenSortByAsc(ColumnProcessName).ThenSortByAsc(ColumnCount).ThenSortByAsc(ColumnResource)
		m.Process = m.Process.WithRows(generateRowsFromData(profile.Telemetry, "Process")).WithColumns(generateColumns("Process"))
		m.Process = m.Process.SortByAsc(ColumnNamespace).ThenSortByAsc(ColumnPodname).ThenSortByAsc(ColumnProcessName).ThenSortByAsc(ColumnCount).ThenSortByAsc(ColumnResource)
		m.Network = m.Network.WithRows(generateRowsFromData(profile.Telemetry, "Network")).WithColumns(generateColumns("Network"))
		m.Network = m.Network.SortByAsc(ColumnNamespace).ThenSortByAsc(ColumnPodname).ThenSortByAsc(ColumnProcessName).ThenSortByAsc(ColumnCount).ThenSortByAsc(ColumnResource)
		m.Syscall = m.Syscall.WithRows(generateRowsFromData(profile.Telemetry, "Syscall")).WithColumns(generateColumns("Syscall"))
		m.Syscall = m.Syscall.SortByAsc(ColumnNamespace).ThenSortByAsc(ColumnPodname).ThenSortByAsc(ColumnProcessName).ThenSortByAsc(ColumnCount).ThenSortByAsc(ColumnResource)
		profile.TelMutex.RUnlock()

		return m, waitForActivity()

	}

	return m, tea.Batch(cmds...)
}

func (m *Model) recalculateTable() {
	m.File = m.File.WithTargetWidth(m.width)
	m.Network = m.Network.WithTargetWidth(m.width)
	m.Process = m.Process.WithTargetWidth(m.width)
	m.Syscall = m.Syscall.WithTargetWidth(m.width)
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
	case syscallview:
		// s := lipgloss.NewStyle().MaxHeight(m.height).MaxWidth(m.width)
		total = s.Render(lipgloss.JoinVertical(lipgloss.Top, lipgloss.JoinVertical(lipgloss.Top,
			help,
			RowCount,
			m.tabs.View(),
			lipgloss.JoinVertical(lipgloss.Center, pad.Render(m.Syscall.View()))),
		))
	}
	return total

}

// Profile Row Data to display
type Profile struct {
	Namespace     string
	ContainerName string
	Process       string
	Resource      string
	Result        string
	Data          string
}

// Frequency and Timestamp data for another map
type Frequency struct {
	freq int
	time string
}

func isLaterTimestamp(timestamp1, timestamp2 string) bool {
	t1, err := time.Parse(time.RFC3339, timestamp1)
	if err != nil {
		// Handle error, use some default value, or return false if you prefer
		return false
	}

	t2, err := time.Parse(time.RFC3339, timestamp2)
	if err != nil {
		// Handle error, use some default value, or return false if you prefer
		return false
	}

	return t1.After(t2)
}

// AggregateSummary used to aggregate summary data for a less cluttered view of file and process data
func AggregateSummary(inputMap map[Profile]*Frequency, Operation string) map[Profile]*Frequency {
	outputMap := make(map[Profile]*Frequency)
	var fileArr []string
	fileSumMap := make(map[Profile]*Frequency)
	updatedSumMap := make(map[Profile]*Frequency)
	if Operation == "Network" || Operation == "Syscall" {
		return inputMap
	}
	for prof, count := range inputMap {
		if Operation == "File" || Operation == "Process" {
			fileArr = append(fileArr, prof.Resource)
			fileSumMap[prof] = count
		} else {
			updatedSumMap[prof] = count
		}
	}
	inputMap = updatedSumMap
	aggregatedPaths := common.AggregatePaths(fileArr)
	for summary, countTime := range fileSumMap {
		for _, path := range aggregatedPaths {
			if strings.HasPrefix(summary.Resource, path.Path) && (len(summary.Resource) == len(path.Path) || summary.Resource[len(strings.TrimSuffix(path.Path, "/"))] == '/') {
				summary.Resource = path.Path
				break
			}
		}
		if existingFreq, ok := outputMap[summary]; ok {
			// If the prof already exists, update the frequency and timestamp if needed
			existingFreq.freq += countTime.freq

			if isLaterTimestamp(countTime.time, existingFreq.time) {
				existingFreq.time = countTime.time
			}
			outputMap[summary] = existingFreq
		} else {
			outputMap[summary] = countTime
		}
	}

	return outputMap
}

func generateRowsFromData(data []pb.Log, Operation string) []table.Row {
	var s SomeData
	m := make(map[Profile]int)
	w := make(map[Profile]*Frequency)
	for _, entry := range data {

		if entry.Operation == Operation {
			if (entry.NamespaceName == o1.Namespace) ||
				(entry.PodName == o1.Pod) ||
				(entry.ContainerName == o1.Container) ||
				(len(o1.Namespace) == 0 && len(o1.Pod) == 0 && len(o1.Container) == 0) {
				var p Profile

				if entry.Operation == "Syscall" {
					p = Profile{
						Namespace:     entry.NamespaceName,
						ContainerName: entry.ContainerName,
						Process:       entry.ProcessName,
						Resource:      entry.Data,
						Result:        entry.Result,
					}
				} else {
					p = Profile{
						Namespace:     entry.NamespaceName,
						ContainerName: entry.ContainerName,
						Process:       entry.ProcessName,
						Resource:      entry.Resource,
						Result:        entry.Result,
					}
				}

				f := &Frequency{
					time: entry.UpdatedTime,
				}
				w[p] = f
				m[p]++
				w[p].freq = m[p]

			}
		}
	}

	finalmap := AggregateSummary(w, Operation)
	for r, frequency := range finalmap {
		row := table.NewRow(table.RowData{
			ColumnNamespace:   r.Namespace,
			ColumnPodname:     r.ContainerName,
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
	o1 = Options{
		Namespace: o.Namespace,
		Pod:       o.Pod,
		GRPC:      o.GRPC,
		Container: o.Container,
	}
	p := tea.NewProgram(NewModel(), tea.WithAltScreen())
	go func() {
		err := profile.GetLogs(o1.GRPC)
		if err != nil {
			p.Quit()
			profile.ErrChan <- err
		}
	}()

	os.Stderr = nil
	if err := p.Start(); err != nil {
		log.Fatal(err)
	}
	select {
	case err := <-profile.ErrChan:
		log.Errorf("failed to start observer. Error=%s", err.Error())
	default:
		break
	}

}
