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
		name                     string
		request                  *http.Request
		replacerUpstreamWrappers []*upstreamWrapper
		output                   []*reverseproxy.Upstream
	}{
		{
			name:    "TestGetDinUpstreams succesful, both 0 priority",
			request: &http.Request{},
			replacerUpstreamWrappers: []*upstreamWrapper{
				{
					upstream: upstream1,
					Priority: 0,
				},
				{
					upstream: upstream2,
					Priority: 0,
				},
			},
			output: []*reverseproxy.Upstream{upstream1, upstream2},
		},
		{
			name:    "TestGetDinUpstreams succesful, both 2 priority",
			request: &http.Request{},
			replacerUpstreamWrappers: []*upstreamWrapper{
				{
					upstream: upstream1,
					Priority: 2,
				},
				{
					upstream: upstream2,
					Priority: 2,
				},
			},
			output: []*reverseproxy.Upstream{upstream1, upstream2},
		},
		{
			name:    "TestGetDinUpstreams succesful, different priorities",
			request: &http.Request{},
			replacerUpstreamWrappers: []*upstreamWrapper{
				{
					upstream: upstream1,
					Priority: 1,
				},
				{
					upstream: upstream2,
					Priority: 2,
				},
			},
			output: []*reverseproxy.Upstream{upstream1},
		},
		{
			name:                     "TestGetDinUpstreams succesful, no priorities",
			request:                  &http.Request{},
			replacerUpstreamWrappers: []*upstreamWrapper{},
			output:                   []*reverseproxy.Upstream{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.request = tt.request.WithContext(context.WithValue(tt.request.Context(), caddy.ReplacerCtxKey, caddy.NewReplacer()))
			repl := tt.request.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
			repl.Set(DinUpstreamsContextKey, tt.replacerUpstreamWrappers)

			upstreams, _ := dinUpstreams.GetUpstreams(tt.request)
			if !reflect.DeepEqual(upstreams, tt.output) {
				t.Errorf("GetUpstreams() = %v, want %v", upstreams, tt.output)
			}
		})
	}
}
