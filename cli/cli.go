package cli

import (
	tui "agent-harness/cli/tui"
	"bufio"
	"fmt"
	"os"
	"strings"
)

func StartCli() {
	// Create a scanner to read from standard input
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println(tui.Blue("Welcome to the Agent-Space! Your personal AI assistant"))

	for {
		// Print a custom terminal prompt symbol
		fmt.Print(tui.Magenta("agent-space> "))

		// Read input
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())

		// Process input/commands
		switch input {
		case "":
			continue
		case "exit":
			fmt.Println(tui.Red("Goodbye!"))
			return
		case "help":
			fmt.Println("Available commands: help, exit")
		default:
			fmt.Printf(tui.Green("Available information: %s\n"), input)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "error reading input: %v\n", err)
	}
}
