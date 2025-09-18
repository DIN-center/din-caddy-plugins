package network

import (
	"context"
	"time"
)

// Retry performs an operation with exponential backoff up to maxAttempts times.
// It will retry on any error. Use RetryWithChecker for custom retry logic.
func Retry(ctx context.Context, maxAttempts int, operation func() error) error {
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Check context cancellation
		if ctx.Err() != nil {
			return ctx.Err()
		}

		err := operation()
		if err == nil {
			return nil // Success!
		}

		lastErr = err

		// Don't sleep after the last attempt
		if attempt < maxAttempts-1 {
			// Simple exponential backoff: 100ms, 200ms, 400ms, 800ms...
			delay := time.Duration(100<<attempt) * time.Millisecond
			if delay > 5*time.Second {
				delay = 5 * time.Second // Cap at 5 seconds
			}

			select {
			case <-time.After(delay):
				// Continue to next attempt
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return lastErr
}

// RetryWithChecker performs an operation with exponential backoff and custom retry logic.
// The isRetryable function determines whether an error should trigger a retry.
// If isRetryable returns false, the operation stops immediately with that error.
func RetryWithChecker(
	ctx context.Context,
	maxAttempts int,
	isRetryable func(error) bool,
	operation func() error,
) error {
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if we should retry
		if !isRetryable(err) {
			return err // Non-retryable error, stop immediately
		}

		// Same backoff logic
		if attempt < maxAttempts-1 {
			delay := time.Duration(100<<attempt) * time.Millisecond
			if delay > 5*time.Second {
				delay = 5 * time.Second
			}

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return lastErr
}