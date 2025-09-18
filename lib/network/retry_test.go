package network

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRetry(t *testing.T) {
	tests := []struct {
		name            string
		maxAttempts     int
		operation       func(attempts *int) func() error
		expectedAttempts int
		expectError     bool
	}{
		{
			name:        "successful on first attempt",
			maxAttempts: 3,
			operation: func(attempts *int) func() error {
				return func() error {
					*attempts++
					return nil
				}
			},
			expectedAttempts: 1,
			expectError:     false,
		},
		{
			name:        "successful on third attempt",
			maxAttempts: 5,
			operation: func(attempts *int) func() error {
				return func() error {
					*attempts++
					if *attempts < 3 {
						return errors.New("temporary error")
					}
					return nil
				}
			},
			expectedAttempts: 3,
			expectError:     false,
		},
		{
			name:        "all attempts fail",
			maxAttempts: 3,
			operation: func(attempts *int) func() error {
				return func() error {
					*attempts++
					return errors.New("persistent error")
				}
			},
			expectedAttempts: 3,
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attempts := 0
			err := Retry(context.Background(), tt.maxAttempts, tt.operation(&attempts))

			assert.Equal(t, tt.expectedAttempts, attempts)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRetryWithChecker(t *testing.T) {
	tests := []struct {
		name            string
		maxAttempts     int
		operation       func(attempts *int) func() error
		isRetryable     func(error) bool
		expectedAttempts int
		expectError     bool
	}{
		{
			name:        "non-retryable error stops immediately",
			maxAttempts: 5,
			operation: func(attempts *int) func() error {
				return func() error {
					*attempts++
					return errors.New("non-retryable")
				}
			},
			isRetryable: func(err error) bool {
				return err.Error() != "non-retryable"
			},
			expectedAttempts: 1,
			expectError:     true,
		},
		{
			name:        "retryable errors continue until success",
			maxAttempts: 5,
			operation: func(attempts *int) func() error {
				return func() error {
					*attempts++
					if *attempts < 3 {
						return errors.New("retryable")
					}
					return nil
				}
			},
			isRetryable: func(err error) bool {
				return err.Error() == "retryable"
			},
			expectedAttempts: 3,
			expectError:     false,
		},
		{
			name:        "mix of retryable and non-retryable",
			maxAttempts: 5,
			operation: func(attempts *int) func() error {
				return func() error {
					*attempts++
					if *attempts == 1 {
						return errors.New("retryable")
					}
					return errors.New("non-retryable")
				}
			},
			isRetryable: func(err error) bool {
				return err.Error() == "retryable"
			},
			expectedAttempts: 2,
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attempts := 0
			err := RetryWithChecker(
				context.Background(),
				tt.maxAttempts,
				tt.isRetryable,
				tt.operation(&attempts),
			)

			assert.Equal(t, tt.expectedAttempts, attempts)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRetryContextCancellation(t *testing.T) {
	t.Run("context cancelled before operation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		attempts := 0
		err := Retry(ctx, 3, func() error {
			attempts++
			return errors.New("should not be called")
		})

		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
		assert.Equal(t, 0, attempts)
	})

	t.Run("context cancelled during retry delay", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		attempts := 0
		err := Retry(ctx, 5, func() error {
			attempts++
			return errors.New("always fails")
		})

		assert.Error(t, err)
		// Should have attempted at least once but not all 5 times
		assert.GreaterOrEqual(t, attempts, 1)
		assert.Less(t, attempts, 5)
	})
}

func TestRetryBackoff(t *testing.T) {
	t.Run("exponential backoff timing", func(t *testing.T) {
		start := time.Now()
		attempts := 0

		_ = Retry(context.Background(), 3, func() error {
			attempts++
			return errors.New("always fails")
		})

		elapsed := time.Since(start)
		// Should have delays of 100ms and 200ms = 300ms total
		// Allow some tolerance for execution time
		assert.Greater(t, elapsed, 250*time.Millisecond)
		assert.Less(t, elapsed, 400*time.Millisecond)
		assert.Equal(t, 3, attempts)
	})
}