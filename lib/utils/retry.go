package utils

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// RetryConfig holds configuration for retry logic
type RetryConfig struct {
	MaxRetries       int           // Maximum number of retry attempts (0 = no retries)
	InitialDelay     time.Duration // Initial delay between retries
	MaxDelay         time.Duration // Maximum delay between retries
	BackoffMultiplier float64       // Multiplier for exponential backoff
	JitterFactor     float64       // Jitter factor (0.0 to 1.0) to prevent thundering herd
}

// DefaultRetryConfig returns a sensible default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:       3,
		InitialDelay:     1 * time.Second,
		MaxDelay:         30 * time.Second,
		BackoffMultiplier: 2.0,
		JitterFactor:     0.1, // 10% jitter
	}
}

// RetryableFunc is a function that can be retried
type RetryableFunc func() error

// RetryableError interface for errors that can indicate if they're retryable
type RetryableError interface {
	error
	IsRetryable() bool
}

// RetryWithBackoff executes a function with exponential backoff retry logic
func RetryWithBackoff(ctx context.Context, config *RetryConfig, fn RetryableFunc) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastErr error
	delay := config.InitialDelay

	// Try once without counting as a retry
	if err := fn(); err == nil {
		return nil
	} else {
		lastErr = err
		// Check if error is explicitly non-retryable
		if retryableErr, ok := err.(RetryableError); ok && !retryableErr.IsRetryable() {
			return err
		}
	}

	// Perform retries
	for attempt := 0; attempt < config.MaxRetries; attempt++ {
		// Calculate delay with jitter
		jitteredDelay := addJitter(delay, config.JitterFactor)
		
		// Wait with context cancellation support
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		case <-time.After(jitteredDelay):
			// Continue with retry
		}

		// Try the function again
		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
			// Check if error is explicitly non-retryable
			if retryableErr, ok := err.(RetryableError); ok && !retryableErr.IsRetryable() {
				return err
			}
		}

		// Calculate next delay with exponential backoff
		delay = time.Duration(float64(delay) * config.BackoffMultiplier)
		if delay > config.MaxDelay {
			delay = config.MaxDelay
		}
	}

	return fmt.Errorf("operation failed after %d retries: %w", config.MaxRetries, lastErr)
}

// RetryWithBackoffTyped is a generic version that returns typed results
func RetryWithBackoffTyped[T any](ctx context.Context, config *RetryConfig, fn func() (T, error)) (T, error) {
	var result T
	err := RetryWithBackoff(ctx, config, func() error {
		var err error
		result, err = fn()
		return err
	})
	return result, err
}

// addJitter adds random jitter to a delay to prevent thundering herd
func addJitter(delay time.Duration, jitterFactor float64) time.Duration {
	if jitterFactor <= 0 {
		return delay
	}
	
	// Calculate jitter range
	jitterRange := float64(delay) * jitterFactor
	// Add random jitter (both positive and negative)
	jitter := (rand.Float64() - 0.5) * 2 * jitterRange
	
	// Apply jitter and ensure result is positive
	newDelay := float64(delay) + jitter
	if newDelay < 0 {
		newDelay = 0
	}
	
	return time.Duration(newDelay)
}

// ExponentialBackoff calculates the delay for a given retry attempt
func ExponentialBackoff(attempt int, initialDelay time.Duration, maxDelay time.Duration, multiplier float64) time.Duration {
	if attempt <= 0 {
		return initialDelay
	}
	
	delay := float64(initialDelay) * math.Pow(multiplier, float64(attempt))
	if delay > float64(maxDelay) {
		return maxDelay
	}
	
	return time.Duration(delay)
}