package modules

import (
	"context"
	"net/http"
	"net/http/httptest"
	reflect "reflect"
	"testing"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"

	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
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
		services map[string][]*upstreamWrapper
		hasErr   bool
	}{
		{
			name:    "successful request",
			request: httptest.NewRequest("GET", "http://localhost:8000/eth", nil),
			services: map[string][]*upstreamWrapper{
				"eth": {
					&upstreamWrapper{},
				},
			},
			hasErr: false,
		},
		{
			name:    "unsuccessful request, path not found",
			request: httptest.NewRequest("GET", "http://localhost:8000/xxx", nil),
			services: map[string][]*upstreamWrapper{
				"eth": {
					&upstreamWrapper{},
				},
			},
			hasErr: true,
		},
		{
			name:     "unsuccessful request, service map is empty",
			request:  httptest.NewRequest("GET", "http://localhost:8000/eth", nil),
			services: map[string][]*upstreamWrapper{},
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

func TestUrlToUpstreamWrapper(t *testing.T) {
	tests := []struct {
		name   string
		urlstr string
		outPut *upstreamWrapper
		hasErr bool
	}{
		{
			name:   "passing localhost",
			urlstr: "http://localhost:8080",
			outPut: &upstreamWrapper{
				HttpUrl:  "http://localhost:8080",
				path:     "",
				Headers:  make(map[string]string),
				upstream: &reverseproxy.Upstream{Dial: "localhost:8080"},
				Priority: 0,
			},
			hasErr: false,
		},
		{
			name:   "passing fullurl with key",
			urlstr: "https://eth.rpc.test.cloud:443/key",
			outPut: &upstreamWrapper{
				HttpUrl:  "https://eth.rpc.test.cloud:443/key",
				path:     "/key",
				Headers:  make(map[string]string),
				upstream: &reverseproxy.Upstream{Dial: "eth.rpc.test.cloud:443"},
				Priority: 0,
			},
			hasErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstreamWrapper, err := urlToUpstreamWrapper(tt.urlstr)
			if err != nil && !tt.hasErr {
				t.Errorf("urlToUpstreamWrapper() = %v, want %v", err, tt.hasErr)
			}
			if !reflect.DeepEqual(upstreamWrapper, tt.outPut) {
				t.Errorf("urlToUpstreamWrapper() = %v, want %v", upstreamWrapper, tt.outPut)
			}
		})
	}
}