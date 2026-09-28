package state

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func TestOpenBackend(t *testing.T) {
	for _, tc := range []struct {
		name    string
		env     string
		yaml    string
		backend string
		err     string
	}{
		{name: "memory without Redis", env: "HITL_STATE_STORE=inmemory", backend: "inmemory"},
		{name: "memory ignores Redis options", env: "HITL_STATE_STORE=inmemory\nREDIS_ADDR=invalid\nREDIS_DB=invalid", backend: "inmemory"},
		{name: "YAML without env", yaml: "state:\n  backend: inmemory\n", backend: "inmemory"},
		{name: "YAML overrides env", env: "HITL_STATE_STORE=redis", yaml: "state:\n  backend: inmemory\n", backend: "inmemory"},
		{name: "empty YAML uses env", env: "HITL_STATE_STORE=inmemory", yaml: "state: {}", backend: "inmemory"},
		{name: "redis env", env: "HITL_STATE_STORE=redis", backend: "redis"},
		{name: "redis YAML", env: "HITL_STATE_STORE=inmemory", yaml: "state:\n  backend: redis\n", backend: "redis"},
		{name: "default needs no configuration", backend: "inmemory"},
		{name: "blank settings use default", env: "HITL_STATE_STORE=", yaml: "state:\n  backend: ''\n", backend: "inmemory"},
		{name: "Redis requires address", env: "HITL_STATE_STORE=redis", err: "REDIS_ADDR"},
		{name: "unknown backend", env: "HITL_STATE_STORE=unknown", err: "unsupported state backend"},
		{name: "invalid env backend with memory override", env: "HITL_STATE_STORE=unknown", yaml: "state:\n  backend: inmemory\n", err: "HITL_STATE_STORE in .env: unsupported state backend"},
		{name: "invalid env backend with redis override", env: "HITL_STATE_STORE=unknown", yaml: "state:\n  backend: redis\n", err: "HITL_STATE_STORE in .env: unsupported state backend"},
		{name: "invalid YAML", yaml: "state: [", err: "parse " + ConfigFile},
		{name: "invalid env", env: "HITL_STATE_STORE=\"unterminated", err: "read .env"},
		{name: "invalid TTL", env: "HITL_STATE_STORE=inmemory\nHITL_STATE_TTL=bad", err: "not a duration"},
		{name: "negative TTL", env: "HITL_STATE_STORE=inmemory\nHITL_STATE_TTL=-1s", err: "negative"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			env := tc.env
			if tc.backend == "redis" {
				server := miniredis.RunT(t)
				env += "\nREDIS_ADDR=" + server.Addr()
			}
			if env != "" {
				if err := os.WriteFile(".env", []byte(env), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if tc.yaml != "" {
				if err := os.WriteFile(ConfigFile, []byte(tc.yaml), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			store, err := Open(context.Background())
			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("Open error = %v, want %q", err, tc.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = store.Close() })
			switch store.(type) {
			case *InMemoryStore:
				if tc.backend != "inmemory" {
					t.Fatalf("got in-memory, want %s", tc.backend)
				}
			case *RedisStore:
				if tc.backend != "redis" {
					t.Fatalf("got redis, want %s", tc.backend)
				}
			default:
				t.Fatalf("unexpected store %T", store)
			}
			if err := store.Put(context.Background(), "key", "value"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestOpenInMemoryTTL(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want time.Duration
	}{{"", DefaultTTL}, {"0", 0}, {"5m", 5 * time.Minute}} {
		t.Run(tc.raw, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := os.WriteFile(".env", []byte("HITL_STATE_STORE=inmemory\nHITL_STATE_TTL="+tc.raw), 0o600); err != nil {
				t.Fatal(err)
			}
			store, err := Open(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = store.Close() })
			if got := store.(*InMemoryStore).TTL(); got != tc.want {
				t.Fatalf("TTL = %v, want %v", got, tc.want)
			}
		})
	}
}
