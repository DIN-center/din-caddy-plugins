package modules

import (
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	networklib "github.com/DIN-center/din-caddy-plugins/lib/network"
	prom "github.com/DIN-center/din-caddy-plugins/lib/prometheus"
	"github.com/DIN-center/din-caddy-plugins/lib/utils"
)

// TestLoopbackHealthCheck tests the loopback health check functionality
func TestLoopbackHealthCheck(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name                   string
		caddyPort              string
		mockHandlerSetup       func(*networklib.MockNetworkHandler)
		mockPrometheusSetup    func(*prom.MockIPrometheusClient)
		expectPrometheusMetric bool
		expectedResponseStatus int
		handlerError           error
	}{
		{
			name:      "successful_loopback_health_check",
			caddyPort: "8080",
			mockHandlerSetup: func(mockHandler *networklib.MockNetworkHandler) {
				// Setup handler expectations
				mockHandler.EXPECT().GetHealthCheckMethod().Return("eth_blockNumber").AnyTimes()
				mockHandler.EXPECT().GetType().Return(networklib.EVMHandlerType).AnyTimes()
				mockHandler.EXPECT().GetRequestType().Return(networklib.RequestTypeRPC).AnyTimes()
				mockHandler.EXPECT().GetHealthCheckHTTPMethod().Return("POST").AnyTimes()
				mockHandler.EXPECT().CreateHealthCheckPayload("eth_blockNumber").Return([]byte(`{"jsonrpc":"2.0","method":"eth_blockNumber","id":1}`), nil).AnyTimes()
				mockHandler.EXPECT().GetLatestBlockNumber(
					gomock.Any(), // URL will contain loopback address
					gomock.Any(), // Headers
					gomock.Any(), // HTTP Client
					gomock.Nil(), // No auth for loopback
					1,            // Single attempt
				).Return(&networklib.LatestBlockResult{
					BlockNumber:    12345,
					HealthStatus:   networklib.Healthy,
					ResponseStatus: 200,
					Metadata:       make(map[string]interface{}),
				}, nil)
			},
			mockPrometheusSetup: func(mockProm *prom.MockIPrometheusClient) {
				mockProm.EXPECT().HandleNetworkHealthCheckMetric(gomock.Any()).Do(func(data *prom.PromNetworkHealthCheckMetricData) {
					assert.Equal(t, "test-network", data.Network)
					assert.Equal(t, 200, data.ResponseStatus)
					assert.True(t, data.Duration > 0)
				})
			},
			expectPrometheusMetric: true,
			expectedResponseStatus: 200,
		},
		{
			name:      "failed_loopback_health_check_server_error",
			caddyPort: "8080",
			mockHandlerSetup: func(mockHandler *networklib.MockNetworkHandler) {
				mockHandler.EXPECT().GetHealthCheckMethod().Return("eth_blockNumber").AnyTimes()
				mockHandler.EXPECT().GetType().Return(networklib.EVMHandlerType).AnyTimes()
				mockHandler.EXPECT().GetRequestType().Return(networklib.RequestTypeRPC).AnyTimes()
				mockHandler.EXPECT().GetHealthCheckHTTPMethod().Return("POST").AnyTimes()
				mockHandler.EXPECT().CreateHealthCheckPayload("eth_blockNumber").Return([]byte(`{"jsonrpc":"2.0","method":"eth_blockNumber","id":1}`), nil).AnyTimes()
				mockHandler.EXPECT().GetLatestBlockNumber(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Nil(),
					1,
				).Return(&networklib.LatestBlockResult{
					BlockNumber:    0,
					HealthStatus:   networklib.Unhealthy,
					ResponseStatus: 500,
					Metadata:       make(map[string]interface{}),
				}, errors.New("internal server error"))
			},
			mockPrometheusSetup: func(mockProm *prom.MockIPrometheusClient) {
				mockProm.EXPECT().HandleNetworkHealthCheckMetric(gomock.Any()).Do(func(data *prom.PromNetworkHealthCheckMetricData) {
					assert.Equal(t, "test-network", data.Network)
					assert.Equal(t, 500, data.ResponseStatus)
					assert.True(t, data.Duration > 0)
				})
			},
			expectPrometheusMetric: true,
			expectedResponseStatus: 500,
			handlerError:           errors.New("internal server error"),
		},
		{
			name:      "failed_loopback_health_check_timeout",
			caddyPort: "8080",
			mockHandlerSetup: func(mockHandler *networklib.MockNetworkHandler) {
				mockHandler.EXPECT().GetHealthCheckMethod().Return("eth_blockNumber").AnyTimes()
				mockHandler.EXPECT().GetType().Return(networklib.EVMHandlerType).AnyTimes()
				mockHandler.EXPECT().GetRequestType().Return(networklib.RequestTypeRPC).AnyTimes()
				mockHandler.EXPECT().GetHealthCheckHTTPMethod().Return("POST").AnyTimes()
				mockHandler.EXPECT().CreateHealthCheckPayload("eth_blockNumber").Return([]byte(`{"jsonrpc":"2.0","method":"eth_blockNumber","id":1}`), nil).AnyTimes()
				mockHandler.EXPECT().GetLatestBlockNumber(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Nil(),
					1,
				).Return(&networklib.LatestBlockResult{
					BlockNumber:    0,
					HealthStatus:   networklib.Unhealthy,
					ResponseStatus: 0, // No response due to timeout
					Metadata:       make(map[string]interface{}),
				}, errors.New("request timeout"))
			},
			mockPrometheusSetup: func(mockProm *prom.MockIPrometheusClient) {
				mockProm.EXPECT().HandleNetworkHealthCheckMetric(gomock.Any()).Do(func(data *prom.PromNetworkHealthCheckMetricData) {
					assert.Equal(t, "test-network", data.Network)
					assert.Equal(t, 0, data.ResponseStatus)
					assert.True(t, data.Duration > 0)
				})
			},
			expectPrometheusMetric: true,
			expectedResponseStatus: 0,
			handlerError:           errors.New("request timeout"),
		},
		{
			name:      "no_caddy_port_set",
			caddyPort: "", // Empty port
			mockHandlerSetup: func(mockHandler *networklib.MockNetworkHandler) {
				// No handler calls expected when port is empty
			},
			mockPrometheusSetup: func(mockProm *prom.MockIPrometheusClient) {
				// Metric should still be sent even on error
				mockProm.EXPECT().HandleNetworkHealthCheckMetric(gomock.Any()).Do(func(data *prom.PromNetworkHealthCheckMetricData) {
					assert.Equal(t, "test-network", data.Network)
					assert.Equal(t, 0, data.ResponseStatus)
					assert.True(t, data.Duration > 0)
				})
			},
			expectPrometheusMetric: true,
			expectedResponseStatus: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			mockHandler := networklib.NewMockNetworkHandler(ctrl)
			mockProm := prom.NewMockIPrometheusClient(ctrl)

			// Create network
			n, err := NewNetwork("test-network", networklib.EVMHandlerType, utils.Environment("test"), LoopbackConfig{
				Port: tt.caddyPort,
				ApiKey: DefaultLoopbackApiKey,
			})
			assert.NoError(t, err)

			// Set dependencies
			n.handler = mockHandler
			n.PrometheusClient = mockProm
			n.logger = logger.NewLoggerClient(zap.NewNop(), utils.EnvTest)

			// Setup mocks
			if tt.mockHandlerSetup != nil {
				tt.mockHandlerSetup(mockHandler)
			}
			if tt.mockPrometheusSetup != nil {
				tt.mockPrometheusSetup(mockProm)
			}

			// Execute loopback health check
			startTime := time.Now()
			n.LoopbackHealthCheck()
			duration := time.Since(startTime)

			// Verify duration is reasonable (should be quick for loopback)
			assert.True(t, duration < 5*time.Second, "Loopback health check took too long: %v", duration)
		})
	}
}

// TestSendHealthCheckMetric tests the metric sending functionality
func TestSendHealthCheckMetric(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name           string
		provider       string
		providerName   string
		responseStatus int
		healthStatus   string
		blockNumber    int64
		priority       int
		environment    string
		expectedMetric *prom.PromHealthCheckMetricData
	}{
		{
			name:           "send_healthy_metric",
			provider:       "provider1.com",
			providerName:   "provider1",
			responseStatus: 200,
			healthStatus:   "Healthy",
			blockNumber:    12345,
			priority:       0,
			environment:    "production",
			expectedMetric: &prom.PromHealthCheckMetricData{
				Network:        "test-network",
				Provider:       "provider1.com",
				ProviderName:   "provider1",
				ResponseStatus: 200,
				HealthStatus:   "Healthy",
				BlockNumber:    12345,
				Priority:       0,
				Environment:    "production",
			},
		},
		{
			name:           "send_warning_metric",
			provider:       "provider2.com",
			providerName:   "provider2",
			responseStatus: 200,
			healthStatus:   "Warning",
			blockNumber:    12340,
			priority:       1,
			environment:    "staging",
			expectedMetric: &prom.PromHealthCheckMetricData{
				Network:        "test-network",
				Provider:       "provider2.com",
				ProviderName:   "provider2",
				ResponseStatus: 200,
				HealthStatus:   "Warning",
				BlockNumber:    12340,
				Priority:       1,
				Environment:    "staging",
			},
		},
		{
			name:           "send_unhealthy_metric",
			provider:       "provider3.com",
			providerName:   "provider3",
			responseStatus: 500,
			healthStatus:   "Unhealthy",
			blockNumber:    0,
			priority:       2,
			environment:    "test",
			expectedMetric: &prom.PromHealthCheckMetricData{
				Network:        "test-network",
				Provider:       "provider3.com",
				ProviderName:   "provider3",
				ResponseStatus: 500,
				HealthStatus:   "Unhealthy",
				BlockNumber:    0,
				Priority:       2,
				Environment:    "test",
			},
		},
		{
			name:           "send_rate_limit_metric",
			provider:       "provider4.com",
			providerName:   "provider4",
			responseStatus: 429,
			healthStatus:   "Warning",
			blockNumber:    12345,
			priority:       0,
			environment:    "production",
			expectedMetric: &prom.PromHealthCheckMetricData{
				Network:        "test-network",
				Provider:       "provider4.com",
				ProviderName:   "provider4",
				ResponseStatus: 429,
				HealthStatus:   "Warning",
				BlockNumber:    12345,
				Priority:       0,
				Environment:    "production",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock
			mockProm := prom.NewMockIPrometheusClient(ctrl)

			// Create network
			n, err := NewNetwork("test-network", networklib.EVMHandlerType, utils.Environment(tt.environment), LoopbackConfig{Port: "8080", ApiKey: DefaultLoopbackApiKey})
			assert.NoError(t, err)

			// Set dependencies
			n.PrometheusClient = mockProm

			// Setup expectation
			mockProm.EXPECT().HandleHealthCheckMetric(gomock.Any()).Do(func(data *prom.PromHealthCheckMetricData) {
				assert.Equal(t, tt.expectedMetric.Network, data.Network)
				assert.Equal(t, tt.expectedMetric.Provider, data.Provider)
				assert.Equal(t, tt.expectedMetric.ResponseStatus, data.ResponseStatus)
				assert.Equal(t, tt.expectedMetric.HealthStatus, data.HealthStatus)
				assert.Equal(t, tt.expectedMetric.BlockNumber, data.BlockNumber)
				assert.Equal(t, tt.expectedMetric.Priority, data.Priority)
				assert.Equal(t, tt.expectedMetric.Environment, data.Environment)
			})

			// Execute
			n.sendHealthCheckMetric(tt.provider, tt.providerName, tt.responseStatus, tt.healthStatus, tt.blockNumber, tt.priority, tt.environment)
		})
	}
}
