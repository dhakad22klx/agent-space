package humanintheloop

var toolsRequiringApproval = map[string]bool{
	"send_updates_to_manager": true,
}

// RequiresApproval reports whether the given tool require human approval before running.
func RequiresApproval(tool string) bool {
	return toolsRequiringApproval[tool]
}
