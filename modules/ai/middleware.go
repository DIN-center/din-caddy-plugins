package ai

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
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
	_ caddy.Validator             = (*DinAIMiddleware)(nil)
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
	logger       *zap.Logger
	quit         chan struct{}
	client       libai.IStreamingHTTPClient
	healthClient libai.IStreamingHTTPClient

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

	if m.HealthcheckInterval <= 0 {
		m.HealthcheckInterval = DefaultHCInterval
	}
	if m.HealthcheckThreshold <= 0 {
		m.HealthcheckThreshold = DefaultHCThreshold
	}
	if m.RequestAttemptCount <= 0 {
		m.RequestAttemptCount = DefaultRequestAttemptCount
	}

	// Initialize HTTP clients with appropriate timeouts.
	m.client = newDefaultStreamingClient()
	m.healthClient = newHealthCheckClient()

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
		go runHealthChecks(m.Tiers, m.healthClient, m.HealthcheckInterval, m.logger, m.quit)
	}

	m.logger.Info("DIN AI middleware provisioned",
		zap.Int("tiers", len(m.Tiers)),
		zap.Int("healthcheck_interval", m.HealthcheckInterval))

	return nil
}

// Validate checks the middleware configuration for correctness — called after Provision().
func (m *DinAIMiddleware) Validate() error {
	if m.HealthcheckInterval <= 0 {
		return fmt.Errorf("healthcheck_interval must be positive, got %d", m.HealthcheckInterval)
	}
	if m.HealthcheckThreshold <= 0 {
		return fmt.Errorf("healthcheck_threshold must be positive, got %d", m.HealthcheckThreshold)
	}
	if m.RequestAttemptCount <= 0 {
		return fmt.Errorf("request_attempt_count must be positive, got %d", m.RequestAttemptCount)
	}
	if len(m.Tiers) == 0 {
		return fmt.Errorf("at least one tier must be defined")
	}
	for tierName, tier := range m.Tiers {
		if len(tier.Providers) == 0 {
			return fmt.Errorf("tier '%s' must have at least one provider", tierName)
		}
		for _, p := range tier.Providers {
			if p.ModelID == "" {
				return fmt.Errorf("provider '%s' in tier '%s' requires a model", p.Name, tierName)
			}
			if p.AdapterType != AdapterOpenAI && p.AdapterType != AdapterAnthropic {
				return fmt.Errorf("provider '%s' has unknown adapter type '%s'", p.Name, p.AdapterType)
			}
			if p.InputCostPer1M <= 0 || p.OutputCostPer1M <= 0 {
				return fmt.Errorf("provider '%s' in tier '%s' requires cost configuration (input_per_1m and output_per_1m)", p.Name, tierName)
			}
		}
	}
	return nil
}

// Cleanup stops all goroutines when Caddy reloads or shuts down.
func (m *DinAIMiddleware) Cleanup() error {
	m.cleanupOnce.Do(func() {
		if m.quit != nil {
			close(m.quit)
		}
		if m.logger != nil {
			m.logger.Info("DIN AI middleware cleaned up")
		}
	})
	return nil
}

// ServeHTTP handles incoming AI API requests.
func (m *DinAIMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	requestPath := path.Clean(r.URL.Path)
	// Only handle POST /v1/chat/completions.
	if r.Method != http.MethodPost || requestPath != "/v1/chat/completions" {
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
	const maxBodySize = 10 << 20
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodySize+1))
	if err != nil {
		return writeErrorResponse(w, http.StatusBadRequest,
			"failed to read request body", "invalid_request_error")
	}
	if len(body) > maxBodySize {
		return writeErrorResponse(w, http.StatusRequestEntityTooLarge,
			"request body exceeds 10MB limit", "invalid_request_error")
	}

	// Parse request once to check stream flag and prepare for model overwrite.
	var rawFields map[string]json.RawMessage
	if err := json.Unmarshal(body, &rawFields); err != nil {
		return writeErrorResponse(w, http.StatusBadRequest,
			"invalid JSON in request body", "invalid_request_error")
	}
	var isStream bool
	if streamRaw, ok := rawFields["stream"]; ok {
		_ = json.Unmarshal(streamRaw, &isStream)
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

	// Read session header.
	sessionID := r.Header.Get("X-DIN-Session-Id")

	// Read optimization mode header.
	optimizeMode := r.Header.Get("X-DIN-Optimize")
	if optimizeMode == "" {
		optimizeMode = OptimizeLatency
	}
	if optimizeMode != OptimizeLatency && optimizeMode != OptimizeCost && optimizeMode != OptimizeBalanced {
		return writeErrorResponse(w, http.StatusBadRequest,
			fmt.Sprintf("invalid X-DIN-Optimize value: %s (must be 'latency', 'cost', or 'balanced')", optimizeMode),
			"invalid_request_error")
	}

	// Generate request ID.
	requestID := generateRequestID()

	// Attempt to serve the request with retry.
	maxAttempts := m.RequestAttemptCount
	if maxAttempts <= 0 {
		maxAttempts = DefaultRequestAttemptCount
	}

	var lastErr error
	tried := make(map[string]bool)
	attemptCount := 0
	provider := m.selectUntried(tier, sessionID, tried, optimizeMode)
	retriedCurrentProvider := false

	for attemptCount < maxAttempts && provider != nil {

		adapter := getAdapterForType(provider.AdapterType)

		// Overwrite model in request body.
		modifiedBody, err := overwriteModel(rawFields, provider.ModelID)
		if err != nil {
			lastErr = err
			attemptCount++
			if attemptCount >= maxAttempts {
				break
			}
			tried[provider.Name] = true
			provider = m.selectUntried(tier, sessionID, tried, optimizeMode)
			retriedCurrentProvider = false
			continue
		}

		if isStream {
			result, err := attemptStream(r.Context(), provider, adapter, modifiedBody, m.client, m.logger)
			if err != nil {
				provider.MarkPingWarning()
				m.logger.Warn("streaming attempt failed",
					zap.String("provider", provider.Name),
					zap.Int("attempt", attemptCount+1),
					zap.Error(err))
				lastErr = err
				attemptCount++

				if attemptCount >= maxAttempts {
					break
				}

				if shouldRetrySameProviderForStream(err, retriedCurrentProvider) {
					retriedCurrentProvider = true
					retryAfter := streamRetryAfter(err)
					if retryAfter <= 0 {
						retryAfter = defaultRetryBackoff
					}
					if sleepErr := sleepWithContext(r.Context(), retryAfter); sleepErr != nil {
						lastErr = sleepErr
						break
					}
					continue
				}

				tried[provider.Name] = true
				provider = m.selectUntried(tier, sessionID, tried, optimizeMode)
				retriedCurrentProvider = false
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
			usage, streamErr := streamToClient(r.Context(), w, result.resp.Body, result.reader, result.adapter, m.logger)
			if streamErr != nil {
				provider.MarkPingWarning()
				RecordRequest(tierName, provider.Name, provider.ModelID, "502")
				m.logger.Warn("streaming response terminated due to transform error",
					zap.String("provider", provider.Name),
					zap.Error(streamErr))
				return nil
			}

			// Record metrics.
			RecordRequest(tierName, provider.Name, provider.ModelID, "200")
			if usage != nil {
				RecordTokens(tierName, provider.Name, provider.ModelID,
					usage.PromptTokens, usage.CompletionTokens)
				cost := provider.CostForTokens(usage.PromptTokens, usage.CompletionTokens)
				RecordCost(tierName, provider.Name, provider.ModelID, cost)
				// Write cost as SSE comment after [DONE] — ignored by standard clients,
				// parseable by DIN-aware clients.
				fmt.Fprintf(w, ": din-cost %.6f\n\n", cost)
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
			}

			return nil
		}

		// Non-streaming request.
		var respBody []byte
		var statusCode int
		var respHeaders http.Header
		respBody, statusCode, respHeaders, provider, err = m.attemptNonStreaming(r.Context(), provider, adapter, modifiedBody)
		if err != nil {
			provider.MarkPingWarning()
			m.logger.Warn("non-streaming attempt failed",
				zap.String("provider", provider.Name),
				zap.Int("attempt", attemptCount+1),
				zap.Error(err))
			lastErr = err
			attemptCount++
			if attemptCount >= maxAttempts {
				break
			}
			tried[provider.Name] = true
			provider = m.selectUntried(tier, sessionID, tried, optimizeMode)
			retriedCurrentProvider = false
			continue
		}

		if statusCode != http.StatusOK {
			provider.MarkPingWarning()
			lastErr = fmt.Errorf("provider %s returned HTTP %d", provider.Name, statusCode)
			attemptCount++
			if attemptCount >= maxAttempts {
				break
			}

			if isRetryableStatus(statusCode) && !retriedCurrentProvider {
				retriedCurrentProvider = true
				retryAfter := defaultRetryBackoff
				if parsed, ok := retryDelayFromHeaders(respHeaders); ok {
					retryAfter = parsed
				}
				if sleepErr := sleepWithContext(r.Context(), retryAfter); sleepErr != nil {
					lastErr = sleepErr
					break
				}
				continue
			}

			tried[provider.Name] = true
			provider = m.selectUntried(tier, sessionID, tried, optimizeMode)
			retriedCurrentProvider = false
			continue
		}

		// Transform response to OpenAI format.
		transformed, err := adapter.TransformResponse(respBody)
		if err != nil {
			// Transform errors are our code's fault, not the provider's — no health update.
			lastErr = err
			attemptCount++
			if attemptCount >= maxAttempts {
				break
			}
			tried[provider.Name] = true
			provider = m.selectUntried(tier, sessionID, tried, optimizeMode)
			retriedCurrentProvider = false
			continue
		}

		// Extract usage for metrics and cost calculation.
		var resp libai.ChatCompletionResponse
		if err := json.Unmarshal(transformed, &resp); err == nil && resp.Usage != nil {
			RecordTokens(tierName, provider.Name, provider.ModelID,
				resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
			cost := provider.CostForTokens(resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
			RecordCost(tierName, provider.Name, provider.ModelID, cost)
			w.Header().Set("X-DIN-Cost", fmt.Sprintf("%.6f", cost))
		}

		setResponseHeaders(w, provider, tierName, sessionID, requestID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(transformed); err != nil {
			m.logger.Warn("failed to write response body", zap.Error(err))
		}

		RecordRequest(tierName, provider.Name, provider.ModelID, "200")
		return nil
	}

	// All attempts failed.
	RecordRequest(tierName, "unknown", "unknown", "502")
	errMsg := "all provider attempts failed"
	if lastErr != nil {
		errMsg = fmt.Sprintf("all provider attempts failed: %v", lastErr)
	}
	return writeErrorResponse(w, http.StatusBadGateway, errMsg, "upstream_error")
}

// selectUntried picks a provider that hasn't been tried yet.
func (m *DinAIMiddleware) selectUntried(tier *Tier, sessionID string, tried map[string]bool, optimizeMode string) *AIProvider {
	if len(tried) == 0 {
		return tier.SelectProvider(sessionID, optimizeMode)
	}

	// For retries, use optimization-mode-aware selection excluding tried providers.
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

	// Create a temporary tier for selection — no session hash for retries.
	tempTier := &Tier{Name: tier.Name, Providers: untried}
	return tempTier.SelectProvider("", optimizeMode)
}

// attemptNonStreaming makes a non-streaming request to a provider.
func (m *DinAIMiddleware) attemptNonStreaming(
	ctx context.Context,
	provider *AIProvider,
	adapter libai.ProviderAdapter,
	body []byte,
) ([]byte, int, http.Header, *AIProvider, error) {
	transformedBody, extraHeaders, err := adapter.TransformRequest(body)
	if err != nil {
		return nil, 0, nil, provider, fmt.Errorf("transform request: %w", err)
	}

	headers := make(map[string]string)
	for k, v := range provider.Headers {
		headers[k] = v
	}
	for k, v := range extraHeaders {
		headers[k] = v
	}
	headers["Content-Type"] = "application/json"

	respBody, statusCode, respHeaders, err := m.client.Post(ctx, provider.HttpUrl, headers, transformedBody)
	if err != nil {
		return nil, 0, nil, provider, fmt.Errorf("post: %w", err)
	}

	return respBody, statusCode, respHeaders, provider, nil
}

func isRetryableStatus(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode == http.StatusServiceUnavailable
}

func retryDelayFromHeaders(headers http.Header) (time.Duration, bool) {
	if headers == nil {
		return 0, false
	}
	retryAfterRaw := headers.Get("Retry-After")
	if retryAfterRaw == "" {
		return 0, false
	}
	delay, err := parseRetryAfter(retryAfterRaw, time.Now(), defaultRetryAfterCap)
	if err != nil {
		return 0, false
	}
	return delay, true
}

func shouldRetrySameProviderForStream(err error, alreadyRetried bool) bool {
	if alreadyRetried {
		return false
	}
	var streamErr *streamAttemptError
	if !errors.As(err, &streamErr) {
		return false
	}
	return isRetryableStatus(streamErr.statusCode)
}

func streamRetryAfter(err error) time.Duration {
	var streamErr *streamAttemptError
	if !errors.As(err, &streamErr) {
		return 0
	}
	return streamErr.retryAfter
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

// overwriteModel replaces the model field in the pre-parsed request fields and re-marshals.
func overwriteModel(rawFields map[string]json.RawMessage, model string) ([]byte, error) {
	modelJSON, err := json.Marshal(model)
	if err != nil {
		return nil, err
	}
	rawFields["model"] = modelJSON
	return json.Marshal(rawFields)
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
//
// NOTE: This runs at Caddyfile parse time (during UnmarshalCaddyfile), not at
// request time. Environment variable changes after Caddy starts require a
// graceful reload (`caddy reload`) to take effect.
func expandEnvVars(s string) string {
	searchFrom := 0
	for {
		idx := strings.Index(s[searchFrom:], "{env.")
		if idx == -1 {
			return s
		}
		start := searchFrom + idx
		end := strings.Index(s[start:], "}")
		if end == -1 {
			return s
		}
		end += start
		envKey := s[start+5 : end]
		envVal := os.Getenv(envKey)
		if envVal == "" {
			fmt.Fprintf(os.Stderr, "[WARN] din_ai: environment variable %q is not set\n", envKey)
		}
		s = s[:start] + envVal + s[end+1:]
		// Advance past the substituted value to prevent re-scanning it.
		searchFrom = start + len(envVal)
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
	body, err := json.Marshal(resp)
	if err != nil {
		escapedMsg := strings.ReplaceAll(message, `"`, `\"`)
		escapedType := strings.ReplaceAll(errType, `"`, `\"`)
		body = []byte(fmt.Sprintf(`{"error":{"message":"%s","type":"%s"}}`, escapedMsg, escapedType))
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write(body)
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
			if val <= 0 {
				return d.Errf("healthcheck_interval must be positive, got %d", val)
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
			if val <= 0 {
				return d.Errf("healthcheck_threshold must be positive, got %d", val)
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
			if val <= 0 {
				return d.Errf("request_attempt_count must be positive, got %d", val)
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

		if _, exists := m.Tiers[tierName]; exists {
			return d.Errf("duplicate tier name: '%s'", tierName)
		}

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
	seen := make(map[string]bool)
	for d.NextBlock(3) {
		providerName := d.Val()
		if seen[providerName] {
			return d.Errf("duplicate provider name '%s' in tier '%s'", providerName, tier.Name)
		}
		seen[providerName] = true
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

			case "cost":
				for d.NextBlock(5) {
					key := d.Val()
					if !d.NextArg() {
						return d.ArgErr()
					}
					val, err := strconv.ParseFloat(d.Val(), 64)
					if err != nil {
						return d.Errf("invalid cost value for '%s': %v", key, err)
					}
					if val <= 0 {
						return d.Errf("cost '%s' must be positive, got %f", key, val)
					}
					switch key {
					case "input_per_1m":
						provider.InputCostPer1M = val
					case "output_per_1m":
						provider.OutputCostPer1M = val
					default:
						return d.Errf("unknown cost option: %s (expected 'input_per_1m' or 'output_per_1m')", key)
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
		if provider.AdapterType != AdapterOpenAI && provider.AdapterType != AdapterAnthropic {
			return d.Errf("provider '%s' has unknown adapter type '%s' (must be '%s' or '%s')",
				providerName, provider.AdapterType, AdapterOpenAI, AdapterAnthropic)
		}
		if provider.InputCostPer1M <= 0 || provider.OutputCostPer1M <= 0 {
			return d.Errf("provider '%s' requires a cost block with both input_per_1m and output_per_1m", providerName)
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

// --- HTTP Clients ---

type defaultStreamingClient struct {
	client *http.Client
}

// newDefaultStreamingClient creates the HTTP client for AI requests (streaming and non-streaming).
// No timeout is set because streaming responses can run indefinitely; cancellation is handled
// via context (client disconnect or request timeout).
func newDefaultStreamingClient() *defaultStreamingClient {
	return &defaultStreamingClient{
		client: &http.Client{},
	}
}

// newHealthCheckClient creates an HTTP client with a short timeout for health checks.
// Health checks are simple ping requests that should complete quickly.
func newHealthCheckClient() *defaultStreamingClient {
	return &defaultStreamingClient{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *defaultStreamingClient) Post(ctx context.Context, url string, headers map[string]string, payload []byte) ([]byte, int, http.Header, error) {
	resp, err := c.PostStream(ctx, url, headers, payload)
	if err != nil {
		return nil, 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return body, resp.StatusCode, resp.Header.Clone(), err
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
