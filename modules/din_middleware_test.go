package modules

import (
	"context"
	"net/http"
	"net/http/httptest"
	reflect "reflect"
	"testing"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
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

	test := []struct {
		name     string
		request  *http.Request
		services map[string]*service
		hasErr   bool
	}{
		{
			name:    "successful request",
			request: httptest.NewRequest("GET", "http://localhost:8000/eth", nil),
			services: map[string]*service{
				"eth": {
					Name:      "eth",
					Runtime:   "ethereum",
					Providers: map[string]*provider{},
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

			err := dinMiddleware.ServeHTTP(rw, tt.request, caddyhttp.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error { return nil }))
			if err != nil && !tt.hasErr {
				t.Errorf("ServeHTTP() = %v, want %v", err, tt.hasErr)
			}
		})
	}
}

func TestDinMiddlewareProvision(t *testing.T) {
	dinMiddleware := new(DinMiddleware)

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
					Runtime:     "ethereum",
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
				},
			},
			hasErr: false,
		},
		{
			name: "Provision() populated 2 service, 1 upstreams successful",
			services: map[string]*service{
				"eth": {
					Name:        "eth",
					Runtime:     "ethereum",
					HCThreshold: 2,
					HCInterval:  5,
					Providers: map[string]*provider{
						"localhost:8000": {
							HttpUrl: "http://localhost:8000/eth",
						},
					},
					CheckedProviders: map[string][]healthCheckEntry{},
				},
				"starknet-mainnet": {
					Name:        "eth",
					Runtime:     StarknetRuntime,
					HCThreshold: 2,
					HCInterval:  5,
					Providers: map[string]*provider{
						"localhost:8000": {
							HttpUrl: "http://localhost:8000/starknet-mainnet",
						},
					},
					CheckedProviders: map[string][]healthCheckEntry{},
				},
			},
			hasErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dinMiddleware.Services = tt.services
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
