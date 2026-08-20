package tui

import (
	"bufio"
	"io"
	"os"
	"strconv"
	"strings"
)

// Reader reads a line at a time, redrawing it as it is typed so a palette can
// appear underneath. It owns the prompt: callers must not print one themselves,
// or two will appear.
type Reader struct {
	// Suggest returns the palette rows for the line typed so far, and nothing
	// when there are none to show. Leaving it nil makes this a plain line
	// editor. It is a hook rather than an interface because cli imports tui,
	// so the dependency cannot run the other way.
	Suggest func(line string) []Item

	in          *bufio.Reader
	out         *Output
	fd          int
	interactive bool
}

// NewReader wires a reader to the stream Output writes on, settling once
// whether this session can be drawn on at all.
func NewReader(out *Output) *Reader {
	stdin := os.Stdin

	return &Reader{
		// One buffered reader for the whole session and never one per line:
		// bytes already pulled in from a paste would be dropped in between.
		in:  bufio.NewReader(stdin),
		out: out,
		fd:  int(stdin.Fd()),
		// Both streams have to be terminals. Stdin decides whether raw mode is
		// possible at all; stdout matters because piping into a file should
		// fill it with text, not with cursor movements.
		interactive: isTerminal(stdin) && isTTY(out.stdout),
	}
}

// ReadLine prints the prompt and returns the line the user submitted. With the
// palette open, Enter returns the highlighted row's label as though it had been
// typed, which is what keeps callers down to a single way of dispatching.
func (r *Reader) ReadLine(prompt string) (string, error) {
	if !r.interactive {
		return r.readPlain()
	}

	state, err := rawMode(r.fd)
	if err != nil {
		return r.readPlain()
	}
	// Raw mode lasts one line and no longer. That is what lets every other
	// caller of Output keep writing plain "\n" between prompts, and the defer
	// covers a panic as well as a return: a terminal left raw is a terminal
	// the user has to run reset on.
	defer state.restore(r.fd)

	return r.readEdited(prompt)
}

// readPlain serves pipes, redirects, and platforms without raw mode. A last
// line with no newline before the end of input is still a line.
func (r *Reader) readPlain() (string, error) {
	line, err := r.in.ReadString('\n')
	line = strings.TrimRight(line, "\r\n")

	if err != nil && line != "" {
		return line, nil
	}

	return line, err
}

// readEdited is the key loop. The palette is not a mode it switches into: it is
// a function of the line so far, recomputed on every edit, which is why typing
// a slash mid-line or backspacing over one needs no handling of its own.
func (r *Reader) readEdited(prompt string) (string, error) {
	var (
		line []rune
		menu palette
	)

	r.refresh(&menu, line)
	r.draw(prompt, line, &menu)

	for {
		pressed, err := r.readKey()
		if err != nil {
			return "", err
		}

		switch pressed.kind {
		case keyEnter:
			submitted := string(line)
			if item, open := menu.current(); open {
				submitted = item.Label
			}
			r.commit(prompt, line)

			return submitted, nil

		case keyEOF:
			if len(line) > 0 {
				continue
			}
			r.commit(prompt, line)

			return "", io.EOF

		case keyInterrupt:
			// Signals are off while raw, so ctrl-c is ours to answer: it
			// throws away a line that has something on it, and leaves when
			// there is nothing left to throw away.
			if len(line) == 0 {
				r.commit(prompt, line)

				return "", io.EOF
			}
			line = line[:0]

		case keyBackspace:
			if len(line) > 0 {
				line = line[:len(line)-1]
			}

		case keyComplete:
			// Filling the line in instead of running it is the difference
			// between completing and choosing.
			if item, open := menu.current(); open {
				line = []rune(item.Label)
			}

		case keyUp:
			menu.up()

		case keyDown:
			menu.down()

		case keyRune:
			line = append(line, pressed.ch)

		default:
			continue
		}

		// Moving the highlight works within the list it was given; everything
		// else changed the line, so that list is now stale.
		if pressed.kind != keyUp && pressed.kind != keyDown {
			r.refresh(&menu, line)
		}

		// Mid-paste there is no point drawing a frame nobody will see. Waiting
		// for the buffer to drain turns thousands of them into one.
		if r.in.Buffered() == 0 {
			r.draw(prompt, line, &menu)
		}
	}
}

func (r *Reader) refresh(menu *palette, line []rune) {
	if r.Suggest == nil {
		menu.set(nil)
		return
	}

	menu.set(r.Suggest(string(line)))
}

// draw replaces everything from the input line down. Composing it all and
// writing once is what keeps the frame from flickering.
func (r *Reader) draw(prompt string, line []rune, menu *palette) {
	var frame strings.Builder

	// Clearing to the end of the screen, rather than drawing over the rows
	// that were there, is why a shrinking list leaves nothing behind.
	frame.WriteString("\r\x1b[J")
	frame.WriteString(Magenta(prompt))
	frame.WriteString(string(line))

	rows := menu.render(termWidth(r.fd))
	for _, row := range rows {
		// A bare "\n" would only drop a line: raw mode turned off the
		// translation that adds the carriage return.
		frame.WriteString("\r\n")
		frame.WriteString(row)
	}

	// Back to where the cursor belongs, by relative moves only. Saving and
	// restoring an absolute position breaks at the bottom of the screen, where
	// drawing the rows scrolls the view and the saved row is no longer the
	// input line. The column is counted off the unpainted strings, since the
	// color escapes in the painted ones occupy no cells.
	move(&frame, len(rows), 'A')
	frame.WriteString("\r")
	move(&frame, runes(prompt)+len(line), 'C')

	r.out.Print(Frame, frame.String())
}

// commit leaves the finished line on screen with the palette erased and the
// cursor at the start of a fresh line, which is where ordinary output expects
// to pick up.
func (r *Reader) commit(prompt string, line []rune) {
	r.out.Print(Frame, "\r\x1b[J"+Magenta(prompt)+string(line)+"\r\n")
}

// move writes a cursor movement, or nothing at all for a distance of zero.
// Writing the escape anyway would move one cell, because a sequence with no
// number in it means one.
func move(frame *strings.Builder, cells int, direction rune) {
	if cells > 0 {
		frame.WriteString("\x1b[")
		frame.WriteString(strconv.Itoa(cells))
		frame.WriteString(string(direction))
	}
}

// isTTY asks of a writer what isTerminal asks of stdin, for the streams Output
// holds as plain writers. It is also what makes NewOutputTo into a buffer fall
// back on its own, so the CLI stays testable.
func isTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && isTerminal(f)
}
