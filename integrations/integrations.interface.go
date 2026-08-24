// Package integrations checks that the credentials for an outside service
// actually work. A verifier declares what it needs and how to check it; it
// never touches the terminal, so the CLI stays the only thing that talks to the
// user and a verifier can be tested with a plain map of answers.
package integrations

import (
	"context"
	"errors"
)

// Field is one answer a verifier needs before it can run. The CLI turns each
// field into a prompt, which is why the wording here is user-facing rather than
// a bare key name.
type Field struct {
	Key      string // how Verify looks the answer up
	Label    string // shown at the prompt
	Help     string // one line printed above the prompt, when it needs saying
	Default  string // used when the answer is left blank
	Secret   bool   // hidden while typed, and withheld from the transcript
	Optional bool   // blank is allowed even though there is no default
}

// Required reports whether the user must answer. A field with a default never
// has to be typed, and an optional one may be left empty.
func (f Field) Required() bool { return !f.Optional && f.Default == "" }

// Input holds the collected answers, keyed by Field.Key.
type Input map[string]string

// Get returns the answer for key, or the empty string when it was not asked
// for or was left blank.
func (i Input) Get(key string) string { return i[key] }

// Fact is one thing a check learned along the way, worth showing next to the
// verdict: which account answered, what the token may do, what is left of the
// rate limit.
type Fact struct {
	Label string
	Value string
}

// Result is the verdict of one check. OK false is a check that ran and came
// back negative — a token the service rejected — while an error from Verify
// means the check itself could not be made, which is a different problem and
// deserves different wording.
type Result struct {
	OK      bool
	Summary string
	Facts   []Fact
}

// ErrNotImplemented is what a verifier returns while it is still a placeholder.
// It travels as an error rather than as Result{OK: false}, because a check that
// was never written did not run and refuse a credential — it did not happen at
// all, and saying otherwise would report a working token as a bad one.
var ErrNotImplemented = errors.New("this check is not implemented yet")

// IVerifier is the contract every integration implements, so the CLI dispatches
// on this rather than on a list of services it has to know about.
type IVerifier interface {
	// Name is what the user types to reach this verifier, lowercase.
	Name() string

	// Description is the one line shown when listing integrations.
	Description() string

	// Fields lists what to ask for, in the order it should be asked.
	Fields() []Field

	// Verify makes the actual check with the answers given.
	Verify(ctx context.Context, in Input) (Result, error)
}
