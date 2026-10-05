package cli

import (
	"context"
	"fmt"
	"io"
	"os"
)

const processUsage = `Usage: justsay [command]

Commands:
  setup      Configure Gemini and Human-in-the-Loop
  update     Show update availability
  version    Show build information
  help       Show this help

Run justsay without a command to start the agent.
Inside the agent, type help for integration commands.
`

type processCommands struct {
	out, errOut  io.Writer
	setup, start func() error
	update       func(context.Context) error
	version      func() string
}

// Run dispatches process arguments before opening a terminal or model client.
// It returns an exit code so failures also work correctly in shell scripts.
func Run(ctx context.Context, args []string) int {
	c := processCommands{out: os.Stdout, errOut: os.Stderr, version: func() string { return "version command is not implemented yet" }}
	c.setup = func() error {
		in, err := newTerminalInput()
		if err != nil {
			return err
		}
		defer func() { _ = in.close() }()
		return runSetup(in, in.stdout())
	}
	c.start = func() error {
		return startCli(ctx)
	}
	c.update = func(context.Context) error {
		_, err := fmt.Fprintln(c.out, "Update command is not implemented yet.")
		return err
	}
	return c.run(ctx, args)
}

func (c processCommands) run(ctx context.Context, args []string) int {
	var err error
	if len(args) == 0 {
		err = c.start()
	} else if len(args) != 1 {
		err = fmt.Errorf("commands do not accept extra arguments")
		_, _ = fmt.Fprint(c.errOut, processUsage)
	} else {
		switch args[0] {
		case "help", "--help", "-h":
			_, err = fmt.Fprint(c.out, processUsage)
		case "version", "--version":
			_, err = fmt.Fprintln(c.out, "JustSay "+c.version())
		case "setup":
			err = c.setup()
		case "update":
			err = c.update(ctx)
		default:
			err = fmt.Errorf("unknown command %q", args[0])
			_, _ = fmt.Fprint(c.errOut, processUsage)
		}
	}
	if err != nil {
		_, _ = fmt.Fprintf(c.errOut, "justsay: %v\n", err)
		return 1
	}
	return 0
}
