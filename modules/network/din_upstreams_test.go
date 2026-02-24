package network

import (
	"container/list"
	"context"
	"net/http"
	"net/url"
	reflect "reflect"
	"testing"

	"github.com/DIN-center/din-caddy-plugins/lib/health"
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
	globalNetworkMutex.Lock()
	globalNetworkRegistry["ethereum"] = testNetwork
	globalNetworkMutex.Unlock()
	defer func() {
		globalNetworkMutex.Lock()
		delete(globalNetworkRegistry, "ethereum")
		globalNetworkMutex.Unlock()
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
					upstream: upstream1,
					Priority: 0,
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: health.Healthy})
						return l
					}(),
				},
				upstream2.Dial: {
					upstream: upstream2,
					Priority: 0,
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: health.Healthy})
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
					upstream: upstream1,
					Priority: 0,
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: health.Healthy})
						return l
					}(),
				},
				upstream2.Dial: {
					upstream: upstream2,
					Priority: 0,
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: health.Healthy})
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
					upstream: upstream1,
					Priority: 0,
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: health.Healthy})
						return l
					}(),
				},
				upstream2.Dial: {
					upstream: upstream2,
					Priority: 0,
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: health.Warning})
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
					upstream: upstream1,
					Priority: 0,
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: health.Warning})
						return l
					}(),
				},
				upstream2.Dial: {
					upstream: upstream2,
					Priority: 0,
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: health.Warning})
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
					upstream: upstream1,
					Priority: 0,
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: health.Warning})
						return l
					}(),
				},
				upstream2.Dial: {
					upstream: upstream2,
					Priority: 0,
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: health.Unhealthy})
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
					upstream: upstream1,
					Priority: 1,
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: health.Healthy})
						return l
					}(),
				},
				upstream2.Dial: {
					upstream: upstream2,
					Priority: 1,
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: health.Healthy})
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
					upstream: upstream1,
					Priority: 0,
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: health.Healthy})
						return l
					}(),
				},
				upstream2.Dial: {
					upstream: upstream2,
					Priority: 1,
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: health.Healthy})
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
					upstream: upstream1,
					Priority: 0,
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: health.Warning})
						return l
					}(),
				},
				upstream2.Dial: {
					upstream: upstream2,
					Priority: 1,
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: health.Healthy})
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
					upstream: upstream1,
					Priority: 0,
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: health.Unhealthy})
						return l
					}(),
				},
				upstream2.Dial: {
					upstream: upstream2,
					Priority: 1,
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: health.Unhealthy})
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
			globalNetworkMutex.Lock()
			testNetwork.Providers = tt.replacerProviders
			globalNetworkMutex.Unlock()

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
						t.Errorf("GetUpstreams() missing expected upstream: %v", dial)
					}
				}

				for dial := range actualDials {
					if !expectedDials[dial] {
						t.Errorf("GetUpstreams() contains unexpected upstream: %v", dial)
					}
				}
			}
		})
	}
}

func TestGetDinUpstreams_WithExcludedProviders(t *testing.T) {
	testNetwork := &network{
		Name:      "ethereum",
		Providers: make(map[string]*provider),
	}
	globalNetworkMutex.Lock()
	globalNetworkRegistry["ethereum"] = testNetwork
	globalNetworkMutex.Unlock()
	defer func() {
		globalNetworkMutex.Lock()
		delete(globalNetworkRegistry, "ethereum")
		globalNetworkMutex.Unlock()
	}()

	dinUpstreams := new(DinUpstreams)

	upstream1 := &reverseproxy.Upstream{Dial: "provider-a:8000"}
	upstream2 := &reverseproxy.Upstream{Dial: "provider-b:8001"}
	upstream3 := &reverseproxy.Upstream{Dial: "provider-c:8002"}

	healthyHistory := func() *list.List {
		l := list.New()
		l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: health.Healthy})
		return l
	}

	tests := []struct {
		name              string
		providers         map[string]*provider
		excludedProviders map[string]struct{}
		expectedCount     int
		expectedDials     []string
	}{
		{
			name: "exclude one provider, other remains",
			providers: map[string]*provider{
				"provider-a:8000": {upstream: upstream1, Priority: 0, host: "provider-a:8000", blockHistory: healthyHistory()},
				"provider-b:8001": {upstream: upstream2, Priority: 0, host: "provider-b:8001", blockHistory: healthyHistory()},
			},
			excludedProviders: map[string]struct{}{"provider-a:8000": {}},
			expectedCount:     1,
			expectedDials:     []string{"provider-b:8001"},
		},
		{
			name: "exclude priority 0 provider, fall through to priority 1",
			providers: map[string]*provider{
				"provider-a:8000": {upstream: upstream1, Priority: 0, host: "provider-a:8000", blockHistory: healthyHistory()},
				"provider-b:8001": {upstream: upstream2, Priority: 1, host: "provider-b:8001", blockHistory: healthyHistory()},
			},
			excludedProviders: map[string]struct{}{"provider-a:8000": {}},
			expectedCount:     1,
			expectedDials:     []string{"provider-b:8001"},
		},
		{
			name: "no exclusions, all providers returned",
			providers: map[string]*provider{
				"provider-a:8000": {upstream: upstream1, Priority: 0, host: "provider-a:8000", blockHistory: healthyHistory()},
				"provider-b:8001": {upstream: upstream2, Priority: 0, host: "provider-b:8001", blockHistory: healthyHistory()},
			},
			excludedProviders: nil,
			expectedCount:     2,
			expectedDials:     []string{"provider-a:8000", "provider-b:8001"},
		},
		{
			name: "all providers excluded, empty pool",
			providers: map[string]*provider{
				"provider-a:8000": {upstream: upstream1, Priority: 0, host: "provider-a:8000", blockHistory: healthyHistory()},
				"provider-b:8001": {upstream: upstream2, Priority: 0, host: "provider-b:8001", blockHistory: healthyHistory()},
			},
			excludedProviders: map[string]struct{}{"provider-a:8000": {}, "provider-b:8001": {}},
			expectedCount:     0,
			expectedDials:     []string{},
		},
		{
			name: "exclude two of three, one remains",
			providers: map[string]*provider{
				"provider-a:8000": {upstream: upstream1, Priority: 0, host: "provider-a:8000", blockHistory: healthyHistory()},
				"provider-b:8001": {upstream: upstream2, Priority: 0, host: "provider-b:8001", blockHistory: healthyHistory()},
				"provider-c:8002": {upstream: upstream3, Priority: 1, host: "provider-c:8002", blockHistory: healthyHistory()},
			},
			excludedProviders: map[string]struct{}{"provider-a:8000": {}, "provider-b:8001": {}},
			expectedCount:     1,
			expectedDials:     []string{"provider-c:8002"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			globalNetworkMutex.Lock()
			testNetwork.Providers = tt.providers
			globalNetworkMutex.Unlock()

			req := &http.Request{URL: &url.URL{Path: "/ethereum/eth_blockNumber"}}
			req = req.WithContext(context.WithValue(req.Context(), caddy.ReplacerCtxKey, caddy.NewReplacer()))
			repl := req.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
			repl.Set(DinUpstreamsContextKey, tt.providers)
			if tt.excludedProviders != nil {
				repl.Set(DinExcludedProvidersContextKey, tt.excludedProviders)
			}

			upstreams, _ := dinUpstreams.GetUpstreams(req)
			if len(upstreams) != tt.expectedCount {
				t.Errorf("GetUpstreams() returned %d upstreams, want %d", len(upstreams), tt.expectedCount)
				return
			}

			actualDials := make(map[string]bool)
			for _, u := range upstreams {
				actualDials[u.Dial] = true
			}
			for _, dial := range tt.expectedDials {
				if !actualDials[dial] {
					t.Errorf("GetUpstreams() missing expected upstream: %v", dial)
				}
			}
		})
	}
}
