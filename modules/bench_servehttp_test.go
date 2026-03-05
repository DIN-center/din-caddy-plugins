package modules

import (
	"bytes"
	"container/list"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	networklib "github.com/DIN-center/din-caddy-plugins/lib/network"
	"github.com/DIN-center/din-caddy-plugins/lib/utils"
	ws "github.com/DIN-center/din-caddy-plugins/lib/watcherscore"
)

// mockSuccessHandler simulates the reverse proxy returning a success JSON-RPC response.
var mockSuccessHandler = caddyhttp.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x1234567"}`))
	return nil
})

// newBenchMiddleware creates a fully provisioned DinMiddleware for benchmarking.
func newBenchMiddleware(t testing.TB, withMethodFilter bool) (*DinMiddleware, *networklib.MockNetworkHandler) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockHandler := networklib.NewMockNetworkHandler(ctrl)
	mockHandler.EXPECT().ExtractMethod(gomock.Any(), gomock.Any()).Return("eth_blockNumber", nil).AnyTimes()
	mockHandler.EXPECT().GetRequestType().Return(networklib.RequestTypeRPC).AnyTimes()
	mockHandler.EXPECT().ProcessRequest(gomock.Any()).Return(nil).AnyTimes()
	mockHandler.EXPECT().ParseResponse(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockHandler.EXPECT().ConfigureRequestPath(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockHandler.EXPECT().GetHealthCheckMethod().Return("eth_blockNumber").AnyTimes()
	mockHandler.EXPECT().CreateHealthCheckPayload(gomock.Any()).Return([]byte(`{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`), nil).AnyTimes()
	mockHandler.EXPECT().GetHealthCheckHTTPMethod().Return("POST").AnyTimes()
	mockHandler.EXPECT().ParseBlockNumberResponse(gomock.Any(), gomock.Any()).Return(int64(100), nil).AnyTimes()
	mockHandler.EXPECT().CreateBlockRequest(gomock.Any(), gomock.Any(), gomock.Any()).Return([]byte(`{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["0x64",false],"id":1}`), nil).AnyTimes()
	mockHandler.EXPECT().ParseBlockResponse(gomock.Any()).Return(nil, nil).AnyTimes()
	mockHandler.EXPECT().ExtractBlockHash(gomock.Any()).Return("0x123abc").AnyTimes()
	mockHandler.EXPECT().GetBlockByNumberMethod().Return("eth_getBlockByNumber").AnyTimes()
	mockHandler.EXPECT().FormatBlockHeight(gomock.Any()).Return("0x64").AnyTimes()

	p := &provider{
		blockHistory: func() *list.List {
			l := list.New()
			l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: Healthy})
			return l
		}(),
	}
	p.SafeUpdateScore(ws.NewEmptyScore())

	benchLogger := logger.NewLoggerClient(zap.NewNop(), utils.EnvTest)

	net := &network{
		Name:                    "eth",
		handler:                 mockHandler,
		MaxRequestPayloadSizeKB: DefaultMaxRequestPayloadSizeKB,
		RequestAttemptCount:     1,
		logger:                  benchLogger,
		Providers: map[string]*provider{
			"localhost:9999": p,
		},
	}

	if withMethodFilter != false {
		// leave nil to test the no-filter fast path
	}

	dm := new(DinMiddleware)
	dm.testMode = true
	dm.logger = logger.NewLoggerClient(zap.NewNop(), utils.EnvTest)
	dm.handlerRegistry = networklib.DefaultRegistry
	dm.Networks = map[string]*network{"eth": net}
	dm.DynamicLoadBalancing = DynamicLoadBalancingConfig{Enabled: false}

	return dm, mockHandler
}

// newBenchRequest builds a reusable JSON-RPC POST request with a Caddy replacer.
func newBenchRequest(b *testing.B, body string) *http.Request {
	b.Helper()
	req := httptest.NewRequest("POST", "http://localhost:8000/eth", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), caddy.ReplacerCtxKey, caddy.NewReplacer()))
	return req
}

// BenchmarkServeHTTP_RPC_HappyPath measures the full ServeHTTP round-trip for a
// typical JSON-RPC request with no retries and no method filtering.
func BenchmarkServeHTTP_RPC_HappyPath(b *testing.B) {
	dm, _ := newBenchMiddleware(b, false)
	body := `{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := newBenchRequest(b, body)
		rw := httptest.NewRecorder()
		repl := req.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
		repl.Set(RequestProviderKey, "localhost:9999")
		_ = dm.ServeHTTP(rw, req, mockSuccessHandler)
	}
}

// BenchmarkServeHTTP_RPC_LargeBody measures overhead with a larger request body (~1KB).
func BenchmarkServeHTTP_RPC_LargeBody(b *testing.B) {
	dm, _ := newBenchMiddleware(b, false)
	// Build a body close to 1KB to measure io.ReadAll and buffer costs.
	params := strings.Repeat(`"0x` + strings.Repeat("a", 62) + `",`, 15)
	body := `{"jsonrpc":"2.0","method":"eth_call","params":[` + params + `"0xlatest"],"id":1}`

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := newBenchRequest(b, body)
		rw := httptest.NewRecorder()
		repl := req.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
		repl.Set(RequestProviderKey, "localhost:9999")
		_ = dm.ServeHTTP(rw, req, mockSuccessHandler)
	}
}

// BenchmarkServeHTTP_Parallel exercises goroutine-level parallelism to expose
// lock contention under concurrent load (realistic production scenario).
func BenchmarkServeHTTP_Parallel(b *testing.B) {
	dm, _ := newBenchMiddleware(b, false)
	body := `{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := newBenchRequest(b, body)
			rw := httptest.NewRecorder()
			repl := req.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
			repl.Set(RequestProviderKey, "localhost:9999")
			_ = dm.ServeHTTP(rw, req, mockSuccessHandler)
		}
	})
}

// BenchmarkResponseWriterWrapper isolates the cost of allocating and writing
// to a ResponseWriterWrapper (the buffer used for retry capture).
func BenchmarkResponseWriterWrapper(b *testing.B) {
	rw := httptest.NewRecorder()
	payload := []byte(`{"jsonrpc":"2.0","id":1,"result":"0x1234567"}`)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rww := NewResponseWriterWrapper(rw)
		_, _ = rww.Write(payload)
	}
}

// BenchmarkBodyReadAndRestore isolates the cost of io.ReadAll + bytes.NewBuffer
// that happens on every request for body persistence across retries.
func BenchmarkBodyReadAndRestore(b *testing.B) {
	body := `{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`
	bodyBytes := []byte(body)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := io.NopCloser(bytes.NewReader(bodyBytes))
		data, _ := io.ReadAll(r)
		_ = io.NopCloser(bytes.NewBuffer(data))
	}
}

// BenchmarkPathExtraction isolates the cost of the URL path parsing done at
// the top of every ServeHTTP call.
func BenchmarkPathExtraction(b *testing.B) {
	path := "/eth"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fullPath := strings.TrimPrefix(path, "/")
		pathSegments := strings.Split(fullPath, "/")
		_ = pathSegments[0]
	}
}
