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
	Notice
	Trace
	Answer
	Warn
	Error
	Frame
)

// style is everything this layer decides about a type.
type style struct {
	paint   func(string) string // nil leaves the text uncolored
	stderr  bool
	newline bool
}

// styles is the rendering table: change how a type appears everywhere by
// editing one row.
var styles = map[Type]style{
	Plain:    {newline: true},
	Banner:   {paint: Blue, newline: true},
	Farewell: {paint: Red, newline: true},
	Prompt:   {paint: Magenta},
	Notice:   {paint: Gray, newline: true},
	Trace:    {paint: Gray, newline: true},
	Answer:   {paint: Green, newline: true},
	Warn:     {paint: Yellow, newline: true},
	Error:    {paint: Red, stderr: true, newline: true},
	// Frame is the reader redrawing the input line: it composed the whole
	// thing itself, line endings and colors included, so nothing is added.
	Frame: {},
}

// Output is the one place output passes through. Callers say what a message
// means; this decides how it is written.
type Output struct {
	stdout io.Writer
	stderr io.Writer
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

func (o *Output) Banner(text string)   { o.Print(Banner, text) }
func (o *Output) Farewell(text string) { o.Print(Farewell, text) }
func (o *Output) Prompt(text string)   { o.Print(Prompt, text) }
func (o *Output) Plain(text string)    { o.Print(Plain, text) }
func (o *Output) Notice(text string)   { o.Print(Notice, text) }
func (o *Output) Trace(text string)    { o.Print(Trace, text) }
func (o *Output) Answer(text string)   { o.Print(Answer, text) }
func (o *Output) Warn(text string)     { o.Print(Warn, text) }
func (o *Output) Error(text string)    { o.Print(Error, text) }

// Tracef and Errorf spare callers a Sprintf at the call site.
func (o *Output) Tracef(format string, args ...any) {
	o.Print(Trace, fmt.Sprintf(format, args...))
}

func (o *Output) Errorf(format string, args ...any) {
	o.Print(Error, fmt.Sprintf(format, args...))
}

// Print is the choke point every other method funnels into: the single spot
// that colors the text, picks the stream, and later decides what to store.
func (o *Output) Print(messageType Type, text string) {
	s, ok := styles[messageType]
	if !ok {
		s = styles[Plain]
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
