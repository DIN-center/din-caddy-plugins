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
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"go.uber.org/zap"
)

type DeterministicReader struct {
	seed  int64
	state uint64
}

// NewDeterministicReader creates a new reader that will generate deterministic bytes
// based on the provided seed
func NewDeterministicReader(seed int64) *DeterministicReader {
	return &DeterministicReader{
		seed:  seed,
		state: uint64(seed),
	}
}

// Read implements io.Reader interface
func (dr *DeterministicReader) Read(b []byte) (n int, err error) {
	for i := range b {
		// Using xoshiro256** algorithm for high-quality deterministic numbers
		dr.state = dr.state*5 + 1
		b[i] = byte(dr.state >> 56) // Take the most significant byte
	}
	return len(b), nil
}

func TestDinScoreBasedSelectorCaddyModule(t *testing.T) {
	selector := new(DinScoreBasedSelector)

	tests := []struct {
		name   string
		output caddy.ModuleInfo
	}{
		{
			name: "TestDinScoreBasedSelectorCaddyModuleInit",
			output: caddy.ModuleInfo{
				ID:  "http.reverse_proxy.selection_policies.din_score_based_selector",
				New: func() caddy.Module { return new(DinScoreBasedSelector) },
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modInfo := selector.CaddyModule()
			if modInfo.ID != tt.output.ID {
				t.Errorf("CaddyModule() = %v, want %v", modInfo.ID, tt.output.ID)
			}
			if reflect.TypeOf(modInfo.New()) != reflect.TypeOf(tt.output.New()) {
				t.Errorf("CaddyModule() = %v, want %v", modInfo.New(), tt.output.New())
			}
		})
	}
}

type mockSelector struct {
	alwaysReturnUpstream *reverseproxy.Upstream
}

func (m *mockSelector) Select(_ reverseproxy.UpstreamPool, _ *http.Request, _ http.ResponseWriter) *reverseproxy.Upstream {
	return m.alwaysReturnUpstream
}

func TestDinScoreBasedSelectorSelect(t *testing.T) {

	upstream_mock := &reverseproxy.Upstream{
		Dial: "mock",
	}
	upstream_foo := &reverseproxy.Upstream{
		Dial: "foo",
	}
	upstream_bar := &reverseproxy.Upstream{
		Dial: "bar",
	}

	selector := DinScoreBasedSelector{
		logger:   zap.NewNop(), // suppress logger here because we repeat the test multiple times
		fallback: &mockSelector{alwaysReturnUpstream: upstream_mock},
	}

	type Output struct {
		upstream    *reverseproxy.Upstream
		target_prob float64
	}

	tests := []struct {
		name                string
		request             *http.Request
		pool                reverseproxy.UpstreamPool
		providers           map[string]*provider
		smartRoutingEnabled bool
		repeat              int
		output              []Output
	}{
		{
			name:                "Score based routing disabled, use fallback => upstream selected",
			request:             &http.Request{},
			pool:                reverseproxy.UpstreamPool{upstream_mock},
			providers:           nil,
			smartRoutingEnabled: false,
			repeat:              1,
			output:              []Output{{upstream: upstream_mock, target_prob: 1.0}},
		},
		{
			name:                "Score based routing enabled, no providers score => no upstream selected",
			request:             &http.Request{},
			pool:                reverseproxy.UpstreamPool{},
			providers:           nil,
			smartRoutingEnabled: true,
			repeat:              1,
			output:              []Output{{upstream: nil, target_prob: 0.0}},
		},
		{
			name:    "Score based routing enabled, single provider (score is empty) => upstream selected",
			request: &http.Request{},
			pool:    reverseproxy.UpstreamPool{upstream_foo},
			providers: map[string]*provider{
				upstream_foo.Dial: {
					Score: ws.NewEmptyScore(),
				},
			},
			smartRoutingEnabled: true,
			repeat:              1,
			output:              []Output{{upstream: upstream_foo, target_prob: 1.0}},
		},
		{
			name:    "Score based routing enabled, single provider (score has a value > 0.0) => upstream selected",
			request: &http.Request{},
			pool:    reverseproxy.UpstreamPool{upstream_bar},
			providers: map[string]*provider{
				upstream_bar.Dial: {
					Score: ws.MustCreateScore(1.0, time.Now().UTC()),
				},
			},
			smartRoutingEnabled: true,
			repeat:              1,
			output:              []Output{{upstream: upstream_bar, target_prob: 1.0}},
		},
		{
			name:    "Score based routing enabled, single provider (score has a value, but it's 0.0) => no upstream selected",
			request: &http.Request{},
			pool:    reverseproxy.UpstreamPool{upstream_bar},
			providers: map[string]*provider{
				upstream_bar.Dial: {
					Score: ws.MustCreateScore(0.0, time.Now().UTC()),
				},
			},
			smartRoutingEnabled: true,
			repeat:              1,
			output:              []Output{{upstream: nil, target_prob: 0.0}},
		},
		{
			name:    "Score based routing enabled, single provider (score has a value, but it is stale) => upstream selected",
			request: &http.Request{},
			pool:    reverseproxy.UpstreamPool{upstream_bar},
			providers: map[string]*provider{
				upstream_bar.Dial: {
					Score: ws.MustCreateScore(0.0, time.Now().UTC().Add(-time.Minute*StaleScoreGracePeriodInMinutes-1)),
				},
			},
			smartRoutingEnabled: true,
			repeat:              1,
			output:              []Output{{upstream: upstream_bar, target_prob: 1.0}},
		},
		{
			name:    "Score based routing enabled, two providers (same score) => upstream selected with same odds",
			request: &http.Request{},
			pool:    reverseproxy.UpstreamPool{upstream_bar, upstream_foo},
			providers: map[string]*provider{
				upstream_bar.Dial: {
					Score: ws.MustCreateScore(0.8, time.Now().UTC()),
				},
				upstream_foo.Dial: {
					Score: ws.MustCreateScore(0.8, time.Now().UTC()),
				},
			},
			smartRoutingEnabled: true,
			repeat:              1000, // less than 10000 is not enough to get a stable result
			output:              []Output{{upstream: upstream_bar, target_prob: 0.5}, {upstream: upstream_foo, target_prob: 0.5}},
		},
		{
			name:    "Score based routing enabled, two providers (bar valid, foo valid) => upstream selected according to odds",
			request: &http.Request{},
			pool:    reverseproxy.UpstreamPool{upstream_bar, upstream_foo},
			providers: map[string]*provider{
				upstream_bar.Dial: {
					Score: ws.MustCreateScore(0.92, time.Now().UTC()),
				},
				upstream_foo.Dial: {
					Score: ws.MustCreateScore(0.67, time.Now().UTC()),
				},
			},
			smartRoutingEnabled: true,
			repeat:              1000, // less than 10000 is not enough to get a stable result
			output: []Output{{upstream: upstream_bar, target_prob: 0.5786}, // note that prob = odd / (odd + 1)
				{upstream: upstream_foo, target_prob: 0.4214}},
		},
		{
			name:    "Score based routing enabled, two providers (bar valid, foo invalid) => upstream selected according to odds",
			request: &http.Request{},
			pool:    reverseproxy.UpstreamPool{upstream_bar, upstream_foo},
			providers: map[string]*provider{
				upstream_bar.Dial: {
					Score: ws.MustCreateScore(0.92, time.Now().UTC()),
				},
				upstream_foo.Dial: {
					Score: ws.NewEmptyScore(), // foo has no score, score will be defaulted to 0.5
				},
			},
			smartRoutingEnabled: true,
			repeat:              1000, // less than 10000 is not enough to get a stable result
			output: []Output{{upstream: upstream_bar, target_prob: 0.6479}, // note that prob = odd / (odd + 1)
				{upstream: upstream_foo, target_prob: 0.3521}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replace crypto/rand with a deterministic reader for the test
			rand.Reader = NewDeterministicReader(1234567890)

			tt.request = tt.request.WithContext(context.WithValue(tt.request.Context(), caddy.ReplacerCtxKey, caddy.NewReplacer()))
			repl := tt.request.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
			repl.Set(DinUpstreamsContextKey, tt.providers)
			repl.Set(DinScoreBasedRoutingContextKey, tt.smartRoutingEnabled)

			results := make([]*reverseproxy.Upstream, tt.repeat)
			upstream_count := make(map[*reverseproxy.Upstream]int)
			for i := 0; i < tt.repeat; i++ {
				upstream := selector.Select(tt.pool, tt.request, nil)
				results[i] = upstream
				upstream_count[upstream]++
			}

			if tt.repeat > 1 {
				for _, expected := range tt.output {
					effective_prob := float64(upstream_count[expected.upstream]) / float64(tt.repeat)
					if math.Abs(effective_prob-expected.target_prob) > 0.02 { // allow 2% error margin
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
