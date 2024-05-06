package modules

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	reflect "reflect"
	"testing"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
)

func TestSelectCaddyModule(t *testing.T) {
	dinSelect := new(DinSelect)

	tests := []struct {
		name   string
		output caddy.ModuleInfo
	}{
		{
			name: "TestSelectCaddyModuleInit",
			output: caddy.ModuleInfo{
				ID:  "http.reverse_proxy.selection_policies.dinupstreams",
				New: func() caddy.Module { return new(DinSelect) },
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modInfo := dinSelect.CaddyModule()
			if modInfo.ID != tt.output.ID {
				t.Errorf("CaddyModule() = %v, want %v", modInfo.ID, tt.output.ID)
			}
			if reflect.TypeOf(modInfo.New()) != reflect.TypeOf(tt.output.New()) {
				t.Errorf("CaddyModule() = %v, want %v", modInfo.New(), tt.output.New())
			}
		})
	}
}

func TestDinSelect(t *testing.T) {
	dinSelect := new(DinSelect)
	dinSelect.CaddyModule()
	selector := new(reverseproxy.HeaderHashSelection)
	selector.Field = "Din-Session-Id"
	selector.FallbackRaw = json.RawMessage(`{"policy":"random"}`)
	selector.Provision(caddy.Context{})
	dinSelect.selector = selector

	upstream1 := &reverseproxy.Upstream{
		Dial: "localhost:8000",
	}
	upstream2 := &reverseproxy.Upstream{
		Dial: "localhost:8001",
	}

	tests := []struct {
		name                     string
		pool                     reverseproxy.UpstreamPool
		r                        *http.Request
		rw                       http.ResponseWriter
		replacerUpstreamWrappers []*upstreamWrapper
		output                   *reverseproxy.Upstream
	}{
		{
			name: "TestDinSelect successful",
			pool: reverseproxy.UpstreamPool{upstream1, upstream2},
			r:    httptest.NewRequest("GET", "http://localhost:8000/eth", nil),
			rw:   httptest.NewRecorder(),
			replacerUpstreamWrappers: []*upstreamWrapper{
				{
					upstream: upstream1,
					path:     "/eth",
				},
				{
					upstream: upstream2,
					path:     "/eth",
				},
			},
			output: &reverseproxy.Upstream{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.r = tt.r.WithContext(context.WithValue(tt.r.Context(), caddy.ReplacerCtxKey, caddy.NewReplacer()))
			repl := tt.r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
			repl.Set(DinUpstreamsContextKey, tt.replacerUpstreamWrappers)

			upstream := dinSelect.Select(tt.pool, tt.r, tt.rw)
			if upstream != tt.output {
				t.Errorf("Select() = %v, want %v", upstream, tt.output)
			}
		})
	}
}
