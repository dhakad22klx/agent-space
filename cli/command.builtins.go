package cli

import (
	agent "agent-harness/agent"
	tui "agent-harness/cli/tui"
	"context"
)

// The commands the CLI has always had, now registered like any other so there
// is one path into all of them instead of a switch beside the registry.

func newHelpCommand(out *tui.Output, registry *Registry) Command {
	return Command{
		Name:        "help",
		Description: "list the available commands",
		Run: func(ctx context.Context) error {
			// Asking the registry means this cannot fall out of date the way a
			// hand-written list of command names does.
			for _, item := range registry.Suggest(slash) {
				out.Plain("  " + item.Label + "  " + tui.Gray(item.Hint))
			}

			return nil
		},
	}
}

func newResetCommand(out *tui.Output, assistant *agent.Agent) Command {
	return Command{
		Name:        "reset",
		Description: "clear the conversation history",
		Run: func(ctx context.Context) error {
			// The prompt runs without a provider; only answering needs one.
			if assistant != nil {
				assistant.Reset()
			}
			out.Notice("conversation cleared")

			return nil
		},
	}
}

// newExitCommand reports through the flag rather than calling os.Exit, so the
// loop unwinds normally and the reader's deferred restore of the terminal
// actually runs.
func newExitCommand(out *tui.Output, quit *bool) Command {
	return Command{
		Name:        "exit",
		Description: "leave agent-space",
		Run: func(ctx context.Context) error {
			out.Farewell("Goodbye!")
			*quit = true

			return nil
		},
	}
}
