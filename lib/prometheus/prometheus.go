package prometheus

import "github.com/prometheus/client_golang/prometheus"

// prometheus metric initialization
var (
	DinRequestCount *prometheus.CounterVec
)

// RegisterMetrics registers the prometheus metrics
func RegisterMetrics() {
	// Register request count metric for inbound din http requests
	DinRequestCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "din_http_request_count",
			Help: "Metric for counting din http requests with service, method, and provider labels",
		},
		[]string{"service", "method", "provider"},
	)
	prometheus.MustRegister(DinRequestCount)
}
