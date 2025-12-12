# Interfaces & Types

This document provides a reference for all key interfaces, types, constants, and enums used throughout DIN Caddy Plugins.

## Core Interfaces

### IAuthClient

**File**: `lib/auth/interface.go`

Authentication client interface for SIWE and OIDC implementations.

```go
type IAuthClient interface {
    // Start establishes sessions with the provider
    Start() error

    // Error returns the client's error state
    Error() error

    // GetToken retrieves the current auth token
    GetToken() (*AuthToken, error)

    // Sign adds authentication to an HTTP request
    Sign(r *http.Request) error

    // Stop cleans up the client
    Stop()
}
```

### IHTTPClient

**File**: `lib/http/interface.go`

HTTP client interface for making requests.

```go
type IHTTPClient interface {
    // Do executes an HTTP request
    Do(req *http.Request) (*http.Response, error)

    // DoWithContext executes with context
    DoWithContext(ctx context.Context, req *http.Request) (*http.Response, error)
}
```

### IPrometheusClient

**File**: `lib/prometheus/interface.go`

Metrics collection interface.

```go
type IPrometheusClient interface {
    // RecordRequest records a request metric
    RecordRequest(network, provider, method string, status int, duration time.Duration)

    // RecordHealthCheck records a health check result
    RecordHealthCheck(network, provider string, status HealthStatus, duration time.Duration)

    // RecordError records an error
    RecordError(network, provider, errorType string)

    // SetProviderHealth sets provider health gauge
    SetProviderHealth(network, provider string, status HealthStatus)

    // SetProviderBlockNumber sets provider block number gauge
    SetProviderBlockNumber(network, provider string, blockNum uint64)

    // SetProviderScore sets provider score gauge
    SetProviderScore(network, provider string, score float64)
}
```

### IWatcherScoreManager

**File**: `lib/watcherscore/interface.go`

Watcher score management interface.

```go
type IWatcherScoreManager interface {
    // Start begins score synchronization
    Start() error

    // Stop halts synchronization
    Stop()

    // GetScore returns the current score for a provider
    GetScore(providerName string) (float64, error)

    // GetAllScores returns scores for all providers
    GetAllScores() map[string]float64

    // UpdateScores manually triggers a score update
    UpdateScores() error
}
```

### NetworkHandler

**File**: `lib/network/handlers.go`

Network handler interface for blockchain-specific logic.

```go
type NetworkHandler interface {
    // Identity
    GetType() HandlerType
    GetName() string
    GetRequestType() RequestType

    // Request processing
    ProcessRequest(r *http.Request) error
    ExtractMethod(body []byte) (string, error)
    ConfigureRequestPath(r *http.Request, path string) error

    // Response handling
    ParseResponse(body []byte) (*JSONRPCResponse, error)
    IsRetryableError(err error) bool

    // Block operations
    GetLatestBlockNumber(ctx context.Context, client IHTTPClient, url string, headers map[string]string) (uint64, error)
    GetBlockByNumber(ctx context.Context, client IHTTPClient, url string, blockNum uint64, headers map[string]string) (*Block, error)
    FormatBlockHeight(height uint64) string

    // Health checks
    GetHealthCheckMethod() string
    CreateHealthCheckPayload() ([]byte, error)
    ParseHealthCheckResponse(body []byte) (uint64, error)

    // Chain ID
    GetChainID() string
    ValidateChainID(body []byte, expected string) (bool, error)
    ParseChainIDResponse(body []byte) (string, error)

    // Archive mode
    SupportsArchiveMode() bool
    PerformArchiveCheck(ctx context.Context, client IHTTPClient, url string, headers map[string]string) (bool, error)

    // Block info
    RequiresSeparateBlockInfoCall() bool
    GetBlockInfoMethod() string
    CreateBlockInfoPayload(blockNum uint64) ([]byte, error)

    // Lifecycle
    Initialize(ctx context.Context) error
}
```

---

## Enums and Constants

### HandlerType

**File**: `modules/consts.go`

```go
type HandlerType string

const (
    EVMHandler            HandlerType = "evm"
    BeaconHandler         HandlerType = "beacon-chain"
    StarknetHandler       HandlerType = "starknet"
    SolanaHandler         HandlerType = "solana"
    BitcoinHandler        HandlerType = "bitcoin"
    BitcoinEsploraHandler HandlerType = "bitcoin-esplora"
    TronHandler           HandlerType = "tron-full-node"
)
```

### RequestType

**File**: `lib/network/handlers.go`

```go
type RequestType int

const (
    RequestTypeRPC     RequestType = iota  // JSON-RPC (POST)
    RequestTypeREST                        // REST API (GET/POST)
    RequestTypeGraphQL                     // GraphQL
)
```

### HealthStatus

**File**: `modules/consts.go`

```go
type HealthStatus int

const (
    Healthy   HealthStatus = iota  // 0 - Fully operational
    Warning                        // 1 - Issues but serving traffic
    Unhealthy                      // 2 - Should not receive traffic
)
```

---

## Context Keys

**File**: `modules/consts.go`

```go
const (
    // DinUpstreamsContextKey stores eligible providers
    DinUpstreamsContextKey = "din.internal.upstreams"

    // RequestProviderKey stores the selected provider name
    RequestProviderKey = "request_provider"

    // RequestProviderPriorityKey stores the provider's priority
    RequestProviderPriorityKey = "request_provider_priority"

    // RequestBodyKey stores the original request body
    RequestBodyKey = "request_body"

    // RequestMethodKey stores the extracted RPC method
    RequestMethodKey = "request_method"

    // HealthStatusKey stores the provider's health status
    HealthStatusKey = "health_status"

    // BlockNumberKey stores the latest block number
    BlockNumberKey = "block_number"

    // NetworkNameKey stores the network name
    NetworkNameKey = "network_name"
)
```

---

## Data Types

### AuthToken

**File**: `lib/auth/interface.go`

```go
type AuthToken struct {
    // Headers to include in authenticated requests
    Headers map[string]string `json:"headers,omitempty"`

    // Token expiration timestamp
    ExpiresAt UnixTime `json:"expires_at,omitempty"`

    // Maximum number of uses (0 = unlimited)
    MaxUses int `json:"max_uses,omitempty"`

    // Current usage count
    Uses int `json:"uses,omitempty"`
}
```

### UnixTime

**File**: `lib/auth/interface.go`

Custom time type for JSON serialization.

```go
type UnixTime struct {
    time.Time
}

func (t UnixTime) MarshalJSON() ([]byte, error) {
    return []byte(strconv.FormatInt(t.Unix(), 10)), nil
}

func (t *UnixTime) UnmarshalJSON(data []byte) error {
    unix, _ := strconv.ParseInt(string(data), 10, 64)
    t.Time = time.Unix(unix, 0)
    return nil
}
```

### JSONRPCRequest

**File**: `lib/http/types.go`

```go
type JSONRPCRequest struct {
    JSONRPC string      `json:"jsonrpc"`
    Method  string      `json:"method"`
    Params  interface{} `json:"params,omitempty"`
    ID      interface{} `json:"id"`
}
```

### JSONRPCResponse

**File**: `lib/http/types.go`

```go
type JSONRPCResponse struct {
    JSONRPC string          `json:"jsonrpc"`
    Result  json.RawMessage `json:"result,omitempty"`
    Error   *JSONRPCError   `json:"error,omitempty"`
    ID      interface{}     `json:"id"`
}
```

### JSONRPCError

**File**: `lib/http/types.go`

```go
type JSONRPCError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Data    string `json:"data,omitempty"`
}
```

### ProviderMetric

**File**: `lib/watcherscore/types.go`

```go
type ProviderMetric struct {
    Name      string    `json:"name"`
    Value     float64   `json:"value"`      // 0.0 to 1.0
    Timestamp time.Time `json:"timestamp"`
}
```

### Score

**File**: `lib/watcherscore/types.go`

```go
type Score struct {
    Value     float64            `json:"value"`
    Timestamp time.Time          `json:"timestamp"`
    Metrics   map[string]float64 `json:"metrics"`
}
```

### ScoreFormula

**File**: `lib/watcherscore/types.go`

```go
type ScoreFormula struct {
    Weights map[string]float64 `json:"weights"`
}

var DefaultScoreFormula = ScoreFormula{
    Weights: map[string]float64{
        "block_consistency": 0.4,
        "state_consistency": 0.4,
        "latency":           0.2,
    },
}
```

---

## Configuration Types

### DinMiddleware

**File**: `modules/din_middleware.go`

```go
type DinMiddleware struct {
    // Network configurations
    Services map[string]*network `json:"services,omitempty"`

    // Registry configuration
    DinRegistry *RegistryConfig `json:"din_registry,omitempty"`

    // Dynamic load balancing configuration
    DynamicLoadBalancing *DynamicLoadBalancingConfig `json:"dynamic_load_balancing,omitempty"`
}
```

### RegistryConfig

**File**: `modules/din_middleware.go`

```go
type RegistryConfig struct {
    Endpoint        string `json:"endpoint,omitempty"`
    ContractAddress string `json:"contract_address,omitempty"`
    BlockEpoch      int    `json:"block_epoch,omitempty"`
    CheckInterval   int    `json:"check_interval,omitempty"`
    RetryAttempts   int    `json:"retry_attempts,omitempty"`
    RetryDelay      int    `json:"retry_delay,omitempty"`
}
```

### DynamicLoadBalancingConfig

**File**: `modules/din_middleware.go`

```go
type DynamicLoadBalancingConfig struct {
    Enabled           bool               `json:"enabled,omitempty"`
    WatcherURL        string             `json:"watcher_url,omitempty"`
    SyncInterval      int                `json:"sync_interval,omitempty"`
    GracePeriod       int                `json:"grace_period,omitempty"`
    ConvergencePeriod int                `json:"convergence_period,omitempty"`
    Weights           map[string]float64 `json:"weights,omitempty"`
}
```

### CaddyfileFlags

**File**: `modules/network.go`

Tracks which fields were set via Caddyfile.

```go
type CaddyfileFlags struct {
    HandlerType    bool
    ChainId        bool
    Methods        bool
    HCInterval     bool
    HCThreshold    bool
    HCTimeout      bool
    BlockLagLimit  bool
    BlockJumpLimit bool
    ArchiveEnabled bool
    Providers      map[string]ProviderCaddyfileFlags
}

type ProviderCaddyfileFlags struct {
    Priority bool
    Headers  bool
    Methods  bool
    Auth     bool
}
```

---

## Default Values

### Health Check Defaults

```go
const (
    DefaultHCInterval     = 5      // seconds
    DefaultHCThreshold    = 2      // failures
    DefaultHCTimeout      = 5000   // milliseconds
    DefaultBlockLagLimit  = 5      // blocks
    DefaultBlockJumpLimit = 100    // blocks
)
```

### Block History Sizes

```go
const (
    providerBlockHistorySize = 10
    networkBlockHistorySize  = 128
)
```

### Registry Defaults

```go
const (
    DefaultBlockEpoch    = 2000  // blocks
    DefaultCheckInterval = 60    // seconds
    DefaultRetryAttempts = 3
    DefaultRetryDelay    = 2     // seconds
)
```

### Dynamic LB Defaults

```go
const (
    DefaultSyncInterval      = 30   // seconds
    DefaultGracePeriod       = 60   // seconds
    DefaultConvergencePeriod = 120  // seconds
)
```

---

## Error Types

### Common Errors

```go
var (
    ErrNetworkNotFound    = errors.New("network not found")
    ErrProviderNotFound   = errors.New("provider not found")
    ErrNoHealthyProviders = errors.New("no healthy providers available")
    ErrInvalidChainID     = errors.New("chain ID mismatch")
    ErrAuthFailed         = errors.New("authentication failed")
    ErrTokenExpired       = errors.New("token expired")
)
```

---

## Mock Interfaces

Mock implementations are generated using `go.uber.org/mock`:

| Interface | Mock File |
|-----------|-----------|
| `IAuthClient` | `lib/auth/interface_mock.go` |
| `IHTTPClient` | `lib/http/interface_mock.go` |
| `IPrometheusClient` | `lib/prometheus/interface_mock.go` |
| `IWatcherScoreManager` | `lib/watcherscore/interface_mock.go` |
| `NetworkHandler` | `lib/network/interface_mock.go` |

Generate mocks:
```bash
make generate-mocks
```

---

## Related Documentation

- [Core Modules](./02-core-modules.md) - Module implementations
- [Network Handlers](./04-network-handlers.md) - Handler interface details
- [Testing](./13-testing.md) - Using mocks in tests
