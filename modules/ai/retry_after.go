package ai

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const defaultRetryAfterCap = 5 * time.Second

func parseRetryAfter(headerValue string, now time.Time, capDuration time.Duration) (time.Duration, error) {
	if capDuration <= 0 {
		capDuration = defaultRetryAfterCap
	}

	value := strings.TrimSpace(headerValue)
	if value == "" {
		return 0, fmt.Errorf("retry-after is empty")
	}

	// Retry-After can be delta-seconds.
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds < 0 {
			return 0, fmt.Errorf("retry-after cannot be negative")
		}
		delay := time.Duration(seconds) * time.Second
		if delay > capDuration {
			return capDuration, nil
		}
		return delay, nil
	}

	// Retry-After can be an HTTP-date.
	retryAt, err := httpDateParse(value)
	if err != nil {
		return 0, fmt.Errorf("invalid retry-after: %w", err)
	}
	delay := retryAt.Sub(now)
	if delay < 0 {
		return 0, nil
	}
	if delay > capDuration {
		return capDuration, nil
	}
	return delay, nil
}

func httpDateParse(v string) (time.Time, error) {
	if ts, err := time.Parse(time.RFC1123, v); err == nil {
		return ts, nil
	}
	return time.Parse(time.RFC1123Z, v)
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
