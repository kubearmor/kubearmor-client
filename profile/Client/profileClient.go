// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

// Package profileclient to handle profiling of kubearmor telemetry events
package profileclient

import (
	"bytes"
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evertras/bubble-table/table"
	pb "github.com/kubearmor/KubeArmor/protobuf"
	profile "github.com/kubearmor/kubearmor-client/profile"
	log "github.com/sirupsen/logrus"
)

// Column keys
const (
	ColumnLogSource     = "LogSource"
	ColumnNamespace     = "Namespace"
	ColumnContainerName = "ContainerName"
	ColumnProcessName   = "ProcName"
	ColumnResource      = "Resource"
	ColumnResult        = "Result"
	ColumnCount         = "Count"
	ColumnTimestamp     = "Timestamp"
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
	// ColumnStyle for column color
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
	Save      bool
}

var ProfileOpts Options

// Model for main Bubble Tea
type Model struct {
	File         table.Model
	FileRows     []table.Row
	FileRowIndex map[string]int

	Process         table.Model
	ProcessRows     []table.Row
	ProcessRowIndex map[string]int

	Network         table.Model
	NetworkRows     []table.Row
	NetworkRowIndex map[string]int

	Syscall         table.Model
	SyscallRows     []table.Row
	SyscallRowIndex map[string]int

	tabs     tea.Model
	keys     keyMap
	quitting bool
	help     help.Model

	height int
	width  int

	state sessionState
}

func waitForNextEvent() tea.Cmd {
	return func() tea.Msg {
		return <-profile.EventChan
	}
}

func generateColumns(Operation string) []table.Column {
	LogSource := table.NewFlexColumn(ColumnLogSource, "LogSource", 1).WithStyle(ColumnStyle).WithFiltered(true)

	CountCol := table.NewFlexColumn(ColumnCount, "Count", 1).WithStyle(ColumnStyle).WithFiltered(true)

	Namespace := table.NewFlexColumn(ColumnNamespace, "Namespace", 2).WithStyle(ColumnStyle).WithFiltered(true)

	ContainerName := table.NewFlexColumn(ColumnContainerName, "ContainerName", 4).WithStyle(ColumnStyle).WithFiltered(true)

	ProcName := table.NewFlexColumn(ColumnProcessName, "ProcessName", 3).WithStyle(ColumnStyle).WithFiltered(true)

	Resource := table.NewFlexColumn(ColumnResource, Operation, 6).WithStyle(
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("202")).
			Align(lipgloss.Center)).WithFiltered(true)

	Result := table.NewFlexColumn(ColumnResult, "Result", 1).WithStyle(ColumnStyle).WithFiltered(true)

	Timestamp := table.NewFlexColumn(ColumnTimestamp, "TimeStamp", 3).WithStyle(ColumnStyle)

	return []table.Column{
		LogSource,
		Namespace,
		ContainerName,
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
		waitForNextEvent(),
	)
}

// NewModel initializates new bubbletea model
func NewModel() Model {
	model := Model{
		File:         table.New(generateColumns("File")).WithBaseStyle(styleBase).WithPageSize(30).Filtered(true),
		FileRows:     []table.Row{},
		FileRowIndex: make(map[string]int),

		Process:         table.New(generateColumns("Process")).WithBaseStyle(styleBase).WithPageSize(30).Filtered(true),
		ProcessRows:     []table.Row{},
		ProcessRowIndex: make(map[string]int),

		Network:         table.New(generateColumns("Network")).WithBaseStyle(styleBase).WithPageSize(30).Filtered(true),
		NetworkRows:     []table.Row{},
		NetworkRowIndex: make(map[string]int),

		Syscall:         table.New(generateColumns("Syscall")).WithBaseStyle(styleBase).WithPageSize(30).Filtered(true),
		SyscallRows:     []table.Row{},
		SyscallRowIndex: make(map[string]int),

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

		case "e":
			var file string
			var err error
			switch m.state {
			case fileview:
				file, err = m.ExportFileJSON()
			case processview:
				file, err = m.ExportProcessJSON()
			case syscallview:
				file, err = m.ExportSyscallJSON()
			case networkview:
				file, err = m.ExportNetworkJSON()
			default:
				// Optionally log or handle unknown operations
				fmt.Println("Unknown operation")
			}
			if err != nil {
				panic(err)
			}
			fmt.Println("Exported json data to file:", file)

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
	case pb.Log:
		if isCorrectLog(msg) {
			m.updateTableWithNewEntry(msg)
		}

		return m, waitForNextEvent()
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) updateTableWithNewEntry(msg pb.Log) {
	switch msg.Operation {
	case "File":
		key := makeKeyFromEntry(msg)
		if idx, ok := m.FileRowIndex[key]; ok {
			row := m.FileRows[idx]
			count, ok := row.Data[ColumnCount].(int)
			if ok {
				count++
				m.FileRows[idx].Data[ColumnCount] = count
			}

			m.FileRows[idx].Data[ColumnTimestamp] = msg.UpdatedTime
		} else {
			newRow := generateRowFromLog(msg)
			m.FileRows = append(m.FileRows, newRow)
			m.FileRowIndex[key] = len(m.FileRows) - 1
		}

		m.File = m.File.WithRows(m.FileRows)
		m.File = m.File.SortByAsc(ColumnNamespace).ThenSortByAsc(ColumnContainerName).ThenSortByAsc(ColumnProcessName).ThenSortByAsc(ColumnCount).ThenSortByAsc(ColumnResource)

	case "Process":
		key := makeKeyFromEntry(msg)
		if idx, ok := m.ProcessRowIndex[key]; ok {
			row := m.ProcessRows[idx]
			count, ok := row.Data[ColumnCount].(int)
			if ok {
				count++
				m.ProcessRows[idx].Data[ColumnCount] = count
			}

			m.ProcessRows[idx].Data[ColumnTimestamp] = msg.UpdatedTime
		} else {
			newRow := generateRowFromLog(msg)
			m.ProcessRows = append(m.ProcessRows, newRow)
			m.ProcessRowIndex[key] = len(m.ProcessRows) - 1
		}

		m.Process = m.Process.WithRows(m.ProcessRows)
		m.Process = m.Process.SortByAsc(ColumnNamespace).ThenSortByAsc(ColumnContainerName).ThenSortByAsc(ColumnProcessName).ThenSortByAsc(ColumnCount).ThenSortByAsc(ColumnResource)

	case "Network":
		key := makeKeyFromEntry(msg)
		if idx, ok := m.NetworkRowIndex[key]; ok {
			row := m.NetworkRows[idx]
			count, ok := row.Data[ColumnCount].(int)
			if ok {
				count++
				m.NetworkRows[idx].Data[ColumnCount] = count
			}

			m.NetworkRows[idx].Data[ColumnTimestamp] = msg.UpdatedTime
		} else {
			newRow := generateRowFromLog(msg)
			m.NetworkRows = append(m.NetworkRows, newRow)
			m.NetworkRowIndex[key] = len(m.NetworkRows) - 1
		}
		m.Network = m.Network.WithRows(m.NetworkRows)
		m.Network = m.Network.SortByAsc(ColumnNamespace).ThenSortByAsc(ColumnContainerName).ThenSortByAsc(ColumnProcessName).ThenSortByAsc(ColumnCount).ThenSortByAsc(ColumnResource)

	case "Syscall":
		key := makeKeyFromEntry(msg)
		if idx, ok := m.SyscallRowIndex[key]; ok {
			row := m.SyscallRows[idx]
			count, ok := row.Data[ColumnCount].(int)
			if ok {
				count++
				m.SyscallRows[idx].Data[ColumnCount] = count
			}

			m.SyscallRows[idx].Data[ColumnTimestamp] = msg.UpdatedTime
		} else {
			newRow := generateRowFromLog(msg)
			m.SyscallRows = append(m.SyscallRows, newRow)
			m.SyscallRowIndex[key] = len(m.SyscallRows) - 1
		}

		m.Syscall = m.Syscall.WithRows(m.SyscallRows)
		m.Syscall = m.Syscall.SortByAsc(ColumnNamespace).ThenSortByAsc(ColumnContainerName).ThenSortByAsc(ColumnProcessName).ThenSortByAsc(ColumnCount).ThenSortByAsc(ColumnResource)
	}
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
	RowCount := lipgloss.JoinHorizontal(
		lipgloss.Left,
		lipgloss.NewStyle().
			Foreground(helptheme).
			Render(fmt.Sprintf("Max Rows: %d", m.Process.PageSize())),
	)
	helpKey := m.help.Styles.FullDesc.Foreground(helptheme).Padding(0, 0, 1)
	help := lipgloss.JoinHorizontal(
		lipgloss.Left,
		helpKey.Render(m.help.FullHelpView(m.keys.FullHelp())),
	)

	content := func(view string) string {
		return lipgloss.JoinVertical(
			lipgloss.Top,
			help,
			RowCount,
			m.tabs.View(),
			lipgloss.JoinVertical(lipgloss.Center, pad.Render(view)),
		)
	}

	var view string
	switch m.state {
	case processview:
		view = m.Process.View()
	case fileview:
		view = m.File.View()
	case networkview:
		view = m.Network.View()
	case syscallview:
		view = m.Syscall.View()
	default:
		view = ""
	}

	return lipgloss.NewStyle().
		Height(m.height).
		MaxHeight(m.height).
		Render(content(view))
}

func (m *Model) ExportProcessJSON() (string, error) {
	return ExportRowsToJSON(generateColumns("Process"), m.ProcessRows, "Process")
}

func (m *Model) ExportFileJSON() (string, error) {
	return ExportRowsToJSON(generateColumns("File"), m.FileRows, "File")
}

func (m *Model) ExportNetworkJSON() (string, error) {
	return ExportRowsToJSON(generateColumns("Network"), m.NetworkRows, "Network")
}

func (m *Model) ExportSyscallJSON() (string, error) {
	return ExportRowsToJSON(generateColumns("Syscall"), m.SyscallRows, "Syscall")
}

// Profile Row Data to display
type Profile struct {
	LogSource     string `json:"log-source"`
	Namespace     string `json:"namespace"`
	ContainerName string `json:"container-name"`
	Process       string `json:"process"`
	Resource      string `json:"resource"`
	Result        string `json:"result"`
	Data          string `json:"data"`
	Count         int    `json:"count"`
	Time          string `json:"time"`
}

// Start entire TUI
func Start() {
	p := tea.NewProgram(NewModel(), tea.WithAltScreen())
	go func() {
		err := profile.GetLogs(ProfileOpts.GRPC)
		if err != nil {
			p.Quit()
			profile.ErrChan <- err
		}
	}()

	os.Stderr = nil
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
	select {
	case err := <-profile.ErrChan:
		log.Errorf("failed to start observer. Error=%s", err.Error())
	default:
		break
	}
}
