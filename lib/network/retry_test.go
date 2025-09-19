package network

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
)

func TestRetry(t *testing.T) {
	tests := []struct {
		name             string
		maxAttempts      int
		operation        func(attempts *int) func() error
		expectedAttempts int
		expectError      bool
		expectedLogs     map[string]int // expected log messages and their counts
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
			expectError:      false,
			expectedLogs: map[string]int{
				"Operation succeeded":               1,
				"Retry sequence completed successfully": 1,
			},
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
			expectError:      false,
			expectedLogs: map[string]int{
				"Operation failed, will retry":      2,
				"Retrying operation after backoff":  2,
				"Operation succeeded":                1,
				"Retry sequence completed successfully": 1,
			},
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
			expectError:      true,
			expectedLogs: map[string]int{
				"Operation failed, will retry":     2,
				"Operation failed on final attempt": 1,
				"Retrying operation after backoff": 2,
				"Retry sequence failed":            1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create observer to capture logs
			core, observed := observer.New(zap.DebugLevel)
			logger := zap.New(core)

			attempts := 0
			err := Retry(
				context.Background(),
				logger,
				"test_operation",
				tt.maxAttempts,
				tt.operation(&attempts),
				zap.String("test_field", "test_value"),
			)

			// Check results
			assert.Equal(t, tt.expectedAttempts, attempts)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Check logs
			logs := observed.All()
			logMessages := make(map[string]int)
			for _, log := range logs {
				logMessages[log.Message]++
			}

			for expectedMsg, expectedCount := range tt.expectedLogs {
				actualCount := logMessages[expectedMsg]
				assert.Equal(t, expectedCount, actualCount,
					"Expected %d log entries for '%s', got %d", expectedCount, expectedMsg, actualCount)
			}

			// Verify common fields are present
			for _, log := range logs {
				assert.Contains(t, log.ContextMap(), "operation")
				assert.Equal(t, "test_operation", log.ContextMap()["operation"])
				assert.Contains(t, log.ContextMap(), "max_attempts")
				assert.Equal(t, int64(tt.maxAttempts), log.ContextMap()["max_attempts"])
				assert.Contains(t, log.ContextMap(), "test_field")
				assert.Equal(t, "test_value", log.ContextMap()["test_field"])
			}
		})
	}
}

func TestRetryWithChecker(t *testing.T) {
	tests := []struct {
		name             string
		maxAttempts      int
		operation        func(attempts *int) func() error
		isRetryable      func(error) bool
		expectedAttempts int
		expectError      bool
		expectedLogs     map[string]int
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
			expectError:      true,
			expectedLogs: map[string]int{
				"Non-retryable error encountered": 1,
				"Retry sequence failed":           1,
			},
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
			expectError:      false,
			expectedLogs: map[string]int{
				"Retryable error encountered, will retry": 2,
				"Retrying operation after backoff":        2,
				"Operation succeeded":                      1,
				"Retry sequence completed successfully":   1,
			},
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
			expectError:      true,
			expectedLogs: map[string]int{
				"Retryable error encountered, will retry": 1,
				"Retrying operation after backoff":        1,
				"Non-retryable error encountered":         1,
				"Retry sequence failed":                   1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create observer to capture logs
			core, observed := observer.New(zap.DebugLevel)
			logger := zap.New(core)

			attempts := 0
			err := RetryWithChecker(
				context.Background(),
				logger,
				"test_operation",
				tt.maxAttempts,
				tt.isRetryable,
				tt.operation(&attempts),
				zap.String("test_field", "test_value"),
			)

			// Check results
			assert.Equal(t, tt.expectedAttempts, attempts)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Check logs
			logs := observed.All()
			logMessages := make(map[string]int)
			for _, log := range logs {
				logMessages[log.Message]++
			}

			for expectedMsg, expectedCount := range tt.expectedLogs {
				actualCount := logMessages[expectedMsg]
				assert.Equal(t, expectedCount, actualCount,
					"Expected %d log entries for '%s', got %d", expectedCount, expectedMsg, actualCount)
			}

			// Check retryability field in error logs
			for _, log := range logs {
				if log.Message == "Retryable error encountered, will retry" ||
				   log.Message == "Non-retryable error encountered" ||
				   log.Message == "Retryable error on final attempt" {
					assert.Contains(t, log.ContextMap(), "retryable")
				}
			}
		})
	}
}

func TestRetryContextCancellation(t *testing.T) {
	t.Run("context cancelled before operation", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		attempts := 0
		err := Retry(ctx, logger, "test_operation", 3, func() error {
			attempts++
			return errors.New("should not be called")
		})

		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
		assert.Equal(t, 0, attempts, "Should not execute operation when context is already cancelled")
	})

	t.Run("context cancelled during retry delay", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		attempts := 0
		err := Retry(ctx, logger, "test_operation", 5, func() error {
			attempts++
			return errors.New("always fails")
		})

		assert.Error(t, err)
		// Should have attempted at least once but not all 5 times
		assert.GreaterOrEqual(t, attempts, 1)
		assert.Less(t, attempts, 5)
	})
}

func TestRetryBackoffTiming(t *testing.T) {
	t.Run("exponential backoff timing", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		start := time.Now()
		attempts := 0

		_ = Retry(context.Background(), logger, "test_operation", 3, func() error {
			attempts++
			return errors.New("always fails")
		})

		elapsed := time.Since(start)
		// Should have exponential delays between attempts
		// First retry after ~500ms, second after ~750ms = ~1.25s total minimum
		// Allow some tolerance for execution time
		assert.Greater(t, elapsed, 1*time.Second)
		assert.Less(t, elapsed, 3*time.Second)
		assert.Equal(t, 3, attempts)
	})
}

func TestRetryLoggingFields(t *testing.T) {
	t.Run("verify all logging fields are present", func(t *testing.T) {
		core, observed := observer.New(zap.DebugLevel)
		logger := zap.New(core)

		attempts := 0
		err := RetryWithChecker(
			context.Background(),
			logger,
			"complex_operation",
			3,
			func(err error) bool { return true },
			func() error {
				attempts++
				if attempts < 2 {
					return errors.New("will succeed next time")
				}
				return nil
			},
			zap.String("provider", "test-provider"),
			zap.String("network", "test-network"),
			zap.Int("custom_field", 42),
		)

		assert.NoError(t, err)

		// Check that custom fields are present in all logs
		logs := observed.All()
		for _, log := range logs {
			assert.Contains(t, log.ContextMap(), "operation")
			assert.Equal(t, "complex_operation", log.ContextMap()["operation"])
			assert.Contains(t, log.ContextMap(), "provider")
			assert.Equal(t, "test-provider", log.ContextMap()["provider"])
			assert.Contains(t, log.ContextMap(), "network")
			assert.Equal(t, "test-network", log.ContextMap()["network"])
			assert.Contains(t, log.ContextMap(), "custom_field")
			assert.Equal(t, int64(42), log.ContextMap()["custom_field"])
		}

		// Check specific logs have timing information
		for _, log := range logs {
			if log.Message == "Retryable error encountered, will retry" {
				assert.Contains(t, log.ContextMap(), "attempt")
				assert.Contains(t, log.ContextMap(), "attempt_duration")
				assert.Contains(t, log.ContextMap(), "total_elapsed")
			}
			if log.Message == "Operation succeeded" {
				assert.Contains(t, log.ContextMap(), "attempt")
				assert.Contains(t, log.ContextMap(), "attempt_duration")
				// Success log uses total_duration, not total_elapsed
				assert.Contains(t, log.ContextMap(), "total_duration")
			}
			if log.Message == "Retrying operation after backoff" {
				assert.Contains(t, log.ContextMap(), "backoff_duration")
				assert.Contains(t, log.ContextMap(), "total_elapsed")
			}
		}
	})
}

func TestRetryPermanentError(t *testing.T) {
	t.Run("permanent error is unwrapped in final log", func(t *testing.T) {
		core, observed := observer.New(zap.DebugLevel)
		logger := zap.New(core)

		err := RetryWithChecker(
			context.Background(),
			logger,
			"test_operation",
			3,
			func(err error) bool { return false }, // All errors are non-retryable
			func() error {
				return errors.New("permanent failure")
			},
		)

		assert.Error(t, err)
		assert.Equal(t, "permanent failure", err.Error())

		// Find the final failure log
		logs := observed.All()
		var foundFinalLog bool
		for _, log := range logs {
			if log.Message == "Retry sequence failed" {
				foundFinalLog = true
				assert.Contains(t, log.ContextMap(), "permanent_error")
				assert.Equal(t, true, log.ContextMap()["permanent_error"])
			}
		}
		assert.True(t, foundFinalLog, "Should have logged final failure")
	})
}