package prometheus

import (
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"

	"go.uber.org/zap"
)

const (
	DinRequestCountMetricName                      = "din_http_request_count"
	DinRequestDurationMetricName                   = "din_http_request_duration_milliseconds"
	DinRequestBodyBytesMetricName                  = "din_http_request_body_bytes"
	DinHealthCheckCountMetricName                  = "din_health_check_count"
	DinHealthCheckBlockNumberMetricName            = "din_health_check_block_number"
	DinNetworkHealthCheckCountMetricName           = "din_network_health_check_count"
	DinNetworkRequestHealthCheckDurationMetricName = "din_network_request_health_check_duration_milliseconds"
)

// PrometheusClient is a struct that holds the prometheus client
type PrometheusClient struct {
	logger    *logger.LoggerClient
	machineID string
}

// NewPrometheusClient returns a new prometheus client
func NewPrometheusClient(logger *logger.LoggerClient, machineId string) *PrometheusClient {
	return &PrometheusClient{
		logger:    logger,
		machineID: machineId,
	}
}

// prometheus metric initialization
var (
	// Din Client Request Metrics
	DinRequestCount                *prometheus.CounterVec
	DinRequestDurationMilliseconds *prometheus.HistogramVec
	DinRequestBodyBytes            *prometheus.HistogramVec

	// Din Provider LevelHealth Check Metrics
	DinProviderHealthCheckCount       *prometheus.CounterVec
	DinProviderHealthCheckBlockNumber *prometheus.GaugeVec

	// Din Network Level Health Check Metrics
	DinNetworkHealthCheckCount                       *prometheus.CounterVec
	DinNetworkRequestHealthCheckDurationMilliseconds *prometheus.HistogramVec
)

// RegisterMetrics registers the prometheus metrics
func RegisterMetrics() {
	// Register request count metric for inbound din http requests
	DinRequestCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: DinRequestCountMetricName,
			Help: "Metric for counting the number of requests to the din http server",
		},
		[]string{"service", "method", "provider", "provider_name", "api_key", "host_name", "response_status", "health_status", "machine_id", "environment"},
	)
	DinRequestDurationMilliseconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: DinRequestDurationMetricName,
			Help: "Metric for measuring the duration of requests to the din http server",
			Buckets: []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 12,
				15, 20, 25, 30, 40, 50, 75, 100, 200,
				300, 500, 750, 1000, 2000, 3000, 5000,
				7000, 10000, 20000, 30000, 40000, 50000,
				60000, 70000, 80000, 90000, 100000}, //
		},
		[]string{"service", "method", "provider", "provider_name", "host_name", "response_status", "health_status", "machine_id", "environment"},
	)

	DinRequestBodyBytes = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    DinRequestBodyBytesMetricName,
			Help:    "Metric for measuring the size of the request body in bytes",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "method", "provider", "provider_name", "host_name", "response_status", "health_status", "machine_id", "environment"},
	)

	// Register health check count metric for din health checks
	DinProviderHealthCheckCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: DinHealthCheckCountMetricName,
			Help: "Metric for counting din health checks with network, provider, response_status and health_status",
		},
		[]string{"service", "provider", "provider_name", "response_status", "health_status", "priority", "machine_id", "environment"},
	)

	DinProviderHealthCheckBlockNumber = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: DinHealthCheckBlockNumberMetricName,
			Help: "Metric for storing the block number of the latest health check",
		},
		[]string{"service", "provider", "provider_name", "machine_id", "environment"},
	)

	prometheus.MustRegister(DinRequestCount, DinProviderHealthCheckCount, DinRequestDurationMilliseconds, DinRequestBodyBytes, DinProviderHealthCheckBlockNumber)

	// Register network level health check metrics
	DinNetworkHealthCheckCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: DinNetworkHealthCheckCountMetricName,
			Help: "Metric for counting network-level health checks",
		},
		[]string{"service", "response_status", "machine_id", "environment"},
	)

	DinNetworkRequestHealthCheckDurationMilliseconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: DinNetworkRequestHealthCheckDurationMetricName,
			Help: "Metric for measuring the duration of network-level health checks",
			Buckets: []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 12,
				15, 20, 25, 30, 40, 50, 75, 100, 200,
				300, 500, 750, 1000, 2000, 3000, 5000,
				7000, 10000, 20000, 30000, 40000, 50000,
				60000, 70000, 80000, 90000, 100000}, //
		},
		[]string{"service", "response_status", "machine_id", "environment"},
	)

	prometheus.MustRegister(DinNetworkHealthCheckCount, DinNetworkRequestHealthCheckDurationMilliseconds)
}

type PromRequestMetricData struct {
	Method         string
	Network        string
	Provider       string
	ProviderName   string
	ApiKey         string
	HostName       string
	ResponseStatus int
	HealthStatus   string
	Priority       int
	Environment    string
}

// HandleRequestMetrics increments prometheus metric based on request data passed in
func (p *PrometheusClient) HandleRequestMetrics(data *PromRequestMetricData, duration time.Duration, requestBody *din_http.JSONRPCRequest) {
	// Use method from data struct instead of requestBody to handle both RPC and REST requests
	method := data.Method
	network := strings.TrimPrefix(data.Network, "/")
	status := strconv.Itoa(data.ResponseStatus)

	durationMS := duration.Milliseconds()

	p.logger.Debug("Request metric data", zap.String("network", network), zap.String("method", method), zap.String("provider", data.Provider), zap.String("provider_name", data.ProviderName), zap.String("host_name", data.HostName), zap.String("response_status", status), zap.String("health_status", data.HealthStatus), zap.Int("priority", data.Priority), zap.Int64("duration_milliseconds", durationMS), zap.String("environment", data.Environment))

	// Increment prometheus counter metric based on request data
	DinRequestCount.WithLabelValues(network, method, data.Provider, data.ProviderName, data.ApiKey, data.HostName, status, data.HealthStatus, p.machineID, data.Environment).Inc()

	// Sample 25% of requests for histogram metrics to reduce costs (histograms are expensive)
	if rand.Intn(4) == 0 {
		DinRequestDurationMilliseconds.WithLabelValues(network, method, data.Provider, data.ProviderName, data.HostName, status, data.HealthStatus, p.machineID, data.Environment).Observe(float64(durationMS))
	}
}

type PromHealthCheckMetricData struct {
	Network        string
	Provider       string
	ProviderName   string
	ResponseStatus int
	HealthStatus   string
	BlockNumber    int64
	Priority       int
	Environment    string
}

func (p *PrometheusClient) HandleHealthCheckMetric(data *PromHealthCheckMetricData) {
	network := strings.TrimPrefix(data.Network, "/")
	priority := strconv.Itoa(data.Priority)

	p.logger.Debug("Latest block metric data", zap.String("network", network), zap.String("provider", data.Provider), zap.String("provider_name", data.ProviderName), zap.String("health_status", data.HealthStatus), zap.Int("priority", data.Priority), zap.String("environment", data.Environment))

	// Increment prometheus metric based on request data
	DinProviderHealthCheckCount.WithLabelValues(network, data.Provider, data.ProviderName, strconv.Itoa(data.ResponseStatus), data.HealthStatus, priority, p.machineID, data.Environment).Inc()

	// Update prometheus metric based on block number
	DinProviderHealthCheckBlockNumber.WithLabelValues(network, data.Provider, data.ProviderName, p.machineID, data.Environment).Set(float64(data.BlockNumber))
}

// PromNetworkHealthCheckMetricData holds data for network level health check metrics
type PromNetworkHealthCheckMetricData struct {
	Network        string
	ResponseStatus int
	Duration       time.Duration
	Environment    string
}

// HandleNetworkHealthCheckMetric increments prometheus metrics for network level health checks
func (p *PrometheusClient) HandleNetworkHealthCheckMetric(data *PromNetworkHealthCheckMetricData) {
	network := strings.TrimPrefix(data.Network, "/")
	status := strconv.Itoa(data.ResponseStatus)
	durationMS := data.Duration.Milliseconds()

	p.logger.Debug("Network health check metric data",
		zap.String("network", network),
		zap.String("response_status", status),
		zap.Int64("duration_milliseconds", durationMS),
		zap.String("machine_id", p.machineID),
		zap.String("environment", data.Environment),
	)

	DinNetworkHealthCheckCount.WithLabelValues(network, status, p.machineID, data.Environment).Inc()

	// Sample 50% of health checks for histogram metrics to reduce costs
	if rand.Intn(2) == 0 {
		DinNetworkRequestHealthCheckDurationMilliseconds.WithLabelValues(network, status, p.machineID, data.Environment).Observe(float64(durationMS))
	}
}
