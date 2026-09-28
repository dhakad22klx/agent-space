package state

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"justsay-harness/providers"
)

// Both backends must preserve the same snapshot and missing-entry semantics.
func TestStoreSnapshots(t *testing.T) {
	for _, backend := range []string{"inmemory", "redis"} {
		t.Run(backend, func(t *testing.T) {
			var store Store = NewInMemoryStore(0)
			if backend == "redis" {
				server := miniredis.RunT(t)
				store = NewRedisStore(redis.NewClient(&redis.Options{Addr: server.Addr()}), "", 0)
			}
			t.Cleanup(func() { _ = store.Close() })
			ctx := context.Background()
			var got AgentState
			if found, err := store.Get(ctx, "session", &got); found || err != nil {
				t.Fatalf("missing Get = %v, %v", found, err)
			}
			original := AgentState{
				SessionID: "session", Status: StatusWaitingApproval, Step: 2,
				History: []providers.Message{{Role: providers.RoleUser, Text: "hello"}},
				PendingApproval: &PendingApproval{ID: "approval", ToolCall: providers.ToolCall{
					Name: "send", Args: map[string]any{"to": "person"},
				}},
			}
			if err := store.Put(ctx, "session", original); err != nil {
				t.Fatal(err)
			}
			if found, err := store.Get(ctx, "session", &got); !found || err != nil {
				t.Fatalf("Get = %v, %v", found, err)
			}
			if !reflect.DeepEqual(got, original) {
				t.Fatalf("snapshot = %#v, want %#v", got, original)
			}
			// Neither a mutation after Put nor a mutation after Get changes storage.
			original.History[0].Text = "changed input"
			original.PendingApproval.ToolCall.Args["to"] = "changed input"
			got.History[0].Text = "changed output"
			got.PendingApproval.ToolCall.Args["to"] = "changed output"
			var reread AgentState
			if found, err := store.Get(ctx, "session", &reread); !found || err != nil {
				t.Fatalf("Get = %v, %v", found, err)
			}
			if reread.History[0].Text != "hello" || reread.PendingApproval.ToolCall.Args["to"] != "person" {
				t.Fatalf("snapshot was mutated: %+v", reread)
			}
			reread.Status = StatusCompleted
			if err := store.Put(ctx, "session", reread); err != nil {
				t.Fatal(err)
			}
			if found, err := store.Get(ctx, "session", &got); !found || err != nil || got.Status != StatusCompleted {
				t.Fatalf("updated Get = %v, %v, status %s", found, err, got.Status)
			}
			for range 2 {
				if err := store.Delete(ctx, "session"); err != nil {
					t.Fatal(err)
				}
			}
			if found, err := store.Get(ctx, "session", &got); found || err != nil {
				t.Fatalf("deleted Get = %v, %v", found, err)
			}
		})
	}
}

func TestInMemoryErrors(t *testing.T) {
	s := NewInMemoryStore(0)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	var got string
	for _, key := range []string{"", " \t"} {
		if err := s.Put(ctx, key, "value"); err == nil {
			t.Error("Put accepted empty key")
		}
		if _, err := s.Get(ctx, key, &got); err == nil {
			t.Error("Get accepted empty key")
		}
		if err := s.Delete(ctx, key); err == nil {
			t.Error("Delete accepted empty key")
		}
	}
	if err := s.Put(ctx, "key", "saved"); err != nil {
		t.Fatal(err)
	}
	if err := s.Put(ctx, "key", make(chan int)); err == nil {
		t.Fatal("Put accepted an unencodable value")
	}
	if found, err := s.Get(ctx, "key", &got); !found || err != nil || got != "saved" {
		t.Fatalf("failed Put changed entry: %v, %v, %q", found, err, got)
	}
	if _, err := s.Get(ctx, "key", nil); err == nil {
		t.Error("Get accepted an invalid destination")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := s.Put(cancelled, "key", "changed"); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled Put = %v", err)
	}
	if _, err := s.Get(cancelled, "key", &got); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled Get = %v", err)
	}
	if err := s.Delete(cancelled, "key"); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled Delete = %v", err)
	}
	if found, err := s.Get(ctx, "key", &got); !found || err != nil || got != "saved" {
		t.Fatalf("cancelled operation changed entry: %v, %v, %q", found, err, got)
	}
	for range 2 {
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Put(ctx, "key", "value"); err == nil {
		t.Error("Put succeeded after Close")
	}
	if _, err := s.Get(ctx, "key", &got); err == nil {
		t.Error("Get succeeded after Close")
	}
	if err := s.Delete(ctx, "key"); err == nil {
		t.Error("Delete succeeded after Close")
	}
}

func TestInMemoryExpiry(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	s := NewInMemoryStore(time.Minute)
	s.now = func() time.Time { return now }
	t.Cleanup(func() { _ = s.Close() })
	put := func(key string) {
		t.Helper()
		if err := s.Put(ctx, key, key); err != nil {
			t.Fatal(err)
		}
	}
	check := func(key string, want bool) {
		t.Helper()
		var got string
		if found, err := s.Get(ctx, key, &got); found != want || err != nil {
			t.Fatalf("Get(%q) = %v, %v; want found %v", key, found, err, want)
		}
	}
	put("expires")
	put("renewed")
	put("unread")
	now = now.Add(30 * time.Second)
	put("renewed")
	now = now.Add(30 * time.Second)
	check("expires", false)
	check("renewed", true)
	put("new") // A write collects entries that expired without being read.
	if _, exists := s.entries["unread"]; exists {
		t.Error("expired unread entry was retained")
	}
	now = now.Add(30 * time.Second)
	check("renewed", false)

	permanent := NewInMemoryStore(0)
	permanent.now = func() time.Time { return now }
	t.Cleanup(func() { _ = permanent.Close() })
	if err := permanent.Put(ctx, "key", "value"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(365 * 24 * time.Hour)
	var got string
	if found, err := permanent.Get(ctx, "key", &got); !found || err != nil {
		t.Fatalf("zero TTL expired: %v, %v", found, err)
	}
}

func TestInMemoryConcurrentAccess(t *testing.T) {
	s := NewInMemoryStore(time.Minute)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	var wg sync.WaitGroup
	for worker := range 16 {
		wg.Go(func() {
			key := fmt.Sprintf("worker-%d", worker)
			for i := range 100 {
				if err := s.Put(ctx, key, i); err != nil {
					t.Error(err)
					return
				}
				var got int
				if found, err := s.Get(ctx, key, &got); !found || err != nil || got != i {
					t.Errorf("Get = %v, %v, %d; want %d", found, err, got, i)
					return
				}
				if err := s.Delete(ctx, key); err != nil {
					t.Error(err)
				}
				// Readers must always see a complete snapshot despite competing writes.
				if err := s.Put(ctx, "shared", []int{i, i}); err != nil {
					t.Error(err)
				}
				var pair []int
				if found, err := s.Get(ctx, "shared", &pair); !found || err != nil || len(pair) != 2 || pair[0] != pair[1] {
					t.Errorf("inconsistent shared snapshot: %v, %v, %v", found, err, pair)
				}
			}
		})
	}
	wg.Wait()
}
