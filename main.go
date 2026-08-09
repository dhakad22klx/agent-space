package main

import (
	"fmt"
	"os"
	"agent-harness/cli"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {

	p := tea.NewProgram(cli.NewModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "agent-space: %v\n", err)
		os.Exit(1)
	}
}
