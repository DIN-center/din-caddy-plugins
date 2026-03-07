package ai

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
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
	counter := DinAIRequestCount.WithLabelValues("balanced", "openai", "gpt-4o", "200")
	before := testutil.ToFloat64(counter)
	RecordRequest("balanced", "openai", "gpt-4o", "200")
	after := testutil.ToFloat64(counter)
	assert.Equal(t, before+1, after)
}

func TestRecordTokens(t *testing.T) {
	ensureMetricsRegistered(t)
	promptCounter := DinAITokensTotal.WithLabelValues("balanced", "openai", "gpt-4o", "prompt")
	completionCounter := DinAITokensTotal.WithLabelValues("balanced", "openai", "gpt-4o", "completion")
	beforePrompt := testutil.ToFloat64(promptCounter)
	beforeCompletion := testutil.ToFloat64(completionCounter)
	RecordTokens("balanced", "openai", "gpt-4o", 100, 50)
	afterPrompt := testutil.ToFloat64(promptCounter)
	afterCompletion := testutil.ToFloat64(completionCounter)
	assert.Equal(t, beforePrompt+100, afterPrompt)
	assert.Equal(t, beforeCompletion+50, afterCompletion)
}

func TestRecordTokensZeroValues(t *testing.T) {
	ensureMetricsRegistered(t)
	promptCounter := DinAITokensTotal.WithLabelValues("fast", "groq", "llama", "prompt")
	completionCounter := DinAITokensTotal.WithLabelValues("fast", "groq", "llama", "completion")
	beforePrompt := testutil.ToFloat64(promptCounter)
	beforeCompletion := testutil.ToFloat64(completionCounter)
	RecordTokens("fast", "groq", "llama", 0, 0)
	afterPrompt := testutil.ToFloat64(promptCounter)
	afterCompletion := testutil.ToFloat64(completionCounter)
	assert.Equal(t, beforePrompt, afterPrompt)
	assert.Equal(t, beforeCompletion, afterCompletion)
}

func TestRecordCost(t *testing.T) {
	ensureMetricsRegistered(t)
	counter := DinAICostTotal.WithLabelValues("balanced", "openai", "gpt-4o")
	before := testutil.ToFloat64(counter)
	RecordCost("balanced", "openai", "gpt-4o", 0.0025)
	after := testutil.ToFloat64(counter)
	assert.InDelta(t, before+0.0025, after, 1e-12)
}

func TestRecordCost_ZeroIgnored(t *testing.T) {
	ensureMetricsRegistered(t)
	counter := DinAICostTotal.WithLabelValues("balanced", "openai", "gpt-4o")
	before := testutil.ToFloat64(counter)
	RecordCost("balanced", "openai", "gpt-4o", 0)
	after := testutil.ToFloat64(counter)
	assert.Equal(t, before, after)
}

func TestDinAICostTotal_Registered(t *testing.T) {
	ensureMetricsRegistered(t)
	assert.NotNil(t, DinAICostTotal)
}

func TestRecordHealthCheck(t *testing.T) {
	ensureMetricsRegistered(t)
	counter := DinAIHealthCheckCount.WithLabelValues("openai", "200", "healthy")
	before := testutil.ToFloat64(counter)
	RecordHealthCheck("openai", "200", "healthy")
	after := testutil.ToFloat64(counter)
	assert.Equal(t, before+1, after)
}

func ensureMetricsRegistered(t *testing.T) {
	t.Helper()
	RegisterAIMetrics()
}
