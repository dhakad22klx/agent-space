package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"justsay-harness/internal"
)

func homeVariable() string {
	switch runtime.GOOS {
	case "windows":
		return "USERPROFILE"
	case "plan9":
		return "home"
	default:
		return "HOME"
	}
}

func TestStartDefaultUsesConfigDirAcrossWorkingDirectories(t *testing.T) {
	home := t.TempDir()
	t.Setenv(homeVariable(), home)
	wantDir := filepath.Join(home, internal.ConfigDir, DefaultDir)
	for range 2 {
		workingDir := t.TempDir()
		t.Chdir(workingDir)
		s, err := Start("")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = s.Close() })
		if want := filepath.Join(wantDir, s.ID()+".jsonl"); s.Path() != want {
			t.Fatalf("session path = %q; want %q", s.Path(), want)
		}
		s.Append("user", "hello")
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
		file, err := os.Open(s.Path())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = file.Close() })
		decoder := json.NewDecoder(file)
		var header, entry Entry
		if err := decoder.Decode(&header); err != nil {
			t.Fatal(err)
		}
		if err := decoder.Decode(&entry); err != nil {
			t.Fatal(err)
		}
		if header.Kind != "session" || header.ID != s.ID() || entry.Kind != "user" || entry.Text != "hello" {
			t.Fatalf("unexpected transcript: %+v, %+v", header, entry)
		}
		if _, err := os.Stat(filepath.Join(workingDir, DefaultDir)); !os.IsNotExist(err) {
			t.Fatalf("sessions directory exists in working directory: %v", err)
		}
	}
}

func TestStartExplicitDirectory(t *testing.T) {
	t.Setenv(homeVariable(), "")
	dir := filepath.Join(t.TempDir(), "transcripts")
	s, err := Start(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if want := filepath.Join(dir, s.ID()+".jsonl"); s.Path() != want {
		t.Fatalf("session path = %q; want %q", s.Path(), want)
	}
}

func TestStartDefaultMissingHome(t *testing.T) {
	if runtime.GOOS == "android" || runtime.GOOS == "ios" {
		t.Skip("UserHomeDir provides a platform fallback")
	}
	t.Setenv(homeVariable(), "")
	t.Chdir(t.TempDir())
	s, err := Start("")
	if s != nil {
		_ = s.Close()
	}
	if s != nil || err == nil {
		t.Fatalf("Start without home = %v, %v; want nil session and error", s, err)
	}
	if _, err := os.Stat(DefaultDir); !os.IsNotExist(err) {
		t.Fatalf("sessions directory exists in working directory: %v", err)
	}
}
