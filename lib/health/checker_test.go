package health

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestChecker_StartStop(t *testing.T) {
	var count int32
	c := NewChecker(CheckerConfig{
		Interval: 10 * time.Millisecond,
	}, func(ctx context.Context) {
		atomic.AddInt32(&count, 1)
	})

	c.Start()
	time.Sleep(55 * time.Millisecond)
	c.Stop()

	got := atomic.LoadInt32(&count)
	if got < 2 {
		t.Errorf("expected at least 2 ticks, got %d", got)
	}
}

func TestChecker_RunImmediately(t *testing.T) {
	var count int32
	c := NewChecker(CheckerConfig{
		Interval:       1 * time.Hour, // won't tick in test
		RunImmediately: true,
	}, func(ctx context.Context) {
		atomic.AddInt32(&count, 1)
	})

	c.Start()
	// The immediate check runs synchronously before Start returns
	c.Stop()

	got := atomic.LoadInt32(&count)
	if got != 1 {
		t.Errorf("expected exactly 1 immediate check, got %d", got)
	}
}

func TestChecker_PreventOverlap(t *testing.T) {
	var concurrent int32
	var maxConcurrent int32

	c := NewChecker(CheckerConfig{
		Interval:       10 * time.Millisecond,
		PreventOverlap: true,
	}, func(ctx context.Context) {
		cur := atomic.AddInt32(&concurrent, 1)
		for {
			old := atomic.LoadInt32(&maxConcurrent)
			if cur <= old || atomic.CompareAndSwapInt32(&maxConcurrent, old, cur) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond) // Longer than interval
		atomic.AddInt32(&concurrent, -1)
	})

	c.Start()
	time.Sleep(100 * time.Millisecond)
	c.Stop()

	got := atomic.LoadInt32(&maxConcurrent)
	if got != 1 {
		t.Errorf("expected max concurrency of 1 with overlap prevention, got %d", got)
	}
}

func TestChecker_ContextCancellation(t *testing.T) {
	var cancelledCtx context.Context
	done := make(chan struct{})

	c := NewChecker(CheckerConfig{
		Interval: 10 * time.Millisecond,
	}, func(ctx context.Context) {
		cancelledCtx = ctx
		select {
		case <-done:
		default:
			close(done)
		}
	})

	c.Start()
	<-done // Wait for at least one check
	c.Stop()

	// Give the goroutine time to process quit and cancel context
	time.Sleep(20 * time.Millisecond)

	if cancelledCtx.Err() != context.Canceled {
		t.Errorf("expected context to be cancelled after Stop, got %v", cancelledCtx.Err())
	}
}

func TestChecker_StopIdempotent(t *testing.T) {
	c := NewChecker(CheckerConfig{
		Interval: 10 * time.Millisecond,
	}, func(ctx context.Context) {})

	c.Start()

	// Multiple stops should not panic
	c.Stop()
	c.Stop()
	c.Stop()
}

func TestChecker_QuitChannel(t *testing.T) {
	c := NewChecker(CheckerConfig{
		Interval: 10 * time.Millisecond,
	}, func(ctx context.Context) {})

	c.Start()

	select {
	case <-c.Quit():
		t.Error("quit channel should not be closed yet")
	default:
	}

	c.Stop()

	select {
	case <-c.Quit():
		// expected
	case <-time.After(100 * time.Millisecond):
		t.Error("quit channel should be closed after Stop")
	}
}
