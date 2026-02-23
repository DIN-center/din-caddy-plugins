package ai

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var metricsOnce sync.Once

const (
	DinAIRequestCountMetricName     = "din_ai_request_count"
	DinAITokensTotalMetricName      = "din_ai_tokens_total"
	DinAIHealthCheckCountMetricName = "din_ai_health_check_count"
	DinAIRequestDurationMetricName  = "din_ai_request_duration_milliseconds"
	DinAITTFTDurationMetricName     = "din_ai_ttft_duration_milliseconds"
	DinAICostTotalMetricName        = "din_ai_cost_total_usd"
)

var (
	DinAIRequestCount     *prometheus.CounterVec
	DinAITokensTotal      *prometheus.CounterVec
	DinAIHealthCheckCount *prometheus.CounterVec

	DinAICostTotal *prometheus.CounterVec

	// Registered but disabled by default due to cardinality concerns.
	DinAIRequestDuration *prometheus.HistogramVec
	DinAITTFTDuration    *prometheus.HistogramVec
)

// RegisterAIMetrics registers all AI-specific Prometheus metrics.
// Safe to call multiple times — registration only happens once.
func RegisterAIMetrics() {
	metricsOnce.Do(registerAIMetricsOnce)
}

func registerAIMetricsOnce() {
	DinAIRequestCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: DinAIRequestCountMetricName,
			Help: "Total number of AI API requests",
		},
		[]string{"tier", "provider", "model", "status_code"},
	)

	DinAITokensTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: DinAITokensTotalMetricName,
			Help: "Total tokens consumed by AI API requests",
		},
		[]string{"tier", "provider", "model", "token_type"},
	)

	DinAIHealthCheckCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: DinAIHealthCheckCountMetricName,
			Help: "Total number of AI provider health checks",
		},
		[]string{"provider", "status_code", "health_status"},
	)

	DinAICostTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: DinAICostTotalMetricName,
			Help: "Total estimated cost in USD of AI API requests",
		},
		[]string{"tier", "provider", "model"},
	)

	DinAIRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    DinAIRequestDurationMetricName,
			Help:    "AI API request duration in milliseconds (disabled by default)",
			Buckets: prometheus.ExponentialBuckets(100, 2, 10),
		},
		[]string{"tier", "provider", "model"},
	)

	DinAITTFTDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    DinAITTFTDurationMetricName,
			Help:    "AI provider time-to-first-token in milliseconds (disabled by default)",
			Buckets: prometheus.ExponentialBuckets(50, 2, 10),
		},
		[]string{"tier", "provider", "model"},
	)

	prometheus.MustRegister(
		DinAIRequestCount,
		DinAITokensTotal,
		DinAIHealthCheckCount,
		DinAICostTotal,
		DinAIRequestDuration,
		DinAITTFTDuration,
	)
}

// RecordRequest records an AI request metric.
func RecordRequest(tier, provider, model, statusCode string) {
	DinAIRequestCount.WithLabelValues(tier, provider, model, statusCode).Inc()
}

// RecordTokens records token usage metrics.
func RecordTokens(tier, provider, model string, promptTokens, completionTokens int) {
	if promptTokens > 0 {
		DinAITokensTotal.WithLabelValues(tier, provider, model, "prompt").Add(float64(promptTokens))
	}
	if completionTokens > 0 {
		DinAITokensTotal.WithLabelValues(tier, provider, model, "completion").Add(float64(completionTokens))
	}
}

// RecordCost records an estimated cost metric in USD.
func RecordCost(tier, provider, model string, costUSD float64) {
	if costUSD > 0 {
		DinAICostTotal.WithLabelValues(tier, provider, model).Add(costUSD)
	}
}

// RecordHealthCheck records a health check metric.
func RecordHealthCheck(provider, statusCode, healthStatus string) {
	DinAIHealthCheckCount.WithLabelValues(provider, statusCode, healthStatus).Inc()
}
