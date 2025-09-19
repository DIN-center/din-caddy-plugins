# Retry System Documentation

## Overview

The DIN Caddy Plugins use a centralized retry system built on top of the battle-tested [`github.com/cenkalti/backoff/v5`](https://github.com/cenkalti/backoff) package. This system provides automatic retry logic with exponential backoff for transient failures while offering comprehensive logging for observability.

## Architecture

### Core Components

1. **`lib/network/retry.go`** - Central retry implementation
2. **Exponential Backoff** - Automatically increases delay between retries
3. **Enhanced Logging** - Structured logs with timing and context information
4. **Conditional Retry Logic** - Support for retryable vs non-retryable errors

### Key Features

- **Exponential backoff** with jitter to prevent thundering herd
- **Context-aware** - Respects context cancellation
- **Comprehensive logging** for SRE observability
- **Configurable max attempts** per operation
- **Support for permanent (non-retryable) errors
- **Structured logging fields** for easy querying

## API Reference

### `Retry` Function

Simple retry mechanism for operations that should retry on any error.

```go
func Retry(
    ctx context.Context,
    logger Logger,
    operationName string,
    maxAttempts int,
    operation func() error,
    fields ...zap.Field,
) error
```

**Parameters:**
- `ctx` - Context for cancellation
- `logger` - Logger interface (supports both zap.Logger and LoggerClient)
- `operationName` - Descriptive name for the operation (used in logs)
- `maxAttempts` - Maximum number of attempts before giving up
- `operation` - The function to retry
- `fields` - Additional structured logging fields

**Example:**
```go
err := networklib.Retry(
    context.Background(),
    logger,
    "get_registry_data",
    3,
    func() error {
        return client.GetRegistryData()
    },
    zap.String("registry", "din"),
    zap.String("endpoint", "https://api.example.com"),
)
```

### `RetryWithChecker` Function

Advanced retry mechanism with custom retry logic based on error type.

```go
func RetryWithChecker(
    ctx context.Context,
    logger Logger,
    operationName string,
    maxAttempts int,
    isRetryable func(error) bool,
    operation func() error,
    fields ...zap.Field,
) error
```

**Parameters:**
- `ctx` - Context for cancellation
- `logger` - Logger interface
- `operationName` - Descriptive name for the operation
- `maxAttempts` - Maximum number of attempts
- `isRetryable` - Function to determine if an error should trigger a retry
- `operation` - The function to retry
- `fields` - Additional structured logging fields

**Example:**
```go
err := networklib.RetryWithChecker(
    context.Background(),
    logger,
    "chain_id_check",
    5,
    func(err error) bool {
        // Only retry on network errors, not on validation errors
        return IsNetworkError(err)
    },
    func() error {
        return validateChainID()
    },
    zap.String("provider", "infura"),
    zap.String("network", "ethereum"),
)
```

## Retry Behavior

### Exponential Backoff Strategy

The retry system uses exponential backoff with the following characteristics:

1. **Initial Interval**: 500ms (default)
2. **Multiplier**: 1.5x (default)
3. **Randomization Factor**: ±50% jitter
4. **Max Interval**: 60 seconds (default)

Example backoff sequence:
```
Attempt 1: Immediate
Attempt 2: ~500ms delay (250-750ms with jitter)
Attempt 3: ~750ms delay (375-1125ms with jitter)
Attempt 4: ~1.125s delay (562-1687ms with jitter)
...continues exponentially...
```

### Non-Retryable Errors

When using `RetryWithChecker`, if the `isRetryable` function returns `false`, the retry loop stops immediately and returns the error. This is useful for:

- **Business logic errors** (e.g., invalid credentials)
- **Client errors** (e.g., 400 Bad Request)
- **Permanent failures** (e.g., resource not found)

## Logging Structure

### Log Levels

- **Debug**: Individual attempt results
- **Info**: Retry notifications and final outcomes
- **Warn/Error**: Used by calling code for business-specific issues

### Structured Fields

Every log entry includes these base fields:
- `operation` - The operation name
- `max_attempts` - Maximum configured attempts
- Custom fields passed via the `fields` parameter

#### Per-Attempt Fields
- `attempt` - Current attempt number
- `attempt_duration` - Duration of this attempt
- `total_elapsed` - Total time since first attempt
- `error` - The error that occurred (if any)
- `retryable` - Whether the error is retryable (RetryWithChecker only)

#### Backoff Notification Fields
- `backoff_duration` - Time until next retry
- `total_elapsed` - Total elapsed time

#### Final Result Fields
- `total_attempts` - Number of attempts made
- `total_duration` - Total time taken
- `permanent_error` - Whether failure was due to non-retryable error

### Example Log Output

```json
{
  "level": "debug",
  "msg": "Retryable error encountered, will retry",
  "operation": "chain_id_check",
  "max_attempts": 3,
  "provider": "infura",
  "network": "ethereum",
  "error": "connection timeout",
  "retryable": true,
  "attempt": 1,
  "attempt_duration": "5.2s",
  "total_elapsed": "5.2s"
}

{
  "level": "info",
  "msg": "Retrying operation after backoff",
  "operation": "chain_id_check",
  "max_attempts": 3,
  "provider": "infura",
  "network": "ethereum",
  "attempt": 1,
  "backoff_duration": "750ms",
  "total_elapsed": "5.2s"
}

{
  "level": "debug",
  "msg": "Operation succeeded",
  "operation": "chain_id_check",
  "max_attempts": 3,
  "provider": "infura",
  "network": "ethereum",
  "attempt": 2,
  "attempt_duration": "1.3s",
  "total_duration": "7.25s"
}
```

## Usage Patterns

### Pattern 1: Simple Retry for Registry Calls

```go
err := networklib.Retry(
    ctx,
    d.logger,
    "get_registry_data",
    d.Registry.RetryMaxAttempts+1,
    func() error {
        data, err = d.DingoClient.GetRegistryData()
        return err
    },
    zap.String("registry_endpoint", d.Registry.EndpointUrl),
)
```

### Pattern 2: Conditional Retry for Network Operations

```go
err := networklib.RetryWithChecker(
    ctx,
    n.logger,
    "get_latest_block_number",
    n.RequestAttemptCount,
    func(err error) bool {
        statusCode := 0
        if lastResult != nil {
            statusCode = lastResult.ResponseStatus
        }
        return n.handler.IsRetryableError(err, statusCode)
    },
    func() error {
        res, err := n.handler.GetLatestBlockNumber(...)
        // Process result
        return err
    },
    zap.String("provider", providerHost),
    zap.String("network", n.Name),
)
```

### Pattern 3: Archive Mode Check with Retry

```go
return networklib.RetryWithChecker(
    ctx,
    n.logger,
    "archive_check",
    n.RequestAttemptCount,
    func(err error) bool {
        return n.handler.IsRetryableError(err, 0)
    },
    func() error {
        return n.handler.PerformArchiveCheck(...)
    },
    zap.String("provider", provider.host),
    zap.String("block_height", blockHeightString),
)
```

## Configuration

### Setting Max Attempts

Max attempts are typically configured at the network or middleware level:

```go
// In network configuration
n.RequestAttemptCount = 3  // Will retry up to 3 times

// In middleware configuration
d.Registry.RetryMaxAttempts = 2  // Will retry up to 3 times (0-indexed)
```

### Context with Timeout

Use context with timeout to set an overall time limit:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

err := networklib.Retry(ctx, logger, "operation", 5, func() error {
    // Operation that might take time
    return performOperation()
})
```

## Best Practices

### 1. Choose the Right Retry Function

- Use `Retry` for operations that should retry on any error
- Use `RetryWithChecker` when you need to distinguish between retryable and non-retryable errors

### 2. Set Appropriate Max Attempts

- Network operations: 3-5 attempts
- Registry/configuration fetches: 2-3 attempts
- Critical operations: Consider higher attempts with longer timeouts

### 3. Add Meaningful Context Fields

Always include relevant context in the fields parameter:
```go
// Good - includes context
networklib.Retry(ctx, logger, "fetch_block", 3, operation,
    zap.String("provider", provider),
    zap.String("network", network),
    zap.Int64("block_number", blockNum),
)

// Bad - missing context
networklib.Retry(ctx, logger, "fetch_block", 3, operation)
```

### 4. Handle Non-Retryable Errors Appropriately

```go
func isRetryable(err error) bool {
    // Don't retry on client errors
    if httpErr, ok := err.(HTTPError); ok {
        if httpErr.StatusCode >= 400 && httpErr.StatusCode < 500 {
            return false  // Client error, don't retry
        }
    }

    // Don't retry on context cancellation
    if errors.Is(err, context.Canceled) {
        return false
    }

    // Retry on other errors (network, server errors, etc.)
    return true
}
```

### 5. Monitor Retry Metrics

Use the structured logs to monitor:
- Retry rates per operation
- Average number of attempts
- Total time spent in retries
- Success rates after retries

## Troubleshooting

### High Retry Rates

If seeing high retry rates:
1. Check `retryable` field in logs to understand error types
2. Review `attempt_duration` to identify slow operations
3. Look at `backoff_duration` to ensure proper backoff
4. Consider adjusting `max_attempts` or timeout values

### Debugging Failed Operations

Look for these log patterns:
1. `"msg": "Non-retryable error encountered"` - Permanent failure
2. `"msg": "Operation failed on final attempt"` - Exhausted retries
3. `"error": "context canceled"` - Operation was cancelled
4. Check `total_duration` vs individual `attempt_duration` to identify delays

### Performance Optimization

- Reduce `max_attempts` for non-critical operations
- Implement circuit breakers for frequently failing endpoints
- Use shorter timeouts for fast-fail scenarios
- Consider implementing fallback mechanisms

## Migration Guide

### From Direct backoff Usage

Before:
```go
b := backoff.NewExponentialBackOff()
_, err := backoff.Retry(ctx, operation,
    backoff.WithBackOff(b),
    backoff.WithMaxTries(3),
)
```

After:
```go
err := networklib.Retry(ctx, logger, "operation_name", 3, operation,
    zap.String("context", "value"),
)
```

### From Manual Retry Loops

Before:
```go
for attempt := 0; attempt < maxAttempts; attempt++ {
    err := operation()
    if err == nil {
        return nil
    }
    if !isRetryable(err) {
        return err
    }
    time.Sleep(backoffDuration(attempt))
}
```

After:
```go
err := networklib.RetryWithChecker(
    ctx, logger, "operation_name", maxAttempts,
    isRetryable, operation,
    zap.String("context", "value"),
)
```

## Testing

### Unit Testing with Retry

```go
func TestWithRetry(t *testing.T) {
    logger := zaptest.NewLogger(t)
    attempts := 0

    err := networklib.Retry(
        context.Background(),
        logger,
        "test_operation",
        3,
        func() error {
            attempts++
            if attempts < 2 {
                return errors.New("temporary error")
            }
            return nil
        },
    )

    assert.NoError(t, err)
    assert.Equal(t, 2, attempts)
}
```

### Mocking for Tests

When testing code that uses retry, you can:
1. Set `maxAttempts` to 1 to disable retries
2. Use a cancelled context to skip retries
3. Mock the operation to succeed immediately

## Related Documentation

- [Exponential Backoff Algorithm](https://en.wikipedia.org/wiki/Exponential_backoff)
- [cenkalti/backoff Documentation](https://github.com/cenkalti/backoff)
- [Structured Logging Best Practices](https://www.datadoghq.com/blog/go-logging/)
- [Circuit Breaker Pattern](https://martinfowler.com/bliki/CircuitBreaker.html)