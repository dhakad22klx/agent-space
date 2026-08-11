package tools

import "context"

// Schema is how a tool introduces itself to the model: what it is called, what
// it is for, and which arguments it accepts. Parameters is a plain JSON Schema
// object so any provider can forward it without a translation step.
type Schema struct {
	Name        string
	Description string
	Parameters  map[string]any
}

// Tool is a single capability the agent can hand to the model. Call receives
// the arguments the model produced, already decoded from JSON.
type Tool interface {
	Schema() Schema
	Call(ctx context.Context, args map[string]any) (string, error)
}

// Registry is the set of tools an agent may use, keyed by the name the model
// sees.
type Registry struct {
	byName map[string]Tool
	order  []string
}

// NewRegistry registers the given tools, keeping their order.
func NewRegistry(list ...Tool) *Registry {
	r := &Registry{byName: make(map[string]Tool, len(list))}
	for _, tool := range list {
		r.Add(tool)
	}

	return r
}

// Add registers a tool, replacing any tool already using the same name.
func (r *Registry) Add(tool Tool) {
	name := tool.Schema().Name
	if _, exists := r.byName[name]; !exists {
		r.order = append(r.order, name)
	}

	r.byName[name] = tool
}

// Get looks up the tool the model asked for.
func (r *Registry) Get(name string) (Tool, bool) {
	tool, ok := r.byName[name]
	return tool, ok
}

// Schemas lists every registered tool in registration order; this is the
// catalogue sent to the model on each request.
func (r *Registry) Schemas() []Schema {
	out := make([]Schema, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.byName[name].Schema())
	}

	return out
}
