package prometheus

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// PrometheusClient is a struct that holds the prometheus client
type PrometheusClient struct {
	logger    *zap.Logger
	machineID string
}

// NewPrometheusClient returns a new prometheus client
func NewPrometheusClient(logger *zap.Logger) *PrometheusClient {
	return &PrometheusClient{
		logger:    logger,
		machineID: getMachineId(),
	}
}

// prometheus metric initialization
var (
	// Din Client Request Metrics
	DinRequestCount                *prometheus.CounterVec
	DinRequestDurationMilliseconds *prometheus.HistogramVec
	DinRequestBodyBytes            *prometheus.HistogramVec

	// Din Health Check Metrics
	DinHealthCheckCount    *prometheus.CounterVec
	DinProviderBlockNumber *prometheus.GaugeVec
)

// RegisterMetrics registers the prometheus metrics
func RegisterMetrics() {
	// Register request count metric for inbound din http requests
	DinRequestCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "din_http_request_count",
			Help: "Metric for counting the number of requests to the din http server",
		},
		[]string{"service", "method", "provider", "host_name", "response_status", "health_status", "machine_id"},
	)
	DinRequestDurationMilliseconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "din_http_request_duration_milliseconds",
			Help:    "Metric for measuring the duration of requests to the din http server",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "method", "provider", "host_name", "response_status", "health_status", "machine_id"},
	)

	DinRequestBodyBytes = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "din_http_request_body_bytes",
			Help:    "Metric for measuring the size of the request body in bytes",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "method", "provider", "host_name", "response_status", "health_status", "machine_id"},
	)

	DinProviderBlockNumber = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "din_http_provider_block_number",
			Help: "Metric for measuring the latest block number of the request",
		},
		[]string{"service", "provider", "machine_id"},
	)

	// Register health check count metric for din health checks
	DinHealthCheckCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "din_health_check_count",
			Help: "Metric for counting din health checks with service, provider, response_status and health_status",
		},
		[]string{"service", "provider", "response_status", "health_status", "machine_id"},
	)

	prometheus.MustRegister(DinRequestCount, DinHealthCheckCount, DinRequestDurationMilliseconds, DinRequestBodyBytes, DinProviderBlockNumber)
}

type PromRequestMetricData struct {
	Method         string
	Service        string
	Provider       string
	HostName       string
	ResponseStatus int
	HealthStatus   string
}

// HandleRequestMetrics increments prometheus metric based on request data passed in
func (p *PrometheusClient) HandleRequestMetrics(data *PromRequestMetricData, reqBodyBytes []byte, duration time.Duration) {
	// First extract method data from body
	// define struct to hold request data
	var requestBody struct {
		Method string `json:"method,omitempty"`
	}
	err := json.Unmarshal(reqBodyBytes, &requestBody)
	if err != nil {
		p.logger.Warn("Error decoding request body", zap.Error(err), zap.Int("response_status", http.StatusBadRequest))
	}
	var method string
	if requestBody.Method != "" {
		method = requestBody.Method
	}

	service := strings.TrimPrefix(data.Service, "/")
	status := strconv.Itoa(data.ResponseStatus)

	durationMS := duration.Milliseconds()

	reqBodyByteSize := len(reqBodyBytes)

	p.logger.Debug("Request metric data", zap.String("service", service), zap.String("method", method), zap.String("provider", data.Provider), zap.String("host_name", data.HostName), zap.String("response_status", status), zap.String("health_status", data.HealthStatus), zap.Int64("duration_milliseconds", durationMS), zap.Int("body_size", reqBodyByteSize), zap.String("machine_id", p.machineID))

	// Increment prometheus counter metric based on request data
	DinRequestCount.WithLabelValues(service, method, data.Provider, data.HostName, status, data.HealthStatus, p.machineID).Inc()

	// Observe prometheus histogram based on request duration and data
	DinRequestDurationMilliseconds.WithLabelValues(service, method, data.Provider, data.HostName, status, data.HealthStatus, p.machineID).Observe(float64(durationMS))

	// Observe prometheus histogram based on request body size and data
	DinRequestBodyBytes.WithLabelValues(service, method, data.Provider, data.HostName, status, data.HealthStatus, p.machineID).Observe(float64(reqBodyByteSize))
}

type PromLatestBlockMetricData struct {
	Service        string
	Provider       string
	ResponseStatus int
	HealthStatus   string
	BlockNumber    int64
}

// handleLatestBlockMetric increments prometheus metric based on latest block number health check data
func (p *PrometheusClient) HandleLatestBlockMetric(data *PromLatestBlockMetricData) {
	service := strings.TrimPrefix(data.Service, "/")
	status := strconv.Itoa(data.ResponseStatus)

	p.logger.Debug("Latest block metric data", zap.String("service", service), zap.String("provider", data.Provider), zap.String("response_status", status), zap.String("health_status", data.HealthStatus), zap.String("machine_id", p.machineID))

	// Increment prometheus metric based on request data
	DinHealthCheckCount.WithLabelValues(service, data.Provider, status, data.HealthStatus, p.machineID).Inc()

	// Set the latest block number for the provider
	DinProviderBlockNumber.WithLabelValues(service, data.Provider, p.machineID).Set(float64(data.BlockNumber))
}

// Util Functions

// getMachineId returns a unique string for the current running process
func getMachineId() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "UNKNOWN"
	}
	currentPid := os.Getpid()
	return fmt.Sprintf("@%s:%d", hostname, currentPid)
}
