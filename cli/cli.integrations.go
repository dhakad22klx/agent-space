package cli

import (
	integrations "agent-harness/integrations"
	github "agent-harness/integrations/github"
	telegram "agent-harness/integrations/telegram"
)

// newVerifiers is the extension point for integrations, and the only file that
// knows which ones exist. Parsing the command, asking for input and reporting
// the verdict are all written against integrations.IVerifier, so adding Slack
// means a new package under integrations/ and one more line in this list.
func newVerifiers() *integrations.Registry {
	return integrations.NewRegistry(
		github.New(),
		telegram.New(),
	)
}
