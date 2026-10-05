package config

import (
	"os"
	"strings"
	"testing"

	"justsay-harness/credentials"
)

func TestLoadIgnoresPreviousProviderSections(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile(Path(), []byte("gemini:\n  model: previous-model\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := credentials.Open("")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Set("gemini", map[string]string{"api_key": "previous-key", "model": "previous-model"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	values, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if values["GEMINI_MODEL"] != "" || values["GEMINI_API_KEY"] != "" {
		t.Fatal("previous provider sections were read")
	}
}

func TestModelProviderSettings(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile(Path(), []byte("model_provider:\n  name: gemini\n  model: yaml-model\n  api_key: yaml-key\ncustom: preserved\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := credentials.Open("")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Set(providerSection, map[string]string{"api_key": "saved-key", "model": "saved-model", "extra": "preserved"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Set("telegram", map[string]string{"token": "saved-token"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	values, err := Load()
	if err != nil || values["MODEL_PROVIDER"] != "gemini" || values["GEMINI_MODEL"] != "saved-model" || values["GEMINI_API_KEY"] != "saved-key" {
		t.Fatal("provider values were not loaded from credentials")
	}
	values["GEMINI_MODEL"] = "selected-model"
	if err := SaveSecrets(values); err != nil {
		t.Fatal(err)
	}
	values, err = Runtime()
	if err != nil || values["GEMINI_MODEL"] != "selected-model" {
		t.Fatal("model was not loaded from credentials")
	}
	if err := SaveSettings(values); err != nil {
		t.Fatal(err)
	}
	store, err = credentials.Open("")
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]string
	if found, err := store.Get(providerSection, &record); !found || err != nil || record["model"] != "selected-model" || record["api_key"] != "saved-key" || record["extra"] != "preserved" {
		t.Fatal("saving model lost existing provider values")
	}
	if found, err := store.Get("telegram", &record); !found || err != nil || record["token"] != "saved-token" {
		t.Fatal("unrelated credentials changed")
	}
	yaml, err := os.ReadFile(Path())
	if err != nil || !strings.Contains(string(yaml), "model_provider:") || !strings.Contains(string(yaml), "custom: preserved") {
		t.Fatal("YAML provider settings were not saved")
	}
	if !strings.Contains(string(yaml), "name: gemini") || strings.Contains(string(yaml), "model:") || strings.Contains(string(yaml), "api_key:") {
		t.Fatal("YAML must contain the provider name without the model or API key")
	}
}

func TestProviderCredentialsHaveNoFallback(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile(Path(), []byte("model_provider:\n  name: gemini\n  model: yaml-model\n  api_key: yaml-key\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".env", []byte("GEMINI_MODEL=env-model\nGEMINI_API_KEY=env-key\nMOCK_AGENT_CALL=true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, load := range []func() (map[string]string, error){Load, Runtime} {
		values, err := load()
		if err != nil {
			t.Fatal(err)
		}
		if values["GEMINI_MODEL"] != "" || values["GEMINI_API_KEY"] != "" {
			t.Fatal("provider credentials were read outside credentials.json")
		}
		if values["MODEL_PROVIDER"] != "gemini" {
			t.Fatal("provider name was not read from config.yml")
		}
	}
}
