package ai

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRetryAfter_Seconds(t *testing.T) {
	d, err := parseRetryAfter("2", time.Now(), 10*time.Second)
	require.NoError(t, err)
	assert.Equal(t, 2*time.Second, d)
}

func TestParseRetryAfter_HTTPDate(t *testing.T) {
	now := time.Date(2026, 2, 23, 10, 0, 0, 0, time.UTC)
	target := now.Add(3 * time.Second).Format(time.RFC1123)

	d, err := parseRetryAfter(target, now, 10*time.Second)
	require.NoError(t, err)
	assert.Equal(t, 3*time.Second, d)
}

func TestParseRetryAfter_HTTPDateRFC850(t *testing.T) {
	now := time.Date(2026, 2, 23, 10, 0, 0, 0, time.UTC)
	target := now.Add(2 * time.Second).Format(time.RFC850)

	d, err := parseRetryAfter(target, now, 10*time.Second)
	require.NoError(t, err)
	assert.Equal(t, 2*time.Second, d)
}

func TestParseRetryAfter_HTTPDateInPast(t *testing.T) {
	now := time.Date(2026, 2, 23, 10, 0, 0, 0, time.UTC)
	target := now.Add(-2 * time.Second).Format(time.RFC1123)

	d, err := parseRetryAfter(target, now, 10*time.Second)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), d)
}

func TestParseRetryAfter_Invalid(t *testing.T) {
	_, err := parseRetryAfter("not-a-date", time.Now(), 10*time.Second)
	require.Error(t, err)
}

func TestParseRetryAfter_CapApplied(t *testing.T) {
	d, err := parseRetryAfter("60", time.Now(), 1*time.Second)
	require.NoError(t, err)
	assert.Equal(t, 1*time.Second, d)
}

func TestParseRetryAfter_NegativeSeconds(t *testing.T) {
	_, err := parseRetryAfter("-5", time.Now(), 10*time.Second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "negative")
}

func TestSleepWithContext_Cancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	err := sleepWithContext(ctx, 5*time.Second)
	require.Error(t, err)
	assert.Less(t, time.Since(start), 200*time.Millisecond)
}
