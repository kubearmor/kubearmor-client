package profile

import (
	"log"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/common-nighthawk/go-figure"
	"github.com/evertras/bubble-table/table"
	pb "github.com/kubearmor/KubeArmor/protobuf"
	klog "github.com/kubearmor/kubearmor-client/log"
)

const (
	columnKeyID     = "id"
	columnKeyStatus = "status"
)

type responseMsg klog.EventInfo

func waitForActivity() tea.Cmd {
	return func() tea.Msg {
		return responseMsg(<-eventChan)
	}
}

type Model struct {
	table table.Model
}

type SomeData struct {
	freq map[string]int
}

func NewModel() Model {
	return Model{
		table: table.New(generateColumns()),
	}
}

func generateColumns() []table.Column {
	statusCol := table.NewColumn(columnKeyStatus, "Count", 10).WithStyle(
		lipgloss.NewStyle().
			Faint(true).
			Foreground(lipgloss.Color("#09ff00")).
			Align(lipgloss.Center))

	return []table.Column{
		table.NewColumn(columnKeyID, "Resource", 40).WithStyle(
			lipgloss.NewStyle().
				Align(lipgloss.Center)),
		statusCol,
	}
}

func (m Model) Init() tea.Cmd {
	// temp := os.Stderr
	os.Stderr = nil
	go GetLogs()
	// os.Stderr = temp
	return tea.Batch(
		waitForActivity(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	m.table, cmd = m.table.Update(msg)
	cmds = append(cmds, cmd)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			cmds = append(cmds, tea.Quit)

		case "u":
		}
	case responseMsg:
		TelMutex.RLock()
		m.table = m.table.WithRows(generateRowsFromData(Telemetry)).WithColumns(generateColumns())
		TelMutex.RUnlock()
		m.table = m.table.SortByDesc(columnKeyStatus)

		return m, waitForActivity()
		// })

	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	body := strings.Builder{}
	pad := lipgloss.NewStyle().Padding(1)

	body.WriteString(pad.Render(m.table.View()))

	return body.String()
}

func generateRowsFromData(data []pb.Log) []table.Row {
	rows := []table.Row{}
	var s SomeData
	s.freq = make(map[string]int)

	for _, entry := range data {
		if entry.Operation == "File" {
			s.freq[entry.Resource] = s.freq[entry.Resource] + 1
		}
	}

	for key, element := range s.freq {
		row := table.NewRow(table.RowData{
			columnKeyID:     key,
			columnKeyStatus: element,
		})

		rows = append(rows, row)

	}

	return rows
}

func Start() {
	myFigure := figure.NewFigure("Karmor", "", true)
	myFigure.Print()
	p := tea.NewProgram(NewModel())

	if err := p.Start(); err != nil {
		log.Fatal(err)
	}
}
