package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// maxReadBytes caps how much of a file is handed back to the model, so one
	// large file cannot eat the whole context window.
	maxReadBytes = 64 * 1024

	// maxSuggestions bounds how many near misses are reported back.
	maxSuggestions = 8

	// binarySniffBytes is how much of a file is inspected before deciding it
	// is not text.
	binarySniffBytes = 8 * 1024

	// searchDepth and searchTimeout keep the fallback search cheap.
	searchDepth   = 6
	searchTimeout = 5 * time.Second
)

// existsScript asks bash what a path is: a directory, an existing file, or
// nothing at all.
const existsScript = `if [ -d "$1" ]; then echo dir; elif [ -e "$1" ]; then echo file; else echo missing; fi`

// searchScript looks for files whose name matches $PATTERN case-insensitively,
// under the roots passed as arguments. Heavy directories are pruned and the
// output is capped so a wide search stays cheap.
const searchScript = `find "$@" -maxdepth ${DEPTH} \
	\( -type d \( -name '.git' -o -name 'node_modules' -o -name 'vendor' -o -name '.cache' -o -name 'snap' \) -prune \) -o \
	\( -type f -iname "$PATTERN" -print \) 2>/dev/null | head -n ${LIMIT}`

// ReadFile reads a text file from disk. Paths that do not resolve are looked up
// with bash, so a wrong case or a wrong directory comes back as a list of near
// misses instead of a dead end.
type ReadFile struct {
	// Roots are the directories a relative path is resolved against, and the
	// directories searched when nothing resolves.
	Roots []string
}

// NewReadFile searches the current working directory first, then the user's
// home directory.
func NewReadFile() (*ReadFile, error) {
	var roots []string

	if wd, err := os.Getwd(); err == nil {
		roots = append(roots, wd)
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		roots = append(roots, home)
	}
	if len(roots) == 0 {
		return nil, errors.New("no searchable directory: neither the working directory nor the home directory resolved")
	}

	return &ReadFile{Roots: roots}, nil
}

// Schema tells the model when and how to call this tool.
func (r *ReadFile) Schema() Schema {
	return Schema{
		Name: "read_file",
		Description: "Read a text file from the local filesystem and return its contents. " +
			"Use it whenever answering needs what a file actually contains instead of a guess. " +
			"The path may be absolute, start with ~, or be relative to the working directory or the home directory. " +
			"If the path does not exist, the tool searches for files with a similar name and lists them; " +
			"when that happens, call read_file again with one of the exact paths it reported.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": `Path of the file to read, for example "cli/cli.go", "~/Downloads/Home.txt" or "/etc/hosts".`,
				},
			},
			"required": []string{"path"},
		},
	}
}

// Call resolves the requested path and returns the file contents.
func (r *ReadFile) Call(ctx context.Context, args map[string]any) (string, error) {
	path, _ := args["path"].(string)
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New(`argument "path" is required`)
	}

	full, err := r.locate(ctx, path)
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(full)
	if err != nil {
		return "", fmt.Errorf("cannot read %s: %w", full, err)
	}

	// Handing binary to the model is worse than refusing: it reads as noise,
	// costs a fortune in tokens, and invites an answer invented from garbage.
	if isBinary(data) {
		return "", fmt.Errorf("%s is not a text file, so its contents cannot be read as text; "+
			"do not guess what it contains", full)
	}

	if len(data) > maxReadBytes {
		return fmt.Sprintf("%s\n...[truncated: showing first %d of %d bytes]",
			data[:maxReadBytes], maxReadBytes, len(data)), nil
	}

	return string(data), nil
}

// isBinary reports whether data looks like something other than text, judging
// by the same sniff git uses: a NUL byte near the start.
func isBinary(data []byte) bool {
	head := data
	if len(head) > binarySniffBytes {
		head = head[:binarySniffBytes]
	}

	return bytes.IndexByte(head, 0) >= 0 || !utf8.Valid(head)
}

// locate turns the path the model asked for into a real file. A path that
// resolves nowhere becomes an error listing similarly named files, which the
// agent hands back to the model so it can retry with a real path.
func (r *ReadFile) locate(ctx context.Context, path string) (string, error) {
	sawDir := false

	for _, candidate := range r.candidates(path) {
		switch kind(ctx, candidate) {
		case "file":
			return candidate, nil
		case "dir":
			sawDir = true
		}
	}

	if sawDir {
		return "", fmt.Errorf("%s is a directory, not a file", path)
	}

	matches := r.search(ctx, filepath.Base(path))
	if len(matches) == 0 {
		return "", fmt.Errorf("no file named %s was found. The search covered %s down to %d levels, "+
			"skipping .git, node_modules, vendor, .cache and snap. Reading is not limited to those "+
			"directories, so if you know the full path, pass it directly",
			path, strings.Join(r.Roots, " and "), searchDepth)
	}

	return "", fmt.Errorf("no file at %s. These existing files have a similar name:\n%s\n"+
		"Call read_file again with whichever exact path is the one that was meant.",
		path, strings.Join(matches, "\n"))
}

// candidates lists the absolute paths a request could mean, in the order they
// should be tried.
func (r *ReadFile) candidates(path string) []string {
	if rest, found := strings.CutPrefix(path, "~"); found {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, rest)
		}
	}

	if filepath.IsAbs(path) {
		return []string{filepath.Clean(path)}
	}

	out := make([]string, 0, len(r.Roots))
	seen := make(map[string]bool, len(r.Roots))
	for _, root := range r.Roots {
		full := filepath.Join(root, path)
		if !seen[full] {
			seen[full] = true
			out = append(out, full)
		}
	}

	return out
}

// kind asks bash what the path is: "file", "dir" or "missing".
func kind(ctx context.Context, path string) string {
	out, err := bash(ctx, existsScript, nil, path)
	if err != nil {
		return "missing"
	}

	return strings.TrimSpace(out)
}

// search looks for files named like base, first by the exact name and then by
// the name as a fragment, so "home.txt" also finds "my-home.txt.bak".
func (r *ReadFile) search(ctx context.Context, base string) []string {
	patterns := []string{base, "*" + base + "*"}
	if stem := strings.TrimSuffix(base, filepath.Ext(base)); stem != "" && stem != base {
		patterns = append(patterns, "*"+stem+"*")
	}

	var found []string
	seen := make(map[string]bool)

	for _, pattern := range patterns {
		env := []string{
			"PATTERN=" + pattern,
			fmt.Sprintf("DEPTH=%d", searchDepth),
			fmt.Sprintf("LIMIT=%d", maxSuggestions),
		}

		// A search that timed out or died still prints what it found before
		// giving up, so use whatever came back.
		out, _ := bash(ctx, searchScript, env, r.Roots...)

		for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || seen[line] {
				continue
			}
			seen[line] = true
			found = append(found, "  "+line)
			if len(found) == maxSuggestions {
				return found
			}
		}

		// The exact-name pass is the strongest signal; only widen the search
		// when it came back empty.
		if len(found) > 0 {
			break
		}
	}

	return found
}

// bash runs a script with the given extra environment. Arguments land in "$@",
// never inside the script text, so a path from the model cannot become code.
func bash(ctx context.Context, script string, env []string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, searchTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", append([]string{"-c", script, "read_file"}, args...)...)
	cmd.Env = append(os.Environ(), env...)

	out, err := cmd.Output()

	return string(out), err
}
