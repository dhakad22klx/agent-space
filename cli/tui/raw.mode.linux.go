//go:build linux

package tui

import (
	"os"

	"golang.org/x/sys/unix"
)

// termState holds the settings raw mode replaced, so restore can put the
// terminal back exactly as it was found.
type termState struct{ prev unix.Termios }

// rawMode asks for every keystroke as it is typed rather than a line at a time,
// which is the whole reason the palette can appear while the user is still
// typing. The flags cleared here are that difference: canonical mode and echo
// on the way in, and on the way out the newline translation that would
// otherwise stair-step every frame the reader draws.
func rawMode(fd int) (*termState, error) {
	prev, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return nil, err
	}

	raw := *prev
	raw.Iflag &^= unix.BRKINT | unix.ICRNL | unix.INPCK | unix.ISTRIP | unix.IXON
	raw.Oflag &^= unix.OPOST
	raw.Lflag &^= unix.ECHO | unix.ICANON | unix.IEXTEN | unix.ISIG
	raw.Cc[unix.VMIN] = 1  // hand us a byte as soon as there is one
	raw.Cc[unix.VTIME] = 0 // and never wait around for a second

	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &raw); err != nil {
		return nil, err
	}

	return &termState{prev: *prev}, nil
}

func (s *termState) restore(fd int) error {
	return unix.IoctlSetTermios(fd, unix.TCSETS, &s.prev)
}

// termWidth is how wide a row may be before it wraps onto a second physical
// line, which would put the cursor arithmetic in draw one line out.
func termWidth(fd int) int {
	size, err := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ)
	if err != nil || size.Col == 0 {
		return defaultWidth
	}

	return int(size.Col)
}

// isTerminal reports whether f is a terminal, asked the only way that is
// actually reliable: by trying a call that only terminals answer.
func isTerminal(f *os.File) bool {
	_, err := unix.IoctlGetTermios(int(f.Fd()), unix.TCGETS)
	return err == nil
}
