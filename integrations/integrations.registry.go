package integrations

import "strings"

// Registry is the set of integrations the CLI can verify, keyed by the name the
// user types. It is deliberately the same shape as the tool registry: a lookup
// the dispatcher can ask without knowing what is in it.
type Registry struct {
	byName map[string]IVerifier
	order  []string
}

// NewRegistry registers the given verifiers, keeping their order for listings.
func NewRegistry(list ...IVerifier) *Registry {
	r := &Registry{byName: make(map[string]IVerifier, len(list))}
	for _, verifier := range list {
		r.Add(verifier)
	}

	return r
}

// Add registers a verifier, replacing any verifier already using the same name.
func (r *Registry) Add(verifier IVerifier) {
	name := key(verifier.Name())
	if _, exists := r.byName[name]; !exists {
		r.order = append(r.order, name)
	}

	r.byName[name] = verifier
}

// Get looks up the integration the user asked for. Matching ignores case and
// surrounding space, because the name arrives from a typed command line.
func (r *Registry) Get(name string) (IVerifier, bool) {
	verifier, ok := r.byName[key(name)]
	return verifier, ok
}

// Names lists every registered name in registration order, for usage lines and
// "did you mean" messages.
func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.order))
	out = append(out, r.order...)

	return out
}

// All lists every registered verifier in registration order.
func (r *Registry) All() []IVerifier {
	out := make([]IVerifier, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.byName[name])
	}

	return out
}

// key normalises a name so lookup and registration agree.
func key(name string) string { return strings.ToLower(strings.TrimSpace(name)) }
