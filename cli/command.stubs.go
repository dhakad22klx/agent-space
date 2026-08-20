package cli

import (
	agent "agent-harness/agent"
	tui "agent-harness/cli/tui"
	"context"
)

// The two commands the palette was built for. They take their dependencies as
// parameters already, so growing a real implementation against sessions/ is a
// change inside these functions and nowhere else.

func newSessionCommand(out *tui.Output, assistant *agent.Agent) Command {
	return Command{
		Name:        "session",
		Description: "inspect and manage the current conversation",
		Run: func(ctx context.Context) error {
			out.Notice("session: not implemented yet")
			return nil
		},
	}
}

func newIntegrationsCommand(out *tui.Output, assistant *agent.Agent) Command {
	return Command{
		Name:        "integrations",
		Description: "show the configured providers and tools",
		Run: func(ctx context.Context) error {
			out.Notice("integrations: not implemented yet")
			return nil
		},
	}
}
