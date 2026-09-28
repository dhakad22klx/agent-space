package state

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// InMemoryStore stores independent JSON snapshots within the current process.
// Entries are protected by mu; callers never receive a reference to stored data.
type InMemoryStore struct {
	mu          sync.Mutex
	entries     map[string]memoryEntry
	ttl         time.Duration
	now         func() time.Time
	nextCleanup time.Time
	closed      bool
}

type memoryEntry struct {
	data      []byte
	expiresAt time.Time
}

var _ Store = (*InMemoryStore)(nil)

// NewInMemoryStore creates an isolated store. A zero ttl disables expiry.
// Expired entries are removed on reads and periodically on writes, so no
// background goroutine is needed. Close discards all entries.
func NewInMemoryStore(ttl time.Duration) *InMemoryStore {
	return &InMemoryStore{
		entries: make(map[string]memoryEntry),
		ttl:     ttl,
		now:     time.Now,
	}
}

// TTL is how long a freshly written entry lives, zero meaning no expiry.
func (s *InMemoryStore) TTL() time.Duration { return s.ttl }

// Put encodes value as JSON, replacing the entry and restarting its expiry.
func (s *InMemoryStore) Put(ctx context.Context, key string, value any) error {
	if strings.TrimSpace(key) == "" {
		return errors.New("cannot save state under an empty key")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cannot encode the state for %s: %w", key, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("state store is closed")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	now := s.now()
	entry := memoryEntry{data: encoded}
	if s.ttl > 0 {
		entry.expiresAt = now.Add(s.ttl)
		if !now.Before(s.nextCleanup) {
			for key, entry := range s.entries {
				if !now.Before(entry.expiresAt) {
					delete(s.entries, key)
				}
			}
			s.nextCleanup = now.Add(min(s.ttl, time.Minute))
		}
	}
	s.entries[key] = entry
	return nil
}

// Get decodes a snapshot. Missing and expired entries return false, nil.
func (s *InMemoryStore) Get(ctx context.Context, key string, into any) (bool, error) {
	if strings.TrimSpace(key) == "" {
		return false, errors.New("cannot read state under an empty key")
	}
	entry, found, err := s.read(ctx, key)
	if err != nil || !found {
		return false, err
	}
	// A custom UnmarshalJSON method receives these bytes, so keep the stored
	// snapshot private even if that decoder modifies its input.
	if err := json.Unmarshal(bytes.Clone(entry.data), into); err != nil {
		return false, fmt.Errorf("cannot decode the state for %s: %w", key, err)
	}
	return true, nil
}

// read releases the lock before Get invokes a caller's JSON decoder.
func (s *InMemoryStore) read(ctx context.Context, key string) (memoryEntry, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return memoryEntry{}, false, err
	}
	if s.closed {
		return memoryEntry{}, false, errors.New("state store is closed")
	}
	entry, found := s.entries[key]
	if found && !entry.expiresAt.IsZero() && !s.now().Before(entry.expiresAt) {
		delete(s.entries, key)
		return memoryEntry{}, false, nil
	}
	return entry, found, nil
}

// Delete removes key. Deleting a missing entry succeeds.
func (s *InMemoryStore) Delete(ctx context.Context, key string) error {
	if strings.TrimSpace(key) == "" {
		return errors.New("cannot delete state under an empty key")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.closed {
		return errors.New("state store is closed")
	}
	delete(s.entries, key)
	return nil
}

// Close releases all entries and prevents further operations. It is idempotent.
func (s *InMemoryStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.entries = nil
	return nil
}
