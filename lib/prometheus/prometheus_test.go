package prometheus

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestHandleRequestMetric(t *testing.T) {
	// Initialize the prometheus client
	client := NewPrometheusClient()

	// Register metrics
	RegisterMetrics()

	// Create a new registry and register our metric
	registry := prometheus.NewRegistry()
	registry.MustRegister(DinRequestCount)

	tests := []struct {
		name           string
		reqBodyBytes   []byte
		data           *PromRequestMetricData
		expectedLabels map[string]string
		expectedValue  float64
	}{
		{
			name:         "Valid JSON",
			reqBodyBytes: []byte(`{"method": "eth_getBlockByNumber"}`),
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
			},
			expectedValue: 1,
		},
		{
			name:         "Invalid JSON",
			reqBodyBytes: []byte(`{"method": invalid}`),
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
			},
			expectedValue: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call the function
			client.HandleRequestMetric(tt.reqBodyBytes, tt.data)

			// Use testutil to check if the metric exists with the expected labels and value
			_, err := registry.Gather()
			assert.NoError(t, err)

			metric := testutil.ToFloat64(DinRequestCount.WithLabelValues(
				tt.expectedLabels["service"],
				tt.expectedLabels["method"],
				tt.expectedLabels["provider"],
				tt.expectedLabels["host_name"],
				tt.expectedLabels["response_status"],
				tt.expectedLabels["health_status"],
			))

			assert.Equal(t, tt.expectedValue, metric, "Metric should be incremented once")
		})
	}
}
