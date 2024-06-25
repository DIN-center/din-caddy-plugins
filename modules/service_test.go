package modules

import (
	"errors"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"github.com/golang/mock/gomock"
	"github.com/openrelayxyz/din-caddy-plugins/lib/runtime"
)

func TestHealthCheck(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockRuntimeClient := runtime.NewMockIRuntimeClient(mockCtrl)
	type latestBlockResponse struct {
		latestBlockNumber int64
		statusCode        int
		err               error
	}

	tests := []struct {
		name                string
		service             *service
		latestBlockResponse latestBlockResponse
		want                map[string]*provider
	}{
		{
			name: "1 provider, successful response, has newer blocks, marked healthy",
			service: &service{
				runtimeClient: mockRuntimeClient,
				Providers: map[string]*provider{
					"provider1": {
						upstream: &reverseproxy.Upstream{
							Dial: "provider1",
						},
					},
				},
				LatestBlockNumber: 10,
				CheckedProviders:  map[string][]healthCheckEntry{},
			},
			latestBlockResponse: latestBlockResponse{
				latestBlockNumber: 11,
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
				runtimeClient: mockRuntimeClient,
				Providers: map[string]*provider{
					"provider1": {
						upstream: &reverseproxy.Upstream{
							Dial: "provider1",
						},
					},
				},
				LatestBlockNumber: 10,
				CheckedProviders:  map[string][]healthCheckEntry{},
			},
			latestBlockResponse: latestBlockResponse{
				latestBlockNumber: 11,
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
				runtimeClient: mockRuntimeClient,
				Providers: map[string]*provider{
					"provider1": {
						upstream: &reverseproxy.Upstream{
							Dial: "provider1",
						},
					},
				},
				LatestBlockNumber: 20,
				CheckedProviders:  map[string][]healthCheckEntry{},
			},
			latestBlockResponse: latestBlockResponse{
				latestBlockNumber: 0,
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
				runtimeClient: mockRuntimeClient,
				Providers: map[string]*provider{
					"provider1": {
						upstream: &reverseproxy.Upstream{
							Dial: "provider1",
						},
					},
				},
				LatestBlockNumber: 30,
				CheckedProviders:  map[string][]healthCheckEntry{},
			},
			latestBlockResponse: latestBlockResponse{
				latestBlockNumber: 0,
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
				runtimeClient: mockRuntimeClient,
				Providers: map[string]*provider{
					"provider1": {
						upstream: &reverseproxy.Upstream{
							Dial: "provider1",
						},
					},
				},
				LatestBlockNumber: 40,
				CheckedProviders:  map[string][]healthCheckEntry{},
			},
			latestBlockResponse: latestBlockResponse{
				latestBlockNumber: 40,
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
				runtimeClient: mockRuntimeClient,
				Providers: map[string]*provider{
					"provider1": {
						upstream: &reverseproxy.Upstream{
							Dial: "provider1",
						},
					},
				},
				LatestBlockNumber: 50,
				CheckedProviders:  map[string][]healthCheckEntry{},
			},
			latestBlockResponse: latestBlockResponse{
				latestBlockNumber: 25,
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
				runtimeClient: mockRuntimeClient,
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
				LatestBlockNumber: 100,
				CheckedProviders:  map[string][]healthCheckEntry{},
			},
			latestBlockResponse: latestBlockResponse{
				latestBlockNumber: 101,
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
				runtimeClient: mockRuntimeClient,
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
				LatestBlockNumber: 200,
				CheckedProviders:  map[string][]healthCheckEntry{},
			},
			latestBlockResponse: latestBlockResponse{
				latestBlockNumber: 200,
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
				runtimeClient: mockRuntimeClient,
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
				LatestBlockNumber: 300,
				CheckedProviders:  map[string][]healthCheckEntry{},
			},
			latestBlockResponse: latestBlockResponse{
				latestBlockNumber: 299,
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
			mockRuntimeClient.EXPECT().GetLatestBlockNumber(gomock.Any(), gomock.Any()).Return(tt.latestBlockResponse.latestBlockNumber, tt.latestBlockResponse.statusCode, tt.latestBlockResponse.err).Times(len(tt.service.Providers))

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
