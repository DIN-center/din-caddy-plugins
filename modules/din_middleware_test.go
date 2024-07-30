package modules

import (
	"context"
	"net/http"
	"net/http/httptest"
	reflect "reflect"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/golang/mock/gomock"
	prom "github.com/openrelayxyz/din-caddy-plugins/lib/prometheus"
)

func TestMiddlewareCaddyModule(t *testing.T) {
	dinMiddleware := new(DinMiddleware)

	tests := []struct {
		name   string
		output caddy.ModuleInfo
	}{
		{
			name: "TestMiddlewareCaddyModuleInit",
			output: caddy.ModuleInfo{
				ID:  "http.handlers.din",
				New: func() caddy.Module { return new(DinMiddleware) },
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modInfo := dinMiddleware.CaddyModule()
			if modInfo.ID != tt.output.ID {
				t.Errorf("CaddyModule() = %v, want %v", modInfo.ID, tt.output.ID)
			}
			if reflect.TypeOf(modInfo.New()) != reflect.TypeOf(tt.output.New()) {
				t.Errorf("CaddyModule() = %v, want %v", modInfo.New(), tt.output.New())
			}
		})
	}
}

func TestMiddlewareServeHTTP(t *testing.T) {
	dinMiddleware := new(DinMiddleware)

	now := time.Now()

	test := []struct {
		name     string
		request  *http.Request
		provider string
		services map[string]*service
		hasErr   bool
	}{
		{
			name:     "successful request",
			request:  httptest.NewRequest("GET", "http://localhost:8000/eth", nil),
			provider: "localhost:8000",
			services: map[string]*service{
				"eth": {
					Name: "eth",
					Providers: map[string]*provider{
						"localhost:8000": {
							healthStatus: Healthy,
						},
					},
					CheckedProviders: map[string][]healthCheckEntry{
						"localhost:8000": {
							{
								blockNumber: 1,
								timestamp:   &now,
							},
						},
					},
				},
			},
			hasErr: false,
		},
		{
			name:    "unsuccessful request, path not found",
			request: httptest.NewRequest("GET", "http://localhost:8000/xxx", nil),
			services: map[string]*service{
				"eth": {},
			},
			hasErr: true,
		},
		{
			name:     "unsuccessful request, service map is empty",
			request:  httptest.NewRequest("GET", "http://localhost:8000/eth", nil),
			services: map[string]*service{},
			hasErr:   true,
		},
	}

	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			dinMiddleware.Services = tt.services
			tt.request = tt.request.WithContext(context.WithValue(tt.request.Context(), caddy.ReplacerCtxKey, caddy.NewReplacer()))
			rw := httptest.NewRecorder()

			repl := tt.request.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
			repl.Set(RequestProviderKey, tt.provider)

			err := dinMiddleware.ServeHTTP(rw, tt.request, caddyhttp.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error { return nil }))
			if err != nil && !tt.hasErr {
				t.Errorf("ServeHTTP() = %v, want %v", err, tt.hasErr)
			}
		})
	}
}

func TestDinMiddlewareProvision(t *testing.T) {
	dinMiddleware := new(DinMiddleware)
	mockCtrl := gomock.NewController(t)
	mockPrometheusClient := prom.NewMockIPrometheusClient(mockCtrl)

	tests := []struct {
		name     string
		services map[string]*service
		hasErr   bool
	}{
		{
			name: "Provision() populated 1 service, 2 upstreams successful for ethereum runtime",
			services: map[string]*service{
				"eth": {
					Name:        "eth",
					HCThreshold: 2,
					HCInterval:  5,
					Providers: map[string]*provider{
						"localhost:8000": {
							HttpUrl: "http://localhost:8000/eth",
						},
						"localhost:8001": {
							HttpUrl: "http://localhost:8001/eth",
						},
					},
					CheckedProviders: map[string][]healthCheckEntry{},
					PrometheusClient: mockPrometheusClient,
				},
			},
			hasErr: false,
		},
		{
			name: "Provision() populated 2 service, 1 upstreams successful",
			services: map[string]*service{
				"eth": {
					Name:        "eth",
					HCThreshold: 2,
					HCInterval:  5,
					Providers: map[string]*provider{
						"localhost:8000": {
							HttpUrl: "http://localhost:8000/eth",
						},
					},
					CheckedProviders: map[string][]healthCheckEntry{},
					PrometheusClient: mockPrometheusClient,
				},
				"starknet-mainnet": {
					Name:        "eth",
					HCMethod:    "starknet_blockNumber",
					HCThreshold: 2,
					HCInterval:  5,
					Providers: map[string]*provider{
						"localhost:8000": {
							HttpUrl: "http://localhost:8000/starknet-mainnet",
						},
					},
					CheckedProviders: map[string][]healthCheckEntry{},
					PrometheusClient: mockPrometheusClient,
				},
			},
			hasErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dinMiddleware.Services = tt.services
			mockPrometheusClient.EXPECT().HandleLatestBlockMetric(gomock.Any()).AnyTimes()
			err := dinMiddleware.Provision(caddy.Context{})
			if err != nil && !tt.hasErr {
				t.Errorf("Provision() = %v, want %v", err, tt.hasErr)
			}

			for _, services := range dinMiddleware.Services {
				for _, provider := range services.Providers {
					if provider.upstream.Dial == "" || provider.path == "" {
						t.Errorf("Provision() = %v, want %v", err, tt.hasErr)
					}
				}
			}
		})
	}
}
