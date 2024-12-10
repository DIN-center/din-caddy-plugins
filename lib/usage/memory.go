package usage

import (
	"sync/atomic"
	"time"
	"sync"
	"github.com/google/uuid"
)

type memUsageTracker struct {
	Uses *int64
}

// Use checks whether an auth token is available for use, decrementing counters if appropriate, and
// returning errors if the token is no longer available.
func (ut *memUsageTracker) Use() error {
	if ut.Uses != nil && atomic.AddInt64(ut.Uses, -1) < 0 {
		return ErrRequestLimit
	}
	return nil
}

func NewMemoryTrackerManager() TrackerManager {
	return &memUsageTrackerManager{
		m: make(map[string]UsageTracker),
	}
}

type memUsageTrackerManager struct {
	m    map[string]UsageTracker
	lock sync.RWMutex
}

func (m *memUsageTrackerManager) Create(uses int64, exp time.Time) (string, error) {
	id := uuid.New().String()
	m.lock.Lock()
	tracker := &memUsageTracker{
		Uses: new(int64),
	}
	*tracker.Uses = uses
	m.m[id] = tracker
	m.lock.Unlock()
	time.AfterFunc(time.Until(exp), func() {
		m.lock.Lock()
		delete(m.m, id)
		m.lock.Unlock()
	})
	return id, nil
}

func (m *memUsageTrackerManager) Get(key string) (UsageTracker, bool) {
	m.lock.RLock()
	t, ok := m.m[key]
	m.lock.RUnlock()
	return t, ok
}