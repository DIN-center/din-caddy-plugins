package modules

// TestServeHTTPRequestCountMetrics verifies that HandleRequestMetrics is called exactly once
// for every request that reaches the retry loop in ServeHTTP, regardless of outcome.
//
// Tests labelled "FAILS_BEFORE_FIX" expose the current bug described in:
//   ce1900e – removed shouldLogMetrics = true from non-retryable break paths
//   dd4f8b3 – added new break paths (path 2 & 3) also without setting shouldLogMetrics
//
// Each such test expects HandleRequestMetrics to be called Times(1).
// Before the fix the call never happens, so gomock reports a missing-call failure.
// After the fix they must all pass.

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest"

	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	networklib "github.com/DIN-center/din-caddy-plugins/lib/network"
	prom "github.com/DIN-center/din-caddy-plugins/lib/prometheus"
	"github.com/DIN-center/din-caddy-plugins/lib/utils"
)

// ----------------------------------------------------------------------------
// helpers
// ----------------------------------------------------------------------------

const testRPCBody = `{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`

// nextOK is a next-handler that returns HTTP 200 with a minimal JSON-RPC success body
// and optionally sets RequestProviderKey on the replacer (simulating DinSelect).
func nextOK(providerKey string) caddyhttp.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		if providerKey != "" {
			repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
			repl.Set(RequestProviderKey, providerKey)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":"0x1","id":1}`))
		return nil
	}
}

// nextTransportError is a next-handler that simulates a low-level transport failure.
func nextTransportError() caddyhttp.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		return errors.New("dial tcp: connection refused")
	}
}

// newTestMiddleware wires up a DinMiddleware ready for metrics tests.
// testMode is intentionally false so the Prometheus code-path is exercised.
func newTestMiddleware(t *testing.T, mockProm *prom.MockIPrometheusClient, nets map[string]*network) *DinMiddleware {
	t.Helper()
	return &DinMiddleware{
		testMode:         false,
		logger:           logger.NewLoggerClient(zaptest.NewLogger(t), utils.EnvTest),
		PrometheusClient: mockProm,
		Networks:         nets,
		handlerRegistry:  networklib.DefaultRegistry,
	}
}

// newTestNetwork returns a minimal network with a single provider and the
// supplied handler mock.  RequestAttemptCount defaults to 1.
func newMetricsTestNetwork(handler networklib.NetworkHandler, attemptCount int) *network {
	if attemptCount <= 0 {
		attemptCount = 1
	}
	return &network{
		Name:                    "eth",
		handler:                 handler,
		MaxRequestPayloadSizeKB: DefaultMaxRequestPayloadSizeKB,
		RequestAttemptCount:     attemptCount,
		Providers: map[string]*provider{
			"provider1": {},
		},
	}
}

// newReq builds a POST request with the given body and attaches a fresh Caddy replacer.
func newReq(body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "http://localhost/eth", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req.WithContext(context.WithValue(req.Context(), caddy.ReplacerCtxKey, caddy.NewReplacer()))
}

// ----------------------------------------------------------------------------
// main test
// ----------------------------------------------------------------------------

func TestServeHTTPRequestCountMetrics(t *testing.T) {
	tests := []struct {
		name string
		// failsBeforeFix documents that this test is expected to fail until the
		// three-break-path bug is fixed.  It does NOT skip the test – it is here
		// purely as documentation so reviewers know which cases expose the bug.
		failsBeforeFix bool

		// buildRequest returns the *http.Request to use (allows cancelled contexts).
		buildRequest func() *http.Request

		// buildNetwork returns the network map and configures handler mock expectations.
		buildNetwork func(ctrl *gomock.Controller) map[string]*network

		// setupMockProm configures expectations on the prometheus mock.
		// For bug cases this must set Times(1) – which will fail until the bug is fixed.
		setupMockProm func(m *prom.MockIPrometheusClient)

		// next is the handler that simulates the upstream proxy chain.
		next caddyhttp.HandlerFunc

		// wantErr mirrors whether ServeHTTP itself should return a non-nil error.
		wantErr bool
	}{
		// -----------------------------------------------------------------------
		// CONTROL CASES – must pass before AND after the fix
		// -----------------------------------------------------------------------
		{
			name:           "success on first attempt – metrics recorded",
			failsBeforeFix: false,
			buildRequest:   func() *http.Request { return newReq(testRPCBody) },
			buildNetwork: func(ctrl *gomock.Controller) map[string]*network {
				h := networklib.NewMockNetworkHandler(ctrl)
				h.EXPECT().ProcessRequest(gomock.Any()).Return(nil)
				h.EXPECT().GetRequestType().Return(networklib.RequestTypeRPC).AnyTimes()
				h.EXPECT().ExtractMethod(gomock.Any(), gomock.Any()).Return("eth_blockNumber", nil)
				h.EXPECT().ParseResponse(gomock.Any(), gomock.Any()).Return(nil) // success
				h.EXPECT().GetHealthCheckMethod().Return("").AnyTimes()          // async goroutine
				h.EXPECT().ConfigureRequestPath(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				return map[string]*network{"eth": newMetricsTestNetwork(h, 1)}
			},
			setupMockProm: func(m *prom.MockIPrometheusClient) {
				m.EXPECT().HandleRequestMetrics(gomock.Any(), gomock.Any()).Times(1)
			},
			next:    nextOK("provider1"),
			wantErr: false,
		},
		{
			// Context cancelled before the retry loop starts → handleContextCancellation
			// calls HandleRequestMetrics directly.
			name:           "context cancelled – metrics recorded via handleContextCancellation",
			failsBeforeFix: false,
			buildRequest: func() *http.Request {
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // cancelled before ServeHTTP is called
				req := httptest.NewRequest(http.MethodPost, "http://localhost/eth", strings.NewReader(testRPCBody))
				req.Header.Set("Content-Type", "application/json")
				return req.WithContext(context.WithValue(ctx, caddy.ReplacerCtxKey, caddy.NewReplacer()))
			},
			buildNetwork: func(ctrl *gomock.Controller) map[string]*network {
				h := networklib.NewMockNetworkHandler(ctrl)
				h.EXPECT().ProcessRequest(gomock.Any()).Return(nil)
				h.EXPECT().GetRequestType().Return(networklib.RequestTypeRPC).AnyTimes()
				h.EXPECT().ExtractMethod(gomock.Any(), gomock.Any()).Return("eth_blockNumber", nil)
				// ParseResponse / IsRetryableError not called – context cancelled before next runs
				return map[string]*network{"eth": newMetricsTestNetwork(h, 1)}
			},
			setupMockProm: func(m *prom.MockIPrometheusClient) {
				m.EXPECT().HandleRequestMetrics(gomock.Any(), gomock.Any()).Times(1)
			},
			next:    nextOK("provider1"),
			wantErr: false,
		},

		// -----------------------------------------------------------------------
		// BUG CASES (FAILS_BEFORE_FIX) – expose the metric-drop regression
		// -----------------------------------------------------------------------

		// Path 1 (regression ce1900e): non-retryable application-level error.
		// next.ServeHTTP returns nil (transport OK, HTTP 200), but ParseResponse
		// returns a non-nil error that is neither retryable nor retryable-on-different-provider.
		// The retry loop exits via break without setting shouldLogMetrics.
		// After the loop err == nil → handlePostRequestTasks is never reached.
		// BUG: HandleRequestMetrics is never called.
		{
			name:           "FAILS_BEFORE_FIX: non-retryable app error (path 1) – metrics must be recorded",
			failsBeforeFix: true,
			buildRequest:   func() *http.Request { return newReq(testRPCBody) },
			buildNetwork: func(ctrl *gomock.Controller) map[string]*network {
				h := networklib.NewMockNetworkHandler(ctrl)
				h.EXPECT().ProcessRequest(gomock.Any()).Return(nil)
				h.EXPECT().GetRequestType().Return(networklib.RequestTypeRPC).AnyTimes()
				h.EXPECT().ExtractMethod(gomock.Any(), gomock.Any()).Return("eth_call", nil)
				h.EXPECT().ParseResponse(gomock.Any(), gomock.Any()).Return(errors.New("execution reverted"))
				h.EXPECT().IsRetryableError(gomock.Any(), gomock.Any()).Return(false)
				h.EXPECT().IsRetryableOnDifferentProvider(gomock.Any(), gomock.Any()).Return(false)
				h.EXPECT().ConfigureRequestPath(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				return map[string]*network{"eth": newMetricsTestNetwork(h, 1)}
			},
			setupMockProm: func(m *prom.MockIPrometheusClient) {
				// Times(1) → test FAILS before fix (0 actual calls), PASSES after fix.
				m.EXPECT().HandleRequestMetrics(gomock.Any(), gomock.Any()).Times(1)
			},
			next:    nextOK("provider1"),
			wantErr: false,
		},

		// Path 1 variant: multiple retryable errors on earlier attempts, then a
		// non-retryable error on the final attempt (still never logs).
		{
			name:           "FAILS_BEFORE_FIX: retryable errors then non-retryable final (path 1 variant) – metrics must be recorded",
			failsBeforeFix: true,
			buildRequest:   func() *http.Request { return newReq(testRPCBody) },
			buildNetwork: func(ctrl *gomock.Controller) map[string]*network {
				h := networklib.NewMockNetworkHandler(ctrl)
				h.EXPECT().ProcessRequest(gomock.Any()).Return(nil)
				h.EXPECT().GetRequestType().Return(networklib.RequestTypeRPC).AnyTimes()
				h.EXPECT().ExtractMethod(gomock.Any(), gomock.Any()).Return("eth_call", nil)
				// Attempt 0: retryable error → continue
				// Attempt 1: non-retryable error → break (shouldLogMetrics still false)
				gomock.InOrder(
					h.EXPECT().ParseResponse(gomock.Any(), gomock.Any()).Return(errors.New("rate limit")),
					h.EXPECT().IsRetryableError(gomock.Any(), gomock.Any()).Return(true),
					h.EXPECT().ParseResponse(gomock.Any(), gomock.Any()).Return(errors.New("execution reverted")),
					h.EXPECT().IsRetryableError(gomock.Any(), gomock.Any()).Return(false),
					h.EXPECT().IsRetryableOnDifferentProvider(gomock.Any(), gomock.Any()).Return(false),
				)
				h.EXPECT().ConfigureRequestPath(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				return map[string]*network{"eth": newMetricsTestNetwork(h, 2)}
			},
			setupMockProm: func(m *prom.MockIPrometheusClient) {
				m.EXPECT().HandleRequestMetrics(gomock.Any(), gomock.Any()).Times(1)
			},
			next:    nextOK("provider1"),
			wantErr: false,
		},

		// Path 2 (regression dd4f8b3): IsRetryableOnDifferentProvider returns true
		// but RequestProviderKey is not present in the replacer (DinSelect didn't run or
		// failed to set the key).  The loop breaks at the "cannot identify provider" guard
		// without setting shouldLogMetrics.
		{
			name:           "FAILS_BEFORE_FIX: method-not-found with missing provider key (path 2) – metrics must be recorded",
			failsBeforeFix: true,
			buildRequest:   func() *http.Request { return newReq(testRPCBody) },
			buildNetwork: func(ctrl *gomock.Controller) map[string]*network {
				h := networklib.NewMockNetworkHandler(ctrl)
				h.EXPECT().ProcessRequest(gomock.Any()).Return(nil)
				h.EXPECT().GetRequestType().Return(networklib.RequestTypeRPC).AnyTimes()
				h.EXPECT().ExtractMethod(gomock.Any(), gomock.Any()).Return("eth_call", nil)
				h.EXPECT().ParseResponse(gomock.Any(), gomock.Any()).Return(errors.New("method not found"))
				h.EXPECT().IsRetryableError(gomock.Any(), gomock.Any()).Return(false)
				h.EXPECT().IsRetryableOnDifferentProvider(gomock.Any(), gomock.Any()).Return(true)
				h.EXPECT().ConfigureRequestPath(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				return map[string]*network{"eth": newMetricsTestNetwork(h, 1)}
			},
			setupMockProm: func(m *prom.MockIPrometheusClient) {
				m.EXPECT().HandleRequestMetrics(gomock.Any(), gomock.Any()).Times(1)
			},
			// next does NOT set RequestProviderKey – simulates missing DinSelect key
			next:    nextOK(""),
			wantErr: false,
		},

		// Path 3 (regression dd4f8b3): all providers have been excluded due to
		// -32601 method-not-found failover.  len(excludedProviders) >= len(providers)
		// triggers the "all providers exhausted" break without setting shouldLogMetrics.
		{
			name:           "FAILS_BEFORE_FIX: all providers excluded after method-not-found failover (path 3) – metrics must be recorded",
			failsBeforeFix: true,
			buildRequest:   func() *http.Request { return newReq(testRPCBody) },
			buildNetwork: func(ctrl *gomock.Controller) map[string]*network {
				h := networklib.NewMockNetworkHandler(ctrl)
				h.EXPECT().ProcessRequest(gomock.Any()).Return(nil)
				h.EXPECT().GetRequestType().Return(networklib.RequestTypeRPC).AnyTimes()
				h.EXPECT().ExtractMethod(gomock.Any(), gomock.Any()).Return("eth_call", nil)
				h.EXPECT().ParseResponse(gomock.Any(), gomock.Any()).Return(errors.New("method not found"))
				h.EXPECT().IsRetryableError(gomock.Any(), gomock.Any()).Return(false)
				h.EXPECT().IsRetryableOnDifferentProvider(gomock.Any(), gomock.Any()).Return(true)
				h.EXPECT().ConfigureRequestPath(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				// Single provider: after it is excluded len(excluded)==len(providers)==1 → break
				net := newMetricsTestNetwork(h, 1)
				return map[string]*network{"eth": net}
			},
			setupMockProm: func(m *prom.MockIPrometheusClient) {
				m.EXPECT().HandleRequestMetrics(gomock.Any(), gomock.Any()).Times(1)
			},
			// next sets RequestProviderKey so the provider can be identified and excluded
			next:    nextOK("provider1"),
			wantErr: false,
		},

		// Path 4: transport (Go-level) error on the final attempt.
		// shouldLogMetrics is set to true at line 894, but the if err != nil block at
		// line 914 returns at line 960 before handlePostRequestTasks is reached.
		// The fallback direct-call at line 944 is guarded by !shouldLogMetrics which is
		// false, so it is also skipped.  Net result: no metrics recorded.
		{
			name:           "FAILS_BEFORE_FIX: transport error on final attempt – metrics must be recorded",
			failsBeforeFix: true,
			buildRequest:   func() *http.Request { return newReq(testRPCBody) },
			buildNetwork: func(ctrl *gomock.Controller) map[string]*network {
				h := networklib.NewMockNetworkHandler(ctrl)
				h.EXPECT().ProcessRequest(gomock.Any()).Return(nil)
				h.EXPECT().GetRequestType().Return(networklib.RequestTypeRPC).AnyTimes()
				h.EXPECT().ExtractMethod(gomock.Any(), gomock.Any()).Return("eth_blockNumber", nil)
				// ParseResponse / IsRetryableError not called for transport errors
				h.EXPECT().ConfigureRequestPath(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				return map[string]*network{"eth": newMetricsTestNetwork(h, 1)}
			},
			setupMockProm: func(m *prom.MockIPrometheusClient) {
				m.EXPECT().HandleRequestMetrics(gomock.Any(), gomock.Any()).Times(1)
			},
			next:    nextTransportError(),
			wantErr: true,
		},

		// -----------------------------------------------------------------------
		// INTENTIONAL NON-COUNTING CASES – must pass before AND after the fix
		// (no HandleRequestMetrics expectation → gomock fails if it IS called)
		// -----------------------------------------------------------------------
		{
			// Root path "/" is treated as a health probe and returns 200 immediately.
			name: "empty network path – no metrics",
			buildRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "http://localhost/", http.NoBody)
				return req.WithContext(context.WithValue(req.Context(), caddy.ReplacerCtxKey, caddy.NewReplacer()))
			},
			buildNetwork: func(_ *gomock.Controller) map[string]*network {
				return map[string]*network{"eth": {}}
			},
			setupMockProm: func(_ *prom.MockIPrometheusClient) {}, // no expectations
			next:          nextOK(""),
			wantErr:       false,
		},
		{
			name: "unknown network – no metrics",
			buildRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "http://localhost/unknown", strings.NewReader(testRPCBody))
				req.Header.Set("Content-Type", "application/json")
				return req.WithContext(context.WithValue(req.Context(), caddy.ReplacerCtxKey, caddy.NewReplacer()))
			},
			buildNetwork: func(_ *gomock.Controller) map[string]*network {
				return map[string]*network{"eth": {}}
			},
			setupMockProm: func(_ *prom.MockIPrometheusClient) {},
			next:          nextOK(""),
			wantErr:       true,
		},
		{
			// Empty body on an RPC network is explicitly excluded from metrics
			// (OPTIONS requests, pre-flight, etc.).
			name: "empty body on RPC network – no metrics (intentional skip)",
			buildRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "http://localhost/eth", http.NoBody)
				req.Header.Set("Content-Type", "application/json")
				return req.WithContext(context.WithValue(req.Context(), caddy.ReplacerCtxKey, caddy.NewReplacer()))
			},
			buildNetwork: func(ctrl *gomock.Controller) map[string]*network {
				h := networklib.NewMockNetworkHandler(ctrl)
				h.EXPECT().ProcessRequest(gomock.Any()).Return(nil)
				h.EXPECT().GetRequestType().Return(networklib.RequestTypeRPC).AnyTimes()
				return map[string]*network{"eth": newMetricsTestNetwork(h, 1)}
			},
			setupMockProm: func(_ *prom.MockIPrometheusClient) {},
			next:          nextOK(""),
			wantErr:       true,
		},
		{
			// Request body exceeds MaxRequestPayloadSizeKB.
			name: "payload too large – no metrics",
			buildRequest: func() *http.Request {
				bigBody := `{"key":"` + strings.Repeat("x", 2048) + `"}`
				req := httptest.NewRequest(http.MethodPost, "http://localhost/eth", strings.NewReader(bigBody))
				req.Header.Set("Content-Type", "application/json")
				return req.WithContext(context.WithValue(req.Context(), caddy.ReplacerCtxKey, caddy.NewReplacer()))
			},
			buildNetwork: func(ctrl *gomock.Controller) map[string]*network {
				h := networklib.NewMockNetworkHandler(ctrl)
				h.EXPECT().ProcessRequest(gomock.Any()).Return(nil)
				h.EXPECT().GetRequestType().Return(networklib.RequestTypeRPC).AnyTimes()
				net := newMetricsTestNetwork(h, 1)
				net.MaxRequestPayloadSizeKB = 1 // 1 KB limit
				return map[string]*network{"eth": net}
			},
			setupMockProm: func(_ *prom.MockIPrometheusClient) {},
			next:          nextOK(""),
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			mockProm := prom.NewMockIPrometheusClient(ctrl)
			tt.setupMockProm(mockProm)

			nets := tt.buildNetwork(ctrl)
			d := newTestMiddleware(t, mockProm, nets)

			req := tt.buildRequest()
			rw := httptest.NewRecorder()

			err := d.ServeHTTP(rw, req, tt.next)
			if tt.wantErr && err == nil {
				t.Errorf("ServeHTTP() expected error, got nil")
			} else if !tt.wantErr && err != nil {
				t.Errorf("ServeHTTP() unexpected error: %v", err)
			}
			// gomock.Controller.Finish() (deferred above) verifies that all
			// expected calls to HandleRequestMetrics were actually made.
		})
	}
}
