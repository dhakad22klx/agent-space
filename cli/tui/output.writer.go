package tui

import (
	"fmt"
	"io"
	"os"
)

// Type says what a message is, never how it looks. Callers pick a type and the
// table below decides the rest, which is also what a later save step will read
// to tell what is worth keeping.
type Type int

const (
	Plain Type = iota
	Banner
	Farewell
	Prompt
	Break
	Notice
	Trace
	Answer
	Warn
	Error
)

// style is everything this layer decides about a type: how it is written, and
// whether it belongs in the transcript.
type style struct {
	name    string              // what this type is called in a saved transcript
	paint   func(string) string // nil leaves the text uncolored
	stderr  bool
	newline bool
	keep    bool // false means the session's transcript ignores this type, even if it is printed
}

// styles is the rendering table: change how a type appears everywhere by
// editing one row.
var styles = map[Type]style{
	Plain:    {name: "plain", newline: true, keep: true},
	Banner:   {name: "banner", paint: Blue, newline: true},
	Farewell: {name: "farewell", paint: Red, newline: true},
	Prompt:   {name: "prompt", paint: Magenta},
	Break:    {name: "break", newline: true},
	Notice:   {name: "notice", paint: Gray, newline: true, keep: true},
	Trace:    {name: "trace", paint: Gray, newline: true, keep: true},
	Answer:   {name: "answer", paint: Green, newline: true, keep: true},
	Warn:     {name: "warn", paint: Yellow, newline: true, keep: true},
	Error:    {name: "error", paint: Red, stderr: true, newline: true, keep: true},
}

// Sink receives every message worth keeping, named by kind and still uncolored,
// so a transcript can be recorded without a single caller printing differently.
type Sink interface {
	Append(kind, text string)
}

// Output is the one place output passes through. Callers say what a message
// means; this decides how it is written.
type Output struct {
	stdout io.Writer
	stderr io.Writer
	sink   Sink
}

// NewOutput writes to the process streams.
func NewOutput() *Output {
	return NewOutputTo(os.Stdout, os.Stderr)
}

// NewOutputTo sends output somewhere else, which is what makes the CLI
// redirectable and testable.
func NewOutputTo(stdout, stderr io.Writer) *Output {
	return &Output{stdout: stdout, stderr: stderr}
}

// Record starts copying what is printed into sink. Passing nil stops it.
func (o *Output) Record(sink Sink) { o.sink = sink }

func (o *Output) Banner(text string)   { o.Print(Banner, text) }
func (o *Output) Farewell(text string) { o.Print(Farewell, text) }
func (o *Output) Prompt(text string)   { o.Print(Prompt, text) }
func (o *Output) Plain(text string)    { o.Print(Plain, text) }
func (o *Output) Notice(text string)   { o.Print(Notice, text) }
func (o *Output) Trace(text string)    { o.Print(Trace, text) }
func (o *Output) Answer(text string)   { o.Print(Answer, text) }
func (o *Output) Warn(text string)     { o.Print(Warn, text) }
func (o *Output) Error(text string)    { o.Print(Error, text) }

// Break ends the current line without saying anything, for when the terminal
// swallowed the newline the user typed.
func (o *Output) Break() { o.Print(Break, "") }

// Tracef and Errorf spare callers a Sprintf at the call site.
func (o *Output) Tracef(format string, args ...any) {
	o.Print(Trace, fmt.Sprintf(format, args...))
}

func (o *Output) Errorf(format string, args ...any) {
	o.Print(Error, fmt.Sprintf(format, args...))
}

// Print is the choke point every other method funnels into: the single spot
// that colors the text, picks the stream, and decides what to store.
func (o *Output) Print(messageType Type, text string) {
	s, ok := styles[messageType]
	if !ok {
		s = styles[Plain]
	}

	// The transcript keeps what was said, not how it was painted.
	if o.sink != nil && s.keep {
		o.sink.Append(s.name, text)
	}

	if s.paint != nil {
		text = s.paint(text)
	}

	if s.newline {
		text += "\n"
	}

	writer := o.stdout
	if s.stderr {
		writer = o.stderr
	}

	fmt.Fprint(writer, text)
}
