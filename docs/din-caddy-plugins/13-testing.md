# Testing

This document covers the testing infrastructure, patterns, and practices used in DIN Caddy Plugins.

## Overview

The project has comprehensive test coverage across all packages:

| Package | Test Files | Focus |
|---------|------------|-------|
| `modules/` | 13 | Caddy modules, middleware |
| `lib/network/` | 8 | Network handlers |
| `lib/auth/` | 2 | SIWE, OIDC |
| `lib/watcherscore/` | 6 | Score management |
| `lib/prometheus/` | 2 | Metrics |
| `lib/http/` | 1 | HTTP client |

---

## Running Tests

### All Tests

```bash
make test
```

### With Coverage

```bash
make test-coverage
# Opens coverage report in browser
```

### With Race Detection

```bash
make test-race
```

### Specific Package

```bash
go test ./modules/...
go test ./lib/network/...
```

### Specific Test

```bash
go test -run TestDinMiddleware_ServeHTTP ./modules/...
go test -run TestEVMHandler ./lib/network/...
```

### Verbose Output

```bash
go test -v ./...
```

### Benchmarks

```bash
make benchmark
# or
go test -bench=. ./...
```

---

## Test Structure

### File Naming

- Test files: `*_test.go`
- Mock files: `*_mock.go` or `interface_mock.go`
- Test fixtures: `testdata/` directories

### Package Structure

```
lib/network/
├── evm_handler.go
├── evm_handler_test.go
├── handlers.go
├── handler_test.go
├── interface_mock.go
└── testdata/
    ├── evm_block_response.json
    ├── evm_chainid_response.json
    └── ...
```

---

## Testing Patterns

### Table-Driven Tests

Most tests use table-driven patterns:

```go
func TestEVMHandler_ParseHealthCheckResponse(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected uint64
        wantErr  bool
    }{
        {
            name:     "valid hex response",
            input:    `{"jsonrpc":"2.0","result":"0x10a3b5c","id":1}`,
            expected: 17480540,
            wantErr:  false,
        },
        {
            name:     "invalid response",
            input:    `{"error":"failed"}`,
            expected: 0,
            wantErr:  true,
        },
    }

    handler := NewEVMHandler()

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := handler.ParseHealthCheckResponse([]byte(tt.input))

            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                assert.Equal(t, tt.expected, result)
            }
        })
    }
}
```

### Mock Usage

Mocks are generated using `go.uber.org/mock`:

```go
func TestDinMiddleware_ServeHTTP(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    // Create mocks
    mockHTTP := http_mock.NewMockIHTTPClient(ctrl)
    mockPrometheus := prometheus_mock.NewMockIPrometheusClient(ctrl)

    // Set expectations
    mockHTTP.EXPECT().
        DoWithContext(gomock.Any(), gomock.Any()).
        Return(&http.Response{
            StatusCode: 200,
            Body:       io.NopCloser(strings.NewReader(`{"result":"0x1"}`)),
        }, nil)

    mockPrometheus.EXPECT().
        RecordRequest(gomock.Any(), gomock.Any(), gomock.Any(), 200, gomock.Any())

    // Create middleware with mocks
    middleware := &DinMiddleware{
        httpClient:       mockHTTP,
        prometheusClient: mockPrometheus,
    }

    // Run test
    // ...
}
```

### Test Fixtures

JSON fixtures are stored in `testdata/` directories:

```go
func loadFixture(t *testing.T, name string) []byte {
    t.Helper()
    data, err := os.ReadFile(filepath.Join("testdata", name))
    require.NoError(t, err)
    return data
}

func TestParseResponse(t *testing.T) {
    fixture := loadFixture(t, "evm_block_response.json")
    result, err := handler.ParseResponse(fixture)
    assert.NoError(t, err)
}
```

### HTTP Test Server

For integration tests:

```go
func TestHealthCheck_Integration(t *testing.T) {
    // Create test server
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "jsonrpc": "2.0",
            "result":  "0x10a3b5c",
            "id":      1,
        })
    }))
    defer server.Close()

    // Test against server
    handler := NewEVMHandler()
    blockNum, err := handler.GetLatestBlockNumber(
        context.Background(),
        http.DefaultClient,
        server.URL,
        nil,
    )

    assert.NoError(t, err)
    assert.Equal(t, uint64(17480540), blockNum)
}
```

---

## Mock Generation

### Generate All Mocks

```bash
make generate-mocks
```

### Manual Generation

```bash
mockgen -source=lib/auth/interface.go -destination=lib/auth/interface_mock.go -package=auth
```

### Mock Interfaces

| Interface | Mock Location |
|-----------|---------------|
| `IAuthClient` | `lib/auth/interface_mock.go` |
| `IHTTPClient` | `lib/http/interface_mock.go` |
| `IPrometheusClient` | `lib/prometheus/interface_mock.go` |
| `IWatcherScoreManager` | `lib/watcherscore/interface_mock.go` |
| `NetworkHandler` | `lib/network/interface_mock.go` |

---

## Testing Specific Components

### Network Handler Tests

```go
// lib/network/evm_handler_test.go
func TestEVMHandler_GetLatestBlockNumber(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockHTTP := NewMockIHTTPClient(ctrl)
    mockHTTP.EXPECT().
        DoWithContext(gomock.Any(), gomock.Any()).
        Return(&http.Response{
            StatusCode: 200,
            Body: io.NopCloser(strings.NewReader(
                `{"jsonrpc":"2.0","result":"0x10a3b5c","id":1}`,
            )),
        }, nil)

    handler := NewEVMHandler()
    blockNum, err := handler.GetLatestBlockNumber(
        context.Background(),
        mockHTTP,
        "https://example.com",
        nil,
    )

    assert.NoError(t, err)
    assert.Equal(t, uint64(17480540), blockNum)
}
```

### Middleware Tests

```go
// modules/din_middleware_test.go
func TestDinMiddleware_NetworkResolution(t *testing.T) {
    middleware := &DinMiddleware{
        Services: map[string]*network{
            "ethereum-mainnet": NewNetwork("ethereum-mainnet"),
        },
    }

    tests := []struct {
        path        string
        expected    string
        shouldError bool
    }{
        {"/ethereum-mainnet", "ethereum-mainnet", false},
        {"/ethereum-mainnet/", "ethereum-mainnet", false},
        {"/unknown-network", "", true},
    }

    for _, tt := range tests {
        t.Run(tt.path, func(t *testing.T) {
            req := httptest.NewRequest("POST", tt.path, nil)
            network, err := middleware.resolveNetwork(req)

            if tt.shouldError {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                assert.Equal(t, tt.expected, network.Name)
            }
        })
    }
}
```

### Watcher Score Tests

```go
// lib/watcherscore/watcherscore_manager_test.go
func TestWatcherScoreManager_ComputeScore(t *testing.T) {
    metrics := []ProviderMetric{
        {Name: "block_consistency", Value: 0.9},
        {Name: "state_consistency", Value: 0.8},
        {Name: "latency", Value: 0.7},
    }

    formula := DefaultScoreFormula
    combiner := NewWeightedCombiner(formula.Weights)

    score := combiner.Combine(metrics)

    // Expected: 0.9*0.4 + 0.8*0.4 + 0.7*0.2 = 0.36 + 0.32 + 0.14 = 0.82
    assert.InDelta(t, 0.82, score, 0.001)
}
```

---

## Test Utilities

### Common Test Helpers

```go
// modules/test_utils.go
func createTestNetwork(t *testing.T, name string) *network {
    t.Helper()
    n := NewNetwork(name)
    n.HandlerType = EVMHandler
    n.SetHandler(network.NewEVMHandler())
    return n
}

func createTestProvider(t *testing.T, url string) *provider {
    t.Helper()
    return &provider{
        HttpUrl:      url,
        Priority:     0,
        healthStatus: Healthy,
    }
}
```

### Context Helpers

```go
func contextWithProviders(ctx context.Context, providers map[string]*provider) context.Context {
    return context.WithValue(ctx, DinUpstreamsContextKey, providers)
}

func contextWithMethod(ctx context.Context, method string) context.Context {
    return context.WithValue(ctx, RequestMethodKey, method)
}
```

---

## Coverage

### Generate Coverage Report

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Coverage Thresholds

The project aims for:
- Overall: >80%
- Critical paths: >90%

### View Coverage in Terminal

```bash
go test -cover ./...
```

---

## CI/CD Integration

### GitHub Actions

**File**: `.github/workflows/unit_tests.yml`

```yaml
name: Unit Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.23'

      - name: Run tests
        run: make test

      - name: Run tests with race detector
        run: make test-race

      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          files: coverage.out
```

### E2E Tests

**File**: `.github/workflows/e2e_integration_test.yml`

Integration tests run against real services in a Docker environment.

---

## Best Practices

1. **Use table-driven tests** for comprehensive coverage
2. **Mock external dependencies** using generated mocks
3. **Use test fixtures** for complex JSON responses
4. **Test error conditions** as thoroughly as success cases
5. **Use subtests** (`t.Run`) for organized output
6. **Use `t.Helper()`** in helper functions
7. **Run race detection** in CI (`-race` flag)
8. **Keep tests focused** on single behaviors

---

## Debugging Tests

### Verbose Output

```bash
go test -v -run TestSpecificTest ./...
```

### With Logs

```bash
LOG_LEVEL=debug go test -v ./...
```

### Specific Failure

```bash
go test -v -run TestFailing -count=1 ./...
```

---

## Related Documentation

- [Project Structure](./01-project-structure.md) - Test file locations
- [Interfaces & Types](./12-interfaces-types.md) - Mock interfaces
- [Build & Deployment](./14-build-deployment.md) - CI/CD
