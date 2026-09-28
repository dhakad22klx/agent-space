// Package state stores agent snapshots behind a common interface. In-memory
// storage supports runs within one agent's lifetime; Redis supports persistent
// state shared across processes. Both stores encode snapshots as JSON.
package state

import (
	"context"
	"time"
)

// DefaultTTL is how long a state entry lives unless configured otherwise.
const DefaultTTL = 24 * time.Hour

// Store holds state snapshots and is safe for concurrent use. Values passed
// to Put and destinations passed to Get remain the caller's responsibility.
type Store interface {
	// Put saves value under key, replacing whatever was there.
	Put(ctx context.Context, key string, value any) error

	// Get decodes the entry for key into into. False means there was no entry.
	Get(ctx context.Context, key string, into any) (bool, error)

	// Delete removes the entry for key. Missing entries are not an error.
	Delete(ctx context.Context, key string) error

	// Close releases the resources the store holds.
	Close() error
}
