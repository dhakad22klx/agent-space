// Package session records one run of the CLI as an append-only transcript: a
// file named after the run, one JSON object per line. Nothing here decides what
// is worth keeping; callers do, and the CLI's output layer already knows.
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// DefaultDir is where transcripts land, relative to where the CLI was started.
const DefaultDir = "sessions"

// Entry is one line of a transcript. Kind says what the line is; ID appears
// only on the header line that opens the file.
type Entry struct {
	Time string `json:"time"`
	Kind string `json:"kind"`
	ID   string `json:"id,omitempty"`
	Text string `json:"text,omitempty"`
}

// Session is the transcript of a single run. Appending is safe from several
// goroutines, and a write that fails is remembered rather than raised: losing a
// log line must never take the conversation down with it.
type Session struct {
	id   string
	path string

	mu   sync.Mutex
	file *os.File
	err  error // the first failure, reported by Err and Close
}

// Start mints a session id, creates dir if it is missing, and opens
// dir/<id>.jsonl for the run. An empty dir means DefaultDir.
func Start(dir string) (*Session, error) {
	if dir == "" {
		dir = DefaultDir
	}

	id, err := newID()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("cannot create %s: %w", dir, err)
	}

	path := filepath.Join(dir, id+".jsonl")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("cannot open %s: %w", path, err)
	}

	s := &Session{id: id, path: path, file: file}
	s.write(Entry{Kind: "session", ID: id})

	return s, nil
}

// ID is the session id, which is also the file name without its extension.
func (s *Session) ID() string { return s.id }

// Path is the file being written.
func (s *Session) Path() string { return s.path }

// Append adds one line to the transcript.
func (s *Session) Append(kind, text string) {
	s.write(Entry{Kind: kind, Text: text})
}

// Err reports the first write failure, if the transcript ever lost a line.
func (s *Session) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.err
}

// Close finishes the file and reports the first thing that went wrong, so a
// half-written transcript is not mistaken for a complete one.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.file == nil {
		return s.err
	}

	err := s.file.Close()
	s.file = nil

	if s.err == nil && err != nil {
		s.err = fmt.Errorf("cannot close %s: %w", s.path, err)
	}

	return s.err
}

// write is the one place a line reaches disk: it stamps the entry, encodes it,
// and keeps the first error for later.
func (s *Session) write(e Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.file == nil {
		return
	}

	e.Time = time.Now().UTC().Format(time.Stamp)

	line, err := json.Marshal(e)
	if err != nil {
		s.keep(fmt.Errorf("cannot encode a %s entry: %w", e.Kind, err))
		return
	}

	if _, err := s.file.Write(append(line, '\n')); err != nil {
		s.keep(fmt.Errorf("cannot write to %s: %w", s.path, err))
	}
}

// keep remembers the first failure only; the caller already holds the lock.
func (s *Session) keep(err error) {
	if s.err == nil {
		s.err = err
	}
}

// newID returns a random version 4 UUID, which is unique enough to name a run
// without asking anything outside this process.
func newID() (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("cannot generate a session id: %w", err)
	}

	return id.String(), nil
}
