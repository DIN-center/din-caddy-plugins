package prometheus

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// PrometheusClient is a struct that holds the prometheus client
type PrometheusClient struct {
	logger *zap.Logger
}

// NewPrometheusClient returns a new prometheus client
func NewPrometheusClient(logger *zap.Logger) *PrometheusClient {
	return &PrometheusClient{
		logger: logger,
	}
}

// prometheus metric initialization
var (
	DinRequestCount     *prometheus.CounterVec
	DinHealthCheckCount *prometheus.CounterVec
)

// RegisterMetrics registers the prometheus metrics
func RegisterMetrics() {
	// Register request count metric for inbound din http requests
	DinRequestCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "din_http_request_count",
			Help: "Metric for counting the number of requests to the din http server",
		},
		[]string{"service", "method", "provider", "host_name", "response_status", "health_status"},
	)

	DinHealthCheckCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "din_health_check_count",
			Help: "Metric for counting din health checks with service, provider, response_status and health_status",
		},
		[]string{"service", "provider", "response_status", "health_status"},
	)

	prometheus.MustRegister(DinRequestCount, DinHealthCheckCount)
}

type PromRequestMetricData struct {
	Method         string
	Service        string
	Provider       string
	HostName       string
	ResponseStatus int
	HealthStatus   string
}

// handleRequestMetric increments prometheus metric based on request data passed in
func (p *PrometheusClient) HandleRequestMetric(reqBodyBytes []byte, data *PromRequestMetricData) {
	// First extract method data from body
	// define struct to hold request data
	var requestBody struct {
		Method string `json:"method,omitempty"`
	}
	err := json.Unmarshal(reqBodyBytes, &requestBody)
	if err != nil {
		fmt.Printf("Error decoding request body: %v", http.StatusBadRequest)
	}
	var method string
	if requestBody.Method != "" {
		method = requestBody.Method
	}

	service := strings.TrimPrefix(data.Service, "/")
	status := strconv.Itoa(data.ResponseStatus)

	p.logger.Debug("Request metric data", zap.String("service", service), zap.String("method", method), zap.String("provider", data.Provider), zap.String("host_name", data.HostName), zap.String("status", status), zap.String("health_status", data.HealthStatus))

	// Increment prometheus metric based on request data
	DinRequestCount.WithLabelValues(service, method, data.Provider, data.HostName, status, data.HealthStatus).Inc()
}

type PromLatestBlockMetricData struct {
	Service        string
	Provider       string
	ResponseStatus int
	HealthStatus   string
}

// handleLatestBlockMetric increments prometheus metric based on latest block number health check data
func (p *PrometheusClient) HandleLatestBlockMetric(data *PromLatestBlockMetricData) {
	service := strings.TrimPrefix(data.Service, "/")
	status := strconv.Itoa(data.ResponseStatus)

	p.logger.Debug("Latest block metric data", zap.String("service", service), zap.String("provider", data.Provider), zap.String("status", status))

	// Increment prometheus metric based on request data
	DinHealthCheckCount.WithLabelValues(service, data.Provider, status, data.HealthStatus).Inc()
}
