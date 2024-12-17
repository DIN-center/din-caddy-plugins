package modules

import (
	"fmt"
	"testing"
	"time"

	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	prom "github.com/DIN-center/din-caddy-plugins/lib/prometheus"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"github.com/golang/mock/gomock"
	"go.uber.org/zap"
)

func TestHealthCheck(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockHttpClient := din_http.NewMockIHTTPClient(mockCtrl)
	mockPrometheusClient := prom.NewMockIPrometheusClient(mockCtrl)
	logger := zap.NewNop()

	tests := []struct {
		name               string
		network            *network
		evmSpeedEnabled    bool
		latestBlockResp    []byte
		earliestBlockResp  []byte
		statusCode         int
		err                error
		wantProviderStatus map[string]HealthStatus
		wantLatestBlock    int64
		wantEarliestBlock  int64
	}{
		// {
		// 	name: "single provider, successful response",
		// 	network: &network{
		// 		PrometheusClient: mockPrometheusClient,
		// 		BlockNumberDelta: 10,
		// 		HCThreshold:      3,
		// 		Name:             "test-network",
		// 		logger:           logger,
		// 		EVMSpeedEnabled:  false,
		// 		Providers: map[string]*provider{
		// 			"provider1": {
		// 				healthStatus: Healthy,
		// 				host:         "provider1",
		// 				httpClient:   mockHttpClient,
		// 			},
		// 		},
		// 		latestBlockNumber: 5000000,
		// 		CheckedProviders:  map[string][]healthCheckEntry{},
		// 	},
		// 	latestBlockResp: []byte(`{"jsonrpc": "2.0", "id": 1,"result": "0x4c4b43"}`),
		// 	statusCode:      200,
		// 	err:             nil,
		// 	wantProviderStatus: map[string]HealthStatus{
		// 		"provider1": Healthy,
		// 	},
		// 	wantLatestBlock: 5000003,
		// },
		{
			name: "single provider with EVMSpeed enabled",
			network: &network{
				PrometheusClient: mockPrometheusClient,
				BlockNumberDelta: 10,
				HCThreshold:      3,
				Name:             "test-network",
				logger:           logger,
				EVMSpeedEnabled:  true,
				Providers: map[string]*provider{
					"provider1": {
						healthStatus: Healthy,
						host:         "provider1",
						httpClient:   mockHttpClient,
					},
				},
				latestBlockNumber: 5000000,
				CheckedProviders:  map[string][]healthCheckEntry{},
			},
			latestBlockResp:   []byte(`{"jsonrpc": "2.0", "id": 1,"result": "0x4c4b43"}`),
			earliestBlockResp: []byte(`{"jsonrpc":"2.0","id":1,"result":{"number":"0x1","hash":"0x123"}}`),
			statusCode:        200,
			err:               nil,
			wantProviderStatus: map[string]HealthStatus{
				"provider1": Healthy,
			},
			wantLatestBlock:   5000003,
			wantEarliestBlock: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, provider := range tt.network.Providers {
				// Mock getLatestBlockNumber call
				mockHttpClient.EXPECT().
					Post(provider.HttpUrl, provider.Headers, gomock.Any(), provider.AuthClient()).
					Return(tt.latestBlockResp, &tt.statusCode, tt.err)

				if tt.network.EVMSpeedEnabled {
					// Mock getEarliestBlockNumber call
					expectedPayload := []byte(fmt.Sprintf(`{"jsonrpc":"2.0","method":"%s","params":["%s", false],"id":1}`, DefaultGetBlockNumberMethod, "0x1"))
					mockHttpClient.EXPECT().
						Post(provider.HttpUrl, provider.Headers, expectedPayload, provider.AuthClient()).
						Return(tt.earliestBlockResp, &tt.statusCode, tt.err)
				}

				mockPrometheusClient.EXPECT().
					HandleLatestBlockMetric(gomock.Any()).
					Times(1)
			}

			// Run health check
			tt.network.healthCheck()

			// Verify provider status
			for providerName, provider := range tt.network.Providers {
				wantStatus := tt.wantProviderStatus[providerName]
				if provider.healthStatus != wantStatus {
					t.Errorf("healthCheck() for provider %s got status = %v, want %v",
						providerName, provider.healthStatus, wantStatus)
				}

				// Verify latest block number
				if provider.latestBlockNumber != uint64(tt.wantLatestBlock) {
					t.Errorf("healthCheck() for provider %s got latest block = %v, want %v",
						providerName, provider.latestBlockNumber, tt.wantLatestBlock)
				}

				// Verify earliest block number if EVMSpeed is enabled
				if tt.network.EVMSpeedEnabled && provider.earliestBlockNumber != uint64(tt.wantEarliestBlock) {
					t.Errorf("healthCheck() for provider %s got earliest block = %v, want %v",
						providerName, provider.earliestBlockNumber, tt.wantEarliestBlock)
				}
			}
		})
	}
}

func TestPingHealthCheck(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockPrometheusClient := prom.NewMockIPrometheusClient(mockCtrl)
	logger := zap.NewNop()

	tests := []struct {
		name         string
		providerName string
		provider     *provider
		statusCode   int
		blockNumber  int64
		want         HealthStatus
		wantReturn   bool
		failures     int // Add failures count
	}{
		{
			name:         "successful response, status 200",
			providerName: "provider1",
			provider: &provider{
				healthStatus: Healthy,
				host:         "provider1",
			},
			statusCode:  200,
			blockNumber: 5000000,
			want:        Healthy,
			wantReturn:  false,
			failures:    0,
		},
		{
			name:         "rate limited response, status 429",
			providerName: "provider1",
			provider: &provider{
				healthStatus: Healthy,
				host:         "provider1",
			},
			statusCode:  429,
			blockNumber: 5000000,
			want:        Warning,
			wantReturn:  true,
			failures:    0,
		},
		{
			name:         "error response, status 400, first failure",
			providerName: "provider1",
			provider: &provider{
				healthStatus: Healthy,
				host:         "provider1",
			},
			statusCode:  400,
			blockNumber: 5000000,
			want:        Healthy, // Still healthy after first failure
			wantReturn:  true,
			failures:    0,
		},
		{
			name:         "error response, status 400, exceeds threshold",
			providerName: "provider1",
			provider: &provider{
				healthStatus: Healthy,
				host:         "provider1",
				failures:     3, // Already has 3 failures
			},
			statusCode:  400,
			blockNumber: 5000000,
			want:        Unhealthy, // Now becomes unhealthy
			wantReturn:  true,
			failures:    3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &network{
				PrometheusClient: mockPrometheusClient,
				logger:           logger,
				HCThreshold:      3,
				Name:             "test-network",
			}

			// Set initial failures count
			tt.provider.failures = tt.failures

			if tt.statusCode > 399 {
				mockPrometheusClient.EXPECT().HandleLatestBlockMetric(gomock.Any()).Times(1)
			}

			result := n.pingHealthCheck(tt.providerName, tt.provider, tt.statusCode, tt.blockNumber)

			if result != tt.wantReturn {
				t.Errorf("pingHealthCheck() return = %v, want %v", result, tt.wantReturn)
			}

			if tt.provider.healthStatus != tt.want {
				t.Errorf("pingHealthCheck() got = %v, want %v", tt.provider.healthStatus, tt.want)
			}

			// Verify failures count increased for error responses
			if tt.statusCode > 399 && tt.statusCode != 429 {
				expectedFailures := tt.failures + 1
				if tt.provider.failures != expectedFailures {
					t.Errorf("pingHealthCheck() failures = %v, want %v", tt.provider.failures, expectedFailures)
				}
			}
		})
	}
}

func TestBlockNumberDeltaHealthCheck(t *testing.T) {
	timeNow := time.Now()
	tests := []struct {
		name            string
		providerName    string
		provider        *provider
		blockNumber     int64
		network         *network
		expectUnhealthy bool
		expectedStatus  HealthStatus
	}{
		{
			name:         "single provider - always healthy",
			providerName: "provider1",
			provider: &provider{
				healthStatus: Healthy,
				host:         "provider1",
			},
			blockNumber: 5000030,
			network: &network{
				Providers: map[string]*provider{
					"provider1": {host: "provider1"},
				},
				BlockNumberDelta: 20,
				CheckedProviders: map[string][]healthCheckEntry{
					"provider1": {{blockNumber: 5000000, timestamp: &timeNow}},
				},
			},
			expectUnhealthy: false,
			expectedStatus:  Healthy,
		},
		{
			name:         "provider too far ahead of 75th percentile",
			providerName: "provider1",
			provider: &provider{
				healthStatus: Healthy,
				host:         "provider1",
			},
			blockNumber: 5000030,
			network: &network{
				Providers: map[string]*provider{
					"provider1": {host: "provider1"},
					"provider2": {host: "provider2"},
					"provider3": {host: "provider3"},
				},
				BlockNumberDelta: 20,
				CheckedProviders: map[string][]healthCheckEntry{
					"provider1": {{blockNumber: 5000030, timestamp: &timeNow}},
					"provider2": {{blockNumber: 5000000, timestamp: &timeNow}},
					"provider3": {{blockNumber: 5000000, timestamp: &timeNow}},
				},
			},
			expectUnhealthy: true,
			expectedStatus:  Unhealthy,
		},
		{
			name:         "provider too far behind 75th percentile",
			providerName: "provider1",
			provider: &provider{
				healthStatus: Healthy,
				host:         "provider1",
			},
			blockNumber: 4999970,
			network: &network{
				Providers: map[string]*provider{
					"provider1": {host: "provider1"},
					"provider2": {host: "provider2"},
					"provider3": {host: "provider3"},
				},
				BlockNumberDelta: 20,
				CheckedProviders: map[string][]healthCheckEntry{
					"provider1": {{blockNumber: 4999970, timestamp: &timeNow}},
					"provider2": {{blockNumber: 5000000, timestamp: &timeNow}},
					"provider3": {{blockNumber: 5000000, timestamp: &timeNow}},
				},
			},
			expectUnhealthy: true,
			expectedStatus:  Unhealthy,
		},
		{
			name:         "provider within acceptable range",
			providerName: "provider1",
			provider: &provider{
				healthStatus: Healthy,
				host:         "provider1",
			},
			blockNumber: 5000010,
			network: &network{
				Providers: map[string]*provider{
					"provider1": {host: "provider1"},
					"provider2": {host: "provider2"},
					"provider3": {host: "provider3"},
				},
				BlockNumberDelta: 20,
				CheckedProviders: map[string][]healthCheckEntry{
					"provider1": {{blockNumber: 5000010, timestamp: &timeNow}},
					"provider2": {{blockNumber: 5000000, timestamp: &timeNow}},
					"provider3": {{blockNumber: 5000000, timestamp: &timeNow}},
				},
			},
			expectUnhealthy: false,
			expectedStatus:  Healthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.network.logger = zap.NewNop()
			result := tt.network.blockNumberDeltaHealthCheck(tt.providerName, tt.provider, tt.blockNumber)

			if result != tt.expectUnhealthy {
				t.Errorf("blockNumberDeltaHealthCheck() returned %v, want %v", result, tt.expectUnhealthy)
			}

			if tt.provider.healthStatus != tt.expectedStatus {
				t.Errorf("blockNumberDeltaHealthCheck() health status = %v, want %v", tt.provider.healthStatus, tt.expectedStatus)
			}
		})
	}
}

func TestConsistencyHealthCheck(t *testing.T) {
	timeNow := time.Now()
	tests := []struct {
		name                string
		providerName        string
		provider            *provider
		blockNumber         int64
		network             *network
		want                HealthStatus
		expectedLatestBlock int64
	}{
		{
			name:         "single provider - always healthy",
			providerName: "provider1",
			provider: &provider{
				healthStatus: Healthy,
				host:         "provider1",
			},
			blockNumber: 5000000,
			network: &network{
				Providers: map[string]*provider{
					"provider1": {host: "provider1"},
				},
				BlockLagLimit: 100,
				HCThreshold:   3,
			},
			want:                Healthy,
			expectedLatestBlock: 5000000,
		},
		{
			name:         "provider lagging behind 75th percentile",
			providerName: "provider1",
			provider: &provider{
				healthStatus: Healthy,
				host:         "provider1",
			},
			blockNumber: 4999800,
			network: &network{
				Providers: map[string]*provider{
					"provider1": {host: "provider1"},
					"provider2": {host: "provider2"},
					"provider3": {host: "provider3"},
				},
				BlockLagLimit: 100,
				HCThreshold:   3,
				CheckedProviders: map[string][]healthCheckEntry{
					"provider1": {{blockNumber: 4999800, timestamp: &timeNow}},
					"provider2": {{blockNumber: 5000000, timestamp: &timeNow}},
					"provider3": {{blockNumber: 5000000, timestamp: &timeNow}},
				},
			},
			want:                Warning,
			expectedLatestBlock: 5000000,
		},
		{
			name:         "provider within acceptable range",
			providerName: "provider1",
			provider: &provider{
				healthStatus: Healthy,
				host:         "provider1",
			},
			blockNumber: 5000000,
			network: &network{
				Providers: map[string]*provider{
					"provider1": {host: "provider1"},
					"provider2": {host: "provider2"},
				},
				BlockLagLimit: 100,
				HCThreshold:   3,
				CheckedProviders: map[string][]healthCheckEntry{
					"provider1": {{blockNumber: 5000000, timestamp: &timeNow}},
					"provider2": {{blockNumber: 5000000, timestamp: &timeNow}},
				},
			},
			want:                Healthy,
			expectedLatestBlock: 5000000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.network.logger = zap.NewNop()
			tt.network.consistencyHealthCheck(tt.providerName, tt.provider, tt.blockNumber)

			if tt.provider.healthStatus != tt.want {
				t.Errorf("consistencyHealthCheck() health status = %v, want %v", tt.provider.healthStatus, tt.want)
			}

			if tt.network.latestBlockNumber != tt.expectedLatestBlock {
				t.Errorf("consistencyHealthCheck() latest block = %v, want %v", tt.network.latestBlockNumber, tt.expectedLatestBlock)
			}
		})
	}
}

func TestAddHealthCheckToCheckedProviderList(t *testing.T) {
	timeNow := time.Now()
	timeYesterday := timeNow.AddDate(0, 0, -1)

	tests := []struct {
		name             string
		network          *network
		providerName     string
		healthCheckInput healthCheckEntry
		want             []healthCheckEntry
	}{
		{
			name: "health check entry added to empty list",
			network: &network{
				Providers: map[string]*provider{
					"provider1": {
						upstream: &reverseproxy.Upstream{},
					},
				},
				CheckedProviders: map[string][]healthCheckEntry{},
			},
			providerName: "provider1",
			healthCheckInput: healthCheckEntry{
				blockNumber: 1,
				timestamp:   &timeNow,
			},
			want: []healthCheckEntry{
				{
					blockNumber: 1,
					timestamp:   &timeNow,
				},
			},
		},
		{
			name: "health check entry added to a populated list",
			network: &network{
				Providers: map[string]*provider{
					"provider1": {
						upstream: &reverseproxy.Upstream{},
					},
				},
				CheckedProviders: map[string][]healthCheckEntry{
					"provider1": {
						{
							blockNumber: 1,
							timestamp:   &timeYesterday,
						},
					},
				},
			},
			providerName: "provider1",
			healthCheckInput: healthCheckEntry{
				blockNumber: 2,
				timestamp:   &timeNow,
			},
			want: []healthCheckEntry{
				{
					blockNumber: 2,
					timestamp:   &timeNow,
				},
				{
					blockNumber: 1,
					timestamp:   &timeYesterday,
				},
			},
		},
		{
			name: "health check entry added to a populated list of 10",
			network: &network{
				Providers: map[string]*provider{
					"provider1": {
						upstream: &reverseproxy.Upstream{},
					},
				},
				CheckedProviders: map[string][]healthCheckEntry{
					"provider1": {
						{
							blockNumber: 10,
							timestamp:   &timeYesterday,
						},
						{
							blockNumber: 9,
							timestamp:   &timeYesterday,
						},
						{
							blockNumber: 8,
							timestamp:   &timeYesterday,
						},
						{
							blockNumber: 7,
							timestamp:   &timeYesterday,
						},
						{
							blockNumber: 6,
							timestamp:   &timeYesterday,
						},
						{
							blockNumber: 5,
							timestamp:   &timeYesterday,
						},
						{
							blockNumber: 4,
							timestamp:   &timeYesterday,
						},
						{
							blockNumber: 3,
							timestamp:   &timeYesterday,
						},
						{
							blockNumber: 2,
							timestamp:   &timeYesterday,
						},
						{
							blockNumber: 1,
							timestamp:   &timeYesterday,
						},
					},
				},
			},
			providerName: "provider1",
			healthCheckInput: healthCheckEntry{
				blockNumber: 11,
				timestamp:   &timeNow,
			},
			want: []healthCheckEntry{
				{
					blockNumber: 11,
					timestamp:   &timeNow,
				},
				{
					blockNumber: 10,
					timestamp:   &timeYesterday,
				},
				{
					blockNumber: 9,
					timestamp:   &timeYesterday,
				},
				{
					blockNumber: 8,
					timestamp:   &timeYesterday,
				},
				{
					blockNumber: 7,
					timestamp:   &timeYesterday,
				},
				{
					blockNumber: 6,
					timestamp:   &timeYesterday,
				},
				{
					blockNumber: 5,
					timestamp:   &timeYesterday,
				},
				{
					blockNumber: 4,
					timestamp:   &timeYesterday,
				},
				{
					blockNumber: 3,
					timestamp:   &timeYesterday,
				},
				{
					blockNumber: 2,
					timestamp:   &timeYesterday,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.network.addHealthCheckToCheckedProviderList(tt.providerName, tt.healthCheckInput)

			if len(tt.network.CheckedProviders[tt.providerName]) != len(tt.want) {
				t.Errorf("network.addHealthCheckToCheckedProviderList() for %v  = %v, want %v", tt.providerName, len(tt.network.CheckedProviders[tt.providerName]), len(tt.want))
			}
			if len(tt.want) > 0 {
				if tt.network.CheckedProviders[tt.providerName][0].blockNumber != tt.want[0].blockNumber {
					t.Errorf("network.addHealthCheckToCheckedProviderList() for %v  = %v, want %v", tt.providerName, tt.network.CheckedProviders[tt.providerName][0].blockNumber, tt.want[0].blockNumber)
				}
			}
		})
	}
}

func TestEvaluatecheckedProviders(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name    string
		network *network
		want    map[string]*provider
	}{
		{
			name: "1 provider, has older block, marked Warning",
			network: &network{
				Providers: map[string]*provider{
					"provider1": {
						healthStatus: Healthy,
					},
				},
				latestBlockNumber: 10,
				CheckedProviders: map[string][]healthCheckEntry{
					"provider1": {
						{
							blockNumber: 1,
							timestamp:   nil,
						},
					},
				},
				logger: logger,
			},
			want: map[string]*provider{
				"provider1": {
					healthStatus: Warning,
				},
			},
		},
		{
			name: "1 provider, has newer block, marked healthy",
			network: &network{
				Providers: map[string]*provider{
					"provider1": {
						healthStatus: Healthy,
					},
				},
				latestBlockNumber: 10,
				CheckedProviders: map[string][]healthCheckEntry{
					"provider1": {
						{
							blockNumber: 20,
							timestamp:   nil,
						},
					},
				},
				logger: logger,
			},
			want: map[string]*provider{
				"provider1": {
					healthStatus: Healthy,
				},
			},
		},
		{
			name: "1 provider, has equal block, marked healthy",
			network: &network{
				Providers: map[string]*provider{
					"provider1": {
						healthStatus: Healthy,
					},
				},
				latestBlockNumber: 10,
				CheckedProviders: map[string][]healthCheckEntry{
					"provider1": {
						{
							blockNumber: 10,
							timestamp:   nil,
						},
					},
				},
				logger: logger,
			},
			want: map[string]*provider{
				"provider1": {
					healthStatus: Healthy,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.network.evaluateCheckedProviders()

			for providerName, provider := range tt.network.Providers {
				if provider.healthStatus != tt.want[providerName].healthStatus {
					t.Errorf("network.evaluatecheckedProviders() for %v  = %v, want %v", providerName, provider.healthStatus, tt.want[providerName].healthStatus)
				}
			}
		})
	}
}

func TestGetPercentileBlockNumber(t *testing.T) {
	timeNow := time.Now()
	tests := []struct {
		name       string
		network    *network
		percentile float64
		want       int64
	}{
		{
			name: "empty providers map",
			network: &network{
				Providers:        make(map[string]*provider),
				CheckedProviders: make(map[string][]healthCheckEntry),
			},
			percentile: 0.75,
			want:       0,
		},
		{
			name: "single provider",
			network: &network{
				Providers: map[string]*provider{
					"provider1": {host: "provider1"},
				},
				CheckedProviders: map[string][]healthCheckEntry{
					"provider1": {{blockNumber: 1000, timestamp: &timeNow}},
				},
			},
			percentile: 0.75,
			want:       1000,
		},
		{
			name: "multiple providers - 75th percentile",
			network: &network{
				Providers: map[string]*provider{
					"provider1": {host: "provider1"},
					"provider2": {host: "provider2"},
					"provider3": {host: "provider3"},
					"provider4": {host: "provider4"},
				},
				CheckedProviders: map[string][]healthCheckEntry{
					"provider1": {{blockNumber: 1000, timestamp: &timeNow}},
					"provider2": {{blockNumber: 1100, timestamp: &timeNow}},
					"provider3": {{blockNumber: 1200, timestamp: &timeNow}},
					"provider4": {{blockNumber: 1300, timestamp: &timeNow}},
				},
			},
			percentile: 0.75,
			want:       1200,
		},
		{
			name: "providers with no health checks",
			network: &network{
				Providers: map[string]*provider{
					"provider1": {host: "provider1"},
					"provider2": {host: "provider2"},
				},
				CheckedProviders: make(map[string][]healthCheckEntry),
			},
			percentile: 0.75,
			want:       0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.network.getPercentileBlockNumber(tt.percentile)
			if got != tt.want {
				t.Errorf("getPercentileBlockNumber() = %v, want %v", got, tt.want)
			}
		})
	}
}
