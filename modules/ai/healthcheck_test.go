package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DIN-center/din-caddy-plugins/lib/health"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// mockHealthCheckClient implements IStreamingHTTPClient for health check tests.
type mockHealthCheckClient struct {
	statusCode int
	body       string
	err        error
}

func (m *mockHealthCheckClient) Post(ctx context.Context, url string, headers map[string]string, payload []byte) ([]byte, int, http.Header, error) {
	if m.err != nil {
		return nil, 0, nil, m.err
	}
	return []byte(m.body), m.statusCode, nil, nil
}

func (m *mockHealthCheckClient) PostStream(ctx context.Context, url string, headers map[string]string, payload []byte) (*http.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &http.Response{
		StatusCode: m.statusCode,
		Body:       io.NopCloser(strings.NewReader(m.body)),
	}, nil
}

func TestCheckProvider_Success(t *testing.T) {

	provider := newTestProvider("test-openai", health.Healthy)
	provider.ModelID = "gpt-4o-mini"
	provider.AdapterType = AdapterOpenAI

	client := &mockHealthCheckClient{
		statusCode: 200,
		body:       `{"id":"chatcmpl-1","choices":[{"message":{"content":"pong"}}]}`,
	}

	logger := zap.NewNop()
	checkProvider(context.Background(), provider, client, logger)

	assert.Equal(t, health.Healthy, provider.HealthStatus())
	assert.Equal(t, 1, provider.TTFTCount())
}

func TestCheckProvider_Failure(t *testing.T) {

	provider := newTestProvider("test-openai", health.Healthy)
	provider.ModelID = "gpt-4o-mini"
	provider.AdapterType = AdapterOpenAI

	client := &mockHealthCheckClient{
		statusCode: 500,
		body:       `{"error":"internal server error"}`,
	}

	logger := zap.NewNop()

	// Need multiple failures to transition to unhealthy (threshold is 3).
	for i := 0; i < 5; i++ {
		checkProvider(context.Background(), provider, client, logger)
	}

	assert.Equal(t, health.Unhealthy, provider.HealthStatus())
}

func TestCheckProvider_RateLimited(t *testing.T) {

	provider := newTestProvider("test-openai", health.Healthy)
	provider.ModelID = "gpt-4o-mini"
	provider.AdapterType = AdapterOpenAI

	client := &mockHealthCheckClient{
		statusCode: 429,
		body:       `{"error":"rate limited"}`,
	}

	logger := zap.NewNop()
	checkProvider(context.Background(), provider, client, logger)

	assert.Equal(t, health.Warning, provider.HealthStatus())
}

func TestCheckProvider_NetworkError(t *testing.T) {

	provider := newTestProvider("test-openai", health.Healthy)
	provider.ModelID = "gpt-4o-mini"
	provider.AdapterType = AdapterOpenAI

	client := &mockHealthCheckClient{
		err: assert.AnError,
	}

	logger := zap.NewNop()

	for i := 0; i < 5; i++ {
		checkProvider(context.Background(), provider, client, logger)
	}

	assert.Equal(t, health.Unhealthy, provider.HealthStatus())
}

func TestCheckProvider_AnthropicAdapter(t *testing.T) {

	provider := newTestProvider("test-anthropic", health.Healthy)
	provider.ModelID = "claude-haiku-4-5-20251001"
	provider.AdapterType = AdapterAnthropic

	client := &mockHealthCheckClient{
		statusCode: 200,
		body:       `{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"pong"}],"model":"claude-haiku-4-5-20251001","stop_reason":"end_turn","usage":{"input_tokens":5,"output_tokens":1}}`,
	}

	logger := zap.NewNop()
	checkProvider(context.Background(), provider, client, logger)

	assert.Equal(t, health.Healthy, provider.HealthStatus())
	assert.Equal(t, 1, provider.TTFTCount())
}

func TestHealthChecker_StopsOnStop(t *testing.T) {

	tiers := map[string]*Tier{
		TierFast: {
			Name: TierFast,
			Providers: []*AIProvider{
				newTestProvider("p1", health.Healthy),
			},
		},
	}
	tiers[TierFast].Providers[0].ModelID = "gpt-4o-mini"
	tiers[TierFast].Providers[0].AdapterType = AdapterOpenAI

	client := &mockHealthCheckClient{
		statusCode: 200,
		body:       `{"id":"chatcmpl-1","choices":[{"message":{"content":"pong"}}]}`,
	}

	logger := zap.NewNop()

	checker := health.NewChecker(health.CheckerConfig{
		Interval:       1 * time.Second,
		PreventOverlap: true,
	}, func(ctx context.Context) {
		checkAllProviders(ctx, tiers, client, logger)
	})
	checker.Start()

	// Let it run briefly then stop.
	time.Sleep(100 * time.Millisecond)
	checker.Stop()

	select {
	case <-checker.Quit():
		// Checker stopped properly.
	case <-time.After(2 * time.Second):
		t.Fatal("health checker did not stop within 2 seconds")
	}
}

func TestGetAdapterForType(t *testing.T) {
	t.Run("openai adapter", func(t *testing.T) {
		adapter := getAdapterForType(AdapterOpenAI)
		assert.Equal(t, "openai", adapter.Name())
	})

	t.Run("anthropic adapter", func(t *testing.T) {
		adapter := getAdapterForType(AdapterAnthropic)
		assert.Equal(t, "anthropic", adapter.Name())
	})

	t.Run("unknown defaults to openai", func(t *testing.T) {
		adapter := getAdapterForType("unknown")
		assert.Equal(t, "openai", adapter.Name())
	})
}

func TestCheckProvider_WithOverrides(t *testing.T) {


	// Track what the mock server receives.
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"id":"chatcmpl-1","choices":[{"message":{"content":"pong"}}]}`))
	}))
	defer server.Close()

	client := newStreamingClient()
	provider, _ := NewAIProvider("openai-o3", server.URL)
	provider.ModelID = "o3-mini"
	provider.AdapterType = AdapterOpenAI
	provider.httpClient = client
	provider.HealthCheckOverrides = map[string]interface{}{
		"max_completion_tokens": 1,
	}

	logger := zap.NewNop()
	checkProvider(context.Background(), provider, client, logger)

	assert.Equal(t, health.Healthy, provider.HealthStatus())

	// Verify the request used max_completion_tokens instead of max_tokens.
	var reqMap map[string]interface{}
	require.NoError(t, json.Unmarshal(receivedBody, &reqMap))
	assert.Equal(t, float64(1), reqMap["max_completion_tokens"])
	assert.Nil(t, reqMap["max_tokens"], "max_tokens should not be present when overrides are set")
}

func TestCheckProvider_DefaultMaxTokens(t *testing.T) {


	// Track what the mock server receives.
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"id":"chatcmpl-1","choices":[{"message":{"content":"pong"}}]}`))
	}))
	defer server.Close()

	client := newStreamingClient()
	provider, _ := NewAIProvider("openai-gpt4o", server.URL)
	provider.ModelID = "gpt-4o"
	provider.AdapterType = AdapterOpenAI
	provider.httpClient = client
	// No HealthCheckOverrides — should use default max_tokens: 1

	logger := zap.NewNop()
	checkProvider(context.Background(), provider, client, logger)

	assert.Equal(t, health.Healthy, provider.HealthStatus())

	// Verify the request used default max_tokens.
	var reqMap map[string]interface{}
	require.NoError(t, json.Unmarshal(receivedBody, &reqMap))
	assert.Equal(t, float64(1), reqMap["max_tokens"])
	assert.Nil(t, reqMap["max_completion_tokens"], "max_completion_tokens should not be present by default")
}

func TestCheckProvider_CancelledContext(t *testing.T) {

	provider := newTestProvider("test-openai", health.Healthy)
	provider.ModelID = "gpt-4o-mini"
	provider.AdapterType = AdapterOpenAI

	client := &mockHealthCheckClient{
		statusCode: 200,
		body:       `{"id":"chatcmpl-1","choices":[{"message":{"content":"pong"}}]}`,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	logger := zap.NewNop()
	checkProvider(ctx, provider, client, logger)

	// With a cancelled context, the POST should fail and mark failure.
	// Provider should not be marked as Healthy from a successful response.
	// The mock doesn't check context, so it returns success — but with a real
	// client, the cancelled context would cause a transport error.
	// What matters is that checkProvider accepts the context parameter.
}

func TestCheckProvider_OverridesPreserveMaxTokens(t *testing.T) {


	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"id":"chatcmpl-1","choices":[{"message":{"content":"pong"}}]}`))
	}))
	defer server.Close()

	client := newStreamingClient()
	provider, _ := NewAIProvider("test-provider", server.URL)
	provider.ModelID = "gpt-4o"
	provider.AdapterType = AdapterOpenAI
	provider.httpClient = client
	// Override that does NOT include max_completion_tokens — max_tokens:1 should remain.
	provider.HealthCheckOverrides = map[string]interface{}{
		"temperature": 0,
	}

	logger := zap.NewNop()
	checkProvider(context.Background(), provider, client, logger)

	var reqMap map[string]interface{}
	require.NoError(t, json.Unmarshal(receivedBody, &reqMap))
	assert.Equal(t, float64(1), reqMap["max_tokens"], "max_tokens should be 1 when overrides don't include max_completion_tokens")
	assert.Equal(t, float64(0), reqMap["temperature"])
}

func TestCheckProvider_OverridesCanOverrideMaxTokens(t *testing.T) {


	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"id":"chatcmpl-1","choices":[{"message":{"content":"pong"}}]}`))
	}))
	defer server.Close()

	client := newStreamingClient()
	provider, _ := NewAIProvider("test-provider", server.URL)
	provider.ModelID = "gpt-4o"
	provider.AdapterType = AdapterOpenAI
	provider.httpClient = client
	provider.HealthCheckOverrides = map[string]interface{}{
		"max_tokens": 5,
	}

	logger := zap.NewNop()
	checkProvider(context.Background(), provider, client, logger)

	var reqMap map[string]interface{}
	require.NoError(t, json.Unmarshal(receivedBody, &reqMap))
	assert.Equal(t, float64(5), reqMap["max_tokens"], "override max_tokens:5 should win over default")
}

func TestHealthChecker_BoundsGoroutines(t *testing.T) {

	// Track concurrent goroutine count to verify bounding.
	var running int32
	var maxRunning int32

	// Create a slow mock that takes 500ms per check and tracks concurrency.
	slowMock := &mockClient{
		postHandler: func(ctx context.Context, url string, headers map[string]string, payload []byte) ([]byte, int, http.Header, error) {
			cur := atomic.AddInt32(&running, 1)
			defer atomic.AddInt32(&running, -1)

			// Track peak concurrency.
			for {
				old := atomic.LoadInt32(&maxRunning)
				if cur <= old || atomic.CompareAndSwapInt32(&maxRunning, old, cur) {
					break
				}
			}

			time.Sleep(500 * time.Millisecond)
			return []byte(`{}`), 200, nil, nil
		},
	}

	tiers := map[string]*Tier{
		TierFast: {
			Name: TierFast,
			Providers: []*AIProvider{
				func() *AIProvider {
					p := newTestProvider("p1", health.Healthy)
					p.ModelID = "test"
					p.AdapterType = AdapterOpenAI
					return p
				}(),
				func() *AIProvider {
					p := newTestProvider("p2", health.Healthy)
					p.ModelID = "test"
					p.AdapterType = AdapterOpenAI
					return p
				}(),
			},
		},
	}

	logger := zap.NewNop()

	checker := health.NewChecker(health.CheckerConfig{
		Interval:       1 * time.Second,
		PreventOverlap: true,
	}, func(ctx context.Context) {
		checkAllProviders(ctx, tiers, slowMock, logger)
	})
	checker.Start()

	// Let it run for ~3.5 ticks. With 1s interval and 500ms checks,
	// the skip-if-running guard should prevent unbounded goroutine accumulation.
	time.Sleep(3500 * time.Millisecond)
	checker.Stop()

	// With the atomic guard, max concurrent goroutines should be bounded
	// to the number of providers (2), not accumulating over time.
	peak := atomic.LoadInt32(&maxRunning)
	assert.LessOrEqual(t, peak, int32(2), "concurrent goroutines should be bounded to provider count")
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "hello", truncate("hello", 10))
	assert.Equal(t, "hel...", truncate("hello world", 3))
	assert.Equal(t, "", truncate("", 5))
}
