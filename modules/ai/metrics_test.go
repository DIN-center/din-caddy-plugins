package ai

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegisterAIMetrics(t *testing.T) {
	RegisterAIMetrics()

	assert.NotNil(t, DinAIRequestCount)
	assert.NotNil(t, DinAITokensTotal)
	assert.NotNil(t, DinAIHealthCheckCount)
	assert.NotNil(t, DinAIRequestDuration)
	assert.NotNil(t, DinAITTFTDuration)
}

func TestRegisterAIMetrics_Idempotent(t *testing.T) {
	// Should not panic when called multiple times.
	RegisterAIMetrics()
	RegisterAIMetrics()
	RegisterAIMetrics()

	assert.NotNil(t, DinAIRequestCount)
}

func TestRecordRequest(t *testing.T) {
	ensureMetricsRegistered(t)
	// Should not panic
	RecordRequest("balanced", "openai", "gpt-4o", "200")
}

func TestRecordTokens(t *testing.T) {
	ensureMetricsRegistered(t)
	// Should not panic
	RecordTokens("balanced", "openai", "gpt-4o", 100, 50)
}

func TestRecordTokensZeroValues(t *testing.T) {
	ensureMetricsRegistered(t)
	// Zero values should not record (no panic)
	RecordTokens("fast", "groq", "llama", 0, 0)
}

func TestRecordHealthCheck(t *testing.T) {
	ensureMetricsRegistered(t)
	// Should not panic
	RecordHealthCheck("openai", "200", "healthy")
}

func ensureMetricsRegistered(t *testing.T) {
	t.Helper()
	RegisterAIMetrics()
}
