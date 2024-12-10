package usage

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"time"
)

func TestMemoryUsageTracker_Use(t *testing.T) {
	tracker := &memUsageTracker{
		Uses:      new(int64),
	}
	*tracker.Uses = 20
	// tracker.Error.Store(nil)

	// First batch decrement
	err := tracker.Use()
	assert.NoError(t, err)
	assert.Equal(t, int64(19), *tracker.Uses)

	// Exceed usage
	for i := 0; i < 19; i++ {
		assert.NoError(t, tracker.Use())
	}
	err = tracker.Use()
	assert.Error(t, err)
	assert.Equal(t, ErrRequestLimit, err)
}

func TestMemoryTrackerManager(t *testing.T) {
	tm := NewMemoryTrackerManager()
	exp := time.Now().Add(20 * time.Millisecond)
	key, _ := tm.Create(5, exp)
	tracker, ok := tm.Get(key)
	assert.True(t, ok)
	assert.NoError(t, tracker.Use()) // 4
	assert.NoError(t, tracker.Use()) // 3
	assert.NoError(t, tracker.Use()) // 2
	assert.NoError(t, tracker.Use()) // 1
	assert.NoError(t, tracker.Use()) // 0
	assert.Error(t, tracker.Use())   // -1
	time.Sleep(time.Until(exp))
	_, ok = tm.Get(key)
	assert.False(t, ok)
}