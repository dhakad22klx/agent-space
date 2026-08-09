package main

import (
	"agent-harness/cli"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {

	// No alt screen: render inline so the session stays in the terminal's
	// normal scrollback once the program exits.
	// p := tea.NewProgram(cli.NewModel(), tea.WithAltScreen())
	p := tea.NewProgram(cli.NewModel())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "agent-space: %v\n", err)
		os.Exit(1)
	}
}
