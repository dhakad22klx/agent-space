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

// StartCli runs the read-prompt-answer loop until the user leaves. Nothing here
// writes to a stream directly: out owns every byte the CLI produces.
func StartCli() {
	ctx := context.Background()
	scanner := bufio.NewScanner(os.Stdin)
	out := tui.NewOutput()

	out.Banner("Welcome to the Agent-Space! Your personal AI assistant")

	// The prompt still works without a provider; only answering needs one.
	assistant, err := newAgent(out, newProvider(ctx, out))
	if err != nil {
		out.Errorf("agent unavailable: %v", err)
	}

	for {
		out.Prompt("agent-space>")

		if !scanner.Scan() {
			break
		}

		switch input := strings.TrimSpace(scanner.Text()); input {
		case "":
			continue
		case "exit":
			out.Farewell("Goodbye!")
			return
		case "help":
			out.Plain("Available commands: help, reset, exit")
		case "reset":
			if assistant != nil {
				assistant.Reset()
			}
			out.Notice("conversation cleared")
		default:
			answer(ctx, out, assistant, input)
		}
	}

	if err := scanner.Err(); err != nil {
		out.Errorf("error reading input: %v", err)
	}
}

// newProvider picks the model provider to run with, or nil when none is
// configured. Returning the interface keeps the rest of the CLI vendor-neutral.
func newProvider(ctx context.Context, out *tui.Output) providers.IProvider {
	gemini, err := providers.NewGemini(ctx)
	if err != nil {
		out.Errorf("gemini unavailable: %v", err)
		return nil
	}

	out.Notice("model: " + gemini.Model())

	return gemini
}

// newAgent gives the agent its tools and the trace printed while it works.
func newAgent(out *tui.Output, provider providers.IProvider) (*agent.Agent, error) {
	if provider == nil {
		return nil, nil
	}

	assistant := agent.New(provider, tools.NewRegistry(tools.NewRunBash()))
	assistant.OnToolCall = func(call providers.ToolCall, result providers.ToolResult) {
		out.Tracef("· %s(%s)", call.Name, formatArgs(call.Args))
		if result.IsError {
			out.Warn("  " + result.Output)
		}
	}

	return assistant, nil
}

// answer runs one prompt through the agent and prints the result.
func answer(ctx context.Context, out *tui.Output, assistant *agent.Agent, input string) {
	if assistant == nil {
		out.Error("cannot answer: the agent is not configured")
		return
	}

	reply, err := assistant.Run(ctx, input)
	if err != nil {
		out.Errorf("error: %v", err)
		return
	}

	out.Answer(reply)
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
