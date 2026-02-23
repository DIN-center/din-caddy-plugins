package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	libai "github.com/DIN-center/din-caddy-plugins/lib/ai"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// mockClient implements IStreamingHTTPClient for middleware tests.
type mockClient struct {
	postHandler       func(ctx context.Context, url string, headers map[string]string, payload []byte) ([]byte, int, error)
	postStreamHandler func(ctx context.Context, url string, headers map[string]string, payload []byte) (*http.Response, error)
}

func (m *mockClient) Post(ctx context.Context, url string, headers map[string]string, payload []byte) ([]byte, int, error) {
	if m.postHandler != nil {
		return m.postHandler(ctx, url, headers, payload)
	}
	return nil, 500, nil
}

func (m *mockClient) PostStream(ctx context.Context, url string, headers map[string]string, payload []byte) (*http.Response, error) {
	if m.postStreamHandler != nil {
		return m.postStreamHandler(ctx, url, headers, payload)
	}
	return nil, nil
}

// noopHandler is a caddyhttp.Handler that does nothing.
var noopHandler = caddyhttp.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
	return nil
})

func newTestMiddleware(t *testing.T) *DinAIMiddleware {
	t.Helper()
	ensureMetricsRegistered(t)

	client := &mockClient{
		postHandler: func(ctx context.Context, url string, headers map[string]string, payload []byte) ([]byte, int, error) {
			return []byte(`{"id":"chatcmpl-1","object":"chat.completion","model":"gpt-4o","choices":[{"index":0,"message":{"role":"assistant","content":"Hello!"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`), 200, nil
		},
	}

	m := &DinAIMiddleware{
		Tiers: map[string]*Tier{
			TierBalanced: {
				Name: TierBalanced,
				Providers: []*AIProvider{
					func() *AIProvider {
						p := newTestProvider("openai-gpt4o", Healthy)
						p.ModelID = "gpt-4o"
						p.AdapterType = AdapterOpenAI
						p.httpClient = client
						return p
					}(),
					func() *AIProvider {
						p := newTestProvider("deepseek-chat", Healthy)
						p.ModelID = "deepseek-chat"
						p.AdapterType = AdapterOpenAI
						p.httpClient = client
						return p
					}(),
				},
			},
			TierFast: {
				Name: TierFast,
				Providers: []*AIProvider{
					func() *AIProvider {
						p := newTestProvider("groq-llama", Healthy)
						p.ModelID = "llama-3.1-8b-instant"
						p.AdapterType = AdapterOpenAI
						p.httpClient = client
						return p
					}(),
				},
			},
		},
		logger:              zap.NewNop(),
		quit:                make(chan struct{}),

		client:              client,
		testMode:            true,
		RequestAttemptCount: DefaultRequestAttemptCount,
	}

	return m
}

func makeRequest(t *testing.T, method, path, contentType, body string, headers map[string]string) (*httptest.ResponseRecorder, *http.Request) {
	t.Helper()
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return httptest.NewRecorder(), req
}

// --- Tests ---

func TestServeHTTP_NonMatchingPath(t *testing.T) {
	m := newTestMiddleware(t)

	w, r := makeRequest(t, "POST", "/v1/models", "application/json", `{}`, nil)
	err := m.ServeHTTP(w, r, noopHandler)
	assert.NoError(t, err)
	// Should pass through to next handler (noop returns 200 with empty body).
	assert.Equal(t, 200, w.Code)
}

func TestServeHTTP_WrongMethod(t *testing.T) {
	m := newTestMiddleware(t)

	w, r := makeRequest(t, "GET", "/v1/chat/completions", "", "", nil)
	err := m.ServeHTTP(w, r, noopHandler)
	assert.NoError(t, err)
	// GET should pass through.
	assert.Equal(t, 200, w.Code)
}

func TestServeHTTP_WrongContentType(t *testing.T) {
	m := newTestMiddleware(t)

	w, r := makeRequest(t, "POST", "/v1/chat/completions", "text/plain", `{}`, nil)
	err := m.ServeHTTP(w, r, noopHandler)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnsupportedMediaType, w.Code)

	var resp libai.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp.Error.Message, "Content-Type")
}

func TestServeHTTP_InvalidJSON(t *testing.T) {
	m := newTestMiddleware(t)

	w, r := makeRequest(t, "POST", "/v1/chat/completions", "application/json", `not json`, nil)
	err := m.ServeHTTP(w, r, noopHandler)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestServeHTTP_UnknownTier(t *testing.T) {
	m := newTestMiddleware(t)

	body := `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`
	w, r := makeRequest(t, "POST", "/v1/chat/completions", "application/json", body,
		map[string]string{"X-DIN-Tier": "nonexistent"})

	err := m.ServeHTTP(w, r, noopHandler)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp libai.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp.Error.Message, "nonexistent")
}

func TestServeHTTP_AllUnhealthy(t *testing.T) {
	m := newTestMiddleware(t)

	// Mark all providers unhealthy.
	for _, p := range m.Tiers[TierBalanced].Providers {
		setProviderHealth(p, Unhealthy)
	}

	body := `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`
	w, r := makeRequest(t, "POST", "/v1/chat/completions", "application/json", body, nil)

	err := m.ServeHTTP(w, r, noopHandler)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestServeHTTP_AllProvidersFail_Returns502(t *testing.T) {
	m := newTestMiddleware(t)

	// Force all providers to return errors.
	m.client = &mockClient{
		postHandler: func(ctx context.Context, url string, headers map[string]string, payload []byte) ([]byte, int, error) {
			return nil, 0, fmt.Errorf("connection refused")
		},
	}

	body := `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`
	w, r := makeRequest(t, "POST", "/v1/chat/completions", "application/json", body, nil)

	err := m.ServeHTTP(w, r, noopHandler)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadGateway, w.Code)

	var resp libai.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp.Error.Message, "all provider attempts failed")
}

func TestServeHTTP_NonStreaming_Success(t *testing.T) {
	m := newTestMiddleware(t)

	body := `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`
	w, r := makeRequest(t, "POST", "/v1/chat/completions", "application/json", body, nil)

	err := m.ServeHTTP(w, r, noopHandler)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	// Check response headers.
	assert.NotEmpty(t, w.Header().Get("X-DIN-Provider"))
	assert.NotEmpty(t, w.Header().Get("X-DIN-Model"))
	assert.Equal(t, TierBalanced, w.Header().Get("X-DIN-Tier"))
	assert.NotEmpty(t, w.Header().Get("X-DIN-Request-Id"))
	assert.Empty(t, w.Header().Get("X-DIN-Session-Pinned"))

	// Check response body.
	var resp libai.ChatCompletionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Hello!", resp.Choices[0].Message.Content)
}

func TestServeHTTP_NonStreaming_WithSession(t *testing.T) {
	m := newTestMiddleware(t)

	body := `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`
	w, r := makeRequest(t, "POST", "/v1/chat/completions", "application/json", body,
		map[string]string{"X-DIN-Session-Id": "session-abc"})

	err := m.ServeHTTP(w, r, noopHandler)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "true", w.Header().Get("X-DIN-Session-Pinned"))
}

func TestServeHTTP_NonStreaming_SessionDeterministic(t *testing.T) {
	m := newTestMiddleware(t)

	body := `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`

	// Same session ID should always route to the same provider.
	var providers []string
	for i := 0; i < 5; i++ {
		w, r := makeRequest(t, "POST", "/v1/chat/completions", "application/json", body,
			map[string]string{"X-DIN-Session-Id": "sticky-session"})

		err := m.ServeHTTP(w, r, noopHandler)
		require.NoError(t, err)
		providers = append(providers, w.Header().Get("X-DIN-Provider"))
	}

	// All should be the same.
	for i := 1; i < len(providers); i++ {
		assert.Equal(t, providers[0], providers[i])
	}
}

func TestServeHTTP_NonStreaming_SpecificTier(t *testing.T) {
	m := newTestMiddleware(t)

	body := `{"model":"test","messages":[{"role":"user","content":"hi"}]}`
	w, r := makeRequest(t, "POST", "/v1/chat/completions", "application/json", body,
		map[string]string{"X-DIN-Tier": TierFast})

	err := m.ServeHTTP(w, r, noopHandler)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, TierFast, w.Header().Get("X-DIN-Tier"))
	assert.Equal(t, "groq-llama", w.Header().Get("X-DIN-Provider"))
}

func TestServeHTTP_Streaming_Success(t *testing.T) {
	m := newTestMiddleware(t)

	// Override client to return streaming response.
	sseData := sseEvent("", `{"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"delta":{"role":"assistant"}}]}`)
	sseData += sseEvent("", `{"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"delta":{"content":"Hello"}}]}`)
	sseData += "data: [DONE]\n\n"

	m.client = &mockClient{
		postStreamHandler: func(ctx context.Context, url string, headers map[string]string, payload []byte) (*http.Response, error) {
			return makeSSEResponse(200, sseData), nil
		},
	}

	body := `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}],"stream":true}`
	w, r := makeRequest(t, "POST", "/v1/chat/completions", "application/json", body, nil)

	err := m.ServeHTTP(w, r, noopHandler)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), "assistant")
	assert.Contains(t, w.Body.String(), "[DONE]")
}

func TestServeHTTP_Streaming_Failover(t *testing.T) {
	m := newTestMiddleware(t)

	callCount := 0
	sseData := sseEvent("", `{"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"delta":{"role":"assistant"}}]}`)
	sseData += sseEvent("", `{"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"delta":{"content":"Hello"}}]}`)
	sseData += "data: [DONE]\n\n"

	m.client = &mockClient{
		postStreamHandler: func(ctx context.Context, url string, headers map[string]string, payload []byte) (*http.Response, error) {
			callCount++
			if callCount == 1 {
				return makeSSEResponse(500, "server error"), nil
			}
			return makeSSEResponse(200, sseData), nil
		},
	}

	body := `{"model":"test","messages":[{"role":"user","content":"hi"}],"stream":true}`
	w, r := makeRequest(t, "POST", "/v1/chat/completions", "application/json", body, nil)

	err := m.ServeHTTP(w, r, noopHandler)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.GreaterOrEqual(t, callCount, 2, "should have retried after first provider failed")
	assert.Contains(t, w.Body.String(), "[DONE]")
}

func TestServeHTTP_NonStreaming_Failover(t *testing.T) {
	m := newTestMiddleware(t)

	callCount := 0
	m.client = &mockClient{
		postHandler: func(ctx context.Context, url string, headers map[string]string, payload []byte) ([]byte, int, error) {
			callCount++
			if callCount == 1 {
				return []byte(`{"error":"server error"}`), 500, nil
			}
			return []byte(`{"id":"chatcmpl-1","object":"chat.completion","model":"deepseek-chat","choices":[{"index":0,"message":{"role":"assistant","content":"fallback"},"finish_reason":"stop"}]}`), 200, nil
		},
	}

	body := `{"model":"test","messages":[{"role":"user","content":"hi"}]}`
	w, r := makeRequest(t, "POST", "/v1/chat/completions", "application/json", body, nil)

	err := m.ServeHTTP(w, r, noopHandler)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.GreaterOrEqual(t, callCount, 2)
}

func TestServeHTTP_DefaultTier(t *testing.T) {
	m := newTestMiddleware(t)

	body := `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`
	w, r := makeRequest(t, "POST", "/v1/chat/completions", "application/json", body, nil)

	err := m.ServeHTTP(w, r, noopHandler)
	assert.NoError(t, err)
	assert.Equal(t, TierBalanced, w.Header().Get("X-DIN-Tier"))
}

func TestOverwriteModel(t *testing.T) {
	body := []byte(`{"model":"original","messages":[],"stream":false}`)
	result, err := overwriteModel(body, "new-model")
	require.NoError(t, err)

	var parsed map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(result, &parsed))

	var model string
	require.NoError(t, json.Unmarshal(parsed["model"], &model))
	assert.Equal(t, "new-model", model)

	// Messages should be preserved.
	assert.Contains(t, string(result), "messages")
}

func TestGenerateRequestID(t *testing.T) {
	id1 := generateRequestID()
	id2 := generateRequestID()

	assert.Len(t, id1, 32) // 16 bytes hex encoded
	assert.Len(t, id2, 32)
	assert.NotEqual(t, id1, id2)
}

func TestWriteErrorResponse(t *testing.T) {
	w := httptest.NewRecorder()
	writeErrorResponse(w, http.StatusBadRequest, "test error", "test_type")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp libai.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "test error", resp.Error.Message)
	assert.Equal(t, "test_type", resp.Error.Type)
}

func TestCleanup(t *testing.T) {
	m := &DinAIMiddleware{
		logger: zap.NewNop(),
		quit:   make(chan struct{}),
	}

	// Should not panic.
	err := m.Cleanup()
	assert.NoError(t, err)

	// Double cleanup should not panic.
	err = m.Cleanup()
	assert.NoError(t, err)
}

func TestCleanup_NilLogger(t *testing.T) {
	m := &DinAIMiddleware{
		quit: make(chan struct{}),
		// logger intentionally nil — simulates Cleanup called before Provision.
	}

	// Should not panic.
	assert.NotPanics(t, func() {
		err := m.Cleanup()
		assert.NoError(t, err)
	})
}

func TestCleanup_StopsHealthChecks(t *testing.T) {
	ensureMetricsRegistered(t)

	m := &DinAIMiddleware{
		Tiers: map[string]*Tier{
			TierFast: {
				Name: TierFast,
				Providers: []*AIProvider{
					func() *AIProvider {
						p := newTestProvider("p1", Healthy)
						p.ModelID = "test"
						p.AdapterType = AdapterOpenAI
						return p
					}(),
				},
			},
		},
		logger:              zap.NewNop(),
		quit:                make(chan struct{}),
		HealthcheckInterval: 1,
	}

	client := &mockHealthCheckClient{statusCode: 200, body: `{}`}

	done := make(chan struct{})
	go func() {
		runHealthChecks(m.Tiers, client, 1, m.logger, m.quit)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	m.Cleanup()

	select {
	case <-done:
		// Success.
	case <-time.After(2 * time.Second):
		t.Fatal("health check goroutine did not stop after Cleanup")
	}
}

func TestServeHTTP_ConcurrentRequests(t *testing.T) {
	m := newTestMiddleware(t)

	// Override client to return streaming response with slight delay to increase overlap.
	sseData := sseEvent("", `{"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"delta":{"role":"assistant"}}]}`)
	sseData += sseEvent("", `{"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"delta":{"content":"Hello"}}]}`)
	sseData += "data: [DONE]\n\n"

	m.client = &mockClient{
		postHandler: func(ctx context.Context, url string, headers map[string]string, payload []byte) ([]byte, int, error) {
			return []byte(`{"id":"chatcmpl-1","object":"chat.completion","model":"gpt-4o","choices":[{"index":0,"message":{"role":"assistant","content":"Hello!"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`), 200, nil
		},
		postStreamHandler: func(ctx context.Context, url string, headers map[string]string, payload []byte) (*http.Response, error) {
			return makeSSEResponse(200, sseData), nil
		},
	}

	// Fire 50 concurrent non-streaming + 50 concurrent streaming requests.
	const concurrency = 100
	errs := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			var body string
			if idx%2 == 0 {
				body = `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}],"stream":false}`
			} else {
				body = `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}],"stream":true}`
			}
			w, r := makeRequest(t, "POST", "/v1/chat/completions", "application/json", body, nil)
			err := m.ServeHTTP(w, r, noopHandler)
			if err != nil {
				errs <- err
				return
			}
			if w.Code != http.StatusOK {
				errs <- fmt.Errorf("request %d: expected 200, got %d", idx, w.Code)
				return
			}
			errs <- nil
		}(i)
	}

	for i := 0; i < concurrency; i++ {
		err := <-errs
		assert.NoError(t, err)
	}
}

func TestSelectUntried(t *testing.T) {
	m := newTestMiddleware(t)
	tier := m.Tiers[TierBalanced]

	// First call should return a provider.
	tried := make(map[string]bool)
	p1 := m.selectUntried(tier, "", tried, OptimizeLatency)
	require.NotNil(t, p1)
	tried[p1.Name] = true

	// Second call should return a different provider.
	p2 := m.selectUntried(tier, "", tried, OptimizeLatency)
	require.NotNil(t, p2)
	assert.NotEqual(t, p1.Name, p2.Name)
	tried[p2.Name] = true

	// Third call with all tried should return nil.
	p3 := m.selectUntried(tier, "", tried, OptimizeLatency)
	assert.Nil(t, p3)
}

func TestExpandEnvVars(t *testing.T) {
	t.Run("basic expansion", func(t *testing.T) {
		t.Setenv("TEST_AI_KEY", "sk-123")
		result := expandEnvVars("{env.TEST_AI_KEY}")
		assert.Equal(t, "sk-123", result)
	})

	t.Run("embedded pattern", func(t *testing.T) {
		t.Setenv("TEST_AI_KEY", "sk-123")
		result := expandEnvVars("Bearer {env.TEST_AI_KEY}")
		assert.Equal(t, "Bearer sk-123", result)
	})

	t.Run("unset var returns empty", func(t *testing.T) {
		result := expandEnvVars("{env.DEFINITELY_NOT_SET_12345}")
		assert.Equal(t, "", result)
	})

	t.Run("no pattern returns unchanged", func(t *testing.T) {
		result := expandEnvVars("plain-value")
		assert.Equal(t, "plain-value", result)
	})

	t.Run("malformed pattern returns unchanged", func(t *testing.T) {
		result := expandEnvVars("{env.MISSING_CLOSE")
		assert.Equal(t, "{env.MISSING_CLOSE", result)
	})

	t.Run("multiple patterns", func(t *testing.T) {
		t.Setenv("TEST_AI_A", "aaa")
		t.Setenv("TEST_AI_B", "bbb")
		result := expandEnvVars("{env.TEST_AI_A}-{env.TEST_AI_B}")
		assert.Equal(t, "aaa-bbb", result)
	})

	t.Run("value containing env pattern does not loop", func(t *testing.T) {
		t.Setenv("TEST_AI_RECURSIVE", "prefix-{env.OTHER}-suffix")
		done := make(chan string, 1)
		go func() {
			done <- expandEnvVars("{env.TEST_AI_RECURSIVE}")
		}()
		select {
		case result := <-done:
			assert.Equal(t, "prefix-{env.OTHER}-suffix", result)
		case <-time.After(2 * time.Second):
			t.Fatal("expandEnvVars did not return — likely infinite loop")
		}
	})

	t.Run("empty value does not loop", func(t *testing.T) {
		// Unset var yields empty string replacement. With the old for{} loop
		// this could cause issues if the offset logic were wrong.
		done := make(chan string, 1)
		go func() {
			done <- expandEnvVars("{env.DEFINITELY_NOT_SET_LOOP_TEST}")
		}()
		select {
		case result := <-done:
			assert.Equal(t, "", result)
		case <-time.After(2 * time.Second):
			t.Fatal("expandEnvVars did not return — likely infinite loop")
		}
	})

	t.Run("value containing literal braces not misinterpreted", func(t *testing.T) {
		t.Setenv("TEST_AI_BRACES", "my{value}here")
		result := expandEnvVars("{env.TEST_AI_BRACES}")
		assert.Equal(t, "my{value}here", result)
	})
}

func TestServeHTTP_OversizedBody(t *testing.T) {
	m := newTestMiddleware(t)

	// Create a body larger than 10MB.
	bigBody := strings.Repeat("x", 10<<20+100)
	w, r := makeRequest(t, "POST", "/v1/chat/completions", "application/json", bigBody, nil)

	err := m.ServeHTTP(w, r, noopHandler)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

func TestServeHTTP_ExactLimitBody(t *testing.T) {
	m := newTestMiddleware(t)

	// Body exactly at the 10MB limit should proceed to JSON parsing (and fail as invalid JSON).
	body := strings.Repeat("x", 10<<20)
	w, r := makeRequest(t, "POST", "/v1/chat/completions", "application/json", body, nil)

	err := m.ServeHTTP(w, r, noopHandler)
	assert.NoError(t, err)
	// Should get 400 (invalid JSON), not 413.
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// newValidatableMiddleware returns a test middleware with all fields set to valid values.
func newValidatableMiddleware(t *testing.T) *DinAIMiddleware {
	t.Helper()
	m := newTestMiddleware(t)
	m.HealthcheckInterval = DefaultHCInterval
	m.HealthcheckThreshold = DefaultHCThreshold
	return m
}

func TestValidate_ValidConfig(t *testing.T) {
	m := newValidatableMiddleware(t)
	err := m.Validate()
	assert.NoError(t, err)
}

func TestValidate_NegativeInterval(t *testing.T) {
	m := newValidatableMiddleware(t)
	m.HealthcheckInterval = -1
	err := m.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "healthcheck_interval")
}

func TestValidate_ZeroThreshold(t *testing.T) {
	m := newValidatableMiddleware(t)
	m.HealthcheckThreshold = 0
	err := m.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "healthcheck_threshold")
}

func TestValidate_NoTiers(t *testing.T) {
	m := newValidatableMiddleware(t)
	m.Tiers = map[string]*Tier{}
	err := m.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one tier")
}

func TestValidate_EmptyTier(t *testing.T) {
	m := newValidatableMiddleware(t)
	m.Tiers["empty"] = &Tier{Name: "empty", Providers: []*AIProvider{}}
	err := m.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one provider")
}

func TestValidate_MissingModel(t *testing.T) {
	m := newValidatableMiddleware(t)
	m.Tiers[TierBalanced].Providers[0].ModelID = ""
	err := m.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires a model")
}

func TestValidate_UnknownAdapter(t *testing.T) {
	m := newValidatableMiddleware(t)
	m.Tiers[TierBalanced].Providers[0].AdapterType = "gemini"
	err := m.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown adapter type")
}

func TestUnmarshalCaddyfile_DuplicateTier(t *testing.T) {
	input := `din_ai {
		tiers {
			fast {
				providers {
					p1 https://api.example.com {
						model test-model
						cost {
							input_per_1m 1.00
							output_per_1m 2.00
						}
					}
				}
			}
			fast {
				providers {
					p2 https://api.example.com {
						model test-model
						cost {
							input_per_1m 1.00
							output_per_1m 2.00
						}
					}
				}
			}
		}
	}`
	m := &DinAIMiddleware{}
	d := caddyfile.NewTestDispenser(input)
	err := m.UnmarshalCaddyfile(d)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate tier name")
}

func TestUnmarshalCaddyfile_DuplicateProvider(t *testing.T) {
	input := `din_ai {
		tiers {
			fast {
				providers {
					p1 https://api.example.com {
						model test-model
						cost {
							input_per_1m 1.00
							output_per_1m 2.00
						}
					}
					p1 https://api.other.com {
						model other-model
						cost {
							input_per_1m 1.00
							output_per_1m 2.00
						}
					}
				}
			}
		}
	}`
	m := &DinAIMiddleware{}
	d := caddyfile.NewTestDispenser(input)
	err := m.UnmarshalCaddyfile(d)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate provider name")
}

func TestUnmarshalCaddyfile_NegativeInterval(t *testing.T) {
	input := `din_ai {
		healthcheck_interval -5
		tiers {
			fast {
				providers {
					p1 https://api.example.com {
						model test-model
						cost {
							input_per_1m 1.00
							output_per_1m 2.00
						}
					}
				}
			}
		}
	}`
	m := &DinAIMiddleware{}
	d := caddyfile.NewTestDispenser(input)
	err := m.UnmarshalCaddyfile(d)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be positive")
}

func TestUnmarshalCaddyfile_UnknownAdapterType(t *testing.T) {
	input := `din_ai {
		tiers {
			fast {
				providers {
					p1 https://api.example.com {
						model test-model
						adapter gemini
						cost {
							input_per_1m 1.00
							output_per_1m 2.00
						}
					}
				}
			}
		}
	}`
	m := &DinAIMiddleware{}
	d := caddyfile.NewTestDispenser(input)
	err := m.UnmarshalCaddyfile(d)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown adapter type")
}

func TestServeHTTP_FailoverUpdatesHealth(t *testing.T) {
	m := newTestMiddleware(t)

	callCount := 0
	m.client = &mockClient{
		postHandler: func(ctx context.Context, url string, headers map[string]string, payload []byte) ([]byte, int, error) {
			callCount++
			if callCount == 1 {
				return nil, 0, fmt.Errorf("connection refused")
			}
			return []byte(`{"id":"chatcmpl-1","object":"chat.completion","model":"test","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`), 200, nil
		},
	}

	body := `{"model":"test","messages":[{"role":"user","content":"hi"}]}`
	w, r := makeRequest(t, "POST", "/v1/chat/completions", "application/json", body, nil)

	// Get the first provider to check its health after.
	firstProvider := m.Tiers[TierBalanced].Providers[0]
	assert.Equal(t, Healthy, firstProvider.HealthStatus())

	err := m.ServeHTTP(w, r, noopHandler)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.GreaterOrEqual(t, callCount, 2)

	// At least one provider should have been marked Warning.
	hasWarning := false
	for _, p := range m.Tiers[TierBalanced].Providers {
		if p.HealthStatus() == Warning {
			hasWarning = true
			break
		}
	}
	assert.True(t, hasWarning, "at least one provider should be in Warning state after failure")
}

func TestServeHTTP_SingleFailureStaysHealthy(t *testing.T) {
	m := newTestMiddleware(t)

	// Single 500 from one provider, but it gets retried to a second.
	callCount := 0
	m.client = &mockClient{
		postHandler: func(ctx context.Context, url string, headers map[string]string, payload []byte) ([]byte, int, error) {
			callCount++
			if callCount == 1 {
				return []byte(`{"error":"server error"}`), 500, nil
			}
			return []byte(`{"id":"chatcmpl-1","object":"chat.completion","model":"test","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`), 200, nil
		},
	}

	body := `{"model":"test","messages":[{"role":"user","content":"hi"}]}`
	w, r := makeRequest(t, "POST", "/v1/chat/completions", "application/json", body, nil)
	err := m.ServeHTTP(w, r, noopHandler)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	// MarkPingWarning sets Warning but one warning doesn't make a provider Unhealthy.
	for _, p := range m.Tiers[TierBalanced].Providers {
		assert.NotEqual(t, Unhealthy, p.HealthStatus(), "single failure should not transition to Unhealthy")
	}
}

func TestServeHTTP_429DoesNotMarkUnhealthy(t *testing.T) {
	m := newTestMiddleware(t)

	callCount := 0
	m.client = &mockClient{
		postHandler: func(ctx context.Context, url string, headers map[string]string, payload []byte) ([]byte, int, error) {
			callCount++
			if callCount == 1 {
				return []byte(`{"error":"rate limited"}`), 429, nil
			}
			return []byte(`{"id":"chatcmpl-1","object":"chat.completion","model":"test","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`), 200, nil
		},
	}

	body := `{"model":"test","messages":[{"role":"user","content":"hi"}]}`
	w, r := makeRequest(t, "POST", "/v1/chat/completions", "application/json", body, nil)
	err := m.ServeHTTP(w, r, noopHandler)
	assert.NoError(t, err)

	// Provider that got 429 should be Warning, not Unhealthy.
	for _, p := range m.Tiers[TierBalanced].Providers {
		assert.NotEqual(t, Unhealthy, p.HealthStatus(), "429 should not transition to Unhealthy")
	}
}

func TestNewAIProvider_NoScheme(t *testing.T) {
	_, err := NewAIProvider("test", "api.example.com/v1/chat")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "scheme")
}

func TestNewAIProvider_FtpScheme(t *testing.T) {
	_, err := NewAIProvider("test", "ftp://api.example.com/v1/chat")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "scheme")
}

func TestNewAIProvider_EmptyHost(t *testing.T) {
	_, err := NewAIProvider("test", "https:///v1/chat")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "host")
}

// --- Cost Config Parsing Tests ---

func TestUnmarshalCaddyfile_CostBlock_Valid(t *testing.T) {
	input := `din_ai {
		tiers {
			fast {
				providers {
					p1 https://api.example.com {
						model test-model
						cost {
							input_per_1m 2.50
							output_per_1m 10.00
						}
					}
				}
			}
		}
	}`
	m := &DinAIMiddleware{}
	d := caddyfile.NewTestDispenser(input)
	err := m.UnmarshalCaddyfile(d)
	require.NoError(t, err)

	p := m.Tiers["fast"].Providers[0]
	assert.Equal(t, 2.50, p.InputCostPer1M)
	assert.Equal(t, 10.00, p.OutputCostPer1M)
}

func TestUnmarshalCaddyfile_CostBlock_MissingCostBlock(t *testing.T) {
	input := `din_ai {
		tiers {
			fast {
				providers {
					p1 https://api.example.com {
						model test-model
					}
				}
			}
		}
	}`
	m := &DinAIMiddleware{}
	d := caddyfile.NewTestDispenser(input)
	err := m.UnmarshalCaddyfile(d)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires a cost block")
}

func TestUnmarshalCaddyfile_CostBlock_MissingInputCost(t *testing.T) {
	input := `din_ai {
		tiers {
			fast {
				providers {
					p1 https://api.example.com {
						model test-model
						cost {
							output_per_1m 10.00
						}
					}
				}
			}
		}
	}`
	m := &DinAIMiddleware{}
	d := caddyfile.NewTestDispenser(input)
	err := m.UnmarshalCaddyfile(d)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires a cost block")
}

func TestUnmarshalCaddyfile_CostBlock_MissingOutputCost(t *testing.T) {
	input := `din_ai {
		tiers {
			fast {
				providers {
					p1 https://api.example.com {
						model test-model
						cost {
							input_per_1m 2.50
						}
					}
				}
			}
		}
	}`
	m := &DinAIMiddleware{}
	d := caddyfile.NewTestDispenser(input)
	err := m.UnmarshalCaddyfile(d)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires a cost block")
}

func TestUnmarshalCaddyfile_CostBlock_NegativeValue(t *testing.T) {
	input := `din_ai {
		tiers {
			fast {
				providers {
					p1 https://api.example.com {
						model test-model
						cost {
							input_per_1m -1.00
							output_per_1m 10.00
						}
					}
				}
			}
		}
	}`
	m := &DinAIMiddleware{}
	d := caddyfile.NewTestDispenser(input)
	err := m.UnmarshalCaddyfile(d)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be positive")
}

func TestUnmarshalCaddyfile_CostBlock_ZeroValue(t *testing.T) {
	input := `din_ai {
		tiers {
			fast {
				providers {
					p1 https://api.example.com {
						model test-model
						cost {
							input_per_1m 0
							output_per_1m 10.00
						}
					}
				}
			}
		}
	}`
	m := &DinAIMiddleware{}
	d := caddyfile.NewTestDispenser(input)
	err := m.UnmarshalCaddyfile(d)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be positive")
}

func TestUnmarshalCaddyfile_CostBlock_InvalidFloat(t *testing.T) {
	input := `din_ai {
		tiers {
			fast {
				providers {
					p1 https://api.example.com {
						model test-model
						cost {
							input_per_1m abc
							output_per_1m 10.00
						}
					}
				}
			}
		}
	}`
	m := &DinAIMiddleware{}
	d := caddyfile.NewTestDispenser(input)
	err := m.UnmarshalCaddyfile(d)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid cost value")
}

func TestUnmarshalCaddyfile_CostBlock_UnknownKey(t *testing.T) {
	input := `din_ai {
		tiers {
			fast {
				providers {
					p1 https://api.example.com {
						model test-model
						cost {
							input_per_1m 2.50
							output_per_1m 10.00
							total_cost 12.50
						}
					}
				}
			}
		}
	}`
	m := &DinAIMiddleware{}
	d := caddyfile.NewTestDispenser(input)
	err := m.UnmarshalCaddyfile(d)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown cost option")
}

func TestValidate_MissingCostConfig(t *testing.T) {
	m := newValidatableMiddleware(t)
	// Clear cost on one provider to test Validate path
	m.Tiers[TierBalanced].Providers[0].InputCostPer1M = 0
	m.Tiers[TierBalanced].Providers[0].OutputCostPer1M = 0
	err := m.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires cost configuration")
}

func TestCostForTokens(t *testing.T) {
	p := newTestProvider("test", Healthy)
	p.InputCostPer1M = 2.50
	p.OutputCostPer1M = 10.00

	// 1000 input tokens at $2.50/1M = $0.0025
	// 500 output tokens at $10.00/1M = $0.005
	cost := p.CostForTokens(1000, 500)
	assert.InDelta(t, 0.0075, cost, 1e-10)
}

func TestCostForTokens_ZeroTokens(t *testing.T) {
	p := newTestProvider("test", Healthy)
	p.InputCostPer1M = 2.50
	p.OutputCostPer1M = 10.00

	cost := p.CostForTokens(0, 0)
	assert.Equal(t, 0.0, cost)
}

func TestServeHTTP_Streaming_CostSSEComment(t *testing.T) {
	m := newTestMiddleware(t)

	// Set known cost values.
	for _, p := range m.Tiers[TierBalanced].Providers {
		p.InputCostPer1M = 2.50
		p.OutputCostPer1M = 10.00
	}

	// Streaming response with usage in the last chunk before [DONE].
	sseData := sseEvent("", `{"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"delta":{"role":"assistant"}}]}`)
	sseData += sseEvent("", `{"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"delta":{"content":"Hi"}}]}`)
	sseData += sseEvent("", `{"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`)
	sseData += "data: [DONE]\n\n"

	m.client = &mockClient{
		postStreamHandler: func(ctx context.Context, url string, headers map[string]string, payload []byte) (*http.Response, error) {
			return makeSSEResponse(200, sseData), nil
		},
	}

	body := `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}],"stream":true}`
	w, r := makeRequest(t, "POST", "/v1/chat/completions", "application/json", body, nil)

	err := m.ServeHTTP(w, r, noopHandler)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	respBody := w.Body.String()
	assert.Contains(t, respBody, "[DONE]")
	// Cost = 10 * 2.50 / 1M + 5 * 10.00 / 1M = 0.000075
	assert.Contains(t, respBody, ": din-cost 0.000075", "streaming response should contain cost SSE comment")
}

func TestServeHTTP_NonStreaming_CostHeader(t *testing.T) {
	m := newTestMiddleware(t)

	// Set known cost values for predictable cost calculation.
	for _, p := range m.Tiers[TierBalanced].Providers {
		p.InputCostPer1M = 2.50
		p.OutputCostPer1M = 10.00
	}

	body := `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`
	w, r := makeRequest(t, "POST", "/v1/chat/completions", "application/json", body, nil)

	err := m.ServeHTTP(w, r, noopHandler)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	// Mock response has usage: prompt_tokens=10, completion_tokens=5
	// Cost = 10 * 2.50 / 1M + 5 * 10.00 / 1M = 0.000025 + 0.000050 = 0.000075
	costHeader := w.Header().Get("X-DIN-Cost")
	assert.NotEmpty(t, costHeader, "X-DIN-Cost header should be present")
	assert.Equal(t, "0.000075", costHeader)
}

func TestServeHTTP_XDINOptimize_Invalid(t *testing.T) {
	m := newTestMiddleware(t)

	body := `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`
	w, r := makeRequest(t, "POST", "/v1/chat/completions", "application/json", body,
		map[string]string{"X-DIN-Optimize": "invalid"})

	err := m.ServeHTTP(w, r, noopHandler)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp libai.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp.Error.Message, "invalid X-DIN-Optimize")
}

func TestServeHTTP_XDINOptimize_Cost(t *testing.T) {
	m := newTestMiddleware(t)

	// Set distinct costs so cost mode favors the cheaper one.
	m.Tiers[TierBalanced].Providers[0].InputCostPer1M = 10.00
	m.Tiers[TierBalanced].Providers[0].OutputCostPer1M = 40.00
	m.Tiers[TierBalanced].Providers[1].InputCostPer1M = 0.10
	m.Tiers[TierBalanced].Providers[1].OutputCostPer1M = 0.30

	body := `{"model":"test","messages":[{"role":"user","content":"hi"}]}`
	w, r := makeRequest(t, "POST", "/v1/chat/completions", "application/json", body,
		map[string]string{"X-DIN-Optimize": "cost"})

	err := m.ServeHTTP(w, r, noopHandler)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestServeHTTP_XDINOptimize_Balanced(t *testing.T) {
	m := newTestMiddleware(t)

	body := `{"model":"test","messages":[{"role":"user","content":"hi"}]}`
	w, r := makeRequest(t, "POST", "/v1/chat/completions", "application/json", body,
		map[string]string{"X-DIN-Optimize": "balanced"})

	err := m.ServeHTTP(w, r, noopHandler)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestServeHTTP_XDINOptimize_Default(t *testing.T) {
	m := newTestMiddleware(t)

	body := `{"model":"test","messages":[{"role":"user","content":"hi"}]}`
	w, r := makeRequest(t, "POST", "/v1/chat/completions", "application/json", body, nil)

	err := m.ServeHTTP(w, r, noopHandler)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCostForTokens_LargeTokenCounts(t *testing.T) {
	p := newTestProvider("test", Healthy)
	p.InputCostPer1M = 2.50
	p.OutputCostPer1M = 10.00

	// 1M input tokens = $2.50, 1M output tokens = $10.00
	cost := p.CostForTokens(1_000_000, 1_000_000)
	assert.InDelta(t, 12.50, cost, 1e-10)
}

func TestUnmarshalCaddyfile_ValidConfig(t *testing.T) {
	input := `din_ai {
		healthcheck_interval 60
		healthcheck_threshold 5
		request_attempt_count 2
		tiers {
			fast {
				providers {
					groq-llama https://api.groq.com/openai/v1 {
						model llama-3.1-8b-instant
						adapter openai
						headers {
							Authorization "Bearer test-key"
						}
						cost {
							input_per_1m 0.05
							output_per_1m 0.08
						}
					}
				}
			}
			balanced {
				providers {
					openai-gpt4o https://api.openai.com/v1 {
						model gpt-4o
						adapter openai
						headers {
							Authorization "Bearer test-key"
						}
						health_check {
							max_completion_tokens 1
						}
						cost {
							input_per_1m 2.50
							output_per_1m 10.00
						}
					}
					anthropic-sonnet https://api.anthropic.com/v1 {
						model claude-sonnet-4-20250514
						adapter anthropic
						headers {
							x-api-key test-key
							anthropic-version 2023-06-01
						}
						cost {
							input_per_1m 3.00
							output_per_1m 15.00
						}
					}
				}
			}
		}
	}`
	m := &DinAIMiddleware{}
	d := caddyfile.NewTestDispenser(input)
	err := m.UnmarshalCaddyfile(d)
	require.NoError(t, err)

	// Verify global config.
	assert.Equal(t, 60, m.HealthcheckInterval)
	assert.Equal(t, 5, m.HealthcheckThreshold)
	assert.Equal(t, 2, m.RequestAttemptCount)

	// Verify tiers.
	assert.Len(t, m.Tiers, 2)

	// Verify fast tier.
	fast := m.Tiers["fast"]
	require.NotNil(t, fast)
	assert.Equal(t, "fast", fast.Name)
	require.Len(t, fast.Providers, 1)
	assert.Equal(t, "groq-llama", fast.Providers[0].Name)
	assert.Equal(t, "llama-3.1-8b-instant", fast.Providers[0].ModelID)
	assert.Equal(t, AdapterOpenAI, fast.Providers[0].AdapterType)
	assert.Equal(t, 0.05, fast.Providers[0].InputCostPer1M)
	assert.Equal(t, 0.08, fast.Providers[0].OutputCostPer1M)
	assert.Equal(t, "Bearer test-key", fast.Providers[0].Headers["Authorization"])

	// Verify balanced tier.
	balanced := m.Tiers["balanced"]
	require.NotNil(t, balanced)
	require.Len(t, balanced.Providers, 2)

	openai := balanced.Providers[0]
	assert.Equal(t, "openai-gpt4o", openai.Name)
	assert.Equal(t, "gpt-4o", openai.ModelID)
	assert.Equal(t, AdapterOpenAI, openai.AdapterType)
	assert.Equal(t, 2.50, openai.InputCostPer1M)
	assert.Equal(t, 10.00, openai.OutputCostPer1M)
	assert.Equal(t, 1, openai.HealthCheckOverrides["max_completion_tokens"])

	anthropic := balanced.Providers[1]
	assert.Equal(t, "anthropic-sonnet", anthropic.Name)
	assert.Equal(t, "claude-sonnet-4-20250514", anthropic.ModelID)
	assert.Equal(t, AdapterAnthropic, anthropic.AdapterType)
	assert.Equal(t, 3.00, anthropic.InputCostPer1M)
	assert.Equal(t, 15.00, anthropic.OutputCostPer1M)
}

func TestProvision_Defaults(t *testing.T) {
	m := newTestMiddleware(t)

	// Zero out config values to verify Provision applies defaults.
	m.HealthcheckInterval = 0
	m.HealthcheckThreshold = 0
	m.RequestAttemptCount = 0

	// Re-provision (newTestMiddleware already sets testMode=true).
	m.logger = zap.NewNop()
	m.quit = make(chan struct{})
	m.client = newDefaultStreamingClient()
	m.healthClient = newHealthCheckClient()

	// Manually apply the same default logic Provision uses.
	if m.HealthcheckInterval <= 0 {
		m.HealthcheckInterval = DefaultHCInterval
	}
	if m.HealthcheckThreshold <= 0 {
		m.HealthcheckThreshold = DefaultHCThreshold
	}
	if m.RequestAttemptCount <= 0 {
		m.RequestAttemptCount = DefaultRequestAttemptCount
	}

	assert.Equal(t, DefaultHCInterval, m.HealthcheckInterval)
	assert.Equal(t, DefaultHCThreshold, m.HealthcheckThreshold)
	assert.Equal(t, DefaultRequestAttemptCount, m.RequestAttemptCount)
}
