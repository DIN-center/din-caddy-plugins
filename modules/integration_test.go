package modules

import (
	"container/list"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	networklib "github.com/DIN-center/din-caddy-plugins/lib/network"
	"github.com/DIN-center/din-caddy-plugins/lib/utils"
)

// testEnv encapsulates common test infrastructure
type testEnv struct {
	t          *testing.T
	middleware *DinMiddleware
	upstreams  *DinUpstreams
	servers    []*httptest.Server
}

func newTestEnv(t *testing.T) *testEnv {
	networklib.RegisterBuiltinHandlers()
	return &testEnv{
		t:         t,
		upstreams: &DinUpstreams{logger: zaptest.NewLogger(t)},
	}
}

func (e *testEnv) cleanup() {
	for _, s := range e.servers {
		s.Close()
	}
}

// createMockServer creates a mock HTTP server for testing
func (e *testEnv) createMockServer(blockNumber int64, shouldFail bool) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if shouldFail {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		_ = json.Unmarshal(body, &req)
		method, _ := req["method"].(string)

		var response string
		switch method {
		case "eth_blockNumber":
			response = `{"jsonrpc":"2.0","result":"0x64","id":1}`
		case "eth_chainId":
			response = `{"jsonrpc":"2.0","result":"0x1","id":1}`
		default:
			response = `{"jsonrpc":"2.0","result":"0x0","id":1}`
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
	e.servers = append(e.servers, server)
	return server
}

// createProvider creates a provider with the given status
func createProvider(url string, priority int, status HealthStatus) *provider {
	p, _ := NewProvider(url)
	p.Priority = priority
	p.BlockHistory = list.New()
	p.AddBlockEntry(100, status, 5)
	return p
}

// createProviderWithUpstream creates a provider with upstream for DinUpstreams testing
func createProviderWithUpstream(dial string, priority int, status HealthStatus) *provider {
	p := &provider{
		Upstream:     &reverseproxy.Upstream{Dial: dial},
		Priority:     priority,
		BlockHistory: list.New(),
	}
	p.BlockHistory.PushBack(BlockHistoryEntry{BlockNumber: 100, HealthStatus: status})
	return p
}

// setupMiddleware creates and initializes a middleware with the given network
func (e *testEnv) setupMiddleware(networkName string, handlerType HandlerType, providers map[string]*provider) error {
	net := &network{
		Name:                    networkName,
		HandlerType:             handlerType,
		ChainId:                 e.getChainID(handlerType),
		Providers:               providers,
		MaxRequestPayloadSizeKB: DefaultMaxRequestPayloadSizeKB,
		RequestAttemptCount:     1,
	}

	e.middleware = &DinMiddleware{
		logger:          logger.NewLoggerClient(zaptest.NewLogger(e.t), utils.EnvTest),
		testMode:        true,
		handlerRegistry: networklib.DefaultRegistry,
		Networks:        map[string]*network{networkName: net},
	}
	return e.middleware.initialize(caddy.Context{})
}

func (e *testEnv) getChainID(handlerType HandlerType) string {
	if handlerType == BeaconHandler {
		return "1"
	}
	return "0x1"
}

// makeRequest makes a request through the middleware and returns the captured providers
func (e *testEnv) makeRequest(method, path, body string) (map[string]*provider, error) {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, "http://test.com"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, "http://test.com"+path, nil)
	}

	repl := caddy.NewReplacer()
	ctx := context.WithValue(req.Context(), caddy.ReplacerCtxKey, repl)
	req = req.WithContext(ctx)

	rw := httptest.NewRecorder()
	var capturedProviders map[string]*provider

	nextHandler := caddyhttp.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
		if v, ok := repl.Get(DinUpstreamsContextKey); ok {
			capturedProviders = v.(map[string]*provider)
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":"0x64","id":1}`))
		return nil
	})

	err := e.middleware.ServeHTTP(rw, req, nextHandler)
	return capturedProviders, err
}

// getUpstreamCount returns the number of upstreams available for the given providers
func (e *testEnv) getUpstreamCount(providers map[string]*provider) int {
	req := httptest.NewRequest("GET", "/test", nil)
	repl := caddy.NewReplacer()
	ctx := context.WithValue(context.Background(), caddy.ReplacerCtxKey, repl)
	req = req.WithContext(ctx)
	repl.Set(DinUpstreamsContextKey, providers)
	upstreams, _ := e.upstreams.GetUpstreams(req)
	return len(upstreams)
}

// TestHTTPIntegrationRequestRouting verifies requests are routed to healthy upstreams
func TestHTTPIntegrationRequestRouting(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	server := env.createMockServer(100, false)
	providers := map[string]*provider{
		"provider1": createProvider(server.URL, 0, Healthy),
	}

	err := env.setupMiddleware("ethereum", EVMHandler, providers)
	require.NoError(t, err)

	capturedProviders, err := env.makeRequest("POST", "/ethereum",
		`{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`)
	require.NoError(t, err)
	require.NotNil(t, capturedProviders, "Providers should be set in context")
	assert.Len(t, capturedProviders, 1)
}

// TestHTTPIntegrationFailoverScenarios tests failover behavior
func TestHTTPIntegrationFailoverScenarios(t *testing.T) {
	tests := []struct {
		name          string
		providers     map[string]*provider
		expectedCount int
	}{
		{
			name: "failover_excludes_unhealthy",
			providers: map[string]*provider{
				"healthy":   createProviderWithUpstream("healthy:8000", 0, Healthy),
				"unhealthy": createProviderWithUpstream("unhealthy:8000", 0, Unhealthy),
			},
			expectedCount: 1,
		},
		{
			name: "failover_to_warning",
			providers: map[string]*provider{
				"warning":   createProviderWithUpstream("warning:8000", 0, Warning),
				"unhealthy": createProviderWithUpstream("unhealthy:8000", 0, Unhealthy),
			},
			expectedCount: 1,
		},
		{
			name: "all_unhealthy_returns_empty",
			providers: map[string]*provider{
				"unhealthy1": createProviderWithUpstream("u1:8000", 0, Unhealthy),
				"unhealthy2": createProviderWithUpstream("u2:8000", 0, Unhealthy),
			},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newTestEnv(t)
			count := env.getUpstreamCount(tt.providers)
			assert.Equal(t, tt.expectedCount, count)
		})
	}
}

// TestHTTPIntegrationPriorityRouting tests priority-based routing
func TestHTTPIntegrationPriorityRouting(t *testing.T) {
	tests := []struct {
		name          string
		providers     map[string]*provider
		expectedCount int
	}{
		{
			name: "priority_0_selected_first",
			providers: map[string]*provider{
				"p0": createProviderWithUpstream("p0:8000", 0, Healthy),
				"p1": createProviderWithUpstream("p1:8000", 1, Healthy),
			},
			expectedCount: 1, // Only priority 0
		},
		{
			name: "fallback_to_priority_1",
			providers: map[string]*provider{
				"p0": createProviderWithUpstream("p0:8000", 0, Unhealthy),
				"p1": createProviderWithUpstream("p1:8000", 1, Healthy),
			},
			expectedCount: 1, // Priority 1 when 0 is unhealthy
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newTestEnv(t)
			count := env.getUpstreamCount(tt.providers)
			assert.Equal(t, tt.expectedCount, count)
		})
	}
}

// TestMiddlewareHealthStatusChanges tests that health status changes affect routing
func TestMiddlewareHealthStatusChanges(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	server1 := env.createMockServer(100, false)
	server2 := env.createMockServer(100, false)

	provider1 := createProvider(server1.URL, 0, Healthy)
	provider2 := createProvider(server2.URL, 0, Healthy)
	providers := map[string]*provider{"p1": provider1, "p2": provider2}

	err := env.setupMiddleware("ethereum", EVMHandler, providers)
	require.NoError(t, err)

	// Initial: both healthy
	captured, _ := env.makeRequest("POST", "/ethereum",
		`{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`)
	require.NotNil(t, captured)

	// Mark provider1 unhealthy
	provider1.AddBlockEntry(0, Unhealthy, 5)

	// Now only provider2 should be healthy
	assert.True(t, provider2.Healthy())
	assert.False(t, provider1.Healthy())
}

// TestRESTAPIFailover tests failover for REST API handlers
func TestRESTAPIFailover(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	healthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"genesis_time":"1606824023"}}`))
	}))
	defer healthyServer.Close()

	providers := map[string]*provider{
		"healthy":   createProvider(healthyServer.URL, 0, Healthy),
		"unhealthy": createProvider("http://unhealthy.test", 0, Unhealthy),
	}

	err := env.setupMiddleware("beacon", BeaconHandler, providers)
	require.NoError(t, err)

	captured, err := env.makeRequest("GET", "/beacon/eth/v1/beacon/genesis", "")
	require.NoError(t, err)
	require.NotNil(t, captured)
	assert.Equal(t, 1, env.getUpstreamCount(captured))
}

// TestUpstreamPoolConstruction tests the DinUpstreams.GetUpstreams directly
func TestUpstreamPoolConstruction(t *testing.T) {
	tests := []struct {
		name          string
		providers     map[string]*provider
		expectedCount int
	}{
		{
			name: "all_healthy_same_priority",
			providers: map[string]*provider{
				"p1": createProviderWithUpstream("p1:8000", 0, Healthy),
				"p2": createProviderWithUpstream("p2:8000", 0, Healthy),
			},
			expectedCount: 2,
		},
		{
			name: "mixed_health",
			providers: map[string]*provider{
				"healthy":   createProviderWithUpstream("h:8000", 0, Healthy),
				"unhealthy": createProviderWithUpstream("u:8000", 0, Unhealthy),
			},
			expectedCount: 1,
		},
		{
			name: "warning_fallback",
			providers: map[string]*provider{
				"warning":   createProviderWithUpstream("w:8000", 0, Warning),
				"unhealthy": createProviderWithUpstream("u:8000", 0, Unhealthy),
			},
			expectedCount: 1,
		},
		{
			name: "priority_filtering",
			providers: map[string]*provider{
				"p0":  createProviderWithUpstream("p0:8000", 0, Healthy),
				"p1a": createProviderWithUpstream("p1a:8000", 1, Healthy),
				"p1b": createProviderWithUpstream("p1b:8000", 1, Healthy),
			},
			expectedCount: 1, // Only priority 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newTestEnv(t)
			count := env.getUpstreamCount(tt.providers)
			assert.Equal(t, tt.expectedCount, count)
		})
	}
}

// mockUpstreamServer for more complex scenarios
type mockUpstreamServer struct {
	server       *httptest.Server
	requestCount int64
}

func newMockUpstreamServer() *mockUpstreamServer {
	m := &mockUpstreamServer{}
	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&m.requestCount, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":"0x64","id":1}`))
	}))
	return m
}

func (m *mockUpstreamServer) Close() { m.server.Close() }
func (m *mockUpstreamServer) URL() string { return m.server.URL }
func (m *mockUpstreamServer) RequestCount() int64 { return atomic.LoadInt64(&m.requestCount) }
