package cli

import (
	agent "agent-harness/agent"
	tui "agent-harness/cli/tui"
	providers "agent-harness/providers"
	session "agent-harness/session"
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

	// Every run gets its own transcript, and the id it was filed under is the
	// last thing the user sees, however they leave.
	session := newSession(out)
	if session != nil {
		out.Record(session)
		defer closeSession(out, session)
	}

	// Verification shares the loop's stdin and this run's transcript, so a
	// check can ask for a token without handing the terminal to anything else.
	verifiers := newVerifiers()
	ask := &asker{in: scanner, out: out, record: session}

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

		input := strings.TrimSpace(scanner.Text())

		// What the user typed never passes through out, so the transcript has
		// to be told about it here.
		if session != nil && input != "" {
			session.Append("user", input)
		}

		switch input {
		case "":
			continue
		case "exit":
			out.Farewell("Goodbye!")
			return
		case "help":
			out.Plain("Available commands: help, reset, exit")
			out.Plain("Verify credentials: verify integration name: " + strings.Join(verifiers.Names(), ", "))
		case "reset":
			if assistant != nil {
				assistant.Reset()
			}
			out.Notice("conversation cleared")
		default:
			// A recognised command is handled here rather than by the model,
			// and an unknown integration name is a mistake worth reporting,
			// not a prompt worth answering.
			if names, isVerify := parseVerify(input); isVerify {
				verifyIntegrations(ctx, out, ask, verifiers, names)
				continue
			}

			// answer(ctx, out, assistant, input)
		}
	}

	if err := scanner.Err(); err != nil {
		out.Errorf("error reading input: %v", err)
	}
}

// newSession opens this run's transcript, or nil when it cannot be written: a
// missing log is worth a warning, never a refusal to start.
func newSession(out *tui.Output) *session.Session {
	record, err := session.Start(session.DefaultDir)
	if err != nil {
		out.Warn(fmt.Sprintf("not recording this session: %v", err))
		return nil
	}

	return record
}

// closeSession finishes the transcript and leaves the id behind, so the user
// knows which file this run just became.
func closeSession(out *tui.Output, record *session.Session) {
	out.Notice("session " + record.ID())
	out.Record(nil)

	if err := record.Close(); err != nil {
		out.Warn(fmt.Sprintf("transcript incomplete: %v", err))
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
