package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	// maxCommandOutputBytes caps what a command hands back to the model, so a
	// chatty command cannot eat the whole context window.
	maxCommandOutputBytes = 32 * 1024

	// commandTimeout stops a command that waits forever: without it a prompt
	// for input or a hung network call would freeze the whole agent loop.
	commandTimeout = 60 * time.Second
)

// RunBash runs a shell command on this machine and returns what it printed.
// The command text comes from the model, so this tool can do anything the user
// running the agent can do; it exists for a local assistant working on the
// user's own machine, and is not a sandbox.
type RunBash struct {
	// Dir is where commands run. Empty means the process working directory.
	Dir string

	// Timeout bounds a single command. Zero means commandTimeout.
	Timeout time.Duration
}

// NewRunBash runs commands in the current working directory.
func NewRunBash() *RunBash {
	return &RunBash{Timeout: commandTimeout}
}

// Schema tells the model when and how to call this tool.
func (r *RunBash) Schema() Schema {
	return Schema{
		Name: "run_bash",
		Description: "Run a bash command on the user's machine and return its combined stdout and stderr. " +
			"Use it to inspect or act on the system: list directories, search with grep or find, check git " +
			"state, run builds or tests, or read anything a command can report. " +
			"Pipes, redirection, globs and && chains all work, since the command runs through bash -c. " +
			"The command runs non-interactively, so it must not wait for input; pass flags such as -y instead. " +
			"A command that exits non-zero comes back as an error carrying its output, which is often the " +
			"answer itself, so read it before trying something else.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": `The bash command to run, for example "ls -la ~/Downloads" or "git status --short".`,
				},
			},
			"required": []string{"command"},
		},
	}
}

// Call runs the command and returns its output. A non-zero exit is reported as
// an error so the model sees the failure, with the output kept alongside it.
func (r *RunBash) Call(ctx context.Context, args map[string]any) (string, error) {
	command, _ := args["command"].(string)
	command = strings.TrimSpace(command)
	if command == "" {
		return "", errors.New(`argument "command" is required`)
	}

	timeout := r.Timeout
	if timeout <= 0 {
		timeout = commandTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// stdout and stderr share one buffer so the output reads in the order it
	// was produced, the way it would look in a terminal.
	var buf bytes.Buffer

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	cmd.Dir = r.Dir
	cmd.Env = os.Environ()
	cmd.Stdin = nil
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Run()
	output := clampOutput(buf.String())

	// The deadline fires as a kill, so check it first: otherwise a timeout
	// reads as an ordinary signal death and the real cause is lost.
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("command timed out after %s and was killed%s", timeout, withOutput(output))
	}

	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return "", fmt.Errorf("command exited with status %d%s", exit.ExitCode(), withOutput(output))
	}
	if err != nil {
		return "", fmt.Errorf("cannot run command: %w%s", err, withOutput(output))
	}

	// Silence is success for plenty of commands; say so rather than return an
	// empty string the model could read as a failure.
	if strings.TrimSpace(output) == "" {
		return "(command succeeded with no output)", nil
	}

	return output, nil
}

// clampOutput keeps the head of long output, which is where a command usually
// says what it did.
func clampOutput(out string) string {
	if len(out) <= maxCommandOutputBytes {
		return out
	}

	return fmt.Sprintf("%s\n...[truncated: showing first %d of %d bytes]",
		out[:maxCommandOutputBytes], maxCommandOutputBytes, len(out))
}

// withOutput appends what a failed command printed, since the reason it failed
// is usually in there.
func withOutput(output string) string {
	if strings.TrimSpace(output) == "" {
		return ", printing nothing"
	}

	return ":\n" + output
}
