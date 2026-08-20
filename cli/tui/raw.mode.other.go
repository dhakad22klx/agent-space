//go:build !linux

package tui

import (
	"errors"
	"os"
)

// Raw input is wired up for Linux only, so everywhere else these stand in.
// Failing here is not a problem to report: ReadLine treats it the same as a
// stdin that is not a terminal and falls back to reading plain lines, so the
// prompt still works, just without a palette.

type termState struct{}

func rawMode(fd int) (*termState, error) {
	return nil, errors.New("raw mode is implemented on linux only")
}

func (s *termState) restore(fd int) error { return nil }

func termWidth(fd int) int { return defaultWidth }

func isTerminal(f *os.File) bool { return false }
