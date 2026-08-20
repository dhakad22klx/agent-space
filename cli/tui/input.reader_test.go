package tui

import (
	"bufio"
	"bytes"
	"io"
	"strings"
	"testing"
)

// newTestReader drives a Reader off a fixed script of bytes instead of a
// terminal, which is the only way to press keys in a test.
func newTestReader(keys string, suggest func(string) []Item) (*Reader, *bytes.Buffer) {
	screen := &bytes.Buffer{}

	return &Reader{
		Suggest: suggest,
		in:      bufio.NewReader(strings.NewReader(keys)),
		out:     NewOutputTo(screen, io.Discard),
		fd:      -1, // no terminal, so termWidth falls back to defaultWidth
	}, screen
}

func twoCommands(line string) []Item {
	all := []Item{
		{Label: "/session", Hint: "manage the conversation"},
		{Label: "/integrations", Hint: "show providers and tools"},
	}

	if !strings.HasPrefix(line, "/") {
		return nil
	}

	var matched []Item
	for _, item := range all {
		if strings.HasPrefix(item.Label, line) {
			matched = append(matched, item)
		}
	}

	return matched
}

const (
	up   = "\x1b[A"
	down = "\x1b[B"
	cr   = "\r"
	tab  = "\t"
)

func TestReadEditedSubmits(t *testing.T) {
	tests := []struct {
		name string
		keys string
		want string
	}{
		{"plain line", "hello" + cr, "hello"},
		{"backspace drops a rune", "helxx\x7f\x7flo" + cr, "hello"},
		{"backspace is rune-wise, not byte-wise", "héé\x7f" + cr, "hé"},
		{"ctrl-c clears a line with text on it", "junk\x03kept" + cr, "kept"},
		{"enter with palette open takes the highlight", "/" + cr, "/session"},
		{"down moves the highlight", "/" + down + cr, "/integrations"},
		{"down clamps at the last row", "/" + down + down + down + cr, "/integrations"},
		{"up clamps at the first row", "/" + up + up + cr, "/session"},
		{"typing filters the list", "/in" + cr, "/integrations"},
		{"a filter that matches nothing submits as typed", "/nope" + cr, "/nope"},
		{"backspacing past the slash closes the palette", "x/s\x7f\x7f" + cr, "x"},
		{"a slash mid-line opens nothing", "run /s" + cr, "run /s"},
		{"tab completes without running", "/in" + tab + cr, "/integrations"},
		{"SS3 arrows work like CSI ones", "/\x1bOB" + cr, "/integrations"},
		{"an unknown escape is swallowed, not typed", "a\x1b[1;5Ab" + cr, "ab"},
		{"a filter narrowing past the highlight resets it", "/" + down + "s" + cr, "/session"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reader, _ := newTestReader(tc.keys, twoCommands)

			got, err := reader.readEdited("> ")
			if err != nil {
				t.Fatalf("readEdited: %v", err)
			}

			if got != tc.want {
				t.Errorf("submitted %q, want %q", got, tc.want)
			}
		})
	}
}

func TestReadEditedCtrlD(t *testing.T) {
	// On an empty line ctrl-d ends the session; with text pending it must not.
	reader, _ := newTestReader("\x04", twoCommands)
	if _, err := reader.readEdited("> "); err != io.EOF {
		t.Errorf("ctrl-d on an empty line: got %v, want io.EOF", err)
	}

	reader, _ = newTestReader("text\x04 more"+cr, twoCommands)
	got, err := reader.readEdited("> ")
	if err != nil {
		t.Fatalf("readEdited: %v", err)
	}

	if got != "text more" {
		t.Errorf("ctrl-d mid-line: got %q, want %q", got, "text more")
	}
}

func TestReadEditedCtrlC(t *testing.T) {
	// Ctrl-c on an empty line leaves. It has to be handled here rather than
	// left to a signal: SIGINT would kill the process with the terminal still
	// raw, skipping the restore and leaving the shell with no echo.
	reader, _ := newTestReader("\x03", twoCommands)
	if _, err := reader.readEdited("> "); err != io.EOF {
		t.Errorf("ctrl-c on an empty line: got %v, want io.EOF", err)
	}

	// Clearing a line and then pressing it again must also leave, since the
	// line is empty by then.
	reader, _ = newTestReader("junk\x03\x03", twoCommands)
	if _, err := reader.readEdited("> "); err != io.EOF {
		t.Errorf("ctrl-c twice: got %v, want io.EOF", err)
	}
}

func TestFrameLeavesCursorOnTheInputLine(t *testing.T) {
	// The frame's contract: erase from the input line down, draw, then come
	// back. Without the return moves the next keystroke lands in the palette.
	reader, screen := newTestReader("", twoCommands)

	var menu palette
	menu.set(twoCommands("/"))
	reader.draw("> ", []rune("/"), &menu)

	frame := screen.String()

	if !strings.HasPrefix(frame, "\r\x1b[J") {
		t.Errorf("frame must open by erasing to the end of the screen, got %q", frame[:8])
	}

	if strings.Count(frame, "\r\n") != 2 {
		t.Errorf("want 2 row breaks for 2 rows, got %d in %q", strings.Count(frame, "\r\n"), frame)
	}

	// Two rows down, so two rows back up, then out to just past "> /".
	if !strings.HasSuffix(frame, "\x1b[2A\r\x1b[3C") {
		t.Errorf("frame must return the cursor to the input line, got tail %q", frame[len(frame)-12:])
	}

	if strings.Contains(frame, "\x1b7") || strings.Contains(frame, "\x1b8") {
		t.Error("frame must not save/restore an absolute cursor position: it breaks when drawing scrolls the screen")
	}
}

func TestFrameNeverMovesByZero(t *testing.T) {
	// "\x1b[0A" moves one row, not none, because a sequence with no number in
	// it means one. An empty line with no palette must emit neither move.
	reader, screen := newTestReader("", nil)

	var menu palette
	reader.draw("", nil, &menu)

	for _, zero := range []string{"\x1b[0A", "\x1b[0C"} {
		if frame := screen.String(); strings.Contains(frame, zero) {
			t.Errorf("emitted %q, which moves one cell rather than none: %q", zero, frame)
		}
	}
}

func TestFrameSkipsRedrawMidPaste(t *testing.T) {
	// A paste arrives as one burst. Drawing per character would emit thousands
	// of frames for a screenful of text.
	reader, screen := newTestReader(strings.Repeat("x", 500)+cr, twoCommands)

	if _, err := reader.readEdited("> "); err != nil {
		t.Fatalf("readEdited: %v", err)
	}

	// One opening frame, and the commit. Anything near 500 means the gate is
	// not working.
	if frames := strings.Count(screen.String(), "\x1b[J"); frames > 3 {
		t.Errorf("drew %d frames for one paste, want at most 3", frames)
	}
}

func TestRowsStayOnOneLine(t *testing.T) {
	// A row that wraps takes two physical lines, and the frame's count of how
	// far to move back up is then wrong by one.
	var menu palette
	menu.set(twoCommands("/"))

	for _, width := range []int{80, 20, 8, 3, 1} {
		for _, row := range menu.render(width) {
			if plain := runes(stripColor(row)); plain > width-1 {
				t.Errorf("width %d: row is %d cells wide: %q", width, plain, stripColor(row))
			}
		}
	}
}

func TestReadPlainKeepsLastLineWithoutNewline(t *testing.T) {
	// bufio returns io.EOF together with the final unterminated line. Treating
	// that as failure would silently drop it.
	reader, _ := newTestReader("only", nil)

	got, err := reader.readPlain()
	if err != nil || got != "only" {
		t.Errorf("readPlain = %q, %v; want %q, nil", got, err, "only")
	}
}

// stripColor removes the SGR sequences the palette paints rows with, leaving
// the cells that actually occupy screen width.
func stripColor(text string) string {
	var plain strings.Builder

	for i := 0; i < len(text); {
		if text[i] == 0x1b {
			for i < len(text) && text[i] != 'm' {
				i++
			}
			i++
			continue
		}

		plain.WriteByte(text[i])
		i++
	}

	return plain.String()
}
