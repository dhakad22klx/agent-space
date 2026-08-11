package cli

import (
	agent "agent-harness/agent"
	tui "agent-harness/cli/tui"
	providers "agent-harness/providers"
	tools "agent-harness/tools"
	"bufio"
	"context"
	"encoding/json"
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

	// Give the agent its tools and print each call as it happens
	assistant, err := newAgent(gemini)
	if err != nil {
		fmt.Println(tui.Red("tools unavailable: " + err.Error()))
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
			fmt.Println("Available commands: help, reset, exit")
		case "reset":
			if assistant != nil {
				assistant.Reset()
			}
			fmt.Println(tui.Gray("conversation cleared"))
		default:
			if assistant == nil {
				fmt.Println(tui.Red("cannot answer: the agent is not configured"))
				continue
			}

			answer, err := assistant.Run(ctx, input)
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

// newAgent assembles the agent: the model provider, the tools it may call and
// the trace printed while it works.
func newAgent(gemini *providers.Gemini) (*agent.Agent, error) {
	if gemini == nil {
		return nil, nil
	}

	readFile, err := tools.NewReadFile()
	if err != nil {
		return nil, err
	}

	assistant := agent.New(gemini, tools.NewRegistry(readFile))
	assistant.OnToolCall = func(call providers.ToolCall, result providers.ToolResult) {
		fmt.Println(tui.Gray(fmt.Sprintf("· %s(%s)", call.Name, formatArgs(call.Args))))
		if result.IsError {
			fmt.Println(tui.Yellow("  " + result.Output))
		}
	}

	return assistant, nil
}

// formatArgs renders tool arguments compactly for the trace line.
func formatArgs(args map[string]any) string {
	if len(args) == 0 {
		return ""
	}

	encoded, err := json.Marshal(args)
	if err != nil {
		return fmt.Sprint(args)
	}

	return string(encoded)
}
