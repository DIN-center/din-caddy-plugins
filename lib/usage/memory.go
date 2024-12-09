package usage

import (
	"sync/atomic"
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