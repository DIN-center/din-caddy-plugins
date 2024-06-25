package modules

import (
	"context"
	"net/http"
	reflect "reflect"
	"testing"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
)

func TestUpstreamsCaddyModule(t *testing.T) {
	dinUpstreams := new(DinUpstreams)

	tests := []struct {
		name   string
		output caddy.ModuleInfo
	}{
		{
			name: "TestUpstreamsCaddyModuleInit",
			output: caddy.ModuleInfo{
				ID:  "http.reverse_proxy.upstreams.din_reverse_proxy_policy",
				New: func() caddy.Module { return new(DinUpstreams) },
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modInfo := dinUpstreams.CaddyModule()
			if modInfo.ID != tt.output.ID {
				t.Errorf("CaddyModule() = %v, want %v", modInfo.ID, tt.output.ID)
			}
			if reflect.TypeOf(modInfo.New()) != reflect.TypeOf(tt.output.New()) {
				t.Errorf("CaddyModule() = %v, want %v", modInfo.New(), tt.output.New())
			}
		})
	}
}

func TestGetDinUpstreams(t *testing.T) {
	dinUpstreams := new(DinUpstreams)

	upstream1 := &reverseproxy.Upstream{
		Dial: "localhost:8000",
	}
	upstream2 := &reverseproxy.Upstream{
		Dial: "localhost:8001",
	}

	tests := []struct {
		name              string
		request           *http.Request
		replacerProviders map[string]*provider
		output            []*reverseproxy.Upstream
	}{
		{
			name:    "TestGetDinUpstreams successful, both 0 priority",
			request: &http.Request{},
			replacerProviders: map[string]*provider{
				upstream1.Dial: {
					upstream: upstream1,
					Priority: 0,
				},
				upstream2.Dial: {
					upstream: upstream2,
					Priority: 0,
				},
			},
			output: []*reverseproxy.Upstream{upstream1, upstream2},
		},
		{
			name:    "TestGetDinUpstreams successful, both 0 priority and healthy",
			request: &http.Request{},
			replacerProviders: map[string]*provider{
				upstream1.Dial: {
					upstream:     upstream1,
					Priority:     0,
					healthStatus: Healthy,
				},
				upstream2.Dial: {
					upstream:     upstream2,
					Priority:     0,
					healthStatus: Healthy,
				},
			},
			output: []*reverseproxy.Upstream{upstream1, upstream2},
		},
		{
			name:    "TestGetDinUpstreams successful, both 0 priority and 1 is healthy",
			request: &http.Request{},
			replacerProviders: map[string]*provider{
				upstream1.Dial: {
					upstream:     upstream1,
					Priority:     0,
					healthStatus: Healthy,
				},
				upstream2.Dial: {
					upstream:     upstream2,
					Priority:     0,
					healthStatus: Warning,
				},
			},
			output: []*reverseproxy.Upstream{upstream1},
		},
		{
			name:    "TestGetDinUpstreams successful, both 0 priority and both are warning",
			request: &http.Request{},
			replacerProviders: map[string]*provider{
				upstream1.Dial: {
					upstream:     upstream1,
					Priority:     0,
					healthStatus: Warning,
				},
				upstream2.Dial: {
					upstream:     upstream2,
					Priority:     0,
					healthStatus: Warning,
				},
			},
			output: []*reverseproxy.Upstream{upstream1, upstream2},
		},
		{
			name:    "TestGetDinUpstreams successful, both 0 priority and one is Warning the other is Unhealthy",
			request: &http.Request{},
			replacerProviders: map[string]*provider{
				upstream1.Dial: {
					upstream:     upstream1,
					Priority:     0,
					healthStatus: Warning,
				},
				upstream2.Dial: {
					upstream:     upstream2,
					Priority:     0,
					healthStatus: Unhealthy,
				},
			},
			output: []*reverseproxy.Upstream{upstream1},
		},
		{
			name:    "TestGetDinUpstreams successful, both 1 priority",
			request: &http.Request{},
			replacerProviders: map[string]*provider{
				upstream1.Dial: {
					upstream: upstream1,
					Priority: 1,
				},
				upstream2.Dial: {
					upstream: upstream2,
					Priority: 1,
				},
			},
			output: []*reverseproxy.Upstream{upstream1, upstream2},
		},
		{
			name:    "TestGetDinUpstreams successful, different priorities",
			request: &http.Request{},
			replacerProviders: map[string]*provider{
				upstream1.Dial: {
					upstream:     upstream1,
					Priority:     0,
					healthStatus: Healthy,
				},
				upstream2.Dial: {
					upstream:     upstream2,
					Priority:     1,
					healthStatus: Healthy,
				},
			},
			output: []*reverseproxy.Upstream{upstream1},
		},
		{
			name:    "TestGetDinUpstreams successful, different priorities, different health statues",
			request: &http.Request{},
			replacerProviders: map[string]*provider{
				upstream1.Dial: {
					upstream:     upstream1,
					Priority:     0,
					healthStatus: Warning,
				},
				upstream2.Dial: {
					upstream:     upstream2,
					Priority:     1,
					healthStatus: Healthy,
				},
			},
			output: []*reverseproxy.Upstream{upstream2},
		},
		{
			name:    "TestGetDinUpstreams successful, different priorities, unhealthy health statuses",
			request: &http.Request{},
			replacerProviders: map[string]*provider{
				upstream1.Dial: {
					upstream:     upstream1,
					Priority:     0,
					healthStatus: Unhealthy,
				},
				upstream2.Dial: {
					upstream:     upstream2,
					Priority:     1,
					healthStatus: Unhealthy,
				},
			},
			output: []*reverseproxy.Upstream{},
		},
		{
			name:              "TestGetDinUpstreams succesful, no priorities",
			request:           &http.Request{},
			replacerProviders: map[string]*provider{},
			output:            []*reverseproxy.Upstream{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.request = tt.request.WithContext(context.WithValue(tt.request.Context(), caddy.ReplacerCtxKey, caddy.NewReplacer()))
			repl := tt.request.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
			repl.Set(DinUpstreamsContextKey, tt.replacerProviders)

			upstreams, _ := dinUpstreams.GetUpstreams(tt.request)
			if len(upstreams) != len(tt.output) {
				t.Errorf("GetUpstreams() = %v, want %v", len(upstreams), len(tt.output))
			}
		})
	}
}
