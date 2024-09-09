package prometheus

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestMain(m *testing.M) {
	RegisterMetrics()
}

func TestHandleRequestMetric(t *testing.T) {

	// Initialize the prometheus client
	client := NewPrometheusClient(zap.NewNop(), "test-machine-id")

	// Create a new registry and register our metric
	registry := prometheus.NewRegistry()
	registry.MustRegister(DinRequestCount, DinRequestDurationMilliseconds, DinRequestBodyBytes)

	tests := []struct {
		name           string
		reqBodyBytes   []byte
		duration       time.Duration
		data           *PromRequestMetricData
		machineID      string
		expectedLabels map[string]string
		expectedValue  float64
	}{
		{
			name:         "Valid JSON",
			reqBodyBytes: []byte(`{"method": "eth_getBlockByNumber"}`),
			duration:     1 * time.Second,
			data: &PromRequestMetricData{
				Method:         "POST",
				Service:        "/ethereum",
				Provider:       "infura",
				HostName:       "node1",
				ResponseStatus: 200,
				HealthStatus:   "healthy",
			},
			expectedLabels: map[string]string{
				"service":         "ethereum",
				"method":          "eth_getBlockByNumber",
				"provider":        "infura",
				"host_name":       "node1",
				"response_status": "200",
				"health_status":   "healthy",
				"machine_id":      client.machineID,
			},
			expectedValue: 1,
		},
		{
			name:         "Invalid JSON",
			reqBodyBytes: []byte(`{"method": invalid}`),
			duration:     1 * time.Second,
			data: &PromRequestMetricData{
				Method:         "POST",
				Service:        "/ethereum",
				Provider:       "infura",
				HostName:       "node1",
				ResponseStatus: 200,
				HealthStatus:   "healthy",
			},
			expectedLabels: map[string]string{
				"service":         "ethereum",
				"method":          "",
				"provider":        "infura",
				"host_name":       "node1",
				"response_status": "200",
				"health_status":   "healthy",
				"machine_id":      client.machineID,
			},
			expectedValue: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call the function
			client.HandleRequestMetrics(tt.data, tt.reqBodyBytes, tt.duration)

			// Use  testutil to check if the metric exists with the expected labels and value
			_, err := registry.Gather()
			assert.NoError(t, err)

			metric := testutil.ToFloat64(DinRequestCount.WithLabelValues(
				tt.expectedLabels["service"],
				tt.expectedLabels["method"],
				tt.expectedLabels["provider"],
				tt.expectedLabels["host_name"],
				tt.expectedLabels["response_status"],
				tt.expectedLabels["health_status"],
				tt.expectedLabels["machine_id"],
			))

			assert.Equal(t, tt.expectedValue, metric, "Metric should be incremented once")
		})
	}
}

func TestHandleLatestBlockMetric(t *testing.T) {
	// Initialize the prometheus client
	client := NewPrometheusClient(zap.NewNop(), "test-machine-id")

	// Create a new registry and register our metric
	registry := prometheus.NewRegistry()
	registry.MustRegister(DinHealthCheckCount, DinProviderBlockNumber)

	tests := []struct {
		name           string
		data           *PromLatestBlockMetricData
		expectedLabels map[string]string
	}{
		{
			name: "Valid Data",
			data: &PromLatestBlockMetricData{
				Service:        "/ethereum",
				Provider:       "infura",
				ResponseStatus: 200,
				HealthStatus:   "healthy",
				BlockNumber:    12345,
			},
			expectedLabels: map[string]string{
				"service":         "ethereum",
				"provider":        "infura",
				"response_status": "200",
				"health_status":   "healthy",
				"machine_id":      client.machineID,
			},
		},
		{
			name: "Invalid Data",
			data: &PromLatestBlockMetricData{
				Service:        "/ethereum",
				Provider:       "infura",
				ResponseStatus: 500,
				HealthStatus:   "unhealthy",
				BlockNumber:    -1,
			},
			expectedLabels: map[string]string{
				"service":         "ethereum",
				"provider":        "infura",
				"response_status": "500",
				"health_status":   "unhealthy",
				"machine_id":      client.machineID,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call the function
			client.HandleLatestBlockMetric(tt.data)

			// Use testutil to check if the metric exists with the expected labels and value
			_, err := registry.Gather()
			assert.NoError(t, err)

			metric := testutil.ToFloat64(DinHealthCheckCount.WithLabelValues(
				tt.expectedLabels["service"],
				tt.expectedLabels["provider"],
				tt.expectedLabels["response_status"],
				tt.expectedLabels["health_status"],
				tt.expectedLabels["machine_id"],
			))

			assert.Equal(t, float64(1), metric, "Metric should be incremented once")

			blockNumberMetric := testutil.ToFloat64(DinProviderBlockNumber.WithLabelValues(
				tt.expectedLabels["service"],
				tt.expectedLabels["provider"],
				tt.expectedLabels["machine_id"],
			))

			assert.Equal(t, float64(tt.data.BlockNumber), blockNumberMetric, "Block number metric should be set correctly")
		})
	}
}
