package modules

import (
	"errors"
	"testing"

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
			},
			latestBlockResponse: latestBlockResponse{
				latestBlockNumber: 11,
				statusCode:        200,
				err:               nil,
			},
			want: map[string]*provider{
				"provider1": {
					healthy: true,
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
			},
			latestBlockResponse: latestBlockResponse{
				latestBlockNumber: 0,
				statusCode:        200,
				err:               errors.New(""),
			},
			want: map[string]*provider{
				"provider1": {
					healthy: false,
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
			},
			latestBlockResponse: latestBlockResponse{
				latestBlockNumber: 0,
				statusCode:        400,
				err:               nil,
			},
			want: map[string]*provider{
				"provider1": {
					healthy: false,
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
			},
			latestBlockResponse: latestBlockResponse{
				latestBlockNumber: 40,
				statusCode:        200,
				err:               nil,
			},
			want: map[string]*provider{
				"provider1": {
					healthy: true,
				},
			},
		},
		{
			name: "1 provider, successful response, has smaller block number, marked unhealthy",
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
			},
			latestBlockResponse: latestBlockResponse{
				latestBlockNumber: 25,
				statusCode:        200,
				err:               nil,
			},
			want: map[string]*provider{
				"provider1": {
					healthy: false,
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
			},
			latestBlockResponse: latestBlockResponse{
				latestBlockNumber: 101,
				statusCode:        200,
				err:               nil,
			},
			want: map[string]*provider{
				"provider1": {
					healthy: true,
				},
				"provider2": {
					healthy: true,
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
			},
			latestBlockResponse: latestBlockResponse{
				latestBlockNumber: 200,
				statusCode:        200,
				err:               nil,
			},
			want: map[string]*provider{
				"provider1": {
					healthy: true,
				},
				"provider2": {
					healthy: true,
				},
			},
		},
		{
			name: "2 providers, successful response, both have older blocks, both marked unhealthy",
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
			},
			latestBlockResponse: latestBlockResponse{
				latestBlockNumber: 299,
				statusCode:        200,
				err:               nil,
			},
			want: map[string]*provider{
				"provider1": {
					healthy: false,
				},
				"provider2": {
					healthy: false,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRuntimeClient.EXPECT().GetLatestBlockNumber(gomock.Any(), gomock.Any()).Return(tt.latestBlockResponse.latestBlockNumber, tt.latestBlockResponse.statusCode, tt.latestBlockResponse.err).Times(len(tt.service.Providers))

			tt.service.healthCheck()

			for providerName, provider := range tt.service.Providers {
				if provider.healthy != tt.want[providerName].healthy {
					t.Errorf("service.healthCheck() for %v  = %v, want %v", providerName, provider.healthy, tt.want[providerName].healthy)
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
			name: "1 provider, has older block, marked unhealthy",
			service: &service{
				Providers: map[string]*provider{
					"provider1": {
						healthy: true,
					},
				},
				LatestBlockNumber: 10,
				checkedProviders: map[string][]healthCheckEntry{
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
					healthy: false,
				},
			},
		},
		{
			name: "1 provider, has newer block, marked healthy",
			service: &service{
				Providers: map[string]*provider{
					"provider1": {
						healthy: true,
					},
				},
				LatestBlockNumber: 10,
				checkedProviders: map[string][]healthCheckEntry{
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
					healthy: true,
				},
			},
		},
		{
			name: "1 provider, has equal block, marked healthy",
			service: &service{
				Providers: map[string]*provider{
					"provider1": {
						healthy: true,
					},
				},
				LatestBlockNumber: 10,
				checkedProviders: map[string][]healthCheckEntry{
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
					healthy: true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.service.evaluateCheckedProviders()

			for providerName, provider := range tt.service.Providers {
				if provider.healthy != tt.want[providerName].healthy {
					t.Errorf("service.evaluateCheckedProviders() for %v  = %v, want %v", providerName, provider.healthy, tt.want[providerName].healthy)
				}
			}
		})
	}
}
