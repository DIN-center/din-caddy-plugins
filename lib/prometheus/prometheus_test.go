package prometheus

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	"github.com/DIN-center/din-caddy-plugins/lib/utils"
)

func TestMain(m *testing.M) {
	RegisterMetrics()
}

func TestHandleRequestMetric(t *testing.T) {

	// Initialize the prometheus client
	client := NewPrometheusClient(logger.NewLoggerClient(zap.NewNop(), utils.Environment("test")), "test-machine-id")

	// Create a new registry and register our metric
	registry := prometheus.NewRegistry()
	registry.MustRegister(DinRequestCount, DinRequestDurationMilliseconds, DinRequestBodyBytes)

	tests := []struct {
		name           string
		requestBody    *din_http.JSONRPCRequest
		duration       time.Duration
		data           *PromRequestMetricData
		expectedLabels map[string]string
		expectedValue  float64
	}{
		{
			name:        "Valid JSON",
			requestBody: &din_http.JSONRPCRequest{Method: "eth_getBlockByNumber"},
			duration:    1 * time.Second,
			data: &PromRequestMetricData{
				Network:        "/ethereum",
				Provider:       "infura",
				ProviderName:   "infura",
				ApiKey:         "abc123",
				HostName:       "node1",
				ResponseStatus: 200,
				HealthStatus:   "healthy",
				Environment:    "test",
			},
			expectedLabels: map[string]string{
				"service":         "ethereum",
				"method":          "eth_getBlockByNumber",
				"provider":        "infura",
				"provider_name":   "infura",
				"api_key":         "abc123",
				"host_name":       "node1",
				"response_status": "200",
				"health_status":   "healthy",
				"machine_id":      client.machineID,
				"environment":     "test",
			},
			expectedValue: 1,
		},
		{
			name:        "Invalid JSON (simulated by empty method)",
			requestBody: &din_http.JSONRPCRequest{Method: ""},
			duration:    1 * time.Second,
			data: &PromRequestMetricData{
				Network:        "/ethereum",
				Provider:       "infura",
				ProviderName:   "infura",
				ApiKey:         "abc123",
				HostName:       "node1",
				ResponseStatus: 200,
				HealthStatus:   "healthy",
				Environment:    "test",
			},
			expectedLabels: map[string]string{
				"service":         "ethereum",
				"method":          "",
				"provider":        "infura",
				"provider_name":   "infura",
				"api_key":         "abc123",
				"host_name":       "node1",
				"response_status": "200",
				"health_status":   "healthy",
				"machine_id":      client.machineID,
				"environment":     "test",
			},
			expectedValue: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call the function with the new signature
			client.HandleRequestMetrics(tt.data, tt.duration, tt.requestBody)

			// Use  testutil to check if the metric exists with the expected labels and value
			_, err := registry.Gather()
			assert.NoError(t, err)

			metric := testutil.ToFloat64(DinRequestCount.WithLabelValues(
				tt.expectedLabels["service"],
				tt.expectedLabels["method"],
				tt.expectedLabels["provider"],
				tt.expectedLabels["provider_name"],
				tt.expectedLabels["host_name"],
				tt.expectedLabels["response_status"],
				tt.expectedLabels["health_status"],
				tt.expectedLabels["machine_id"],
				tt.expectedLabels["environment"],
			))

			assert.Equal(t, tt.expectedValue, metric, "Metric should be incremented once")
		})
	}
}

func TestHandleHealthCheckMetric(t *testing.T) {
	// Initialize the prometheus client
	client := NewPrometheusClient(logger.NewLoggerClient(zap.NewNop(), utils.Environment("test")), "test-machine-id")

	// Create a new registry and register our metric
	registry := prometheus.NewRegistry()
	registry.MustRegister(DinProviderHealthCheckCount)

	tests := []struct {
		name           string
		data           *PromHealthCheckMetricData
		expectedLabels map[string]string
	}{
		{
			name: "Valid Data",
			data: &PromHealthCheckMetricData{
				Network:        "/ethereum",
				Provider:       "infura",
				ProviderName:   "infura",
				ResponseStatus: 200,
				HealthStatus:   "healthy",
				Environment:    "test",
			},
			expectedLabels: map[string]string{
				"service":         "ethereum",
				"provider":        "infura",
				"provider_name":   "infura",
				"response_status": "200",
				"health_status":   "healthy",
				"machine_id":      client.machineID,
				"environment":     "test",
			},
		},
		{
			name: "Invalid Data",
			data: &PromHealthCheckMetricData{
				Network:        "/ethereum",
				Provider:       "infura",
				ProviderName:   "infura",
				ResponseStatus: 500,
				HealthStatus:   "unhealthy",
				Environment:    "test",
			},
			expectedLabels: map[string]string{
				"service":         "ethereum",
				"provider":        "infura",
				"provider_name":   "infura",
				"response_status": "500",
				"health_status":   "unhealthy",
				"machine_id":      client.machineID,
				"environment":     "test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call the function
			client.HandleHealthCheckMetric(tt.data)

			// Use testutil to check if the metric exists with the expected labels and value
			_, err := registry.Gather()
			assert.NoError(t, err)

			metric := testutil.ToFloat64(DinProviderHealthCheckCount.WithLabelValues(
				tt.expectedLabels["service"],
				tt.expectedLabels["provider"],
				tt.expectedLabels["provider_name"],
				tt.expectedLabels["response_status"],
				tt.expectedLabels["health_status"],
				tt.expectedLabels["machine_id"],
				tt.expectedLabels["environment"],
			))

			assert.Equal(t, float64(1), metric, "Metric should be incremented once")
		})
	}
}

func TestHandleNetworkHealthCheckMetric(t *testing.T) {
	// Initialize the prometheus client
	client := NewPrometheusClient(logger.NewLoggerClient(zap.NewNop(), utils.Environment("test")), "test-machine-id")

	// Create a new registry and register our metrics
	registry := prometheus.NewRegistry()
	registry.MustRegister(DinNetworkHealthCheckCount, DinNetworkRequestHealthCheckDurationMilliseconds)

	tests := []struct {
		name             string
		data             *PromNetworkHealthCheckMetricData
		expectedLabels   map[string]string
		expectedCount    float64
		expectedDuration float64 // in milliseconds
	}{
		{
			name: "Valid Data - Success",
			data: &PromNetworkHealthCheckMetricData{
				Network:        "/ethereum",
				ResponseStatus: 200,
				Duration:       100 * time.Millisecond,
				Environment:    "test",
			},
			expectedLabels: map[string]string{
				"network":         "ethereum",
				"response_status": "200",
				"machine_id":      client.machineID,
				"environment":     "test",
			},
			expectedCount:    1,
			expectedDuration: 100,
		},
		{
			name: "Valid Data - Error Status",
			data: &PromNetworkHealthCheckMetricData{
				Network:        "/polygon",
				ResponseStatus: 503,
				Duration:       50 * time.Millisecond,
				Environment:    "prod",
			},
			expectedLabels: map[string]string{
				"network":         "polygon",
				"response_status": "503",
				"machine_id":      client.machineID,
				"environment":     "prod",
			},
			expectedCount:    1,
			expectedDuration: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset metrics before each test run to ensure isolation
			DinNetworkHealthCheckCount.Reset()
			DinNetworkRequestHealthCheckDurationMilliseconds.Reset()

			// Call the function
			client.HandleNetworkHealthCheckMetric(tt.data)

			// Use testutil to check if the metric exists with the expected labels and value
			_, err := registry.Gather() // This is more for ensuring the registry is gatherable
			assert.NoError(t, err)

			// Check Counter
			countMetric := testutil.ToFloat64(DinNetworkHealthCheckCount.WithLabelValues(
				tt.expectedLabels["network"],
				tt.expectedLabels["response_status"],
				tt.expectedLabels["machine_id"],
				tt.expectedLabels["environment"],
			))
			assert.Equal(t, tt.expectedCount, countMetric, "Counter metric should be incremented as expected")

			// Check Histogram
			var dtoMetric dto.Metric
			histogram, err := DinNetworkRequestHealthCheckDurationMilliseconds.GetMetricWithLabelValues(
				tt.expectedLabels["network"],
				tt.expectedLabels["response_status"],
				tt.expectedLabels["machine_id"],
				tt.expectedLabels["environment"],
			)
			assert.NoError(t, err, "Error getting histogram metric")
			if hist, ok := histogram.(prometheus.Histogram); ok {
				err = hist.Write(&dtoMetric)
				assert.NoError(t, err, "Error writing histogram to DTO")
				assert.Equal(t, tt.expectedDuration, *dtoMetric.Histogram.SampleSum, "Duration metric sum should be as expected")
			} else {
				t.Fatalf("Expected prometheus.Histogram, got %T", histogram)
			}
		})
	}
}
