// Package telegram verifies a bot token. The check itself is not written yet;
// what is here is the shape it will be written into.
package telegram

import (
	"context"

	integrations "agent-harness/integrations"
)

// api is the Bot API host. The token goes in the path here, not in a header,
// which is why nothing in this package may print a URL.
const api = "https://api.telegram.org"

// Verifier will check a bot token. It holds no state, so one instance serves
// every check.
type Verifier struct{}

// New builds the verifier.
func New() *Verifier { return &Verifier{} }

// Name is what the user types.
func (v *Verifier) Name() string { return "telegram" }

// Description is the line shown when listing integrations.
func (v *Verifier) Description() string {
	return "checks a Telegram bot token by looking up the bot it controls"
}

// Fields asks for the one thing the Bot API authenticates with.
func (v *Verifier) Fields() []integrations.Field {
	return []integrations.Field{
		{
			Key:    "token",
			Label:  "Telegram bot token",
			Help:   "the token BotFather gave you, shaped like 123456789:AA...; it is masked as you type and never written to the transcript",
			Secret: true,
		},
	}
}

// Verify will call {api}/bot{token}/getMe, the request Telegram provides for
// exactly this question. The token is interpolated into the path, so it needs
// escaping before it goes in. A refusal arrives as ok:false with a description
// worth quoting rather than paraphrasing.
func (v *Verifier) Verify(ctx context.Context, in integrations.Input) (integrations.Result, error) {
	return integrations.Result{}, integrations.ErrNotImplemented
}
