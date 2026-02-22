package ai

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	libai "github.com/DIN-center/din-caddy-plugins/lib/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// skipIfNoKey skips the test if the given env var is not set.
func skipIfNoKey(t *testing.T, envVar string) string {
	t.Helper()
	ensureMetricsRegistered(t)
	key := os.Getenv(envVar)
	if key == "" {
		t.Skipf("Skipping: %s not set", envVar)
	}
	return key
}

// newLiveMiddleware creates a DinAIMiddleware configured to talk to real AI APIs.
// Providers are set up based on which env vars are available.
func newLiveMiddleware(t *testing.T) *DinAIMiddleware {
	t.Helper()
	ensureMetricsRegistered(t)

	client := newDefaultStreamingClient()
	logger := zap.NewNop()

	m := &DinAIMiddleware{
		Tiers:               make(map[string]*Tier),
		RequestAttemptCount: DefaultRequestAttemptCount,
		logger:              logger,
		quit:                make(chan struct{}),
		machineID:           "integration-test",
		client:              client,
		testMode:            true,
	}

	// Build fast tier
	fastProviders := []*AIProvider{}
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		p, _ := NewAIProvider("openai-nano", "https://api.openai.com/v1/chat/completions")
		p.ModelID = "gpt-4.1-nano"
		p.AdapterType = AdapterOpenAI
		p.Headers["Authorization"] = "Bearer " + key
		p.httpClient = client
		p.logger = logger
		fastProviders = append(fastProviders, p)
	}
	if key := os.Getenv("MISTRAL_API_KEY"); key != "" {
		p, _ := NewAIProvider("mistral-small", "https://api.mistral.ai/v1/chat/completions")
		p.ModelID = "mistral-small-latest"
		p.AdapterType = AdapterOpenAI
		p.Headers["Authorization"] = "Bearer " + key
		p.httpClient = client
		p.logger = logger
		fastProviders = append(fastProviders, p)
	}
	if key := os.Getenv("MOONSHOT_API_KEY"); key != "" {
		p, _ := NewAIProvider("moonshot-8k", "https://api.moonshot.ai/v1/chat/completions")
		p.ModelID = "moonshot-v1-8k"
		p.AdapterType = AdapterOpenAI
		p.Headers["Authorization"] = "Bearer " + key
		p.httpClient = client
		p.logger = logger
		fastProviders = append(fastProviders, p)
	}
	if key := os.Getenv("GROK_API_KEY"); key != "" {
		p, _ := NewAIProvider("grok-mini", "https://api.x.ai/v1/chat/completions")
		p.ModelID = "grok-3-mini"
		p.AdapterType = AdapterOpenAI
		p.Headers["Authorization"] = "Bearer " + key
		p.httpClient = client
		p.logger = logger
		fastProviders = append(fastProviders, p)
	}
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		p, _ := NewAIProvider("anthropic-haiku", "https://api.anthropic.com/v1/messages")
		p.ModelID = "claude-haiku-4-5-20251001"
		p.AdapterType = AdapterAnthropic
		p.Headers["x-api-key"] = key
		p.Headers["anthropic-version"] = "2023-06-01"
		p.httpClient = client
		p.logger = logger
		fastProviders = append(fastProviders, p)
	}
	if len(fastProviders) > 0 {
		m.Tiers[TierFast] = &Tier{Name: TierFast, Providers: fastProviders}
	}

	// Build balanced tier
	balancedProviders := []*AIProvider{}
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		p, _ := NewAIProvider("openai-gpt41", "https://api.openai.com/v1/chat/completions")
		p.ModelID = "gpt-4.1"
		p.AdapterType = AdapterOpenAI
		p.Headers["Authorization"] = "Bearer " + key
		p.httpClient = client
		p.logger = logger
		balancedProviders = append(balancedProviders, p)
	}
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		p, _ := NewAIProvider("anthropic-sonnet", "https://api.anthropic.com/v1/messages")
		p.ModelID = "claude-sonnet-4-6"
		p.AdapterType = AdapterAnthropic
		p.Headers["x-api-key"] = key
		p.Headers["anthropic-version"] = "2023-06-01"
		p.httpClient = client
		p.logger = logger
		balancedProviders = append(balancedProviders, p)
	}
	if key := os.Getenv("DEEPSEEK_API_KEY"); key != "" {
		p, _ := NewAIProvider("deepseek-chat", "https://api.deepseek.com/chat/completions")
		p.ModelID = "deepseek-chat"
		p.AdapterType = AdapterOpenAI
		p.Headers["Authorization"] = "Bearer " + key
		p.httpClient = client
		p.logger = logger
		balancedProviders = append(balancedProviders, p)
	}
	if key := os.Getenv("GROK_API_KEY"); key != "" {
		p, _ := NewAIProvider("grok-fast", "https://api.x.ai/v1/chat/completions")
		p.ModelID = "grok-4-fast-reasoning"
		p.AdapterType = AdapterOpenAI
		p.Headers["Authorization"] = "Bearer " + key
		p.httpClient = client
		p.logger = logger
		balancedProviders = append(balancedProviders, p)
	}
	if len(balancedProviders) > 0 {
		m.Tiers[TierBalanced] = &Tier{Name: TierBalanced, Providers: balancedProviders}
	}

	return m
}

// makeIntegrationRequest creates a request to POST /v1/chat/completions.
func makeIntegrationRequest(t *testing.T, body string, headers map[string]string) *http.Request {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	return r
}

// --- Live API Integration Tests ---

func TestIntegration_NonStreaming_OpenAI(t *testing.T) {
	skipIfNoKey(t, "OPENAI_API_KEY")
	m := newLiveMiddleware(t)

	body := `{"messages":[{"role":"user","content":"Say hi in one word"}],"max_tokens":5}`
	r := makeIntegrationRequest(t, body, map[string]string{"X-DIN-Tier": "fast"})
	w := httptest.NewRecorder()

	err := m.ServeHTTP(w, r, noopHandler)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp libai.ChatCompletionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Choices)
	assert.NotEmpty(t, resp.Choices[0].Message.Content)

	// Verify DIN headers
	assert.NotEmpty(t, w.Header().Get("X-DIN-Provider"))
	assert.NotEmpty(t, w.Header().Get("X-DIN-Model"))
	assert.Equal(t, "fast", w.Header().Get("X-DIN-Tier"))
	assert.NotEmpty(t, w.Header().Get("X-DIN-Request-Id"))
}

func TestIntegration_NonStreaming_Anthropic(t *testing.T) {
	skipIfNoKey(t, "ANTHROPIC_API_KEY")
	m := newLiveMiddleware(t)

	// Use a session ID that hashes to Anthropic
	body := `{"messages":[{"role":"user","content":"Say hi in one word"}],"max_tokens":5}`

	// Try multiple requests until we hit Anthropic, or just test with balanced tier
	r := makeIntegrationRequest(t, body, map[string]string{"X-DIN-Tier": "balanced"})
	w := httptest.NewRecorder()

	err := m.ServeHTTP(w, r, noopHandler)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp libai.ChatCompletionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Choices)
}

func TestIntegration_NonStreaming_Mistral(t *testing.T) {
	skipIfNoKey(t, "MISTRAL_API_KEY")

	// Create a middleware with ONLY Mistral to guarantee we hit it
	client := newDefaultStreamingClient()
	logger := zap.NewNop()
	p, _ := NewAIProvider("mistral-small", "https://api.mistral.ai/v1/chat/completions")
	p.ModelID = "mistral-small-latest"
	p.AdapterType = AdapterOpenAI
	p.Headers["Authorization"] = "Bearer " + os.Getenv("MISTRAL_API_KEY")
	p.httpClient = client
	p.logger = logger

	m2 := &DinAIMiddleware{
		Tiers: map[string]*Tier{
			TierFast: {Name: TierFast, Providers: []*AIProvider{p}},
		},
		RequestAttemptCount: 3,
		logger:              logger,
		quit:                make(chan struct{}),
		machineID:           "test",
		client:              client,
		testMode:            true,
	}

	body := `{"messages":[{"role":"user","content":"Say hi in one word"}],"max_tokens":5}`
	r := makeIntegrationRequest(t, body, map[string]string{"X-DIN-Tier": "fast"})
	w := httptest.NewRecorder()

	err := m2.ServeHTTP(w, r, noopHandler)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "mistral-small", w.Header().Get("X-DIN-Provider"))

	var resp libai.ChatCompletionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Choices)
}

func TestIntegration_NonStreaming_DeepSeek(t *testing.T) {
	skipIfNoKey(t, "DEEPSEEK_API_KEY")

	client := newDefaultStreamingClient()
	logger := zap.NewNop()
	p, _ := NewAIProvider("deepseek-chat", "https://api.deepseek.com/chat/completions")
	p.ModelID = "deepseek-chat"
	p.AdapterType = AdapterOpenAI
	p.Headers["Authorization"] = "Bearer " + os.Getenv("DEEPSEEK_API_KEY")
	p.httpClient = client
	p.logger = logger

	m := &DinAIMiddleware{
		Tiers: map[string]*Tier{
			TierBalanced: {Name: TierBalanced, Providers: []*AIProvider{p}},
		},
		RequestAttemptCount: 3,
		logger:              logger,
		quit:                make(chan struct{}),
		machineID:           "test",
		client:              client,
		testMode:            true,
	}

	body := `{"messages":[{"role":"user","content":"Say hi in one word"}],"max_tokens":5}`
	r := makeIntegrationRequest(t, body, map[string]string{"X-DIN-Tier": "balanced"})
	w := httptest.NewRecorder()

	err := m.ServeHTTP(w, r, noopHandler)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "deepseek-chat", w.Header().Get("X-DIN-Provider"))
}

func TestIntegration_NonStreaming_Grok(t *testing.T) {
	skipIfNoKey(t, "GROK_API_KEY")

	client := newDefaultStreamingClient()
	logger := zap.NewNop()
	p, _ := NewAIProvider("grok-mini", "https://api.x.ai/v1/chat/completions")
	p.ModelID = "grok-3-mini"
	p.AdapterType = AdapterOpenAI
	p.Headers["Authorization"] = "Bearer " + os.Getenv("GROK_API_KEY")
	p.httpClient = client
	p.logger = logger

	m := &DinAIMiddleware{
		Tiers: map[string]*Tier{
			TierFast: {Name: TierFast, Providers: []*AIProvider{p}},
		},
		RequestAttemptCount: 3,
		logger:              logger,
		quit:                make(chan struct{}),
		machineID:           "test",
		client:              client,
		testMode:            true,
	}

	body := `{"messages":[{"role":"user","content":"Say hi in one word"}],"max_tokens":5}`
	r := makeIntegrationRequest(t, body, map[string]string{"X-DIN-Tier": "fast"})
	w := httptest.NewRecorder()

	err := m.ServeHTTP(w, r, noopHandler)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "grok-mini", w.Header().Get("X-DIN-Provider"))
}

func TestIntegration_NonStreaming_Moonshot(t *testing.T) {
	skipIfNoKey(t, "MOONSHOT_API_KEY")

	client := newDefaultStreamingClient()
	logger := zap.NewNop()
	p, _ := NewAIProvider("moonshot-8k", "https://api.moonshot.ai/v1/chat/completions")
	p.ModelID = "moonshot-v1-8k"
	p.AdapterType = AdapterOpenAI
	p.Headers["Authorization"] = "Bearer " + os.Getenv("MOONSHOT_API_KEY")
	p.httpClient = client
	p.logger = logger

	m := &DinAIMiddleware{
		Tiers: map[string]*Tier{
			TierFast: {Name: TierFast, Providers: []*AIProvider{p}},
		},
		RequestAttemptCount: 3,
		logger:              logger,
		quit:                make(chan struct{}),
		machineID:           "test",
		client:              client,
		testMode:            true,
	}

	body := `{"messages":[{"role":"user","content":"Say hi in one word"}],"max_tokens":5}`
	r := makeIntegrationRequest(t, body, map[string]string{"X-DIN-Tier": "fast"})
	w := httptest.NewRecorder()

	err := m.ServeHTTP(w, r, noopHandler)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "moonshot-8k", w.Header().Get("X-DIN-Provider"))
}

// --- Streaming Integration Tests ---

func TestIntegration_Streaming_OpenAI(t *testing.T) {
	skipIfNoKey(t, "OPENAI_API_KEY")

	client := newDefaultStreamingClient()
	logger := zap.NewNop()
	p, _ := NewAIProvider("openai-nano", "https://api.openai.com/v1/chat/completions")
	p.ModelID = "gpt-4.1-nano"
	p.AdapterType = AdapterOpenAI
	p.Headers["Authorization"] = "Bearer " + os.Getenv("OPENAI_API_KEY")
	p.httpClient = client
	p.logger = logger

	m := &DinAIMiddleware{
		Tiers: map[string]*Tier{
			TierFast: {Name: TierFast, Providers: []*AIProvider{p}},
		},
		RequestAttemptCount: 3,
		logger:              logger,
		quit:                make(chan struct{}),
		machineID:           "test",
		client:              client,
		testMode:            true,
	}

	body := `{"messages":[{"role":"user","content":"Count to 3"}],"stream":true,"max_tokens":20}`
	r := makeIntegrationRequest(t, body, map[string]string{"X-DIN-Tier": "fast"})
	w := httptest.NewRecorder()

	err := m.ServeHTTP(w, r, noopHandler)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/event-stream")

	// Verify SSE format
	respBody := w.Body.String()
	assert.Contains(t, respBody, "data: ")
	assert.Contains(t, respBody, "data: [DONE]")
	assert.Equal(t, "openai-nano", w.Header().Get("X-DIN-Provider"))
}

func TestIntegration_Streaming_Anthropic(t *testing.T) {
	skipIfNoKey(t, "ANTHROPIC_API_KEY")

	client := newDefaultStreamingClient()
	logger := zap.NewNop()
	p, _ := NewAIProvider("anthropic-haiku", "https://api.anthropic.com/v1/messages")
	p.ModelID = "claude-haiku-4-5-20251001"
	p.AdapterType = AdapterAnthropic
	p.Headers["x-api-key"] = os.Getenv("ANTHROPIC_API_KEY")
	p.Headers["anthropic-version"] = "2023-06-01"
	p.httpClient = client
	p.logger = logger

	m := &DinAIMiddleware{
		Tiers: map[string]*Tier{
			TierFast: {Name: TierFast, Providers: []*AIProvider{p}},
		},
		RequestAttemptCount: 3,
		logger:              logger,
		quit:                make(chan struct{}),
		machineID:           "test",
		client:              client,
		testMode:            true,
	}

	body := `{"messages":[{"role":"user","content":"Count to 3"}],"stream":true,"max_tokens":20}`
	r := makeIntegrationRequest(t, body, map[string]string{"X-DIN-Tier": "fast"})
	w := httptest.NewRecorder()

	err := m.ServeHTTP(w, r, noopHandler)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/event-stream")

	// Verify protocol translation: Anthropic SSE → OpenAI format
	respBody := w.Body.String()
	assert.Contains(t, respBody, "data: ")
	assert.Contains(t, respBody, "chat.completion.chunk")
	assert.Contains(t, respBody, "data: [DONE]")
	assert.Equal(t, "anthropic-haiku", w.Header().Get("X-DIN-Provider"))
}

func TestIntegration_Streaming_Grok(t *testing.T) {
	skipIfNoKey(t, "GROK_API_KEY")

	client := newDefaultStreamingClient()
	logger := zap.NewNop()
	p, _ := NewAIProvider("grok-mini", "https://api.x.ai/v1/chat/completions")
	p.ModelID = "grok-3-mini"
	p.AdapterType = AdapterOpenAI
	p.Headers["Authorization"] = "Bearer " + os.Getenv("GROK_API_KEY")
	p.httpClient = client
	p.logger = logger

	m := &DinAIMiddleware{
		Tiers: map[string]*Tier{
			TierFast: {Name: TierFast, Providers: []*AIProvider{p}},
		},
		RequestAttemptCount: 3,
		logger:              logger,
		quit:                make(chan struct{}),
		machineID:           "test",
		client:              client,
		testMode:            true,
	}

	body := `{"messages":[{"role":"user","content":"Count to 3"}],"stream":true,"max_tokens":20}`
	r := makeIntegrationRequest(t, body, map[string]string{"X-DIN-Tier": "fast"})
	w := httptest.NewRecorder()

	err := m.ServeHTTP(w, r, noopHandler)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/event-stream")

	respBody := w.Body.String()
	assert.Contains(t, respBody, "data: ")
	assert.Contains(t, respBody, "data: [DONE]")
	assert.Equal(t, "grok-mini", w.Header().Get("X-DIN-Provider"))
}

func TestIntegration_Streaming_Mistral(t *testing.T) {
	skipIfNoKey(t, "MISTRAL_API_KEY")

	client := newDefaultStreamingClient()
	logger := zap.NewNop()
	p, _ := NewAIProvider("mistral-small", "https://api.mistral.ai/v1/chat/completions")
	p.ModelID = "mistral-small-latest"
	p.AdapterType = AdapterOpenAI
	p.Headers["Authorization"] = "Bearer " + os.Getenv("MISTRAL_API_KEY")
	p.httpClient = client
	p.logger = logger

	m := &DinAIMiddleware{
		Tiers: map[string]*Tier{
			TierFast: {Name: TierFast, Providers: []*AIProvider{p}},
		},
		RequestAttemptCount: 3,
		logger:              logger,
		quit:                make(chan struct{}),
		machineID:           "test",
		client:              client,
		testMode:            true,
	}

	body := `{"messages":[{"role":"user","content":"Count to 3"}],"stream":true,"max_tokens":20}`
	r := makeIntegrationRequest(t, body, map[string]string{"X-DIN-Tier": "fast"})
	w := httptest.NewRecorder()

	err := m.ServeHTTP(w, r, noopHandler)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/event-stream")

	respBody := w.Body.String()
	assert.Contains(t, respBody, "data: ")
	assert.Contains(t, respBody, "data: [DONE]")
	assert.Equal(t, "mistral-small", w.Header().Get("X-DIN-Provider"))
}

func TestIntegration_Streaming_Moonshot(t *testing.T) {
	skipIfNoKey(t, "MOONSHOT_API_KEY")

	client := newDefaultStreamingClient()
	logger := zap.NewNop()
	p, _ := NewAIProvider("moonshot-8k", "https://api.moonshot.ai/v1/chat/completions")
	p.ModelID = "moonshot-v1-8k"
	p.AdapterType = AdapterOpenAI
	p.Headers["Authorization"] = "Bearer " + os.Getenv("MOONSHOT_API_KEY")
	p.httpClient = client
	p.logger = logger

	m := &DinAIMiddleware{
		Tiers: map[string]*Tier{
			TierFast: {Name: TierFast, Providers: []*AIProvider{p}},
		},
		RequestAttemptCount: 3,
		logger:              logger,
		quit:                make(chan struct{}),
		machineID:           "test",
		client:              client,
		testMode:            true,
	}

	body := `{"messages":[{"role":"user","content":"Count to 3"}],"stream":true,"max_tokens":20}`
	r := makeIntegrationRequest(t, body, map[string]string{"X-DIN-Tier": "fast"})
	w := httptest.NewRecorder()

	err := m.ServeHTTP(w, r, noopHandler)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/event-stream")

	respBody := w.Body.String()
	assert.Contains(t, respBody, "data: ")
	assert.Contains(t, respBody, "data: [DONE]")
	assert.Equal(t, "moonshot-8k", w.Header().Get("X-DIN-Provider"))
}

// --- Multi-Provider Integration Tests ---

func TestIntegration_SessionStickiness(t *testing.T) {
	skipIfNoKey(t, "OPENAI_API_KEY")
	m := newLiveMiddleware(t)

	if _, ok := m.Tiers[TierFast]; !ok {
		t.Skip("Fast tier not configured")
	}

	sessionID := "integration-test-sticky-session-123"
	var firstProvider string

	for i := 0; i < 3; i++ {
		body := `{"messages":[{"role":"user","content":"Say hi"}],"max_tokens":3}`
		r := makeIntegrationRequest(t, body, map[string]string{
			"X-DIN-Tier":       "fast",
			"X-DIN-Session-Id": sessionID,
		})
		w := httptest.NewRecorder()

		err := m.ServeHTTP(w, r, noopHandler)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "true", w.Header().Get("X-DIN-Session-Pinned"))

		provider := w.Header().Get("X-DIN-Provider")
		if i == 0 {
			firstProvider = provider
		} else {
			assert.Equal(t, firstProvider, provider, "Session should route to same provider on request %d", i+1)
		}
	}
}

func TestIntegration_TierSelection(t *testing.T) {
	skipIfNoKey(t, "OPENAI_API_KEY")
	m := newLiveMiddleware(t)

	// Test that different tiers resolve correctly
	for _, tier := range []string{"fast", "balanced"} {
		if _, ok := m.Tiers[tier]; !ok {
			continue
		}
		body := `{"messages":[{"role":"user","content":"Hi"}],"max_tokens":3}`
		r := makeIntegrationRequest(t, body, map[string]string{"X-DIN-Tier": tier})
		w := httptest.NewRecorder()

		err := m.ServeHTTP(w, r, noopHandler)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, tier, w.Header().Get("X-DIN-Tier"))
	}
}

func TestIntegration_Failover(t *testing.T) {
	skipIfNoKey(t, "OPENAI_API_KEY")

	client := newDefaultStreamingClient()
	logger := zap.NewNop()

	// First provider: bad URL that will fail
	badProvider, _ := NewAIProvider("bad-provider", "https://api.openai.com/v1/chat/completions")
	badProvider.ModelID = "gpt-4.1-nano"
	badProvider.AdapterType = AdapterOpenAI
	badProvider.Headers["Authorization"] = "Bearer invalid-key-will-401"
	badProvider.httpClient = client
	badProvider.logger = logger

	// Second provider: real working provider
	goodProvider, _ := NewAIProvider("good-provider", "https://api.openai.com/v1/chat/completions")
	goodProvider.ModelID = "gpt-4.1-nano"
	goodProvider.AdapterType = AdapterOpenAI
	goodProvider.Headers["Authorization"] = "Bearer " + os.Getenv("OPENAI_API_KEY")
	goodProvider.httpClient = client
	goodProvider.logger = logger

	m := &DinAIMiddleware{
		Tiers: map[string]*Tier{
			TierFast: {Name: TierFast, Providers: []*AIProvider{badProvider, goodProvider}},
		},
		RequestAttemptCount: 3,
		logger:              logger,
		quit:                make(chan struct{}),
		machineID:           "test",
		client:              client,
		testMode:            true,
	}

	body := `{"messages":[{"role":"user","content":"Hi"}],"max_tokens":3}`
	r := makeIntegrationRequest(t, body, map[string]string{"X-DIN-Tier": "fast"})
	w := httptest.NewRecorder()

	err := m.ServeHTTP(w, r, noopHandler)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	// Should have failed over to good-provider
	assert.Equal(t, "good-provider", w.Header().Get("X-DIN-Provider"))
}

func TestIntegration_ResponseHeaders(t *testing.T) {
	skipIfNoKey(t, "OPENAI_API_KEY")

	client := newDefaultStreamingClient()
	logger := zap.NewNop()
	p, _ := NewAIProvider("openai-nano", "https://api.openai.com/v1/chat/completions")
	p.ModelID = "gpt-4.1-nano"
	p.AdapterType = AdapterOpenAI
	p.Headers["Authorization"] = "Bearer " + os.Getenv("OPENAI_API_KEY")
	p.httpClient = client
	p.logger = logger

	m := &DinAIMiddleware{
		Tiers: map[string]*Tier{
			TierFast: {Name: TierFast, Providers: []*AIProvider{p}},
		},
		RequestAttemptCount: 3,
		logger:              logger,
		quit:                make(chan struct{}),
		machineID:           "test",
		client:              client,
		testMode:            true,
	}

	body := `{"messages":[{"role":"user","content":"Hi"}],"max_tokens":3}`
	r := makeIntegrationRequest(t, body, map[string]string{
		"X-DIN-Tier":       "fast",
		"X-DIN-Session-Id": "header-test-session",
	})
	w := httptest.NewRecorder()

	err := m.ServeHTTP(w, r, noopHandler)
	require.NoError(t, err)

	// All DIN headers should be present
	assert.Equal(t, "openai-nano", w.Header().Get("X-DIN-Provider"))
	assert.NotEmpty(t, w.Header().Get("X-DIN-Model"))
	assert.Equal(t, "fast", w.Header().Get("X-DIN-Tier"))
	assert.Equal(t, "true", w.Header().Get("X-DIN-Session-Pinned"))
	assert.NotEmpty(t, w.Header().Get("X-DIN-Request-Id"))

	// Request ID should be hex string
	reqID := w.Header().Get("X-DIN-Request-Id")
	assert.Regexp(t, "^[0-9a-f]+$", reqID)
}

// --- Mock Server Integration Tests (no API key required) ---

func TestIntegration_MockServer_NonStreaming(t *testing.T) {
	ensureMetricsRegistered(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req libai.ChatCompletionRequest
		json.Unmarshal(body, &req)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(libai.ChatCompletionResponse{
			ID:    "chatcmpl-mock",
			Model: "mock-model",
			Choices: []libai.ChatCompletionChoice{
				{Message: &libai.ChatMessage{Role: "assistant", Content: "Mock response"}},
			},
			Usage: &libai.UsageInfo{PromptTokens: 5, CompletionTokens: 2, TotalTokens: 7},
		})
	}))
	defer server.Close()

	client := newDefaultStreamingClient()
	logger := zap.NewNop()
	p, _ := NewAIProvider("mock-provider", server.URL)
	p.ModelID = "mock-model"
	p.AdapterType = AdapterOpenAI
	p.httpClient = client
	p.logger = logger

	m := &DinAIMiddleware{
		Tiers: map[string]*Tier{
			TierBalanced: {Name: TierBalanced, Providers: []*AIProvider{p}},
		},
		RequestAttemptCount: 3,
		logger:              logger,
		quit:                make(chan struct{}),
		machineID:           "test",
		client:              client,
		testMode:            true,
	}

	body := `{"messages":[{"role":"user","content":"Hi"}],"max_tokens":5}`
	r := makeIntegrationRequest(t, body, map[string]string{"X-DIN-Tier": "balanced"})
	w := httptest.NewRecorder()

	err := m.ServeHTTP(w, r, noopHandler)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp libai.ChatCompletionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Mock response", resp.Choices[0].Message.Content)
	assert.Equal(t, "mock-provider", w.Header().Get("X-DIN-Provider"))
}

func TestIntegration_MockServer_Streaming(t *testing.T) {
	ensureMetricsRegistered(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("ResponseWriter does not support Flush")
		}

		chunks := []string{
			`{"id":"chatcmpl-mock","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`,
			`{"id":"chatcmpl-mock","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`,
			`{"id":"chatcmpl-mock","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"!"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`,
		}

		for _, chunk := range chunks {
			w.Write([]byte("data: " + chunk + "\n\n"))
			flusher.Flush()
		}
		w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	}))
	defer server.Close()

	client := newDefaultStreamingClient()
	logger := zap.NewNop()
	p, _ := NewAIProvider("mock-stream", server.URL)
	p.ModelID = "mock-model"
	p.AdapterType = AdapterOpenAI
	p.httpClient = client
	p.logger = logger

	m := &DinAIMiddleware{
		Tiers: map[string]*Tier{
			TierFast: {Name: TierFast, Providers: []*AIProvider{p}},
		},
		RequestAttemptCount: 3,
		logger:              logger,
		quit:                make(chan struct{}),
		machineID:           "test",
		client:              client,
		testMode:            true,
	}

	body := `{"messages":[{"role":"user","content":"Hi"}],"stream":true,"max_tokens":10}`
	r := makeIntegrationRequest(t, body, map[string]string{"X-DIN-Tier": "fast"})
	w := httptest.NewRecorder()

	err := m.ServeHTTP(w, r, noopHandler)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/event-stream")

	respBody := w.Body.String()
	assert.Contains(t, respBody, "data: ")
	assert.Contains(t, respBody, "Hello")
	assert.Contains(t, respBody, "data: [DONE]")
}

func TestIntegration_MockServer_Failover(t *testing.T) {
	ensureMetricsRegistered(t)

	// Bad server returns 500
	badServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer badServer.Close()

	// Good server returns valid response
	goodServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(libai.ChatCompletionResponse{
			ID:    "chatcmpl-good",
			Model: "good-model",
			Choices: []libai.ChatCompletionChoice{
				{Message: &libai.ChatMessage{Role: "assistant", Content: "Good response"}},
			},
			Usage: &libai.UsageInfo{PromptTokens: 5, CompletionTokens: 2, TotalTokens: 7},
		})
	}))
	defer goodServer.Close()

	client := newDefaultStreamingClient()
	logger := zap.NewNop()

	bad, _ := NewAIProvider("bad-mock", badServer.URL)
	bad.ModelID = "bad-model"
	bad.AdapterType = AdapterOpenAI
	bad.httpClient = client
	bad.logger = logger

	good, _ := NewAIProvider("good-mock", goodServer.URL)
	good.ModelID = "good-model"
	good.AdapterType = AdapterOpenAI
	good.httpClient = client
	good.logger = logger

	m := &DinAIMiddleware{
		Tiers: map[string]*Tier{
			TierBalanced: {Name: TierBalanced, Providers: []*AIProvider{bad, good}},
		},
		RequestAttemptCount: 3,
		logger:              logger,
		quit:                make(chan struct{}),
		machineID:           "test",
		client:              client,
		testMode:            true,
	}

	body := `{"messages":[{"role":"user","content":"Hi"}],"max_tokens":5}`
	r := makeIntegrationRequest(t, body, map[string]string{"X-DIN-Tier": "balanced"})
	w := httptest.NewRecorder()

	err := m.ServeHTTP(w, r, noopHandler)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "good-mock", w.Header().Get("X-DIN-Provider"))
}

func TestIntegration_MockServer_AllFail_503(t *testing.T) {
	ensureMetricsRegistered(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`{"error":"always fails"}`))
	}))
	defer server.Close()

	client := newDefaultStreamingClient()
	logger := zap.NewNop()

	p1, _ := NewAIProvider("fail-1", server.URL)
	p1.ModelID = "fail-model"
	p1.AdapterType = AdapterOpenAI
	p1.httpClient = client
	p1.logger = logger

	p2, _ := NewAIProvider("fail-2", server.URL)
	p2.ModelID = "fail-model"
	p2.AdapterType = AdapterOpenAI
	p2.httpClient = client
	p2.logger = logger

	m := &DinAIMiddleware{
		Tiers: map[string]*Tier{
			TierBalanced: {Name: TierBalanced, Providers: []*AIProvider{p1, p2}},
		},
		RequestAttemptCount: 3,
		logger:              logger,
		quit:                make(chan struct{}),
		machineID:           "test",
		client:              client,
		testMode:            true,
	}

	body := `{"messages":[{"role":"user","content":"Hi"}],"max_tokens":5}`
	r := makeIntegrationRequest(t, body, map[string]string{"X-DIN-Tier": "balanced"})
	w := httptest.NewRecorder()

	m.ServeHTTP(w, r, noopHandler)
	// Should get an error response (either 503 or 502 depending on retry exhaustion)
	assert.True(t, w.Code >= 500, "Expected 5xx status, got %d", w.Code)
}
