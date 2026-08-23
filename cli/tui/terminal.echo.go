package tui

import (
	"os"
	"os/exec"
)

// HideTyping stops the terminal echoing what is typed next, for a token or a
// password, and returns the call that puts echo back.
//
// Hiding is a courtesy, not a guarantee. When the terminal will not cooperate —
// input arriving from a pipe, no stty on the machine — the characters simply
// stay visible instead of the prompt failing, because the check the user asked
// for matters more than how the typing looked. What keeps a secret out of the
// saved transcript is the caller, not this.
func HideTyping() (restore func()) {
	if !stty("-echo") {
		return func() {}
	}

	return func() { stty("echo") }
}

// stty asks the terminal driver to change one setting and reports whether it
// took. stdin is handed over because stty acts on the terminal it is given, not
// on the one its parent happens to have.
func stty(mode string) bool {
	command := exec.Command("stty", mode)
	command.Stdin = os.Stdin

	return command.Run() == nil
}
