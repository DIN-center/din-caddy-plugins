package sessiontracker

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/philippgille/gokv"
	"github.com/philippgille/gokv/syncmap"
)

// Session represents a collection's session with an expiration and quantity allocation.
type Session struct {
	ID         string    `json:"id"`
	Collection string    `json:"collection"`
	Quantity   *big.Int  `json:"quantity"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// LockManager abstracts distributed locking.
type LockManager interface {
	Acquire(ctx context.Context, key string) (Lock, error)
}

// Lock is a handle to a distributed lock.
type Lock interface {
	Release() error
}

// InMemoryLockManager is a simple process-local lock manager for dev/testing.
type InMemoryLockManager struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

// InMemoryLock implements the Lock interface.
type InMemoryLock struct {
	mu  *sync.Mutex
	key string
}

// NewInMemoryLockManager creates a new in-memory lock manager.
func NewInMemoryLockManager() *InMemoryLockManager {
	return &InMemoryLockManager{locks: make(map[string]*sync.Mutex)}
}

// Acquire returns a lock for a given key.
func (m *InMemoryLockManager) Acquire(ctx context.Context, key string) (Lock, error) {
	m.mu.Lock()
	if m.locks == nil {
		m.locks = make(map[string]*sync.Mutex)
	}
	lock, ok := m.locks[key]
	if !ok {
		lock = &sync.Mutex{}
		m.locks[key] = lock
	}
	m.mu.Unlock()

	lock.Lock()
	return &InMemoryLock{mu: lock, key: key}, nil
}

// Release unlocks the in-memory lock.
func (l *InMemoryLock) Release() error {
	l.mu.Unlock()
	return nil
}

// Tracker manages sessions for collections using a gokv.Store.
type Tracker struct {
	store       gokv.Store
	lockManager LockManager
}

// NewTracker creates a new session tracker with the given gokv store and lock manager.
func NewTracker(store gokv.Store, lm LockManager) *Tracker {
	return &Tracker{store: store, lockManager: lm}
}

func NewMemoryTracker() *Tracker {
	return NewTracker(syncmap.NewStore(syncmap.DefaultOptions), NewInMemoryLockManager())
}

// withLock executes a function while holding a distributed lock on the given key.
func (t *Tracker) withLock(ctx context.Context, key string, fn func() error) error {
	if t.lockManager == nil {
		// No distributed lock configured, just run the function.
		return fn()
	}
	lock, err := t.lockManager.Acquire(ctx, key)
	if err != nil {
		return err
	}
	defer lock.Release()
	return fn()
}

// AddSession appends a new session to a collection's indexed list.
func (t *Tracker) AddSession(ctx context.Context, id, collection string, quantity *big.Int, exp time.Time) error {
	s := Session{
		ID:         id,
		Collection: collection,
		Quantity:   new(big.Int).Set(quantity),
		ExpiresAt:  exp,
	}
	return t.withLock(ctx, s.Collection, func() error {
		countKey := t.collectionCountKey(s.Collection)
		var count int
		if found, err := t.store.Get(countKey, &count); err != nil {
			return fmt.Errorf("failed to read session count: %w", err)
		} else if !found {
			count = 0
		}
		count++
		if err := t.store.Set(countKey, count); err != nil {
			return err
		}

		key := t.sessionKey(s.Collection, count)
		return t.store.Set(key, s)
	})
}

// revokeSession removes a specific session and compacts the index by moving the last session into the gap.
// The caller should hold the lock for this collection before invoking this method.
func (t *Tracker) revokeSession(ctx context.Context, collectionID, sessionID string) error {
	countKey := t.collectionCountKey(collectionID)
	var count int
	if found, err := t.store.Get(countKey, &count); err != nil {
		return err
	} else if !found {
		return fmt.Errorf("no sessions found for collection %s", collectionID)
	}

	for i := 1; i <= count; i++ {
		key := t.sessionKey(collectionID, i)
		var s Session
		if found, err := t.store.Get(key, &s); found && s.ID == sessionID {
			// Delete this key
			_ = t.store.Delete(key)

			// If this wasn't the last element, move the last into this slot
			if i != count {
				lastKey := t.sessionKey(collectionID, count)
				var last Session
				if found, err := t.store.Get(lastKey, &last); err == nil && found {
					_ = t.store.Set(key, last)
					_ = t.store.Delete(lastKey)
				}
			}

			// Decrement count
			count--
			return t.store.Set(countKey, count)
		} else if err != nil {
			return err
		}
	}
	return fmt.Errorf("session %s not found for collection %s", sessionID, collectionID)
}

// RevokeSession removes a specific session and compacts the index by moving the last session into the gap.
func (t *Tracker) RevokeSession(ctx context.Context, collectionID, sessionID string) error {
	return t.withLock(ctx, collectionID, func() error {
		return t.revokeSession(ctx, collectionID, sessionID)
	})
}

// GetCollectionQuantity iterates through all sessions of a collection and sums valid Quantity, compacting expired ones.
func (t *Tracker) GetCollectionQuantity(ctx context.Context, collectionID string) (*big.Int, error) {
	result := big.NewInt(0)
	err := t.withLock(ctx, collectionID, func() error {
		countKey := t.collectionCountKey(collectionID)
		var count int
		if found, err := t.store.Get(countKey, &count); err != nil {
			return err
		} else if !found {
			return nil // no sessions
		}

		total := big.NewInt(0)
		now := time.Now()
		i := 1
		for i <= count {
			key := t.sessionKey(collectionID, i)
			var s Session
			if found, err := t.store.Get(key, &s); err == nil && found {
				if s.ExpiresAt.After(now) {
					total.Add(total, s.Quantity)
					i++
					continue
				}
				// Expired session: revoke it, which will move last session into this slot
				_ = t.revokeSession(ctx, collectionID, s.ID)
				count--
				continue // stay on same index since it's replaced
			}
			i++
		}
		result = total
		return nil
	})
	return result, err
}

// --- internal helpers ---

func (t *Tracker) sessionKey(collectionID string, index int) string {
	return fmt.Sprintf("collection:%s:session:%d", collectionID, index)
}

func (t *Tracker) collectionCountKey(collectionID string) string {
	return fmt.Sprintf("collection:%s:count", collectionID)
}
