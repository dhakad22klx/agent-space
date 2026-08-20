package tui

// keyKind is what a keypress means, so the loop that acts on one never has to
// look at bytes.
type keyKind int

const (
	keyIgnore keyKind = iota
	keyRune
	keyEnter
	keyBackspace
	keyComplete
	keyUp
	keyDown
	keyInterrupt
	keyEOF
)

type key struct {
	kind keyKind
	ch   rune // set for keyRune only
}

// readKey turns the next bytes into one keypress. Reading a rune at a time
// means a multi-byte character can never be split across two reads, and neither
// can an escape sequence.
func (r *Reader) readKey() (key, error) {
	c, _, err := r.in.ReadRune()
	if err != nil {
		return key{}, err
	}

	switch c {
	case '\r', '\n':
		// Raw mode leaves the carriage return untranslated, so this is what
		// Enter actually arrives as.
		return key{kind: keyEnter}, nil
	case 0x7f, 0x08:
		return key{kind: keyBackspace}, nil
	case '\t':
		return key{kind: keyComplete}, nil
	case 0x03:
		// Ctrl-C. With signals switched off it is ours to handle, and clearing
		// the line is the more useful thing to do with it than quitting.
		return key{kind: keyInterrupt}, nil
	case 0x04:
		return key{kind: keyEOF}, nil
	case 0x1b:
		return r.readEscape()
	}

	// Remaining control characters are ones we have no use for, and letting
	// them into the line would put unprintable bytes in a command name.
	if c < 0x20 {
		return key{kind: keyIgnore}, nil
	}

	return key{kind: keyRune, ch: c}, nil
}

// readEscape consumes the rest of a sequence so its bytes never reach the line
// as text. Arrow keys arrive two ways: as CSI normally, and as SS3 from
// terminals in application-keypad mode, which is the state tmux commonly
// leaves them in. Missing the second is why arrow keys "work everywhere except
// tmux" in hand-written readers.
func (r *Reader) readEscape() (key, error) {
	c, _, err := r.in.ReadRune()
	if err != nil {
		return key{}, err
	}

	switch c {
	case '[':
		return r.readCSI()
	case 'O':
		final, _, err := r.in.ReadRune()
		if err != nil {
			return key{}, err
		}

		return arrow(final), nil
	}

	return key{kind: keyIgnore}, nil
}

// readCSI walks the parameter bytes to the final one that says what the
// sequence was. Taking a fixed three bytes instead would leave the parameters
// of anything longer behind, so ctrl-up, "\x1b[1;5A", would type "1;5" into the
// line.
func (r *Reader) readCSI() (key, error) {
	for {
		c, _, err := r.in.ReadRune()
		if err != nil {
			return key{}, err
		}

		// A final byte ends the sequence and identifies it.
		if c >= 0x40 && c <= 0x7e {
			return arrow(c), nil
		}

		// Parameter and intermediate bytes continue it. Anything else is a
		// sequence we have lost track of, better dropped than guessed at.
		if c < 0x20 || c > 0x3f {
			return key{kind: keyIgnore}, nil
		}
	}
}

// arrow picks out the two sequences the palette cares about and discards the
// rest, which is what makes a stray mouse report or function key harmless.
func arrow(final rune) key {
	switch final {
	case 'A':
		return key{kind: keyUp}
	case 'B':
		return key{kind: keyDown}
	}

	return key{kind: keyIgnore}
}
