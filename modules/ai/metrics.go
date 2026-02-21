package ai

import "github.com/prometheus/client_golang/prometheus"

const (
	DinAIRequestCountMetricName     = "din_ai_request_count"
	DinAITokensTotalMetricName      = "din_ai_tokens_total"
	DinAIHealthCheckCountMetricName = "din_ai_health_check_count"
	DinAIRequestDurationMetricName  = "din_ai_request_duration_milliseconds"
	DinAITTFTDurationMetricName     = "din_ai_ttft_duration_milliseconds"
)

var (
	DinAIRequestCount     *prometheus.CounterVec
	DinAITokensTotal      *prometheus.CounterVec
	DinAIHealthCheckCount *prometheus.CounterVec

	// Registered but disabled by default due to cardinality concerns.
	DinAIRequestDuration *prometheus.HistogramVec
	DinAITTFTDuration    *prometheus.HistogramVec
)

// RegisterAIMetrics registers all AI-specific Prometheus metrics.
func RegisterAIMetrics() {
	DinAIRequestCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: DinAIRequestCountMetricName,
			Help: "Total number of AI API requests",
		},
		[]string{"tier", "provider", "model", "status_code", "machine_id"},
	)

	DinAITokensTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: DinAITokensTotalMetricName,
			Help: "Total tokens consumed by AI API requests",
		},
		[]string{"tier", "provider", "model", "token_type", "machine_id"},
	)

	DinAIHealthCheckCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: DinAIHealthCheckCountMetricName,
			Help: "Total number of AI provider health checks",
		},
		[]string{"provider", "status_code", "health_status", "machine_id"},
	)

	DinAIRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    DinAIRequestDurationMetricName,
			Help:    "AI API request duration in milliseconds (disabled by default)",
			Buckets: prometheus.ExponentialBuckets(100, 2, 10),
		},
		[]string{"tier", "provider", "model", "machine_id"},
	)

	DinAITTFTDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    DinAITTFTDurationMetricName,
			Help:    "AI provider time-to-first-token in milliseconds (disabled by default)",
			Buckets: prometheus.ExponentialBuckets(50, 2, 10),
		},
		[]string{"tier", "provider", "model", "machine_id"},
	)

	prometheus.MustRegister(
		DinAIRequestCount,
		DinAITokensTotal,
		DinAIHealthCheckCount,
		DinAIRequestDuration,
		DinAITTFTDuration,
	)
}

// RecordRequest records an AI request metric.
func RecordRequest(tier, provider, model, statusCode, machineID string) {
	DinAIRequestCount.WithLabelValues(tier, provider, model, statusCode, machineID).Inc()
}

// RecordTokens records token usage metrics.
func RecordTokens(tier, provider, model, machineID string, promptTokens, completionTokens int) {
	if promptTokens > 0 {
		DinAITokensTotal.WithLabelValues(tier, provider, model, "prompt", machineID).Add(float64(promptTokens))
	}
	if completionTokens > 0 {
		DinAITokensTotal.WithLabelValues(tier, provider, model, "completion", machineID).Add(float64(completionTokens))
	}
}

// RecordHealthCheck records a health check metric.
func RecordHealthCheck(provider, statusCode, healthStatus, machineID string) {
	DinAIHealthCheckCount.WithLabelValues(provider, statusCode, healthStatus, machineID).Inc()
}
