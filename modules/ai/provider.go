package ai

import (
	"fmt"
	"net/url"
	"sync"
	"time"

	libai "github.com/DIN-center/din-caddy-plugins/lib/ai"
	"github.com/DIN-center/din-caddy-plugins/lib/health"
	libprovider "github.com/DIN-center/din-caddy-plugins/lib/provider"
	"go.uber.org/zap"
)

var _ libprovider.Provider = (*AIProvider)(nil)

// AIProvider represents a backend AI model provider with health tracking and TTFT metrics.
type AIProvider struct {
	Name       string            // human-readable name, e.g. "openai-gpt4o"
	ModelID    string            // model identifier sent to the backend, e.g. "gpt-4o"
	HttpUrl    string            // full URL including path, e.g. "https://api.openai.com/v1/chat/completions"
	Host       string            // parsed host for identification
	Path       string            // parsed path
	Headers              map[string]string      // static headers (e.g. Authorization)
	AdapterType          string                 // "openai" or "anthropic"
	HealthCheckOverrides map[string]interface{} // optional per-provider health check params (e.g. max_completion_tokens for reasoning models)
	InputCostPer1M       float64                // USD per 1M input tokens — REQUIRED, configured in Caddyfile
	OutputCostPer1M      float64                // USD per 1M output tokens — REQUIRED, configured in Caddyfile

	httpClient libai.IStreamingHTTPClient
	logger     *zap.Logger

	mu           sync.RWMutex
	healthStatus health.HealthStatus
	failures     int
	successes    int
	hcThreshold  int

	ttftWindow     []time.Duration
	ttftWindowSize int
}

// NewAIProvider creates a new AI provider from a URL string.
func NewAIProvider(name, urlStr string) (*AIProvider, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("URL scheme must be http or https, got %q", u.Scheme)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("URL must have a host")
	}
	return &AIProvider{
		Name:           name,
		HttpUrl:        urlStr,
		Host:           u.Host,
		Path:           u.Path,
		Headers:        make(map[string]string),
		healthStatus:   health.Healthy,
		hcThreshold:    DefaultHCThreshold,
		ttftWindowSize: DefaultTTFTWindowSize,
		ttftWindow:     make([]time.Duration, 0, DefaultTTFTWindowSize),
	}, nil
}

// HealthStatus returns the current health status (thread-safe).
func (p *AIProvider) HealthStatus() health.HealthStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.healthStatus
}

// IsHealthy returns true if the provider is in Healthy state.
func (p *AIProvider) IsHealthy() bool {
	return p.HealthStatus() == health.Healthy
}

// IsAvailable returns true if the provider is Healthy or Warning.
func (p *AIProvider) IsAvailable() bool {
	s := p.HealthStatus()
	return s == health.Healthy || s == health.Warning
}

// MarkPingFailure records a failed health check ping.
func (p *AIProvider) MarkPingFailure() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failures++
	p.successes = 0
	if p.failures > p.hcThreshold {
		p.healthStatus = health.Unhealthy
	}
}

// MarkPingWarning records a rate-limited or degraded health check response.
func (p *AIProvider) MarkPingWarning() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failures = 0
	p.successes = 0
	p.healthStatus = health.Warning
}

// MarkPingSuccess records a successful health check ping.
func (p *AIProvider) MarkPingSuccess() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.successes++
	p.failures = 0
	if p.healthStatus == health.Unhealthy && p.successes > p.hcThreshold {
		p.healthStatus = health.Healthy
	} else if p.healthStatus != health.Unhealthy {
		p.healthStatus = health.Healthy
	}
}

// RecordTTFT records a time-to-first-token measurement in the rolling window.
func (p *AIProvider) RecordTTFT(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ttftWindowSize <= 0 {
		return
	}
	if len(p.ttftWindow) >= p.ttftWindowSize {
		p.ttftWindow = p.ttftWindow[1:]
	}
	p.ttftWindow = append(p.ttftWindow, d)
}

// AvgTTFT returns the average TTFT from the rolling window.
// Returns 0 if no measurements have been recorded.
func (p *AIProvider) AvgTTFT() time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if len(p.ttftWindow) == 0 {
		return 0
	}
	var total time.Duration
	for _, d := range p.ttftWindow {
		total += d
	}
	return total / time.Duration(len(p.ttftWindow))
}

// TTFTCount returns the number of TTFT measurements recorded.
func (p *AIProvider) TTFTCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.ttftWindow)
}

// CostForTokens calculates the actual cost in USD for the given token counts.
func (p *AIProvider) CostForTokens(promptTokens, completionTokens int) float64 {
	inputCost := float64(promptTokens) * p.InputCostPer1M / 1_000_000
	outputCost := float64(completionTokens) * p.OutputCostPer1M / 1_000_000
	return inputCost + outputCost
}

// GetName returns the provider's display name.
func (p *AIProvider) GetName() string { return p.Name }

// GetURL returns the provider's HTTP URL.
func (p *AIProvider) GetURL() string { return p.HttpUrl }

// GetHeaders returns the provider's static request headers.
func (p *AIProvider) GetHeaders() map[string]string { return p.Headers }

// GetHealthStatus returns the current health status (thread-safe).
func (p *AIProvider) GetHealthStatus() health.HealthStatus { return p.HealthStatus() }
