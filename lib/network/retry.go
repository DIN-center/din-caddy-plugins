package network

import (
	"context"
	"time"

	"github.com/cenkalti/backoff/v5"
	"go.uber.org/zap"
)

// Logger interface that can be satisfied by both zap.Logger and LoggerClient
type Logger interface {
	Debug(msg string, fields ...zap.Field)
	Info(msg string, fields ...zap.Field)
}

// Retry performs an operation with exponential backoff and comprehensive logging.
// It will retry on any error up to maxAttempts times.
func Retry(
	ctx context.Context,
	logger Logger,
	operationName string,
	maxAttempts int,
	operation func() error,
	fields ...zap.Field,
) error {
	attemptCount := 0
	startTime := time.Now()

	// Create exponential backoff
	b := backoff.NewExponentialBackOff()

	// Create base fields for logging
	baseFields := append([]zap.Field{
		zap.String("operation", operationName),
		zap.Int("max_attempts", maxAttempts),
	}, fields...)

	// Wrap the operation to add logging
	wrappedOperation := func() (interface{}, error) {
		// Check context before attempting operation
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		attemptCount++
		attemptStart := time.Now()

		err := operation()
		attemptDuration := time.Since(attemptStart)

		if err != nil {
			// Log the error with all context
			logFields := append(baseFields,
				zap.Error(err),
				zap.Int("attempt", attemptCount),
				zap.Duration("attempt_duration", attemptDuration),
				zap.Duration("total_elapsed", time.Since(startTime)),
			)

			if attemptCount < maxAttempts {
				logger.Debug("Operation failed, will retry", logFields...)
			} else {
				logger.Debug("Operation failed on final attempt", logFields...)
			}
			return nil, err
		}

		// Log success
		logger.Debug("Operation succeeded",
			append(baseFields,
				zap.Int("attempt", attemptCount),
				zap.Duration("attempt_duration", attemptDuration),
				zap.Duration("total_duration", time.Since(startTime)),
			)...,
		)
		return nil, nil
	}

	// Create notify function for retry attempts
	notifyFunc := func(err error, duration time.Duration) {
		logger.Info("Retrying operation after backoff",
			append(baseFields,
				zap.Error(err),
				zap.Int("attempt", attemptCount),
				zap.Duration("backoff_duration", duration),
				zap.Duration("total_elapsed", time.Since(startTime)),
			)...,
		)
	}

	// Perform the retry
	_, err := backoff.Retry(
		ctx,
		wrappedOperation,
		backoff.WithBackOff(b),
		backoff.WithMaxTries(uint(maxAttempts)),
		backoff.WithNotify(notifyFunc),
	)

	// Log final result
	if err != nil {
		logger.Info("Retry sequence failed",
			append(baseFields,
				zap.Error(err),
				zap.Int("total_attempts", attemptCount),
				zap.Duration("total_duration", time.Since(startTime)),
			)...,
		)
	} else {
		logger.Debug("Retry sequence completed successfully",
			append(baseFields,
				zap.Int("total_attempts", attemptCount),
				zap.Duration("total_duration", time.Since(startTime)),
			)...,
		)
	}

	return err
}

// RetryWithChecker performs an operation with exponential backoff and custom retry logic.
// The isRetryable function determines whether an error should trigger a retry.
// If isRetryable returns false, the operation stops immediately with that error.
func RetryWithChecker(
	ctx context.Context,
	logger Logger,
	operationName string,
	maxAttempts int,
	isRetryable func(error) bool,
	operation func() error,
	fields ...zap.Field,
) error {
	attemptCount := 0
	startTime := time.Now()

	// Create exponential backoff
	b := backoff.NewExponentialBackOff()

	// Create base fields for logging
	baseFields := append([]zap.Field{
		zap.String("operation", operationName),
		zap.Int("max_attempts", maxAttempts),
	}, fields...)

	// Wrap the operation to add logging and retry logic
	wrappedOperation := func() (interface{}, error) {
		// Check context before attempting operation
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		attemptCount++
		attemptStart := time.Now()

		err := operation()
		attemptDuration := time.Since(attemptStart)

		if err != nil {
			// Check if error is retryable
			retryable := isRetryable(err)

			// Log the error with all context including retryability
			logFields := append(baseFields,
				zap.Error(err),
				zap.Bool("retryable", retryable),
				zap.Int("attempt", attemptCount),
				zap.Duration("attempt_duration", attemptDuration),
				zap.Duration("total_elapsed", time.Since(startTime)),
			)

			if !retryable {
				logger.Debug("Non-retryable error encountered", logFields...)
				return nil, backoff.Permanent(err)
			}

			if attemptCount < maxAttempts {
				logger.Debug("Retryable error encountered, will retry", logFields...)
			} else {
				logger.Debug("Retryable error on final attempt", logFields...)
			}
			return nil, err
		}

		// Log success
		logger.Debug("Operation succeeded",
			append(baseFields,
				zap.Int("attempt", attemptCount),
				zap.Duration("attempt_duration", attemptDuration),
				zap.Duration("total_duration", time.Since(startTime)),
			)...,
		)
		return nil, nil
	}

	// Create notify function for retry attempts
	notifyFunc := func(err error, duration time.Duration) {
		logger.Info("Retrying operation after backoff",
			append(baseFields,
				zap.Error(err),
				zap.Int("attempt", attemptCount),
				zap.Duration("backoff_duration", duration),
				zap.Duration("total_elapsed", time.Since(startTime)),
			)...,
		)
	}

	// Perform the retry
	_, err := backoff.Retry(
		ctx,
		wrappedOperation,
		backoff.WithBackOff(b),
		backoff.WithMaxTries(uint(maxAttempts)),
		backoff.WithNotify(notifyFunc),
	)

	// Log final result
	if err != nil {
		// Unwrap permanent errors for logging
		if permanentErr, ok := err.(*backoff.PermanentError); ok {
			err = permanentErr.Unwrap()
		}

		logger.Info("Retry sequence failed",
			append(baseFields,
				zap.Error(err),
				zap.Int("total_attempts", attemptCount),
				zap.Duration("total_duration", time.Since(startTime)),
				zap.Bool("permanent_error", !isRetryable(err)),
			)...,
		)
	} else {
		logger.Debug("Retry sequence completed successfully",
			append(baseFields,
				zap.Int("total_attempts", attemptCount),
				zap.Duration("total_duration", time.Since(startTime)),
			)...,
		)
	}

	return err
}