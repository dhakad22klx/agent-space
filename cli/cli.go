package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func StartCli() {
	// Create a scanner to read from standard input
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Welcome to the Agent-Space! Your personal AI assistant")

	for {
		// Print a custom terminal prompt symbol
		fmt.Print("agent-space> ")

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
			fmt.Println("Goodbye!")
			return
		case "help":
			fmt.Println("Available commands: help, exit")
		default:
			fmt.Printf("Available information: %s\n", input)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "error reading input: %v\n", err)
	}
}
