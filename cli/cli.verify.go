package cli

import (
	tui "agent-harness/cli/tui"
	integrations "agent-harness/integrations"
	session "agent-harness/session"
	"bufio"
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// verifyPrefixes are the ways the command may be opened. The longer spelling is
// tried first, so "integrations" is not left behind as a name.
var verifyPrefixes = []string{"verify integrations", "verify integration"}

// nameLabels are the noise words allowed between the command and the names, so
// both "verify integration name: telegram" and "verify integration telegram"
// mean the same thing.
var nameLabels = []string{"names:", "name:"}

// parseVerify recognises the verify command and returns the names it asked for.
//
// The second result says whether this line was the command at all: false sends
// it on to the agent untouched, and true with no names is the bare command,
// which the caller answers with a listing. Separating those two keeps a typo
// like "verify integration githb" from being quietly handed to the model.
func parseVerify(input string) ([]string, bool) {
	trimmed := strings.TrimSpace(input)
	lower := strings.ToLower(trimmed)

	rest, found := "", false
	for _, prefix := range verifyPrefixes {
		if lower == prefix {
			return nil, true
		}
		if strings.HasPrefix(lower, prefix+" ") {
			rest, found = strings.TrimSpace(trimmed[len(prefix):]), true
			break
		}
	}

	if !found {
		return nil, false
	}

	for _, label := range nameLabels {
		if strings.HasPrefix(strings.ToLower(rest), label) {
			rest = strings.TrimSpace(rest[len(label):])
			break
		}
	}

	// Commas, semicolons and plain spaces all separate names, because a person
	// typing a list uses whichever comes to hand.
	var names []string
	for _, word := range strings.FieldsFunc(rest, isSeparator) {
		name := strings.ToLower(word)
		if name == "" || name == "and" {
			continue
		}

		names = append(names, name)
	}

	return names, true
}

func isSeparator(r rune) bool {
	return r == ',' || r == ';' || unicode.IsSpace(r)
}

// verifyIntegrations runs the command: one check per name, in the order given.
// An unknown name is reported and the rest still run, since a typo in the third
// name should not discard the two that were right.
func verifyIntegrations(ctx context.Context, out *tui.Output, ask *asker, registry *integrations.Registry, names []string) {
	if len(names) == 0 {
		listIntegrations(out, registry)
		return
	}

	for _, name := range names {
		verifier, ok := registry.Get(name)
		if !ok {
			out.Warn(fmt.Sprintf("no integration called %q; known: %s", name, strings.Join(registry.Names(), ", ")))
			continue
		}

		verifyOne(ctx, out, ask, verifier)
	}
}

// listIntegrations answers the bare command with what could be verified.
func listIntegrations(out *tui.Output, registry *integrations.Registry) {
	out.Plain("Usage: verify integration name: " + strings.Join(registry.Names(), ", "))
	for _, verifier := range registry.All() {
		out.Plain("  " + verifier.Name() + " — " + verifier.Description())
	}
}

// verifyOne collects what a verifier needs and reports what it found. The three
// outcomes stay distinct on purpose: a check that could not run is not the same
// as a credential that was refused.
func verifyOne(ctx context.Context, out *tui.Output, ask *asker, verifier integrations.IVerifier) {
	out.Notice(verifier.Name() + ": " + verifier.Description())

	input := integrations.Input{}
	for _, field := range verifier.Fields() {
		value, ok := ask.field(field)
		if !ok {
			out.Warn("  skipped " + verifier.Name())
			return
		}

		input[field.Key] = value
	}

	result, err := verifier.Verify(ctx, input)
	if errors.Is(err, integrations.ErrNotImplemented) {
		// A placeholder is not a failure, and saying so plainly keeps the
		// skeleton runnable without ever implying a token was judged.
		out.Warn("  " + verifier.Name() + ": nothing was checked — this verifier is still a stub")
		return
	}
	if err != nil {
		out.Errorf("  %s: the check could not be made — %v", verifier.Name(), err)
		return
	}

	if !result.OK {
		out.Warn(fmt.Sprintf("  %s: not verified — %s", verifier.Name(), result.Summary))
		return
	}

	out.Answer(fmt.Sprintf("  %s: verified — %s", verifier.Name(), result.Summary))
	for _, fact := range result.Facts {
		out.Plain("    " + fact.Label + ": " + fact.Value)
	}
}

// asker reads answers from the same stdin the main loop is already reading, so
// a verification happens inside the prompt instead of handing the terminal to
// something else. It owns the scanner rather than opening its own, because two
// readers on one stdin lose input to each other's buffers.
type asker struct {
	in     *bufio.Scanner
	out    *tui.Output
	record *session.Session
}

// field asks for one answer and keeps asking while a required one is left
// blank. The false result means stdin ended: the user gave up, or the input was
// never a terminal, and either way the caller should stop rather than loop.
func (a *asker) field(field integrations.Field) (string, bool) {
	if field.Help != "" {
		a.out.Notice("  " + field.Help)
	}

	label := "  " + field.Label
	if field.Default != "" {
		label += " [" + field.Default + "]"
	}
	label += ": "

	for {
		a.out.Prompt(label)

		typed, ok := a.read(field.Secret)
		if !ok {
			return "", false
		}

		switch value := strings.TrimSpace(typed); {
		case value != "":
			a.remember(field, value)
			return value, true
		case field.Default != "":
			return field.Default, true
		case field.Optional:
			return "", true
		}

		a.out.Warn("  " + field.Label + " is required — Ctrl-D skips this integration")
	}
}

// read takes one line, masking it while it is typed when it is a secret.
func (a *asker) read(secret bool) (string, bool) {
	if !secret {
		return a.line()
	}

	// Masking needs the terminal a character at a time, which only a real
	// terminal will give. Anything else — a pipe, a machine without stty — is
	// read as an ordinary visible line, because refusing the check would be the
	// worse failure.
	if typed, read, masked := tui.MaskedLine(a.out); masked {
		return typed, read
	}

	return a.line()
}

// line reads one whole line from the loop's own scanner. The scanner is shared
// with the main prompt on purpose: a second reader on the same stdin would
// strand input in the other one's buffer.
func (a *asker) line() (string, bool) {
	if !a.in.Scan() {
		return "", false
	}

	return a.in.Text(), true
}

// remember puts the answer in the transcript, since the main loop only records
// what was typed at its own prompt. A secret is noted but not written: the
// point of a saved transcript is what happened, not the token it happened with.
func (a *asker) remember(field integrations.Field, value string) {
	if a.record == nil {
		return
	}

	if field.Secret {
		a.record.Append("user", "<"+field.Key+" withheld>")
		return
	}

	a.record.Append("user", value)
}
