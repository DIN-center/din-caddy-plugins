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

// healthCheckRequest is the minimal request sent to providers for health checking.
var healthCheckRequest = libai.ChatCompletionRequest{
	Model: "", // will be overwritten per provider
	Messages: []libai.ChatMessage{
		{Role: "user", Content: "ping"},
	},
	MaxTokens: intPtr(1),
	Stream:    false,
}

func intPtr(i int) *int { return &i }

// runHealthChecks starts a goroutine that periodically health checks all providers.
// It stops when quit is closed.
func runHealthChecks(
	tiers map[string]*Tier,
	client libai.IStreamingHTTPClient,
	intervalSec int,
	machineID string,
	logger *zap.Logger,
	quit <-chan struct{},
) {
	ticker := time.NewTicker(time.Duration(intervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-quit:
			logger.Info("stopping AI health checks")
			return
		case <-ticker.C:
			for _, tier := range tiers {
				for _, provider := range tier.Providers {
					go checkProvider(provider, client, machineID, logger)
				}
			}
		}
	}
}

// checkProvider performs a single health check against a provider.
func checkProvider(
	provider *AIProvider,
	client libai.IStreamingHTTPClient,
	machineID string,
	logger *zap.Logger,
) {
	// Build health check request with provider's model.
	req := healthCheckRequest
	req.Model = provider.ModelID

	// Get the adapter to transform the request.
	adapter := getAdapterForType(provider.AdapterType)

	body, err := json.Marshal(req)
	if err != nil {
		logger.Error("failed to marshal health check request",
			zap.String("provider", provider.Name),
			zap.Error(err))
		provider.MarkPingFailure()
		RecordHealthCheck(provider.Name, "0", "error", machineID)
		return
	}

	// Transform the request for the provider's API format.
	transformedBody, extraHeaders, err := adapter.TransformRequest(body)
	if err != nil {
		logger.Error("failed to transform health check request",
			zap.String("provider", provider.Name),
			zap.Error(err))
		provider.MarkPingFailure()
		RecordHealthCheck(provider.Name, "0", "error", machineID)
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

	respBody, statusCode, err := client.Post(context.Background(), provider.HttpUrl, headers, transformedBody)
	if err != nil {
		logger.Warn("health check failed",
			zap.String("provider", provider.Name),
			zap.Error(err))
		provider.MarkPingFailure()
		RecordHealthCheck(provider.Name, "0", provider.HealthStatus().String(), machineID)
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
		RecordHealthCheck(provider.Name, "200", provider.HealthStatus().String(), machineID)
		logger.Debug("health check succeeded",
			zap.String("provider", provider.Name),
			zap.Duration("ttft", ttft))

	case statusCode == http.StatusTooManyRequests:
		// Rate limited — provider is alive but degraded.
		provider.MarkPingWarning()
		RecordHealthCheck(provider.Name, "429", provider.HealthStatus().String(), machineID)
		logger.Warn("health check rate limited",
			zap.String("provider", provider.Name))

	default:
		provider.MarkPingFailure()
		RecordHealthCheck(provider.Name, fmt.Sprintf("%d", statusCode), provider.HealthStatus().String(), machineID)
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
