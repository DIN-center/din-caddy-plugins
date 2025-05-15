package prometheus

import (
	"strconv"
	"strings"
	"time"

	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	"github.com/davecgh/go-spew/spew"
	"github.com/prometheus/client_golang/prometheus"

	"go.uber.org/zap"
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
			Name: "din_http_request_count",
			Help: "Metric for counting the number of requests to the din http server",
		},
		[]string{"service", "method", "provider", "host_name", "response_status", "health_status", "machine_id", "environment"},
	)
	DinRequestDurationMilliseconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "din_http_request_duration_milliseconds",
			Help:    "Metric for measuring the duration of requests to the din http server",
			Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000, 30000, 60000}, // 1ms to 60s (1 minute)
		},
		[]string{"service", "method", "provider", "host_name", "response_status", "health_status", "machine_id", "environment"},
	)

	DinRequestBodyBytes = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "din_http_request_body_bytes",
			Help:    "Metric for measuring the size of the request body in bytes",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "method", "provider", "host_name", "response_status", "health_status", "machine_id", "environment"},
	)

	// Register health check count metric for din health checks
	DinProviderHealthCheckCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "din_health_check_count",
			Help: "Metric for counting din health checks with network, provider, response_status and health_status",
		},
		[]string{"service", "provider", "response_status", "health_status", "machine_id", "environment"},
	)

	DinProviderHealthCheckBlockNumber = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "din_health_check_block_number",
			Help: "Metric for storing the block number of the latest health check",
		},
		[]string{"service", "provider", "machine_id", "environment"},
	)

	prometheus.MustRegister(DinRequestCount, DinProviderHealthCheckCount, DinRequestDurationMilliseconds, DinRequestBodyBytes, DinProviderHealthCheckBlockNumber)

	// Register network level health check metrics
	DinNetworkHealthCheckCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "din_network_health_check_count",
			Help: "Metric for counting network-level health checks",
		},
		[]string{"service", "response_status", "machine_id", "environment"},
	)

	DinNetworkRequestHealthCheckDurationMilliseconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "din_network_request_health_check_duration_milliseconds",
			Help:    "Metric for measuring the duration of network-level health checks",
			Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000, 30000, 60000}, // 1ms to 60s (1 minute)
		},
		[]string{"service", "response_status", "machine_id", "environment"},
	)

	prometheus.MustRegister(DinNetworkHealthCheckCount, DinNetworkRequestHealthCheckDurationMilliseconds)
}

type PromRequestMetricData struct {
	Method         string
	Network        string
	Provider       string
	HostName       string
	ResponseStatus int
	HealthStatus   string
	Environment    string
}

// HandleRequestMetrics increments prometheus metric based on request data passed in
func (p *PrometheusClient) HandleRequestMetrics(data *PromRequestMetricData, duration time.Duration, requestBody *din_http.JSONRPCRequest) {
	spew.Dump(data)
	// First extract method data from body
	method := requestBody.Method
	network := strings.TrimPrefix(data.Network, "/")
	status := strconv.Itoa(data.ResponseStatus)

	durationMS := duration.Milliseconds()

	p.logger.Debug("Request metric data", zap.String("network", network), zap.String("method", method), zap.String("provider", data.Provider), zap.String("host_name", data.HostName), zap.String("response_status", status), zap.String("health_status", data.HealthStatus), zap.Int64("duration_milliseconds", durationMS), zap.String("environment", data.Environment))

	// Increment prometheus counter metric based on request data
	DinRequestCount.WithLabelValues(network, method, data.Provider, data.HostName, status, data.HealthStatus, p.machineID, data.Environment).Inc()

	// Observe prometheus histogram based on request duration and data
	DinRequestDurationMilliseconds.WithLabelValues(network, method, data.Provider, data.HostName, status, data.HealthStatus, p.machineID, data.Environment).Observe(float64(durationMS))
}

type PromHealthCheckMetricData struct {
	Network        string
	Provider       string
	ResponseStatus int
	HealthStatus   string
	BlockNumber    int64
	Environment    string
}

func (p *PrometheusClient) HandleHealthCheckMetric(data *PromHealthCheckMetricData) {
	network := strings.TrimPrefix(data.Network, "/")

	p.logger.Debug("Latest block metric data", zap.String("network", network), zap.String("provider", data.Provider), zap.String("health_status", data.HealthStatus), zap.String("environment", data.Environment))

	// Increment prometheus metric based on request data
	DinProviderHealthCheckCount.WithLabelValues(network, data.Provider, strconv.Itoa(data.ResponseStatus), data.HealthStatus, p.machineID, data.Environment).Inc()

	// Update prometheus metric based on block number
	DinProviderHealthCheckBlockNumber.WithLabelValues(network, data.Provider, p.machineID, data.Environment).Set(float64(data.BlockNumber))
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
	DinNetworkRequestHealthCheckDurationMilliseconds.WithLabelValues(network, status, p.machineID, data.Environment).Observe(float64(durationMS))
}
