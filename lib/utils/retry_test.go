package utils

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestRetryableError implementation for testing
type testRetryableError struct {
	message    string
	retryable  bool
}

func (e *testRetryableError) Error() string {
	return e.message
}

func (e *testRetryableError) IsRetryable() bool {
	return e.retryable
}

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()
	
	if config.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries to be 3, got %d", config.MaxRetries)
	}
	if config.InitialDelay != 1*time.Second {
		t.Errorf("Expected InitialDelay to be 1s, got %v", config.InitialDelay)
	}
	if config.MaxDelay != 30*time.Second {
		t.Errorf("Expected MaxDelay to be 30s, got %v", config.MaxDelay)
	}
	if config.BackoffMultiplier != 2.0 {
		t.Errorf("Expected BackoffMultiplier to be 2.0, got %f", config.BackoffMultiplier)
	}
	if config.JitterFactor != 0.1 {
		t.Errorf("Expected JitterFactor to be 0.1, got %f", config.JitterFactor)
	}
}

func TestRetryWithBackoff_Success(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:        3,
		InitialDelay:      10 * time.Millisecond,
		MaxDelay:          100 * time.Millisecond,
		BackoffMultiplier: 2.0,
		JitterFactor:      0,
	}
	
	attempts := 0
	fn := func() error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary error")
		}
		return nil // Success on third attempt
	}
	
	err := RetryWithBackoff(context.Background(), config, fn)
	
	if err != nil {
		t.Errorf("Expected success after retries, got error: %v", err)
	}
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestRetryWithBackoff_MaxRetriesExceeded(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:        2,
		InitialDelay:      10 * time.Millisecond,
		MaxDelay:          100 * time.Millisecond,
		BackoffMultiplier: 2.0,
		JitterFactor:      0,
	}
	
	attempts := 0
	fn := func() error {
		attempts++
		return errors.New("persistent error")
	}
	
	err := RetryWithBackoff(context.Background(), config, fn)
	
	if err == nil {
		t.Errorf("Expected error after max retries, got nil")
	}
	// Initial attempt + 2 retries = 3 total attempts
	if attempts != 3 {
		t.Errorf("Expected 3 attempts (initial + 2 retries), got %d", attempts)
	}
}

func TestRetryWithBackoff_NonRetryableError(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:        3,
		InitialDelay:      10 * time.Millisecond,
		MaxDelay:          100 * time.Millisecond,
		BackoffMultiplier: 2.0,
		JitterFactor:      0,
	}
	
	attempts := 0
	fn := func() error {
		attempts++
		return &testRetryableError{
			message:   "non-retryable error",
			retryable: false,
		}
	}
	
	err := RetryWithBackoff(context.Background(), config, fn)
	
	if err == nil {
		t.Errorf("Expected non-retryable error, got nil")
	}
	if attempts != 1 {
		t.Errorf("Expected 1 attempt for non-retryable error, got %d", attempts)
	}
}

func TestRetryWithBackoff_ContextCancellation(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:        5,
		InitialDelay:      1 * time.Second,
		MaxDelay:          10 * time.Second,
		BackoffMultiplier: 2.0,
		JitterFactor:      0,
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0
	
	fn := func() error {
		attempts++
		if attempts == 2 {
			cancel() // Cancel context during retry
		}
		return errors.New("error")
	}
	
	err := RetryWithBackoff(ctx, config, fn)
	
	if err == nil {
		t.Errorf("Expected context cancellation error, got nil")
	}
	if attempts < 2 {
		t.Errorf("Expected at least 2 attempts before cancellation, got %d", attempts)
	}
}

func TestRetryWithBackoff_ImmediateSuccess(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:        3,
		InitialDelay:      10 * time.Millisecond,
		MaxDelay:          100 * time.Millisecond,
		BackoffMultiplier: 2.0,
		JitterFactor:      0,
	}
	
	attempts := 0
	fn := func() error {
		attempts++
		return nil // Immediate success
	}
	
	err := RetryWithBackoff(context.Background(), config, fn)
	
	if err != nil {
		t.Errorf("Expected immediate success, got error: %v", err)
	}
	if attempts != 1 {
		t.Errorf("Expected 1 attempt for immediate success, got %d", attempts)
	}
}

func TestRetryWithBackoffTyped(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:        2,
		InitialDelay:      10 * time.Millisecond,
		MaxDelay:          100 * time.Millisecond,
		BackoffMultiplier: 2.0,
		JitterFactor:      0,
	}
	
	attempts := 0
	fn := func() (string, error) {
		attempts++
		if attempts < 2 {
			return "", errors.New("temporary error")
		}
		return "success", nil
	}
	
	result, err := RetryWithBackoffTyped(context.Background(), config, fn)
	
	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}
	if result != "success" {
		t.Errorf("Expected result 'success', got '%s'", result)
	}
	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

func TestExponentialBackoff(t *testing.T) {
	tests := []struct {
		name         string
		attempt      int
		initialDelay time.Duration
		maxDelay     time.Duration
		multiplier   float64
		expected     time.Duration
	}{
		{
			name:         "First attempt",
			attempt:      0,
			initialDelay: 1 * time.Second,
			maxDelay:     30 * time.Second,
			multiplier:   2.0,
			expected:     1 * time.Second,
		},
		{
			name:         "Second attempt",
			attempt:      1,
			initialDelay: 1 * time.Second,
			maxDelay:     30 * time.Second,
			multiplier:   2.0,
			expected:     2 * time.Second,
		},
		{
			name:         "Third attempt",
			attempt:      2,
			initialDelay: 1 * time.Second,
			maxDelay:     30 * time.Second,
			multiplier:   2.0,
			expected:     4 * time.Second,
		},
		{
			name:         "Max delay cap",
			attempt:      10,
			initialDelay: 1 * time.Second,
			maxDelay:     10 * time.Second,
			multiplier:   2.0,
			expected:     10 * time.Second, // Should be capped at maxDelay
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExponentialBackoff(tt.attempt, tt.initialDelay, tt.maxDelay, tt.multiplier)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestAddJitter(t *testing.T) {
	delay := 1 * time.Second
	jitterFactor := 0.1
	
	// Test multiple times to ensure jitter is within expected range
	for i := 0; i < 100; i++ {
		jittered := addJitter(delay, jitterFactor)
		
		// Jitter should be within +/- 10% of original delay
		minExpected := time.Duration(float64(delay) * 0.9)
		maxExpected := time.Duration(float64(delay) * 1.1)
		
		if jittered < minExpected || jittered > maxExpected {
			t.Errorf("Jittered delay %v is outside expected range [%v, %v]", 
				jittered, minExpected, maxExpected)
		}
	}
	
	// Test with zero jitter factor
	noJitter := addJitter(delay, 0)
	if noJitter != delay {
		t.Errorf("Expected no jitter with factor 0, got %v instead of %v", noJitter, delay)
	}
}

func TestRetryWithBackoff_NilConfig(t *testing.T) {
	attempts := 0
	fn := func() error {
		attempts++
		if attempts < 2 {
			return errors.New("temporary error")
		}
		return nil
	}
	
	// Pass nil config to test default config usage
	err := RetryWithBackoff(context.Background(), nil, fn)
	
	if err != nil {
		t.Errorf("Expected success with default config, got error: %v", err)
	}
	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}