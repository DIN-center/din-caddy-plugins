package ai

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// mockHealthCheckClient implements IStreamingHTTPClient for health check tests.
type mockHealthCheckClient struct {
	statusCode int
	body       string
	err        error
}

func (m *mockHealthCheckClient) Post(url string, headers map[string]string, payload []byte) ([]byte, int, error) {
	if m.err != nil {
		return nil, 0, m.err
	}
	return []byte(m.body), m.statusCode, nil
}

func (m *mockHealthCheckClient) PostStream(url string, headers map[string]string, payload []byte) (*http.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &http.Response{
		StatusCode: m.statusCode,
		Body:       io.NopCloser(strings.NewReader(m.body)),
	}, nil
}

func TestCheckProvider_Success(t *testing.T) {
	ensureMetricsRegistered(t)
	provider := newTestProvider("test-openai", Healthy)
	provider.ModelID = "gpt-4o-mini"
	provider.AdapterType = AdapterOpenAI

	client := &mockHealthCheckClient{
		statusCode: 200,
		body:       `{"id":"chatcmpl-1","choices":[{"message":{"content":"pong"}}]}`,
	}

	logger := zap.NewNop()
	checkProvider(provider, client, "test-machine", logger)

	assert.Equal(t, Healthy, provider.HealthStatus())
	assert.Equal(t, 1, provider.TTFTCount())
}

func TestCheckProvider_Failure(t *testing.T) {
	ensureMetricsRegistered(t)
	provider := newTestProvider("test-openai", Healthy)
	provider.ModelID = "gpt-4o-mini"
	provider.AdapterType = AdapterOpenAI

	client := &mockHealthCheckClient{
		statusCode: 500,
		body:       `{"error":"internal server error"}`,
	}

	logger := zap.NewNop()

	// Need multiple failures to transition to unhealthy (threshold is 3).
	for i := 0; i < 5; i++ {
		checkProvider(provider, client, "test-machine", logger)
	}

	assert.Equal(t, Unhealthy, provider.HealthStatus())
}

func TestCheckProvider_RateLimited(t *testing.T) {
	ensureMetricsRegistered(t)
	provider := newTestProvider("test-openai", Healthy)
	provider.ModelID = "gpt-4o-mini"
	provider.AdapterType = AdapterOpenAI

	client := &mockHealthCheckClient{
		statusCode: 429,
		body:       `{"error":"rate limited"}`,
	}

	logger := zap.NewNop()
	checkProvider(provider, client, "test-machine", logger)

	assert.Equal(t, Warning, provider.HealthStatus())
}

func TestCheckProvider_NetworkError(t *testing.T) {
	ensureMetricsRegistered(t)
	provider := newTestProvider("test-openai", Healthy)
	provider.ModelID = "gpt-4o-mini"
	provider.AdapterType = AdapterOpenAI

	client := &mockHealthCheckClient{
		err: assert.AnError,
	}

	logger := zap.NewNop()

	for i := 0; i < 5; i++ {
		checkProvider(provider, client, "test-machine", logger)
	}

	assert.Equal(t, Unhealthy, provider.HealthStatus())
}

func TestCheckProvider_AnthropicAdapter(t *testing.T) {
	ensureMetricsRegistered(t)
	provider := newTestProvider("test-anthropic", Healthy)
	provider.ModelID = "claude-3-5-haiku-20241022"
	provider.AdapterType = AdapterAnthropic

	client := &mockHealthCheckClient{
		statusCode: 200,
		body:       `{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"pong"}],"model":"claude-3-5-haiku-20241022","stop_reason":"end_turn","usage":{"input_tokens":5,"output_tokens":1}}`,
	}

	logger := zap.NewNop()
	checkProvider(provider, client, "test-machine", logger)

	assert.Equal(t, Healthy, provider.HealthStatus())
	assert.Equal(t, 1, provider.TTFTCount())
}

func TestRunHealthChecks_StopsOnQuit(t *testing.T) {
	ensureMetricsRegistered(t)
	tiers := map[string]*Tier{
		TierFast: {
			Name: TierFast,
			Providers: []*AIProvider{
				newTestProvider("p1", Healthy),
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
	quit := make(chan struct{})

	done := make(chan struct{})
	go func() {
		runHealthChecks(tiers, client, 1, "test-machine", logger, quit)
		close(done)
	}()

	// Let it run briefly then stop.
	time.Sleep(100 * time.Millisecond)
	close(quit)

	select {
	case <-done:
		// Goroutine exited properly.
	case <-time.After(2 * time.Second):
		t.Fatal("health check goroutine did not stop within 2 seconds")
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

func TestTruncate(t *testing.T) {
	assert.Equal(t, "hello", truncate("hello", 10))
	assert.Equal(t, "hel...", truncate("hello world", 3))
	assert.Equal(t, "", truncate("", 5))
}
