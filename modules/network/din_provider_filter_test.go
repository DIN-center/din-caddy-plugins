package network

import (
	"testing"

	dinHttp "github.com/DIN-center/din-caddy-plugins/lib/http"
)

func TestFilterProviders(t *testing.T) {
	provider1 := &provider{HttpUrl: "http://p1.com", Methods: map[string]struct{}{"eth_call": {}, "eth_blockNumber": {}}}
	provider2 := &provider{HttpUrl: "http://p2.com", Methods: map[string]struct{}{"eth_sendTransaction": {}, "eth_blockNumber": {}}}
	provider3 := &provider{HttpUrl: "http://p3.com", Methods: map[string]struct{}{"eth_call": {}}}

	allProviders := map[string]*provider{
		"p1": provider1,
		"p2": provider2,
		"p3": provider3,
	}

	tests := []struct {
		name              string
		mf                *methodFilter
		request           *dinHttp.JSONRPCRequest
		providersIn       map[string]*provider
		expectedProviders map[string]*provider
	}{
		{
			name:              "No filtered methods",
			mf:                &methodFilter{FilteredMethods: map[string]struct{}{}},
			request:           &dinHttp.JSONRPCRequest{Method: "eth_call"},
			providersIn:       allProviders,
			expectedProviders: allProviders,
		},
		{
			name:              "Empty request method",
			mf:                &methodFilter{FilteredMethods: map[string]struct{}{"eth_call": {}}},
			request:           &dinHttp.JSONRPCRequest{Method: ""},
			providersIn:       allProviders,
			expectedProviders: allProviders,
		},
		{
			name:              "Method not in filtered methods",
			mf:                &methodFilter{FilteredMethods: map[string]struct{}{"eth_blockNumber": {}}},
			request:           &dinHttp.JSONRPCRequest{Method: "eth_call"},
			providersIn:       allProviders,
			expectedProviders: allProviders,
		},
		{
			name:        "Method in filtered methods, some providers support it",
			mf:          &methodFilter{FilteredMethods: map[string]struct{}{"eth_call": {}}},
			request:     &dinHttp.JSONRPCRequest{Method: "eth_call"},
			providersIn: allProviders,
			expectedProviders: map[string]*provider{
				"p1": provider1,
				"p3": provider3,
			},
		},
		{
			name:              "Method in filtered methods, no providers support it",
			mf:                &methodFilter{FilteredMethods: map[string]struct{}{"trace_block": {}}},
			request:           &dinHttp.JSONRPCRequest{Method: "trace_block"},
			providersIn:       allProviders,
			expectedProviders: allProviders, // Current logic returns all providers if no specific provider found
		},
		{
			name:        "Method in filtered methods, all providers support it (for eth_blockNumber)",
			mf:          &methodFilter{FilteredMethods: map[string]struct{}{"eth_blockNumber": {}}},
			request:     &dinHttp.JSONRPCRequest{Method: "eth_blockNumber"},
			providersIn: allProviders,
			expectedProviders: map[string]*provider{
				"p1": provider1,
				"p2": provider2,
			},
		},
		{
			name:        "Method in filtered methods, one provider supports it",
			mf:          &methodFilter{FilteredMethods: map[string]struct{}{"eth_sendTransaction": {}}},
			request:     &dinHttp.JSONRPCRequest{Method: "eth_sendTransaction"},
			providersIn: allProviders,
			expectedProviders: map[string]*provider{
				"p2": provider2,
			},
		},
		{
			name:              "Nil providers map input",
			mf:                &methodFilter{FilteredMethods: map[string]struct{}{"eth_call": {}}},
			request:           &dinHttp.JSONRPCRequest{Method: "eth_call"},
			providersIn:       nil,
			expectedProviders: nil,
		},
		{
			name:              "Empty providers map input",
			mf:                &methodFilter{FilteredMethods: map[string]struct{}{"eth_call": {}}},
			request:           &dinHttp.JSONRPCRequest{Method: "eth_call"},
			providersIn:       map[string]*provider{},
			expectedProviders: map[string]*provider{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := tt.mf.FilterProviders(tt.request, tt.providersIn)
			if !areProviderMapsEqual(filtered, tt.expectedProviders) {
				t.Errorf("FilterProviders() got = %v, want %v", filtered, tt.expectedProviders)
			}
		})
	}
}

// areProviderMapsEqual is a helper function to compare two maps of type map[string]*provider.
// It checks for nil equality, length equality, and then key-value pointer equality.
func areProviderMapsEqual(map1, map2 map[string]*provider) bool {
	// If one map is nil and the other is not, they are not equal.
	if (map1 == nil) != (map2 == nil) {
		return false
	}
	// If both are nil (or by extension, if the above check passes and one is nil, both must be), they are equal.
	if map1 == nil {
		return true
	}
	// At this point, both maps are non-nil. Check if their lengths are different.
	if len(map1) != len(map2) {
		return false
	}
	// Iterate over the first map and check for key existence and pointer equality in the second map.
	for key, val1 := range map1 {
		val2, ok := map2[key]
		// If key doesn't exist in map2, or if the provider pointers are different, maps are not equal.
		if !ok || val1 != val2 {
			return false
		}
	}
	// If all checks pass, the maps are considered equal for the purpose of this test.
	return true
}
