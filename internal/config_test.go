package internal

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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

func TestConfigPathUsesHomeAcrossWorkingDirectories(t *testing.T) {
	home := t.TempDir()
	t.Setenv(homeVariable(), home)
	for range 2 {
		t.Chdir(t.TempDir())
		for _, name := range []string{"config.yml", "credentials.json"} {
			got, err := configPath(".justsay", name)
			want := filepath.Join(home, ".justsay", name)
			if err != nil || got != want {
				t.Fatalf("configPath = %q, %v; want %q", got, err, want)
			}
		}
	}
}

func TestConfigPathLocalAndAbsoluteDirectories(t *testing.T) {
	t.Setenv(homeVariable(), "")
	for _, dir := range []string{".", t.TempDir()} {
		got, err := configPath(dir, "config.yml")
		if err != nil || got != filepath.Join(dir, "config.yml") {
			t.Fatalf("configPath(%q) = %q, %v", dir, got, err)
		}
	}
}

func TestConfigPathMissingHome(t *testing.T) {
	if runtime.GOOS == "android" || runtime.GOOS == "ios" {
		t.Skip("UserHomeDir provides a platform fallback")
	}
	t.Setenv(homeVariable(), "")
	path, err := configPath(".justsay", "credentials.json")
	if path != "" || err == nil || !strings.Contains(err.Error(), "home directory") {
		t.Fatalf("configPath without home = %q, %v", path, err)
	}
}

func TestConfigPathDoesNotCreateDirectoriesOnRead(t *testing.T) {
	home := t.TempDir()
	t.Setenv(homeVariable(), home)
	if _, err := configPath(".justsay", "config.yml"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".justsay")); !os.IsNotExist(err) {
		t.Fatalf("resolving a path created a directory: %v", err)
	}
}
