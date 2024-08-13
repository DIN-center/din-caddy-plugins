package prometheus

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

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
	DinRequestCount *prometheus.CounterVec
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
	prometheus.MustRegister(DinRequestCount)
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

	// Increment prometheus metric based on request data
	DinRequestCount.WithLabelValues(service, method, data.Provider, data.HostName, status, data.HealthStatus).Inc()
}
