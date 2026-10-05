package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestProcessDispatch(t *testing.T) {
	for _, tc := range []struct {
		args   []string
		called string
		code   int
		text   string
	}{
		{nil, "start", 0, ""}, {[]string{"setup"}, "setup", 0, ""},
		{[]string{"update"}, "update", 0, ""}, {[]string{"version"}, "", 0, "JustSay v1.2.3"},
		{[]string{"--version"}, "", 0, "JustSay v1.2.3"},
		{[]string{"--help"}, "", 0, "Usage:"}, {[]string{"-h"}, "", 0, "Usage:"}, {[]string{"help"}, "", 0, "Usage:"},
		{[]string{"typo"}, "", 1, "unknown command"}, {[]string{"setup", "extra"}, "", 1, "extra arguments"},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			var out, errOut bytes.Buffer
			called := ""
			c := processCommands{out: &out, errOut: &errOut, version: func() string { return "v1.2.3" },
				start: func() error { called = "start"; return nil }, setup: func() error { called = "setup"; return nil },
				update: func(context.Context) error { called = "update"; return nil }}
			if code := c.run(context.Background(), tc.args); code != tc.code {
				t.Fatalf("code = %d", code)
			}
			if called != tc.called {
				t.Fatalf("called %q, want %q", called, tc.called)
			}
			if !strings.Contains(out.String()+errOut.String(), tc.text) {
				t.Fatalf("missing %q in output", tc.text)
			}
			if tc.code != 0 && !strings.Contains(errOut.String(), "Usage:") {
				t.Fatal("error lacks usage")
			}
		})
	}
}

func TestProcessFailures(t *testing.T) {
	for _, command := range []string{"", "setup", "update"} {
		t.Run(command, func(t *testing.T) {
			var out bytes.Buffer
			failure := errors.New("actionable failure")
			c := processCommands{out: &out, errOut: &out, start: func() error { return failure },
				setup: func() error { return failure }, update: func(context.Context) error { return failure }}
			var args []string
			if command != "" {
				args = []string{command}
			}
			if code := c.run(context.Background(), args); code != 1 {
				t.Fatalf("code = %d", code)
			}
			if !strings.Contains(out.String(), failure.Error()) {
				t.Fatal("missing error")
			}
		})
	}
}

func TestSlashParsing(t *testing.T) {
	for _, tc := range []struct {
		input, name string
		count       int
	}{
		{" /VERIFY gmail ", "verify", 1}, {"/off", "off", 0}, {"/", "", 0}, {"/verify gmail extra", "verify", 2},
	} {
		name, args := parseCommand(tc.input)
		if !isCommand(tc.input) || name != tc.name || len(args) != tc.count {
			t.Errorf("parse %q = %q %v", tc.input, name, args)
		}
	}
	if isCommand("hello") {
		t.Fatal("ordinary input parsed as command")
	}
}
