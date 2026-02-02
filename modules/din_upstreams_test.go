package modules

import (
	"container/list"
	"context"
	"net/http"
	"net/url"
	reflect "reflect"
	"testing"

	internalnetwork "github.com/DIN-center/din-caddy-plugins/internal/network"
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
	// Set up a test network in the global registry
	testNetwork := &network{
		Name:      "ethereum",
		Providers: make(map[string]*provider),
	}
	internalnetwork.RegisterNetwork("ethereum", testNetwork)
	defer func() {
		internalnetwork.UnregisterNetwork("ethereum")
	}()

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
			name: "TestGetDinUpstreams successful, both 0 Priority",
			request: &http.Request{
				URL: &url.URL{Path: "/ethereum/eth_blockNumber"},
			},
			replacerProviders: map[string]*provider{
				upstream1.Dial: {
					Upstream: upstream1,
					Priority: 0,
					BlockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{BlockNumber: 100, HealthStatus: Healthy})
						return l
					}(),
				},
				upstream2.Dial: {
					Upstream: upstream2,
					Priority: 0,
					BlockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{BlockNumber: 100, HealthStatus: Healthy})
						return l
					}(),
				},
			},
			output: []*reverseproxy.Upstream{upstream1, upstream2},
		},
		{
			name: "TestGetDinUpstreams successful, both 0 Priority and healthy",
			request: &http.Request{
				URL: &url.URL{Path: "/ethereum/eth_blockNumber"},
			},
			replacerProviders: map[string]*provider{
				upstream1.Dial: {
					Upstream: upstream1,
					Priority: 0,
					BlockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{BlockNumber: 100, HealthStatus: Healthy})
						return l
					}(),
				},
				upstream2.Dial: {
					Upstream: upstream2,
					Priority: 0,
					BlockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{BlockNumber: 100, HealthStatus: Healthy})
						return l
					}(),
				},
			},
			output: []*reverseproxy.Upstream{upstream1, upstream2},
		},
		{
			name: "TestGetDinUpstreams successful, both 0 Priority and 1 is healthy",
			request: &http.Request{
				URL: &url.URL{Path: "/ethereum/eth_blockNumber"},
			},
			replacerProviders: map[string]*provider{
				upstream1.Dial: {
					Upstream: upstream1,
					Priority: 0,
					BlockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{BlockNumber: 100, HealthStatus: Healthy})
						return l
					}(),
				},
				upstream2.Dial: {
					Upstream: upstream2,
					Priority: 0,
					BlockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{BlockNumber: 100, HealthStatus: Warning})
						return l
					}(),
				},
			},
			output: []*reverseproxy.Upstream{upstream1},
		},
		{
			name: "TestGetDinUpstreams successful, both 0 Priority and both are warning",
			request: &http.Request{
				URL: &url.URL{Path: "/ethereum/eth_blockNumber"},
			},
			replacerProviders: map[string]*provider{
				upstream1.Dial: {
					Upstream: upstream1,
					Priority: 0,
					BlockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{BlockNumber: 100, HealthStatus: Warning})
						return l
					}(),
				},
				upstream2.Dial: {
					Upstream: upstream2,
					Priority: 0,
					BlockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{BlockNumber: 100, HealthStatus: Warning})
						return l
					}(),
				},
			},
			output: []*reverseproxy.Upstream{upstream1, upstream2},
		},
		{
			name: "TestGetDinUpstreams successful, both 0 Priority and one is Warning the other is Unhealthy",
			request: &http.Request{
				URL: &url.URL{Path: "/ethereum/eth_blockNumber"},
			},
			replacerProviders: map[string]*provider{
				upstream1.Dial: {
					Upstream: upstream1,
					Priority: 0,
					BlockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{BlockNumber: 100, HealthStatus: Warning})
						return l
					}(),
				},
				upstream2.Dial: {
					Upstream: upstream2,
					Priority: 0,
					BlockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{BlockNumber: 100, HealthStatus: Unhealthy})
						return l
					}(),
				},
			},
			output: []*reverseproxy.Upstream{upstream1},
		},
		{
			name: "successful, both 1 Priority",
			request: &http.Request{
				URL: &url.URL{Path: "/ethereum/eth_blockNumber"},
			},
			replacerProviders: map[string]*provider{
				upstream1.Dial: {
					Upstream: upstream1,
					Priority: 1,
					BlockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{BlockNumber: 100, HealthStatus: Healthy})
						return l
					}(),
				},
				upstream2.Dial: {
					Upstream: upstream2,
					Priority: 1,
					BlockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{BlockNumber: 100, HealthStatus: Healthy})
						return l
					}(),
				},
			},
			output: []*reverseproxy.Upstream{upstream1, upstream2},
		},
		{
			name: "TestGetDinUpstreams successful, different priorities",
			request: &http.Request{
				URL: &url.URL{Path: "/ethereum/eth_blockNumber"},
			},
			replacerProviders: map[string]*provider{
				upstream1.Dial: {
					Upstream: upstream1,
					Priority: 0,
					BlockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{BlockNumber: 100, HealthStatus: Healthy})
						return l
					}(),
				},
				upstream2.Dial: {
					Upstream: upstream2,
					Priority: 1,
					BlockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{BlockNumber: 100, HealthStatus: Healthy})
						return l
					}(),
				},
			},
			output: []*reverseproxy.Upstream{upstream1},
		},
		{
			name: "TestGetDinUpstreams successful, different priorities, different health statues",
			request: &http.Request{
				URL: &url.URL{Path: "/ethereum/eth_blockNumber"},
			},
			replacerProviders: map[string]*provider{
				upstream1.Dial: {
					Upstream: upstream1,
					Priority: 0,
					BlockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{BlockNumber: 100, HealthStatus: Warning})
						return l
					}(),
				},
				upstream2.Dial: {
					Upstream: upstream2,
					Priority: 1,
					BlockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{BlockNumber: 100, HealthStatus: Healthy})
						return l
					}(),
				},
			},
			output: []*reverseproxy.Upstream{upstream2},
		},
		{
			name: "TestGetDinUpstreams successful, different priorities, unhealthy health statuses",
			request: &http.Request{
				URL: &url.URL{Path: "/ethereum/eth_blockNumber"},
			},
			replacerProviders: map[string]*provider{
				upstream1.Dial: {
					Upstream: upstream1,
					Priority: 0,
					BlockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{BlockNumber: 100, HealthStatus: Unhealthy})
						return l
					}(),
				},
				upstream2.Dial: {
					Upstream: upstream2,
					Priority: 1,
					BlockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{BlockNumber: 100, HealthStatus: Unhealthy})
						return l
					}(),
				},
			},
			output: []*reverseproxy.Upstream{},
		},
		{
			name: "TestGetDinUpstreams successful, no priorities",
			request: &http.Request{
				URL: &url.URL{Path: "/ethereum/eth_blockNumber"},
			},
			replacerProviders: map[string]*provider{},
			output:            []*reverseproxy.Upstream{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Update the test network with the providers for this test case
			testNetwork.Providers = tt.replacerProviders

			tt.request = tt.request.WithContext(context.WithValue(tt.request.Context(), caddy.ReplacerCtxKey, caddy.NewReplacer()))
			repl := tt.request.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
			repl.Set(DinUpstreamsContextKey, tt.replacerProviders)

			upstreams, _ := dinUpstreams.GetUpstreams(tt.request)
			if len(upstreams) != len(tt.output) {
				t.Errorf("GetUpstreams() = %v, want %v", len(upstreams), len(tt.output))
			} else {
				// Create maps of Dial addresses for easier comparison
				actualDials := make(map[string]bool)
				expectedDials := make(map[string]bool)

				for _, upstream := range upstreams {
					actualDials[upstream.Dial] = true
				}

				for _, upstream := range tt.output {
					expectedDials[upstream.Dial] = true
				}

				// Compare the maps instead of the ordered slices
				for dial := range expectedDials {
					if !actualDials[dial] {
						t.Errorf("GetUpstreams() missing expected Upstream: %v", dial)
					}
				}

				for dial := range actualDials {
					if !expectedDials[dial] {
						t.Errorf("GetUpstreams() contains unexpected Upstream: %v", dial)
					}
				}
			}
		})
	}
}
