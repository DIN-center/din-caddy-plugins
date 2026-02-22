package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	libai "github.com/DIN-center/din-caddy-plugins/lib/ai"
	"go.uber.org/zap"
)

// runHealthChecks starts a goroutine that periodically health checks all providers.
// It stops when quit is closed, and cancels any in-flight health check requests.
func runHealthChecks(
	tiers map[string]*Tier,
	client libai.IStreamingHTTPClient,
	intervalSec int,
	logger *zap.Logger,
	quit <-chan struct{},
) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-quit
		cancel()
	}()

	ticker := time.NewTicker(time.Duration(intervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("stopping AI health checks")
			return
		case <-ticker.C:
			for _, tier := range tiers {
				for _, provider := range tier.Providers {
					go checkProvider(ctx, provider, client, logger)
				}
			}
		}
	}
}

// checkProvider performs a single health check against a provider.
func checkProvider(
	ctx context.Context,
	provider *AIProvider,
	client libai.IStreamingHTTPClient,
	logger *zap.Logger,
) {
	// Build health check request as a map to support per-provider overrides.
	reqMap := map[string]interface{}{
		"model":    provider.ModelID,
		"messages": []map[string]string{{"role": "user", "content": "ping"}},
		"stream":   false,
	}

	// Always set minimal token limit, then apply per-provider overrides.
	reqMap["max_tokens"] = 1
	for k, v := range provider.HealthCheckOverrides {
		reqMap[k] = v
	}
	// OpenAI rejects requests with both max_tokens and max_completion_tokens.
	if _, has := reqMap["max_completion_tokens"]; has {
		delete(reqMap, "max_tokens")
	}

	// Get the adapter to transform the request.
	adapter := getAdapterForType(provider.AdapterType)

	body, err := json.Marshal(reqMap)
	if err != nil {
		logger.Error("failed to marshal health check request",
			zap.String("provider", provider.Name),
			zap.Error(err))
		provider.MarkPingFailure()
		RecordHealthCheck(provider.Name, "0", "error")
		return
	}

	// Transform the request for the provider's API format.
	transformedBody, extraHeaders, err := adapter.TransformRequest(body)
	if err != nil {
		logger.Error("failed to transform health check request",
			zap.String("provider", provider.Name),
			zap.Error(err))
		provider.MarkPingFailure()
		RecordHealthCheck(provider.Name, "0", "error")
		return
	}

	// Build headers.
	headers := make(map[string]string)
	for k, v := range provider.Headers {
		headers[k] = v
	}
	for k, v := range extraHeaders {
		headers[k] = v
	}
	headers["Content-Type"] = "application/json"

	startTime := time.Now()

	respBody, statusCode, err := client.Post(ctx, provider.HttpUrl, headers, transformedBody)
	if err != nil {
		logger.Warn("health check failed",
			zap.String("provider", provider.Name),
			zap.Error(err))
		provider.MarkPingFailure()
		RecordHealthCheck(provider.Name, "0", provider.HealthStatus().String())
		return
	}

	ttft := time.Since(startTime)

	statusStr := http.StatusText(statusCode)
	if statusStr == "" {
		statusStr = "unknown"
	}

	switch {
	case statusCode == http.StatusOK:
		provider.MarkPingSuccess()
		provider.RecordTTFT(ttft)
		RecordHealthCheck(provider.Name, "200", provider.HealthStatus().String())
		logger.Debug("health check succeeded",
			zap.String("provider", provider.Name),
			zap.Duration("ttft", ttft))

	case statusCode == http.StatusTooManyRequests:
		// Rate limited — provider is alive but degraded.
		provider.MarkPingWarning()
		RecordHealthCheck(provider.Name, "429", provider.HealthStatus().String())
		logger.Warn("health check rate limited",
			zap.String("provider", provider.Name))

	default:
		provider.MarkPingFailure()
		RecordHealthCheck(provider.Name, fmt.Sprintf("%d", statusCode), provider.HealthStatus().String())
		logger.Warn("health check failed",
			zap.String("provider", provider.Name),
			zap.Int("status_code", statusCode),
			zap.String("body", truncate(string(respBody), 200)))
	}
}

// getAdapterForType returns the appropriate adapter for a provider type.
func getAdapterForType(adapterType string) libai.ProviderAdapter {
	switch adapterType {
	case AdapterAnthropic:
		return libai.NewAnthropicAdapter()
	default:
		return libai.NewOpenAIAdapter()
	}
}

// truncate limits a string to maxLen characters.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
