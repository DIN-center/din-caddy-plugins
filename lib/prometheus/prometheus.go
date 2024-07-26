package prometheus

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// PrometheusClient is a struct that holds the prometheus client
type PrometheusClient struct{}

// NewPrometheusClient returns a new prometheus client
func NewPrometheusClient() *PrometheusClient {
	return &PrometheusClient{}
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
			Help: "Metric for counting din http requests with service, method, provider, host_name, res_status, res_latency, health_status, and block_number labels",
		},
		[]string{"service", "method", "provider", "host_name", "res_status", "res_latency", "health_status", "block_number"},
	)

	DinHealthCheckCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "din_health_check_count",
			Help: "Metric for counting din health checks with service, provider, res_status, res_latency, and block_number labels",
		},
		[]string{"service", "provider", "res_status", "res_latency", "block_number"},
	)

	prometheus.MustRegister(DinRequestCount, DinHealthCheckCount)
}

type PromRequestMetricData struct {
	Method       string
	Service      string
	Provider     string
	HostName     string
	ResStatus    int
	ResLatency   time.Duration
	HealthStatus string
	BlockNumber  string
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
	latency := strconv.FormatInt(data.ResLatency.Milliseconds(), 10)
	status := strconv.Itoa(data.ResStatus)

	// Increment prometheus metric based on request data
	DinRequestCount.WithLabelValues(service, method, data.Provider, data.HostName, status, latency, data.HealthStatus, data.BlockNumber).Inc()
}

type PromLatestBlockMetricData struct {
	Service     string
	Provider    string
	ResStatus   int
	ResLatency  time.Duration
	BlockNumber string
}

// handleLatestBlockMetric increments prometheus metric based on latest block number health check data
func (p *PrometheusClient) HandleLatestBlockMetric(data *PromLatestBlockMetricData) {
	service := strings.TrimPrefix(data.Service, "/")
	latency := strconv.FormatInt(data.ResLatency.Milliseconds(), 10)
	status := strconv.Itoa(data.ResStatus)

	// Increment prometheus metric based on request data
	DinHealthCheckCount.WithLabelValues(service, data.Provider, status, latency, data.BlockNumber).Inc()
}
