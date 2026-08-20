package cli

import (
	agent "agent-harness/agent"
	tui "agent-harness/cli/tui"
	providers "agent-harness/providers"
	tools "agent-harness/tools"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// prompt keeps its trailing space because the reader redraws it verbatim on
// every keystroke.
const prompt = "agent-space>"

// StartCli runs the read-prompt-answer loop until the user leaves. Nothing here
// writes to a stream directly: out owns every byte the CLI produces.
func StartCli() {
	ctx := context.Background()
	out := tui.NewOutput()

	out.Banner("Welcome to the Agent-Space! Your personal AI assistant")

	// The prompt still works without a provider; only answering needs one.
	assistant, err := newAgent(out, newProvider(ctx, out))
	if err != nil {
		out.Errorf("agent unavailable: %v", err)
	}

	// Registration order is listing order. quit is a flag rather than a return
	// from Run so the loop below ends the same way for every command.
	quit := false
	registry := NewRegistry(
		newSessionCommand(out, assistant),
		newIntegrationsCommand(out, assistant),
		newResetCommand(out, assistant),
		newExitCommand(out, &quit),
	)
	registry.Add(newHelpCommand(out, registry))

	// The reader prints the prompt itself, since it has to redraw it every time
	// the line changes.
	reader := tui.NewReader(out)
	reader.Suggest = registry.Suggest

	for {
		line, err := reader.ReadLine(prompt)
		if err != nil {
			// End of input is the user leaving, not a failure to report.
			if !errors.Is(err, io.EOF) {
				out.Errorf("error reading input: %v", err)
			}

			return
		}

		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}

		// A palette choice comes back as the label the user would have typed,
		// so there is only ever one way in.
		cmd, found := registry.Get(strings.TrimPrefix(input, slash))

		switch {
		case found:
			if err := cmd.Run(ctx); err != nil {
				out.Errorf("%s: %v", cmd.Name, err)
			}
			if quit {
				return
			}
		case strings.HasPrefix(input, slash):
			out.Warn("unknown command: " + input)
		default:
			//comment below line to mock model call (saving request while dev mode, uncomment when real test)
			fmt.Println("input: ", input)
			// answer(ctx, out, assistant, input)
		}
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
