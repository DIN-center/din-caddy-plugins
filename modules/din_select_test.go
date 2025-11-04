package modules

import (
	"context"
	"crypto/rand"
	"math"
	"net/http"
	reflect "reflect"
	"testing"
	"time"

	ws "github.com/DIN-center/din-caddy-plugins/lib/watcherscore"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"go.uber.org/zap"
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
				ID:  "http.reverse_proxy.selection_policies.din_reverse_proxy_policy",
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

func TestDinSelectUnmarshalCaddyfile(t *testing.T) {
	dinSelect := new(DinSelect)
	err := dinSelect.UnmarshalCaddyfile(&caddyfile.Dispenser{})
	if err != nil {
		t.Errorf("UnmarshalCaddyfile() error = %v, want nil", err)
	}
}

func TestDinSelectSelect(t *testing.T) {

	// Register the DinScoreBasedSelector module
	caddy.RegisterModule(DinScoreBasedSelector{})

	// Create a new context and provision the DinSelect module
	ctx, _ := caddy.NewContext(caddy.Context{Context: context.Background()})
	dinSelect := DinSelect{}
	dinSelect.Provision(ctx)
	dinSelect.logger = zap.NewNop() // suppress logger here because we repeat the test multiple times (too many logs)

	//Fake upstreams
	upstream_foo := &reverseproxy.Upstream{
		Dial: "foo",
	}
	upstream_bar := &reverseproxy.Upstream{
		Dial: "bar",
	}

	type Output struct {
		upstream    *reverseproxy.Upstream
		target_prob float64
	}

	tests := []struct {
		name                        string
		request                     *http.Request
		pool                        reverseproxy.UpstreamPool
		providers                   map[string]*provider
		dynamicLoadBalancingEnabled bool
		repeat                      int
		tolerance                   float64
		output                      []Output
	}{
		{
			name:                        "Respect session affinity (header Din-Session-Id) regardless dynamic load balancing => upstream selected to the same provider",
			request:                     &http.Request{Header: http.Header{"Din-Session-Id": []string{"a0xf-1234567890"}}},
			pool:                        reverseproxy.UpstreamPool{upstream_bar, upstream_foo},
			providers:                   nil,
			dynamicLoadBalancingEnabled: true,
			repeat:                      100,
			tolerance:                   0.00, // 0% error margin, session affinity is ALWAYS deterministic
			output:                      []Output{{upstream_bar, 1.0}},
		},
		{
			name:                        "Random selection when no session affinity and no dynamic load balancing => upstream selected randomly",
			request:                     &http.Request{},
			pool:                        reverseproxy.UpstreamPool{upstream_bar, upstream_foo},
			providers:                   nil,
			dynamicLoadBalancingEnabled: false,
			repeat:                      10000,
			tolerance:                   0.02, // 2% error margin
			output:                      []Output{{upstream_foo, 0.5}, {upstream_bar, 0.5}},
		},
		{
			name:    "Dynamic load balancing => upstream selected based on provider scores",
			request: &http.Request{},
			pool:    reverseproxy.UpstreamPool{upstream_bar, upstream_foo},
			providers: map[string]*provider{
				"bar": {
					upstream: upstream_bar,
					Score:    ws.MustCreateScore(0.92, time.Now().UTC()),
				},
				"foo": {
					upstream: upstream_foo,
					Score:    ws.MustCreateScore(0.67, time.Now().UTC()),
				},
			},
			dynamicLoadBalancingEnabled: true,
			repeat:                      10000,
			tolerance:                   0.02, // 2% error margin
			output:                      []Output{{upstream_bar, 0.5786}, {upstream_foo, 0.4214}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replace crypto/rand with a deterministic reader for the test
			rand.Reader = NewDeterministicReader(1234567890)

			tt.request = tt.request.WithContext(context.WithValue(tt.request.Context(), caddy.ReplacerCtxKey, caddy.NewReplacer()))
			repl := tt.request.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
			repl.Set(DinUpstreamsContextKey, tt.providers)
			repl.Set(DinScoreBasedLoadBalancingContextKey, tt.dynamicLoadBalancingEnabled)

			results := make([]*reverseproxy.Upstream, tt.repeat)
			upstream_count := make(map[*reverseproxy.Upstream]int)
			for i := 0; i < tt.repeat; i++ {
				upstream := dinSelect.Select(tt.pool, tt.request, nil)
				results[i] = upstream
				upstream_count[upstream]++
			}

			if tt.repeat > 1 {
				for _, expected := range tt.output {
					effective_prob := float64(upstream_count[expected.upstream]) / float64(tt.repeat)
					if math.Abs(effective_prob-expected.target_prob) > tt.tolerance {
						t.Errorf("Select with prob = %v, want %v", effective_prob, expected.target_prob)
					}
				}
			} else {
				if tt.output[0].upstream != results[0] {
					t.Errorf("Select() = %v, want %v", results[0], tt.output[0].upstream)
				}
			}
		})
	}
}
