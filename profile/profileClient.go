package profile

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	pb "github.com/kubearmor/KubeArmor/protobuf"
)

type model struct {
	logs []pb.Log
}

func initialModel() model {
	profile, _ := KarmorProfileStart("all")
	return model{
		logs: profile,
	}
}

func (m model) Init() tea.Cmd {
	// Just return `nil`, which means "no I/O right now, please."
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// Is it a key press?
	case tea.KeyMsg:

		// Cool, what was the actual key pressed?
		switch msg.String() {

		// These keys should exit the program.
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() string {
	s := "test"
	return s
}

func KarmorStart() {
	p := tea.NewProgram(initialModel())
	if err := p.Start(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
