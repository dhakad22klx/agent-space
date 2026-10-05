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
	if err := os.WriteFile(Path(), []byte("model_provider:\n  model: yaml-model\ncustom: preserved\n"), 0o600); err != nil {
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
	if err != nil || values["GEMINI_MODEL"] != "saved-model" || values["GEMINI_API_KEY"] != "saved-key" {
		t.Fatal("provider values were not loaded from credentials")
	}
	values["GEMINI_MODEL"] = "selected-model"
	if err := SaveSecrets(values); err != nil {
		t.Fatal(err)
	}
	values, err = Runtime()
	if err != nil || values["GEMINI_MODEL"] != "selected-model" {
		t.Fatal("credential model did not override YAML model")
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
}
