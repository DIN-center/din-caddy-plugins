package modules

import (
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"github.com/golang/mock/gomock"
	din_http "github.com/openrelayxyz/din-caddy-plugins/lib/http"
	prom "github.com/openrelayxyz/din-caddy-plugins/lib/prometheus"
	"github.com/pkg/errors"
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
		service             *service
		latestBlockResponse postResponse
		want                map[string]*provider
	}{
		{
			name: "1 provider, successful response, has newer blocks, marked healthy",
			service: &service{
				HTTPClient: mockHttpClient,
				Providers: map[string]*provider{
					"provider1": {
						upstream: &reverseproxy.Upstream{
							Dial: "provider1",
						},
					},
				},
				LatestBlockNumber: 5000000,
				CheckedProviders:  map[string][]healthCheckEntry{},
				PrometheusClient:  mockPrometheusClient,
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
			service: &service{
				HTTPClient: mockHttpClient,
				Providers: map[string]*provider{
					"provider1": {
						upstream: &reverseproxy.Upstream{
							Dial: "provider1",
						},
					},
				},
				LatestBlockNumber: 500,
				CheckedProviders:  map[string][]healthCheckEntry{},
				PrometheusClient:  mockPrometheusClient,
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
			service: &service{
				HTTPClient: mockHttpClient,
				Providers: map[string]*provider{
					"provider1": {
						upstream: &reverseproxy.Upstream{
							Dial: "provider1",
						},
					},
				},
				LatestBlockNumber: 5000000,
				CheckedProviders:  map[string][]healthCheckEntry{},
				PrometheusClient:  mockPrometheusClient,
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
			name: "1 provider, GetLatestBlockNumber fails, marked unhealthy",
			service: &service{
				HTTPClient: mockHttpClient,
				Providers: map[string]*provider{
					"provider1": {
						upstream: &reverseproxy.Upstream{
							Dial: "provider1",
						},
					},
				},
				LatestBlockNumber: 20,
				CheckedProviders:  map[string][]healthCheckEntry{},
				PrometheusClient:  mockPrometheusClient,
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
			service: &service{
				HTTPClient: mockHttpClient,
				Providers: map[string]*provider{
					"provider1": {
						upstream: &reverseproxy.Upstream{
							Dial: "provider1",
						},
					},
				},
				LatestBlockNumber: 30,
				CheckedProviders:  map[string][]healthCheckEntry{},
				PrometheusClient:  mockPrometheusClient,
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
			service: &service{
				HTTPClient: mockHttpClient,
				Providers: map[string]*provider{
					"provider1": {
						upstream: &reverseproxy.Upstream{
							Dial: "provider1",
						},
					},
				},
				LatestBlockNumber: 6310269,
				CheckedProviders:  map[string][]healthCheckEntry{},
				PrometheusClient:  mockPrometheusClient,
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
			service: &service{
				HTTPClient: mockHttpClient,
				Providers: map[string]*provider{
					"provider1": {
						upstream: &reverseproxy.Upstream{
							Dial: "provider1",
						},
					},
				},
				LatestBlockNumber: 7310269,
				CheckedProviders:  map[string][]healthCheckEntry{},
				PrometheusClient:  mockPrometheusClient,
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
			service: &service{
				HTTPClient: mockHttpClient,
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
				LatestBlockNumber: 5310269,
				CheckedProviders:  map[string][]healthCheckEntry{},
				PrometheusClient:  mockPrometheusClient,
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
			service: &service{
				HTTPClient: mockHttpClient,
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
				LatestBlockNumber: 6310269,
				CheckedProviders:  map[string][]healthCheckEntry{},
				PrometheusClient:  mockPrometheusClient,
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
			service: &service{
				HTTPClient: mockHttpClient,
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
				LatestBlockNumber: 7310269,
				CheckedProviders:  map[string][]healthCheckEntry{},
				PrometheusClient:  mockPrometheusClient,
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
			mockHttpClient.EXPECT().Post(gomock.Any(), gomock.Any(), gomock.Any()).Return(tt.latestBlockResponse.postResponseBytes, &tt.latestBlockResponse.statusCode, tt.latestBlockResponse.err).Times(len(tt.service.Providers))
			mockPrometheusClient.EXPECT().HandleLatestBlockMetric(gomock.Any()).Times(len(tt.service.Providers))

			tt.service.healthCheck()

			for providerName, provider := range tt.service.Providers {
				if provider.healthStatus != tt.want[providerName].healthStatus {
					t.Errorf("service.healthCheck() for %v  = %v, want %v", providerName, provider.healthStatus, tt.want[providerName].healthStatus)
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
		service          *service
		providerName     string
		healthCheckInput healthCheckEntry
		want             []healthCheckEntry
	}{
		{
			name: "health check entry added to empty list",
			service: &service{
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
			service: &service{
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
			service: &service{
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
			tt.service.addHealthCheckToCheckedProviderList(tt.providerName, tt.healthCheckInput)

			if len(tt.service.CheckedProviders[tt.providerName]) != len(tt.want) {
				t.Errorf("service.addHealthCheckToCheckedProviderList() for %v  = %v, want %v", tt.providerName, len(tt.service.CheckedProviders[tt.providerName]), len(tt.want))
			}
			if len(tt.want) > 0 {
				if tt.service.CheckedProviders[tt.providerName][0].blockNumber != tt.want[0].blockNumber {
					t.Errorf("service.addHealthCheckToCheckedProviderList() for %v  = %v, want %v", tt.providerName, tt.service.CheckedProviders[tt.providerName][0].blockNumber, tt.want[0].blockNumber)
				}
			}
		})
	}
}

func TestEvaluateCheckedProviders(t *testing.T) {

	tests := []struct {
		name    string
		service *service
		want    map[string]*provider
	}{
		{
			name: "1 provider, has older block, marked Warning",
			service: &service{
				Providers: map[string]*provider{
					"provider1": {
						healthStatus: Healthy,
					},
				},
				LatestBlockNumber: 10,
				CheckedProviders: map[string][]healthCheckEntry{
					"provider1": {
						{
							blockNumber: 1,
							timestamp:   nil,
						},
					},
				},
			},
			want: map[string]*provider{
				"provider1": {
					healthStatus: Warning,
				},
			},
		},
		{
			name: "1 provider, has newer block, marked healthy",
			service: &service{
				Providers: map[string]*provider{
					"provider1": {
						healthStatus: Healthy,
					},
				},
				LatestBlockNumber: 10,
				CheckedProviders: map[string][]healthCheckEntry{
					"provider1": {
						{
							blockNumber: 20,
							timestamp:   nil,
						},
					},
				},
			},
			want: map[string]*provider{
				"provider1": {
					healthStatus: Healthy,
				},
			},
		},
		{
			name: "1 provider, has equal block, marked healthy",
			service: &service{
				Providers: map[string]*provider{
					"provider1": {
						healthStatus: Healthy,
					},
				},
				LatestBlockNumber: 10,
				CheckedProviders: map[string][]healthCheckEntry{
					"provider1": {
						{
							blockNumber: 10,
							timestamp:   nil,
						},
					},
				},
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
			tt.service.evaluateCheckedProviders()

			for providerName, provider := range tt.service.Providers {
				if provider.healthStatus != tt.want[providerName].healthStatus {
					t.Errorf("service.evaluateCheckedProviders() for %v  = %v, want %v", providerName, provider.healthStatus, tt.want[providerName].healthStatus)
				}
			}
		})
	}
}
