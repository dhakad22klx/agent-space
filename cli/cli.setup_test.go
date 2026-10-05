package cli

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"justsay-harness/config"
	"justsay-harness/credentials"
)

type setupInput struct {
	answers []string
	prompts []string
	secrets []bool
}

func (in *setupInput) read(prompt string, secret bool) (string, error) {
	in.prompts = append(in.prompts, prompt)
	in.secrets = append(in.secrets, secret)
	if len(in.answers) == 0 {
		return "", io.EOF
	}
	answer := in.answers[0]
	in.answers = in.answers[1:]
	return answer, nil
}

func TestSetupUsesConfigAndCredentialFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	// Setup must never read or rewrite .env, even when it is malformed.
	legacy := []byte("legacy-secret invalid syntax")
	if err := os.WriteFile(".env", legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := credentials.Open("")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Set("telegram", map[string]string{"bot_token": "existing-token"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.Path(), []byte("state:\n  backend: inmemory\ncustom:\n  untouched: keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	in := &setupInput{answers: []string{"api-secret", "", "yes", "redis", "redis://alice:url-secret@localhost:6379/2", "", "", ""}}
	var out bytes.Buffer
	if err := runSetup(in, &out); err != nil {
		t.Fatal(err)
	}
	values, err := config.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	if values["GEMINI_API_KEY"] != "api-secret" || values["GEMINI_MODEL"] != "gemini-3.5-flash-lite" || values["REDIS_PASSWORD"] != "url-secret" || values["REDIS_USERNAME"] != "alice" {
		t.Fatal("saved values incorrect")
	}
	yaml, err := os.ReadFile(config.Path())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(yaml), "api-secret") || strings.Contains(string(yaml), "url-secret") {
		t.Fatal("secrets stored in config.yml")
	}
	if !strings.Contains(string(yaml), "untouched: keep") {
		t.Fatal("unrelated config fields changed")
	}
	store, err = credentials.Open("")
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]string
	if found, err := store.Get("model_provider", &record); !found || err != nil || record["api_key"] != "api-secret" || record["model"] != "gemini-3.5-flash-lite" {
		t.Fatal("provider API key and model were not saved under model_provider")
	}
	record = nil
	if found, err := store.Get("telegram", &record); !found || err != nil || record["bot_token"] != "existing-token" {
		t.Fatal("existing integration credentials changed")
	}
	info, err := os.Stat(credentials.Path())
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatal("credential permissions incorrect")
	}
	in = &setupInput{answers: make([]string, 8)}
	if err := runSetup(in, &out); err != nil {
		t.Fatal(err)
	}
	values, err = config.Runtime()
	if err != nil || values["GEMINI_API_KEY"] != "api-secret" || values["GEMINI_MODEL"] != "gemini-3.5-flash-lite" || values["REDIS_PASSWORD"] != "url-secret" {
		t.Fatal("defaults not preserved")
	}
	if !in.secrets[0] || !in.secrets[4] || !in.secrets[6] {
		t.Fatal("sensitive fields are not masked")
	}
	for _, secret := range []string{"api-secret", "url-secret", "existing-token", "legacy-secret"} {
		if strings.Contains(out.String()+strings.Join(in.prompts, " "), secret) {
			t.Fatal("secret appeared in output")
		}
	}
	got, _ := os.ReadFile(".env")
	if !bytes.Equal(got, legacy) {
		t.Fatal("setup changed .env")
	}
}

func TestSetupRejectsInvalidOrCancelledInput(t *testing.T) {
	for _, answers := range [][]string{{"", ""}, {"secret"}, {"secret", "", "maybe"}, {"secret", "", "yes", "sqlite"}, {"secret", "", "yes", "redis", "https://secret@host"}} {
		t.Chdir(t.TempDir())
		var out bytes.Buffer
		err := runSetup(&setupInput{answers: answers}, &out)
		if err == nil {
			t.Fatal("invalid setup succeeded")
		}
		if strings.Contains(err.Error()+out.String(), "secret") {
			t.Fatal("error exposed secret")
		}
		for _, path := range []string{config.Path(), credentials.Path()} {
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("failed setup wrote %s", path)
			}
		}
	}
}

func TestSetupStateBackendSelection(t *testing.T) {
	for _, tc := range []struct {
		name, hitl, choice, backend string
		redis                       bool
	}{
		{"HITL disabled", "no", "", "inmemory", false},
		{"default memory", "yes", "", "inmemory", false},
		{"explicit memory", "yes", "inmemory", "inmemory", false},
		{"Redis", "yes", "redis", "redis", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			answers := []string{"api-key", "", tc.hitl}
			if tc.hitl == "yes" {
				answers = append(answers, tc.choice)
			}
			if tc.redis {
				answers = append(answers, "localhost:6379", "", "redis-password", "")
			}
			in := &setupInput{answers: answers}
			if err := runSetup(in, io.Discard); err != nil {
				t.Fatal(err)
			}
			values, err := config.Load()
			if err != nil || values["HITL_STATE_STORE"] != tc.backend {
				t.Fatal("incorrect saved backend")
			}
			askedRedis := false
			for _, prompt := range in.prompts {
				askedRedis = askedRedis || strings.HasPrefix(prompt, "Redis ")
			}
			if askedRedis != tc.redis || len(in.answers) != 0 {
				t.Fatal("incorrect setup prompts")
			}
			// Re-running setup keeps the selected backend when Enter is pressed.
			if err := runSetup(&setupInput{answers: make([]string, len(answers))}, io.Discard); err != nil {
				t.Fatal(err)
			}
			values, err = config.Load()
			if err != nil || values["HITL_STATE_STORE"] != tc.backend {
				t.Fatal("saved backend default changed")
			}
			if tc.redis {
				if err := runSetup(&setupInput{answers: []string{"", "", "", "inmemory"}}, io.Discard); err != nil {
					t.Fatal(err)
				}
				values, err = config.Load()
				if err != nil || values["HITL_STATE_STORE"] != "inmemory" || values["REDIS_PASSWORD"] != "redis-password" {
					t.Fatal("switching backend lost saved Redis credentials")
				}
			}
		})
	}
}
