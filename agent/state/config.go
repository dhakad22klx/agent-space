package state

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// ConfigFile optionally selects the state backend from the working directory.
const ConfigFile = "justsay-config.yml"

// ErrInvalidBackend identifies an unsupported state backend in configuration.
var ErrInvalidBackend = errors.New("invalid state backend")

// Open creates a store using state.backend in justsay-config.yml, then
// HITL_STATE_STORE in .env, falling back to inmemory when neither is set.
// A non-empty HITL_STATE_STORE must be valid even when YAML overrides it.
// In-memory storage does not read Redis options or open a Redis connection.
// The caller must retain the returned store for as long as it needs its state.
func Open(ctx context.Context) (Store, error) {
	env, err := godotenv.Read(".env")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read .env: %w", err)
	}

	backend := strings.TrimSpace(env["HITL_STATE_STORE"])
	switch strings.ToLower(backend) {
	case "", "inmemory", "redis":
	default:
		return nil, fmt.Errorf("HITL_STATE_STORE in .env: %w %q: use inmemory or redis", ErrInvalidBackend, backend)
	}

	data, err := os.ReadFile(ConfigFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read %s: %w", ConfigFile, err)
	}
	if err == nil {
		var cfg struct {
			State struct {
				Backend string `yaml:"backend"`
			} `yaml:"state"`
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parse %s: %w", ConfigFile, err)
		}
		if configured := strings.TrimSpace(cfg.State.Backend); configured != "" {
			backend = configured
		}
	}
	if backend == "" {
		backend = "inmemory"
	}

	switch strings.ToLower(backend) {
	case "inmemory":
		ttl, err := parseTTL(env["HITL_STATE_TTL"])
		if err != nil {
			return nil, err
		}
		return NewInMemoryStore(ttl), nil
	case "redis":
		store, err := openRedis(ctx, env)
		if err != nil {
			return nil, err
		}
		return store, nil
	default:
		return nil, fmt.Errorf("state.backend in %s: %w %q: use inmemory or redis", ConfigFile, ErrInvalidBackend, backend)
	}
}

// parseTTL reads HITL_STATE_TTL as a Go duration. Blank means DefaultTTL and
// zero disables expiry for either backend.
func parseTTL(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DefaultTTL, nil
	}

	ttl, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("HITL_STATE_TTL in .env is not a duration like \"24h\": %w", err)
	}
	if ttl < 0 {
		return 0, fmt.Errorf("HITL_STATE_TTL in .env is negative: %s", raw)
	}

	return ttl, nil
}
