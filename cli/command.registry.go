package cli

import (
	tui "agent-harness/cli/tui"
	"context"
	"strings"
)

// slash is what marks a line as a command rather than something to ask the
// agent, and what the palette keys off.
const slash = "/"

// Command is one thing the user can run by name. Run is a closure so a handler
// can reach whatever StartCli has already built, which is what spares this
// layer from passing the agent and the output around as state.
type Command struct {
	Name        string // without the leading slash
	Description string
	Run         func(ctx context.Context) error
}

// Registry holds the commands in the order they were added, which is the order
// the palette lists them in. Same shape as tools.Registry, for the same reason:
// a map to find one by name, a slice to keep the order stable.
type Registry struct {
	byName map[string]Command
	order  []string
}

// NewRegistry is where the command set is declared, so adding one later is a
// single line at the call site.
func NewRegistry(list ...Command) *Registry {
	registry := &Registry{byName: make(map[string]Command, len(list))}

	for _, cmd := range list {
		registry.Add(cmd)
	}

	return registry
}

// Add registers a command, or replaces one of the same name without moving it
// in the listing.
func (r *Registry) Add(cmd Command) {
	if _, exists := r.byName[cmd.Name]; !exists {
		r.order = append(r.order, cmd.Name)
	}

	r.byName[cmd.Name] = cmd
}

// Get finds a command by bare name. Callers strip the slash first, so "help"
// typed out and "/help" chosen from the palette arrive at the same place.
func (r *Registry) Get(name string) (Command, bool) {
	cmd, found := r.byName[name]
	return cmd, found
}

// Suggest is the palette's entire filter: nothing at all unless the line opens
// with a slash, then whatever still matches what was typed after it. Because
// the reader calls this on every edit, a line that stops matching closes the
// palette on its own.
func (r *Registry) Suggest(line string) []tui.Item {
	if !strings.HasPrefix(line, slash) {
		return nil
	}

	typed := strings.TrimPrefix(line, slash)

	var items []tui.Item
	for _, name := range r.order {
		if !strings.HasPrefix(name, typed) {
			continue
		}

		cmd := r.byName[name]
		items = append(items, tui.Item{Label: slash + cmd.Name, Hint: cmd.Description})
	}

	return items
}
