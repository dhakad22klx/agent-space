package cli

import (
	tui "agent-harness/cli/tui"
	providers "agent-harness/providers"
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
)

func StartCli() {
	ctx := context.Background()

	// Create a scanner to read from standard input
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println(tui.Blue("Welcome to the Agent-Space! Your personal AI assistant"))

	// Set up the model provider; the prompt still works without it
	gemini, err := providers.NewGemini(ctx)
	if err != nil {
		fmt.Println(tui.Red("gemini unavailable: " + err.Error()))
	} else {
		fmt.Println(tui.Gray("model: " + gemini.Model()))
	}

	for {
		// Print a custom terminal prompt symbol
		fmt.Print(tui.Magenta("agent-space>"))

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
			if gemini == nil {
				fmt.Println(tui.Red("cannot answer: gemini is not configured"))
				continue
			}

			answer, err := gemini.Generate(ctx, input)
			if err != nil {
				fmt.Println(tui.Red("error: " + err.Error()))
				continue
			}

			fmt.Println(tui.Green(answer))
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "error reading input: %v\n", err)
	}
}
