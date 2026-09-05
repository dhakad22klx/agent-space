// Package humanintheloop decides which tool calls a person has to approve
// before they run.
package humanintheloop

// toolsRequiringApproval is the rule: a tool named here is held for a human,
// anything else runs on its own. A map rather than a list so the lookup is one
// read on a path the agent takes for every call the model makes.
var toolsRequiringApproval = map[string]bool{
	"send_updates_to_manager": true,
}

// RequiresApproval reports whether a call to this tool has to be approved. A
// tool nobody listed is not gated, which is what the zero value already says.
func RequiresApproval(tool string) bool {
	return toolsRequiringApproval[tool]
}
