// Package config reads agent settings beside the shared credentials file.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"justsay-harness/credentials"
	"justsay-harness/internal"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

const FileName = "config.yml"

const providerSection = "model_provider"

var ErrInvalidBackend = errors.New("invalid state backend")

// Path uses the same resolved directory as the credentials store.
func Path() (string, error) { return internal.ConfigPath(FileName) }

var fields = []struct{ key, section, name string }{
	{"MODEL_PROVIDER", providerSection, "name"},
	{"HITL_ENABLED", "hitl", "enabled"},
	{"MOCK_AGENT_CALL", "agent", "mock_calls"},
	{"HITL_STATE_STORE", "state", "backend"},
	{"HITL_STATE_TTL", "state", "ttl"},
	{"REDIS_ADDR", "redis", "address"},
	{"REDIS_USERNAME", "redis", "username"},
	{"REDIS_KEY_PREFIX", "redis", "key_prefix"},
	{"REDIS_DB", "redis", "db"},
}

var credentialFields = []struct{ section, name, key string }{
	{providerSection, "api_key", "GEMINI_API_KEY"},
	{providerSection, "model", "GEMINI_MODEL"},
	{"redis", "password", "REDIS_PASSWORD"},
}

func credentialRecord(store *credentials.Store, section string) (map[string]string, error) {
	var record map[string]string
	_, err := store.Get(section, &record)
	if err != nil {
		return nil, fmt.Errorf("invalid %s credentials", section)
	}
	return record, nil
}

func document() (map[string]any, error) {
	doc := map[string]any{}
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return doc, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: invalid YAML", path)
	}
	if doc == nil {
		doc = map[string]any{}
	}
	return doc, nil
}

// Load reads YAML settings and secrets without consulting or modifying .env.
func Load() (map[string]string, error) {
	doc, err := document()
	if err != nil {
		return nil, err
	}
	values := map[string]string{}
	for _, field := range fields {
		section, _ := doc[field.section].(map[string]any)
		if value, ok := section[field.name]; ok {
			values[field.key] = fmt.Sprint(value)
		}
	}
	store, err := credentials.Open(credentials.DefaultPath)
	if err != nil {
		return nil, errors.New("cannot read credentials.json")
	}
	for _, field := range credentialFields {
		record, err := credentialRecord(store, field.section)
		if err != nil {
			return nil, err
		}
		if value, ok := record[field.name]; ok {
			values[field.key] = value
		}
	}
	return values, nil
}

// Runtime prefers saved settings and credentials, falling back to local .env
// when the model or API key has not been configured.
func Runtime() (map[string]string, error) {
	values, err := Load()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(values["GEMINI_MODEL"]) != "" && strings.TrimSpace(values["GEMINI_API_KEY"]) != "" {
		return values, nil
	}
	legacy, err := godotenv.Read(".env")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("read .env: invalid or unreadable legacy settings")
	}
	if legacy == nil {
		legacy = map[string]string{}
	}
	switch strings.ToLower(strings.TrimSpace(legacy["HITL_STATE_STORE"])) {
	case "", "inmemory", "redis":
	default:
		return nil, fmt.Errorf("HITL_STATE_STORE: %w", ErrInvalidBackend)
	}
	for key, value := range values {
		if (key == "GEMINI_MODEL" || key == "GEMINI_API_KEY") && strings.TrimSpace(value) == "" {
			continue
		}
		legacy[key] = value
	}
	return legacy, nil
}

// SaveSettings preserves unknown YAML fields. Secrets are saved separately
// through the existing atomic credentials store.
func SaveSettings(values map[string]string) error {
	path, err := Path()
	if err != nil {
		return err
	}
	doc, err := document()
	if err != nil {
		return err
	}
	if provider, ok := doc[providerSection].(map[string]any); ok {
		delete(provider, "model")
		delete(provider, "api_key")
	}
	for _, field := range fields {
		value, ok := values[field.key]
		if !ok {
			continue
		}
		section, _ := doc[field.section].(map[string]any)
		if section == nil {
			section = map[string]any{}
			doc[field.section] = section
		}
		var encoded any = value
		if field.key == "HITL_ENABLED" || field.key == "MOCK_AGENT_CALL" {
			encoded, err = strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("%s must be true or false", field.key)
			}
		}
		section[field.name] = encoded
	}
	if values["HITL_ENABLED"] == "true" {
		hitl := doc["hitl"].(map[string]any)
		if _, ok := hitl["require_approval"]; !ok {
			hitl["require_approval"] = []string{"gmail_send", "send_updates_to_manager"}
		}
	}
	data, err := yaml.Marshal(doc)
	if err != nil {
		return errors.New("cannot encode agent settings")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()); _ = f.Close() }()
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

// ApprovalTools lets installed agents use the policy in config.yml while
// retaining the existing checkout policy when no list has been configured.
func ApprovalTools() ([]string, bool, error) {
	doc, err := document()
	if err != nil {
		return nil, false, err
	}
	hitl, _ := doc["hitl"].(map[string]any)
	raw, found := hitl["require_approval"]
	if !found {
		return nil, false, nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, true, errors.New("hitl.require_approval must be a list of tool names")
	}
	tools := make([]string, 0, len(list))
	for _, item := range list {
		name, ok := item.(string)
		if !ok {
			return nil, true, errors.New("hitl.require_approval must contain tool names")
		}
		tools = append(tools, name)
	}
	return tools, true, nil
}

func SaveSecrets(values map[string]string) error {
	store, err := credentials.Open(credentials.DefaultPath)
	if err != nil {
		return errors.New("cannot read credentials.json")
	}
	for _, field := range credentialFields {
		value, ok := values[field.key]
		if !ok {
			continue
		}
		record, err := credentialRecord(store, field.section)
		if err != nil {
			return err
		}
		if record == nil {
			record = map[string]string{}
		}
		record[field.name] = value
		if err := store.Set(field.section, record); err != nil {
			return err
		}
	}
	return store.Save()
}
