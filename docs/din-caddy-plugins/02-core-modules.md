# Core Caddy Modules

DIN Caddy Plugins consists of four main Caddy modules that work together to route blockchain RPC requests. This document explains each module's purpose, interfaces, and how they interact.

## Module Registration

All modules are registered in `module.go`:

```go
func init() {
    caddy.RegisterModule(modules.DinMiddleware{})
    caddy.RegisterModule(modules.DinUpstreams{})
    caddy.RegisterModule(modules.DinSelect{})
    caddy.RegisterModule(modules.DinScoreBasedSelector{})
    // ... metrics and auth registration
}
```

---

## 1. DinMiddleware

**File**: `modules/din_middleware.go` (1013 lines)

### Purpose

The main HTTP middleware that intercepts all incoming requests, determines the target network, filters eligible providers, and orchestrates the routing decision.

### Caddy Module Identity

```go
func (DinMiddleware) CaddyModule() caddy.ModuleInfo {
    return caddy.ModuleInfo{
        ID:  "http.handlers.din",
        New: func() caddy.Module { return new(DinMiddleware) },
    }
}
```

### Implemented Interfaces

| Interface | Purpose |
|-----------|---------|
| `caddy.Module` | Caddy module registration |
| `caddy.Provisioner` | Setup and initialization |
| `caddy.CleanerUpper` | Cleanup on shutdown |
| `caddyhttp.MiddlewareHandler` | HTTP middleware handling |
| `caddyfile.Unmarshaler` | Caddyfile parsing |

### Configuration Structure

```go
type DinMiddleware struct {
    // Network configurations
    Services map[string]*network `json:"services,omitempty"`

    // Registry configuration
    DinRegistry *RegistryConfig `json:"din_registry,omitempty"`

    // Dynamic load balancing
    DynamicLoadBalancing *DynamicLoadBalancingConfig `json:"dynamic_load_balancing,omitempty"`

    // Internal state
    dinRegistrySync    *dinRegistrySync
    watcherScoreSync   *watcherScoreSync
    prometheusClient   prometheus.IPrometheusClient
    httpClient         dinHttp.IHTTPClient
    logger             *zap.Logger
}
```

### Key Responsibilities

1. **Network Resolution**: Extracts network name from request path
2. **Method Extraction**: Parses RPC method from request body for metrics/routing
3. **Provider Filtering**: Creates filters based on supported methods
4. **Context Population**: Attaches eligible providers to request context
5. **Health Check Orchestration**: Starts/manages health check goroutines
6. **Registry Sync**: Manages DIN Registry synchronization
7. **Watcher Score Sync**: Manages dynamic load balancing score sync

### Lifecycle Methods

```go
// Provision sets up the middleware
func (m *DinMiddleware) Provision(ctx caddy.Context) error

// ServeHTTP handles incoming requests
func (m *DinMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error

// Cleanup stops background services
func (m *DinMiddleware) Cleanup() error
```

### ServeHTTP Flow

```
1. Extract network name from path (/network-name/...)
2. Validate network exists
3. Read and parse request body
4. Extract RPC method name
5. Create provider filter (method support, priority)
6. Get eligible providers
7. Attach to request context
8. Call next handler (reverse proxy)
9. Record metrics
```

### Background Services Started

- Health check goroutine per network
- Registry sync polling (if configured)
- Watcher score sync (if configured)

---

## 2. DinUpstreams

**File**: `modules/din_upstreams.go`

### Purpose

Provides the list of eligible upstream providers for Caddy's reverse proxy to use. Implements the `UpstreamSource` interface that Caddy's reverse proxy calls to get available backends.

### Caddy Module Identity

```go
func (DinUpstreams) CaddyModule() caddy.ModuleInfo {
    return caddy.ModuleInfo{
        ID:  "http.reverse_proxy.upstreams.din",
        New: func() caddy.Module { return new(DinUpstreams) },
    }
}
```

### Implemented Interfaces

| Interface | Purpose |
|-----------|---------|
| `caddy.Module` | Caddy module registration |
| `reverseproxy.UpstreamSource` | Provides upstream list |

### Key Method

```go
func (u *DinUpstreams) GetUpstreams(r *http.Request) ([]*reverseproxy.Upstream, error)
```

This method:
1. Retrieves providers from request context (set by DinMiddleware)
2. Filters out unhealthy providers
3. Returns list of Caddy upstream objects

### Provider Retrieval

Providers are retrieved from request context using:
```go
providers := r.Context().Value(DinUpstreamsContextKey).(map[string]*provider)
```

### Health Filtering

Only returns providers where `provider.Available()` returns `true`:
- Health status is Healthy or Warning
- Provider is not explicitly disabled

---

## 3. DinSelect

**File**: `modules/din_select.go`

### Purpose

Selects one provider from the pool of available upstreams. Implements session affinity using header-based hashing, with score-based selection as fallback.

### Caddy Module Identity

```go
func (DinSelect) CaddyModule() caddy.ModuleInfo {
    return caddy.ModuleInfo{
        ID:  "http.reverse_proxy.selection_policies.din",
        New: func() caddy.Module { return new(DinSelect) },
    }
}
```

### Implemented Interfaces

| Interface | Purpose |
|-----------|---------|
| `caddy.Module` | Caddy module registration |
| `caddy.Provisioner` | Setup |
| `reverseproxy.Selector` | Provider selection |

### Configuration

```go
type DinSelect struct {
    // Header for session affinity
    SessionHeader string `json:"session_header,omitempty"`

    // Fallback selector
    FallbackRaw json.RawMessage `json:"fallback,omitempty"`
    Fallback    reverseproxy.Selector `json:"-"`
}
```

### Selection Algorithm

```go
func (s *DinSelect) Select(pool reverseproxy.UpstreamPool, r *http.Request, rw http.ResponseWriter) *reverseproxy.Upstream
```

1. **Check Session Header**: Look for `Din-Session-Id` header
2. **If Present**: Hash the header value to select consistent upstream (sticky session)
3. **If Absent**: Use fallback selector (typically DinScoreBasedSelector)

### Session Affinity

The header hash provides session stickiness:
- Same session ID always routes to same provider (if available)
- Enables stateful interactions with providers
- Useful for transaction sequences

---

## 4. DinScoreBasedSelector

**File**: `modules/din_scorebased_selector.go`

### Purpose

Selects a provider using weighted random selection based on watcher scores. Providers with higher scores have proportionally higher selection probability.

### Caddy Module Identity

```go
func (DinScoreBasedSelector) CaddyModule() caddy.ModuleInfo {
    return caddy.ModuleInfo{
        ID:  "http.reverse_proxy.selection_policies.din_score_based",
        New: func() caddy.Module { return new(DinScoreBasedSelector) },
    }
}
```

### Implemented Interfaces

| Interface | Purpose |
|-----------|---------|
| `caddy.Module` | Caddy module registration |
| `reverseproxy.Selector` | Provider selection |

### Selection Algorithm

```go
func (s *DinScoreBasedSelector) Select(pool reverseproxy.UpstreamPool, r *http.Request, rw http.ResponseWriter) *reverseproxy.Upstream
```

1. Get network from request context
2. Retrieve provider scores from network's score manager
3. Build cumulative probability distribution
4. Generate random number
5. Select provider based on weighted distribution

### Weighted Random Selection

```
Provider A: Score 0.8  -> Weight 0.8
Provider B: Score 0.6  -> Weight 0.6
Provider C: Score 0.4  -> Weight 0.4

Total: 1.8

Probability A: 0.8/1.8 = 44.4%
Probability B: 0.6/1.8 = 33.3%
Probability C: 0.4/1.8 = 22.2%
```

### Fallback Behavior

If no scores are available, falls back to random selection among available providers.

---

## Module Interaction Flow

```
┌──────────────────────────────────────────────────────────────┐
│                     HTTP Request                              │
└──────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌──────────────────────────────────────────────────────────────┐
│                     DinMiddleware                             │
│   - Resolves network from path                                │
│   - Extracts RPC method                                       │
│   - Filters providers by method support                       │
│   - Attaches providers to context                             │
└──────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌──────────────────────────────────────────────────────────────┐
│                 Caddy Reverse Proxy                           │
│   - Calls DinUpstreams.GetUpstreams()                         │
│   - Calls DinSelect.Select()                                  │
│   - Forwards request to selected upstream                     │
└──────────────────────────────────────────────────────────────┘
                              │
          ┌───────────────────┴───────────────────┐
          │                                       │
          ▼                                       ▼
┌─────────────────────┐                 ┌─────────────────────┐
│    DinUpstreams     │                 │     DinSelect       │
│ - Gets providers    │                 │ - Checks session    │
│   from context      │                 │   header            │
│ - Filters healthy   │                 │ - Hash or fallback  │
│ - Returns pool      │                 │   to score-based    │
└─────────────────────┘                 └─────────────────────┘
                                                  │
                                                  ▼
                                        ┌─────────────────────┐
                                        │ DinScoreBasedSelect │
                                        │ - Weighted random   │
                                        │   by watcher scores │
                                        └─────────────────────┘
```

---

## Global Network Registry

Modules communicate through a global registry:

```go
var globalNetworkRegistry = make(map[string]*network)
var globalNetworkMutex sync.RWMutex
```

This allows DinUpstreams and DinSelect to access network configuration set by DinMiddleware.

### Registry Operations

```go
// Set network in registry
func setNetworkInGlobalRegistry(name string, n *network)

// Get network from registry
func getNetworkFromGlobalRegistry(name string) (*network, bool)

// Remove network from registry
func removeNetworkFromGlobalRegistry(name string)
```

---

## Context Keys

Data is passed between modules via request context:

| Key | Type | Purpose |
|-----|------|---------|
| `DinUpstreamsContextKey` | `map[string]*provider` | Eligible providers |
| `RequestProviderKey` | `string` | Selected provider name |
| `RequestProviderPriorityKey` | `int` | Selected provider priority |
| `RequestBodyKey` | `[]byte` | Original request body |
| `RequestMethodKey` | `string` | Extracted RPC method |
| `HealthStatusKey` | `HealthStatus` | Provider health status |
| `BlockNumberKey` | `uint64` | Latest block number |

---

## Related Documentation

- [Request Flow](./09-request-flow.md) - Detailed request lifecycle
- [Network Configuration](./03-network-configuration.md) - Network and provider setup
- [Dynamic Load Balancing](./07-dynamic-load-balancing.md) - Score-based selection details
- [Caddyfile Configuration](./11-caddyfile-configuration.md) - Configuration syntax
