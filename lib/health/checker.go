package health

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// CheckFunc is the function called on each health check tick.
// The context is cancelled when the checker is stopped.
type CheckFunc func(ctx context.Context)

// CheckerConfig configures the health check loop.
type CheckerConfig struct {
	// Interval between health check ticks.
	Interval time.Duration

	// PreventOverlap skips a tick if the previous check is still running.
	// Used by the AI proxy to avoid piling up slow provider checks.
	PreventOverlap bool

	// RunImmediately executes one check before waiting for the first tick.
	// Used by the network proxy for immediate provider status on startup.
	RunImmediately bool
}

// Checker runs a periodic health check loop with graceful shutdown.
type Checker struct {
	config  CheckerConfig
	checkFn CheckFunc
	quit    chan struct{}
	ctx     context.Context
	cancel  context.CancelFunc
	running int32 // atomic flag for overlap prevention
	once    sync.Once
}

// NewChecker creates a new health checker. Call Start() to begin the loop.
func NewChecker(config CheckerConfig, checkFn CheckFunc) *Checker {
	ctx, cancel := context.WithCancel(context.Background())
	return &Checker{
		config:  config,
		checkFn: checkFn,
		quit:    make(chan struct{}),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start begins the periodic health check loop in a new goroutine.
func (c *Checker) Start() {
	if c.config.RunImmediately {
		c.checkFn(c.ctx)
	}

	ticker := time.NewTicker(c.config.Interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-c.quit:
				c.cancel()
				return
			case <-ticker.C:
				if c.config.PreventOverlap {
					if !atomic.CompareAndSwapInt32(&c.running, 0, 1) {
						continue
					}
					c.checkFn(c.ctx)
					atomic.StoreInt32(&c.running, 0)
				} else {
					c.checkFn(c.ctx)
				}
			}
		}
	}()
}

// Stop signals the health check loop to stop. Safe to call multiple times.
func (c *Checker) Stop() {
	c.once.Do(func() {
		close(c.quit)
	})
}

// Quit returns the quit channel for backward compatibility.
func (c *Checker) Quit() <-chan struct{} {
	return c.quit
}
