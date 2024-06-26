package prometheus

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/davecgh/go-spew/spew"
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
			Help: "Metric for counting din http requests with service, method, provider, and host_name labels",
		},
		[]string{"service", "method", "provider", "host_name", "res_status"},
	)
	prometheus.MustRegister(DinRequestCount)
}

type PromRequestMetricData struct {
	Method    string
	Service   string
	Provider  string
	HostName  string
	ResStatus int
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
	if requestBody.Method != "" {
		data.Method = requestBody.Method
	}

	service := strings.TrimPrefix(data.Service, "/")

	spew.Dump(data)

	// Increment prometheus metric based on request data
	DinRequestCount.WithLabelValues(service, data.Method, data.Provider, data.HostName, strconv.Itoa(data.ResStatus)).Inc()
}
