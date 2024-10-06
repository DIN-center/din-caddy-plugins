package modules

import (
	"testing"
	"time"

	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	prom "github.com/DIN-center/din-caddy-plugins/lib/prometheus"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func TestHealthCheck(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockHttpClient := din_http.NewMockIHTTPClient(mockCtrl)
	mockPrometheusClient := prom.NewMockIPrometheusClient(mockCtrl)

	type postResponse struct {
		postResponseBytes []byte
		statusCode        int
		err               error
	}

	tests := []struct {
		name                string
		network             *network
		latestBlockResponse postResponse
		want                map[string]*provider
	}{
		{
			name: "1 provider, successful response, has newer blocks, marked healthy",
			network: &network{
				httpClient: mockHttpClient,
				Providers: map[string]*provider{
					"provider1": {
						upstream: &reverseproxy.Upstream{
							Dial: "provider1",
						},
					},
				},
				latestBlockNumber: 5000000,
				checkedProviders:  map[string][]healthCheckEntry{},
				prometheusClient:  mockPrometheusClient,
				logger:            zap.NewNop(),
			},
			latestBlockResponse: postResponse{
				postResponseBytes: []byte(`{"jsonrpc": "2.0", "id": 1,"result": "0x60497d"}`),
				statusCode:        200,
				err:               nil,
			},
			want: map[string]*provider{
				"provider1": {
					healthStatus: Healthy,
				},
			},
		},
		{
			name: "1 provider, successful response, has newer blocks, marked healthy, int result response",
			network: &network{
				httpClient: mockHttpClient,
				Providers: map[string]*provider{
					"provider1": {
						upstream: &reverseproxy.Upstream{
							Dial: "provider1",
						},
					},
				},
				latestBlockNumber: 500,
				checkedProviders:  map[string][]healthCheckEntry{},
				prometheusClient:  mockPrometheusClient,
				logger:            zap.NewNop(),
			},
			latestBlockResponse: postResponse{
				postResponseBytes: []byte(`{"jsonrpc": "2.0", "id": 1,"result": 600}`),
				statusCode:        200,
				err:               nil,
			},
			want: map[string]*provider{
				"provider1": {
					healthStatus: Healthy,
				},
			},
		},
		{
			name: "1 provider, successful response, 429 too many request status, mark warning",
			network: &network{
				httpClient: mockHttpClient,
				Providers: map[string]*provider{
					"provider1": {
						upstream: &reverseproxy.Upstream{
							Dial: "provider1",
						},
					},
				},
				latestBlockNumber: 5000000,
				checkedProviders:  map[string][]healthCheckEntry{},
				prometheusClient:  mockPrometheusClient,
				logger:            zap.NewNop(),
			},
			latestBlockResponse: postResponse{
				postResponseBytes: []byte(`{"jsonrpc": "2.0", "id": 1,"result": "0x60497d"}`),
				statusCode:        429,
				err:               nil,
			},
			want: map[string]*provider{
				"provider1": {
					healthStatus: Warning,
				},
			},
		},
		{
			name: "1 provider, GetlatestBlockNumber fails, marked unhealthy",
			network: &network{
				httpClient: mockHttpClient,
				Providers: map[string]*provider{
					"provider1": {
						upstream: &reverseproxy.Upstream{
							Dial: "provider1",
						},
					},
				},
				latestBlockNumber: 20,
				checkedProviders:  map[string][]healthCheckEntry{},
				prometheusClient:  mockPrometheusClient,
				logger:            zap.NewNop(),
			},
			latestBlockResponse: postResponse{
				postResponseBytes: nil,
				statusCode:        200,
				err:               errors.New(""),
			},
			want: map[string]*provider{
				"provider1": {
					healthStatus: Unhealthy,
				},
			},
		},
		{
			name: "1 provider, successful response, error code 400 marked unhealthy",
			network: &network{
				httpClient: mockHttpClient,
				Providers: map[string]*provider{
					"provider1": {
						upstream: &reverseproxy.Upstream{
							Dial: "provider1",
						},
					},
				},
				latestBlockNumber: 30,
				checkedProviders:  map[string][]healthCheckEntry{},
				prometheusClient:  mockPrometheusClient,
				logger:            zap.NewNop(),
			},
			latestBlockResponse: postResponse{
				postResponseBytes: nil,
				statusCode:        400,
				err:               nil,
			},
			want: map[string]*provider{
				"provider1": {
					healthStatus: Unhealthy,
				},
			},
		},
		{
			name: "1 provider, successful response, has equal block number, marked healthy",
			network: &network{
				httpClient: mockHttpClient,
				Providers: map[string]*provider{
					"provider1": {
						upstream: &reverseproxy.Upstream{
							Dial: "provider1",
						},
					},
				},
				latestBlockNumber: 6310269,
				checkedProviders:  map[string][]healthCheckEntry{},
				prometheusClient:  mockPrometheusClient,
				logger:            zap.NewNop(),
			},
			latestBlockResponse: postResponse{
				postResponseBytes: []byte(`{"jsonrpc": "2.0", "id": 1,"result": "0x60497d"}`),
				statusCode:        200,
				err:               nil,
			},
			want: map[string]*provider{
				"provider1": {
					healthStatus: Healthy,
				},
			},
		},
		{
			name: "1 provider, successful response, has smaller block number, marked warning",
			network: &network{
				httpClient: mockHttpClient,
				Providers: map[string]*provider{
					"provider1": {
						upstream: &reverseproxy.Upstream{
							Dial: "provider1",
						},
					},
				},
				latestBlockNumber: 7310269,
				checkedProviders:  map[string][]healthCheckEntry{},
				prometheusClient:  mockPrometheusClient,
				logger:            zap.NewNop(),
			},
			latestBlockResponse: postResponse{
				postResponseBytes: []byte(`{"jsonrpc": "2.0", "id": 1,"result": "0x60497d"}`),
				statusCode:        200,
				err:               nil,
			},
			want: map[string]*provider{
				"provider1": {
					healthStatus: Warning,
				},
			},
		},
		{
			name: "2 providers, successful response, both have newer blocks, both marked healthy",
			network: &network{
				httpClient: mockHttpClient,
				Providers: map[string]*provider{
					"provider1": {
						upstream: &reverseproxy.Upstream{
							Dial: "provider1",
						},
					},
					"provider2": {
						upstream: &reverseproxy.Upstream{
							Dial: "provider2",
						},
					},
				},
				latestBlockNumber: 5310269,
				checkedProviders:  map[string][]healthCheckEntry{},
				prometheusClient:  mockPrometheusClient,
				logger:            zap.NewNop(),
			},
			latestBlockResponse: postResponse{
				postResponseBytes: []byte(`{"jsonrpc": "2.0", "id": 1,"result": "0x60497d"}`),
				statusCode:        200,
				err:               nil,
			},
			want: map[string]*provider{
				"provider1": {
					healthStatus: Healthy,
				},
				"provider2": {
					healthStatus: Healthy,
				},
			},
		},
		{
			name: "2 providers, successful response, both have equal blocks, both marked healthy",
			network: &network{
				httpClient: mockHttpClient,
				Providers: map[string]*provider{
					"provider1": {
						upstream: &reverseproxy.Upstream{
							Dial: "provider1",
						},
					},
					"provider2": {
						upstream: &reverseproxy.Upstream{
							Dial: "provider2",
						},
					},
				},
				latestBlockNumber: 6310269,
				checkedProviders:  map[string][]healthCheckEntry{},
				prometheusClient:  mockPrometheusClient,
				logger:            zap.NewNop(),
			},
			latestBlockResponse: postResponse{
				postResponseBytes: []byte(`{"jsonrpc": "2.0", "id": 1,"result": "0x60497d"}`),
				statusCode:        200,
				err:               nil,
			},
			want: map[string]*provider{
				"provider1": {
					healthStatus: Healthy,
				},
				"provider2": {
					healthStatus: Healthy,
				},
			},
		},
		{
			name: "2 providers, successful response, both have older blocks, both marked warning",
			network: &network{
				httpClient: mockHttpClient,
				Providers: map[string]*provider{
					"provider1": {
						upstream: &reverseproxy.Upstream{
							Dial: "provider1",
						},
					},
					"provider2": {
						upstream: &reverseproxy.Upstream{
							Dial: "provider2",
						},
					},
				},
				latestBlockNumber: 7310269,
				checkedProviders:  map[string][]healthCheckEntry{},
				prometheusClient:  mockPrometheusClient,
				logger:            zap.NewNop(),
			},
			latestBlockResponse: postResponse{
				postResponseBytes: []byte(`{"jsonrpc": "2.0", "id": 1,"result": "0x60497d"}`),
				statusCode:        200,
				err:               nil,
			},
			want: map[string]*provider{
				"provider1": {
					healthStatus: Warning,
				},
				"provider2": {
					healthStatus: Warning,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockHttpClient.EXPECT().Post(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(tt.latestBlockResponse.postResponseBytes, &tt.latestBlockResponse.statusCode, tt.latestBlockResponse.err).Times(len(tt.network.Providers))
			mockPrometheusClient.EXPECT().HandleLatestBlockMetric(gomock.Any()).Times(len(tt.network.Providers)).Times(len(tt.network.Providers))

			tt.network.healthCheck()

			for providerName, provider := range tt.network.Providers {
				if provider.healthStatus != tt.want[providerName].healthStatus {
					t.Errorf("network.healthCheck() %s for %v  = %v, want %v", tt.name, providerName, provider.healthStatus, tt.want[providerName].healthStatus)
				}
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
				checkedProviders: map[string][]healthCheckEntry{},
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
				checkedProviders: map[string][]healthCheckEntry{
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
				checkedProviders: map[string][]healthCheckEntry{
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

			if len(tt.network.checkedProviders[tt.providerName]) != len(tt.want) {
				t.Errorf("network.addHealthCheckToCheckedProviderList() for %v  = %v, want %v", tt.providerName, len(tt.network.checkedProviders[tt.providerName]), len(tt.want))
			}
			if len(tt.want) > 0 {
				if tt.network.checkedProviders[tt.providerName][0].blockNumber != tt.want[0].blockNumber {
					t.Errorf("network.addHealthCheckToCheckedProviderList() for %v  = %v, want %v", tt.providerName, tt.network.checkedProviders[tt.providerName][0].blockNumber, tt.want[0].blockNumber)
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
				checkedProviders: map[string][]healthCheckEntry{
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
				checkedProviders: map[string][]healthCheckEntry{
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
				checkedProviders: map[string][]healthCheckEntry{
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
