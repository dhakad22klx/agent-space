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

// StartCli runs the read-prompt-answer loop until the user leaves.
func StartCli() {
	ctx := context.Background()
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println(tui.Blue("Welcome to the Agent-Space! Your personal AI assistant"))

	// The prompt still works without a provider; only answering needs one.
	assistant, err := newAgent(newProvider(ctx))
	if err != nil {
		fmt.Println(tui.Red("agent unavailable: " + err.Error()))
	}

	for {
		fmt.Print(tui.Magenta("agent-space>"))

		if !scanner.Scan() {
			break
		}

		switch input := strings.TrimSpace(scanner.Text()); input {
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
			answer(ctx, assistant, input)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "error reading input: %v\n", err)
	}
}

// newProvider picks the model provider to run with, or nil when none is
// configured. Returning the interface keeps the rest of the CLI vendor-neutral.
func newProvider(ctx context.Context) providers.IProvider {
	gemini, err := providers.NewGemini(ctx)
	if err != nil {
		fmt.Println(tui.Red("gemini unavailable: " + err.Error()))
		return nil
	}

	fmt.Println(tui.Gray("model: " + gemini.Model()))

	return gemini
}

// newAgent gives the agent its tools and the trace printed while it works.
func newAgent(provider providers.IProvider) (*agent.Agent, error) {
	if provider == nil {
		return nil, nil
	}

	readFile, err := tools.NewReadFile()
	if err != nil {
		return nil, err
	}

	assistant := agent.New(provider, tools.NewRegistry(readFile))
	assistant.OnToolCall = func(call providers.ToolCall, result providers.ToolResult) {
		fmt.Println(tui.Gray(fmt.Sprintf("· %s(%s)", call.Name, formatArgs(call.Args))))
		if result.IsError {
			fmt.Println(tui.Yellow("  " + result.Output))
		}
	}

	return assistant, nil
}

// answer runs one prompt through the agent and prints the result.
func answer(ctx context.Context, assistant *agent.Agent, input string) {
	if assistant == nil {
		fmt.Println(tui.Red("cannot answer: the agent is not configured"))
		return
	}

	reply, err := assistant.Run(ctx, input)
	if err != nil {
		fmt.Println(tui.Red("error: " + err.Error()))
		return
	}

	fmt.Println(tui.Green(reply))
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
