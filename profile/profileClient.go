package profile

import (
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evertras/bubble-table/table"
	pb "github.com/kubearmor/KubeArmor/protobuf"
)

const (
	columnKeyID     = "id"
	columnKeyStatus = "status"
)

type responseMsg struct{}

func listenForActivity(sub chan struct{}) tea.Cmd {
	return func() tea.Msg {
		for {
			time.Sleep(time.Millisecond * time.Duration(rand.Int63n(900)+100))
			sub <- struct{}{}
		}
	}
}

func waitForActivity(sub chan struct{}) tea.Cmd {
	return func() tea.Msg {
		return responseMsg(<-sub)
	}
}

type tel []pb.Log

type Model struct {
	table table.Model
	sub   chan struct{}
	data  []pb.Log
}

type SomeData struct {
	freq map[string]int
}

// func NewSomeData(res string) *tel {
// 	s := &tel{
// 		Resource: res,
// 	}

// 	return s
// }

func NewModel() Model {
	return Model{
		table: table.New(generateColumns(0)),
		sub:   make(chan struct{}),
	}
}

// This data is stored somewhere else, maybe on a client or some other thing
func refreshDataCmd() tea.Msg {
	// This could come from some API or something
	return []*tel{
		(*tel)(&Telemetry),
	}
}

// Generate columns based on how many are critical to show some summary
func generateColumns(numCritical int) []table.Column {
	// Show how many critical there are
	statusStr := fmt.Sprintf("Count")
	statusCol := table.NewColumn(columnKeyStatus, statusStr, 10)

	// if numCritical > 3 {
	// 	// This normally applies the critical style to everything in the column,
	// 	// but in this case we apply a row style which overrides it anyway.
	// 	statusCol = statusCol.WithStyle(styleCritical)
	// }

	return []table.Column{
		table.NewColumn(columnKeyID, "Resource", 10),
		statusCol,
	}
}

func (m Model) Init() tea.Cmd {
	go GetLogs()
	return tea.Batch(
		refreshDataCmd,
		listenForActivity(m.sub),
		waitForActivity(m.sub),
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
		m.data = append(m.data, Telemetry...)
		numCritical := 0
		// Reapply the new data and the new columns based on critical count
		m.table = m.table.WithRows(generateRowsFromData(m.data)).WithColumns(generateColumns(numCritical))

		// This can be from any source, but for demo purposes let's party!
		return m, waitForActivity(m.sub)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	body := strings.Builder{}
	// fmt.Printf("%d\n", len(m.data))
	// body.WriteString(
	// 	fmt.Sprintf(
	// 		"Table demo with updating data!",
	// 	))

	pad := lipgloss.NewStyle().Padding(1)

	body.WriteString(pad.Render(m.table.View()))

	return body.String()
}

func generateRowsFromData(data []pb.Log) []table.Row {
	rows := []table.Row{}
	var s SomeData
	s.freq = make(map[string]int)

	// for _, file := range
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

	p := tea.NewProgram(NewModel())

	if err := p.Start(); err != nil {
		log.Fatal(err)
	}
}
