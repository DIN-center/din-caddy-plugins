package ai

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	libai "github.com/DIN-center/din-caddy-plugins/lib/ai"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"go.uber.org/zap"
)

var (
	_ caddy.Module                = (*DinAIMiddleware)(nil)
	_ caddy.Provisioner           = (*DinAIMiddleware)(nil)
	_ caddy.CleanerUpper          = (*DinAIMiddleware)(nil)
	_ caddyhttp.MiddlewareHandler = (*DinAIMiddleware)(nil)
	_ caddyfile.Unmarshaler       = (*DinAIMiddleware)(nil)
)

// DinAIMiddleware is the main Caddy module for AI smart routing.
// It handles request routing across AI providers with tier-based selection,
// health checking, streaming support, and session stickiness.
type DinAIMiddleware struct {
	Tiers map[string]*Tier `json:"tiers,omitempty"`

	// Configuration
	HealthcheckInterval  int `json:"healthcheck_interval,omitempty"`
	HealthcheckThreshold int `json:"healthcheck_threshold,omitempty"`
	RequestAttemptCount  int `json:"request_attempt_count,omitempty"`

	// Runtime
	logger *zap.Logger
	quit   chan struct{}
	client libai.IStreamingHTTPClient

	// Test mode flag — disables health checks for unit testing.
	testMode bool

	// cleanupOnce ensures Cleanup is only executed once.
	cleanupOnce sync.Once
}

// CaddyModule returns the Caddy module information.
func (*DinAIMiddleware) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.din_ai",
		New: func() caddy.Module { return new(DinAIMiddleware) },
	}
}

// Provision initializes the middleware — called once when Caddy starts.
func (m *DinAIMiddleware) Provision(ctx caddy.Context) error {
	m.logger = ctx.Logger()
	m.quit = make(chan struct{})

	if m.HealthcheckInterval == 0 {
		m.HealthcheckInterval = DefaultHCInterval
	}
	if m.HealthcheckThreshold == 0 {
		m.HealthcheckThreshold = DefaultHCThreshold
	}
	if m.RequestAttemptCount == 0 {
		m.RequestAttemptCount = DefaultRequestAttemptCount
	}

	// Initialize HTTP client.
	m.client = newDefaultStreamingClient()

	// Set health check threshold on all providers.
	for _, tier := range m.Tiers {
		for _, p := range tier.Providers {
			p.mu.Lock()
			p.hcThreshold = m.HealthcheckThreshold
			p.mu.Unlock()
			p.httpClient = m.client
			p.logger = m.logger
		}
	}

	// Start health checks (unless in test mode).
	if !m.testMode {
		go runHealthChecks(m.Tiers, m.client, m.HealthcheckInterval, m.logger, m.quit)
	}

	m.logger.Info("DIN AI middleware provisioned",
		zap.Int("tiers", len(m.Tiers)),
		zap.Int("healthcheck_interval", m.HealthcheckInterval))

	return nil
}

// Cleanup stops all goroutines when Caddy reloads or shuts down.
func (m *DinAIMiddleware) Cleanup() error {
	m.cleanupOnce.Do(func() {
		if m.quit != nil {
			close(m.quit)
		}
		m.logger.Info("DIN AI middleware cleaned up")
	})
	return nil
}

// ServeHTTP handles incoming AI API requests.
func (m *DinAIMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	// Only handle POST /v1/chat/completions.
	if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/v1/chat/completions") {
		return next.ServeHTTP(w, r)
	}

	// Validate Content-Type.
	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		return writeErrorResponse(w, http.StatusUnsupportedMediaType,
			"Content-Type must be application/json", "invalid_request_error")
	}

	// Read request body with size limit (10MB).
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
	if err != nil {
		return writeErrorResponse(w, http.StatusBadRequest,
			"failed to read request body", "invalid_request_error")
	}

	// Parse request to check stream flag.
	var req libai.ChatCompletionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return writeErrorResponse(w, http.StatusBadRequest,
			"invalid JSON in request body", "invalid_request_error")
	}

	// Resolve tier.
	tierName := r.Header.Get("X-DIN-Tier")
	if tierName == "" {
		tierName = TierBalanced
	}
	tier, ok := m.Tiers[tierName]
	if !ok {
		return writeErrorResponse(w, http.StatusBadRequest,
			fmt.Sprintf("unknown tier: %s", tierName), "invalid_request_error")
	}

	// Check for available providers.
	available := tier.GetAvailableProviders()
	if len(available) == 0 {
		return writeErrorResponse(w, http.StatusServiceUnavailable,
			fmt.Sprintf("all providers in tier '%s' are currently unavailable", tierName),
			"service_unavailable")
	}

	// Read session and dynamic headers.
	sessionID := r.Header.Get("X-DIN-Session-Id")
	dynamic := strings.EqualFold(r.Header.Get("X-DIN-Dynamic"), "true")

	// Generate request ID.
	requestID := generateRequestID()

	// Attempt to serve the request with retry.
	maxAttempts := m.RequestAttemptCount
	if maxAttempts > len(available) {
		maxAttempts = len(available)
	}

	var lastErr error
	tried := make(map[string]bool)

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Select provider.
		provider := m.selectUntried(tier, sessionID, dynamic, tried)
		if provider == nil {
			break
		}
		tried[provider.Name] = true

		adapter := getAdapterForType(provider.AdapterType)

		// Overwrite model in request body.
		modifiedBody, err := overwriteModel(body, provider.ModelID)
		if err != nil {
			lastErr = err
			continue
		}

		if req.Stream {
			result, err := attemptStream(r.Context(), provider, adapter, modifiedBody, m.client, m.logger)
			if err != nil {
				m.logger.Warn("streaming attempt failed",
					zap.String("provider", provider.Name),
					zap.Int("attempt", attempt+1),
					zap.Error(err))
				lastErr = err
				continue
			}

			// Success — record TTFT and set response headers.
			provider.RecordTTFT(result.ttft)
			setResponseHeaders(w, provider, tierName, sessionID, requestID)
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.WriteHeader(http.StatusOK)

			// Flush the first chunk.
			if result.firstChunkData != nil {
				writeSSE(w, result.firstChunkData)
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
			}

			// Stream remaining events.
			usage := streamToClient(r.Context(), w, result.resp.Body, result.reader, result.adapter, m.logger)

			// Record metrics.
			RecordRequest(tierName, provider.Name, provider.ModelID, "200")
			if usage != nil {
				RecordTokens(tierName, provider.Name, provider.ModelID,
					usage.PromptTokens, usage.CompletionTokens)
			}

			return nil
		}

		// Non-streaming request.
		respBody, statusCode, provider, err := m.attemptNonStreaming(r.Context(), provider, adapter, modifiedBody)
		if err != nil {
			m.logger.Warn("non-streaming attempt failed",
				zap.String("provider", provider.Name),
				zap.Int("attempt", attempt+1),
				zap.Error(err))
			lastErr = err
			continue
		}

		if statusCode != http.StatusOK {
			lastErr = fmt.Errorf("provider %s returned HTTP %d", provider.Name, statusCode)
			continue
		}

		// Transform response to OpenAI format.
		transformed, err := adapter.TransformResponse(respBody)
		if err != nil {
			lastErr = err
			continue
		}

		// Extract usage for metrics.
		var resp libai.ChatCompletionResponse
		if err := json.Unmarshal(transformed, &resp); err == nil && resp.Usage != nil {
			RecordTokens(tierName, provider.Name, provider.ModelID,
				resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
		}

		setResponseHeaders(w, provider, tierName, sessionID, requestID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(transformed)

		RecordRequest(tierName, provider.Name, provider.ModelID, "200")
		return nil
	}

	// All attempts failed.
	RecordRequest(tierName, "", "", "502")
	errMsg := "all provider attempts failed"
	if lastErr != nil {
		errMsg = fmt.Sprintf("all provider attempts failed: %v", lastErr)
	}
	return writeErrorResponse(w, http.StatusBadGateway, errMsg, "upstream_error")
}

// selectUntried picks a provider that hasn't been tried yet.
func (m *DinAIMiddleware) selectUntried(tier *Tier, sessionID string, dynamic bool, tried map[string]bool) *AIProvider {
	if len(tried) == 0 {
		return tier.SelectProvider(sessionID, dynamic)
	}

	// For retries, use TTFT-weighted selection excluding tried providers.
	available := tier.GetAvailableProviders()
	var untried []*AIProvider
	for _, p := range available {
		if !tried[p.Name] {
			untried = append(untried, p)
		}
	}

	if len(untried) == 0 {
		return nil
	}
	if len(untried) == 1 {
		return untried[0]
	}

	// Create a temporary tier for selection.
	tempTier := &Tier{Name: tier.Name, Providers: untried}
	return tempTier.SelectProvider("", true) // no session hash for retries
}

// attemptNonStreaming makes a non-streaming request to a provider.
func (m *DinAIMiddleware) attemptNonStreaming(
	ctx context.Context,
	provider *AIProvider,
	adapter libai.ProviderAdapter,
	body []byte,
) ([]byte, int, *AIProvider, error) {
	transformedBody, extraHeaders, err := adapter.TransformRequest(body)
	if err != nil {
		return nil, 0, provider, fmt.Errorf("transform request: %w", err)
	}

	headers := make(map[string]string)
	for k, v := range provider.Headers {
		headers[k] = v
	}
	for k, v := range extraHeaders {
		headers[k] = v
	}
	headers["Content-Type"] = "application/json"

	respBody, statusCode, err := m.client.Post(ctx, provider.HttpUrl, headers, transformedBody)
	if err != nil {
		return nil, 0, provider, fmt.Errorf("post: %w", err)
	}

	return respBody, statusCode, provider, nil
}

// setResponseHeaders sets the standard DIN AI response headers.
func setResponseHeaders(w http.ResponseWriter, provider *AIProvider, tierName, sessionID, requestID string) {
	w.Header().Set("X-DIN-Provider", provider.Name)
	w.Header().Set("X-DIN-Model", provider.ModelID)
	w.Header().Set("X-DIN-Tier", tierName)
	w.Header().Set("X-DIN-Request-Id", requestID)
	if sessionID != "" {
		w.Header().Set("X-DIN-Session-Pinned", "true")
	}
}

// overwriteModel replaces the model field in the request body with the provider's model.
func overwriteModel(body []byte, model string) ([]byte, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	modelJSON, err := json.Marshal(model)
	if err != nil {
		return nil, err
	}
	raw["model"] = modelJSON
	return json.Marshal(raw)
}

// generateRequestID creates a unique request ID using crypto/rand.
func generateRequestID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}

// expandEnvVars replaces all {env.VAR_NAME} patterns in a string with their
// environment variable values. Handles both full replacement ({env.X}) and
// embedded patterns (Bearer {env.X}).
func expandEnvVars(s string) string {
	for {
		start := strings.Index(s, "{env.")
		if start == -1 {
			return s
		}
		end := strings.Index(s[start:], "}")
		if end == -1 {
			return s
		}
		end += start
		envKey := s[start+5 : end]
		envVal := os.Getenv(envKey)
		s = s[:start] + envVal + s[end+1:]
	}
}

// writeErrorResponse writes a JSON error response in OpenAI format.
func writeErrorResponse(w http.ResponseWriter, statusCode int, message, errType string) error {
	resp := libai.ErrorResponse{
		Error: &libai.ErrorDetail{
			Message: message,
			Type:    errType,
		},
	}
	body, _ := json.Marshal(resp)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(body)
	return nil
}

// --- Caddyfile Parsing ---

// UnmarshalCaddyfile parses the din_ai directive from the Caddyfile.
func (m *DinAIMiddleware) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	m.Tiers = make(map[string]*Tier)

	d.Next() // consume "din_ai"

	for d.NextBlock(0) {
		switch d.Val() {
		case "healthcheck_interval":
			if !d.NextArg() {
				return d.ArgErr()
			}
			val, err := strconv.Atoi(d.Val())
			if err != nil {
				return d.Errf("invalid healthcheck_interval: %v", err)
			}
			m.HealthcheckInterval = val

		case "healthcheck_threshold":
			if !d.NextArg() {
				return d.ArgErr()
			}
			val, err := strconv.Atoi(d.Val())
			if err != nil {
				return d.Errf("invalid healthcheck_threshold: %v", err)
			}
			m.HealthcheckThreshold = val

		case "request_attempt_count":
			if !d.NextArg() {
				return d.ArgErr()
			}
			val, err := strconv.Atoi(d.Val())
			if err != nil {
				return d.Errf("invalid request_attempt_count: %v", err)
			}
			m.RequestAttemptCount = val

		case "tiers":
			if err := m.parseTiers(d); err != nil {
				return err
			}

		default:
			return d.Errf("unknown din_ai option: %s", d.Val())
		}
	}

	if len(m.Tiers) == 0 {
		return d.Err("at least one tier must be defined")
	}

	return nil
}

// parseTiers parses the tiers block.
func (m *DinAIMiddleware) parseTiers(d *caddyfile.Dispenser) error {
	for d.NextBlock(1) {
		tierName := d.Val()
		tier := &Tier{Name: tierName}

		for d.NextBlock(2) {
			if d.Val() != "providers" {
				return d.Errf("expected 'providers' in tier '%s', got '%s'", tierName, d.Val())
			}

			if err := m.parseProviders(d, tier); err != nil {
				return err
			}
		}

		if len(tier.Providers) == 0 {
			return d.Errf("tier '%s' must have at least one provider", tierName)
		}

		m.Tiers[tierName] = tier
	}

	return nil
}

// parseProviders parses the providers block within a tier.
func (m *DinAIMiddleware) parseProviders(d *caddyfile.Dispenser, tier *Tier) error {
	for d.NextBlock(3) {
		providerName := d.Val()
		if !d.NextArg() {
			return d.Errf("provider '%s' requires a URL", providerName)
		}
		providerURL := d.Val()

		provider, err := NewAIProvider(providerName, providerURL)
		if err != nil {
			return d.Errf("invalid URL for provider '%s': %v", providerName, err)
		}

		// Parse provider options.
		for d.NextBlock(4) {
			switch d.Val() {
			case "model":
				if !d.NextArg() {
					return d.ArgErr()
				}
				provider.ModelID = d.Val()

			case "adapter":
				if !d.NextArg() {
					return d.ArgErr()
				}
				provider.AdapterType = d.Val()

			case "headers":
				for d.NextBlock(5) {
					key := d.Val()
					if !d.NextArg() {
						return d.ArgErr()
					}
					value := expandEnvVars(d.Val())
					provider.Headers[key] = value
				}

			case "health_check":
				provider.HealthCheckOverrides = make(map[string]interface{})
				for d.NextBlock(5) {
					key := d.Val()
					if !d.NextArg() {
						return d.ArgErr()
					}
					val := d.Val()
					if intVal, err := strconv.Atoi(val); err == nil {
						provider.HealthCheckOverrides[key] = intVal
					} else {
						provider.HealthCheckOverrides[key] = val
					}
				}

			default:
				return d.Errf("unknown provider option: %s", d.Val())
			}
		}

		if provider.ModelID == "" {
			return d.Errf("provider '%s' requires a model", providerName)
		}
		if provider.AdapterType == "" {
			provider.AdapterType = AdapterOpenAI // default to openai
		}

		tier.Providers = append(tier.Providers, provider)
	}

	return nil
}

// ParseCaddyfile implements the top-level handler directive.
func (m *DinAIMiddleware) ParseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	err := m.UnmarshalCaddyfile(h.Dispenser)
	return m, err
}

// --- Default Streaming HTTP Client ---

type defaultStreamingClient struct {
	client *http.Client
}

func newDefaultStreamingClient() *defaultStreamingClient {
	return &defaultStreamingClient{
		client: &http.Client{
			Timeout: 120 * time.Second, // 2-minute timeout for streaming responses
		},
	}
}

func (c *defaultStreamingClient) Post(ctx context.Context, url string, headers map[string]string, payload []byte) ([]byte, int, error) {
	resp, err := c.PostStream(ctx, url, headers, payload)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return body, resp.StatusCode, err
}

func (c *defaultStreamingClient) PostStream(ctx context.Context, url string, headers map[string]string, payload []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.client.Do(req)
}
