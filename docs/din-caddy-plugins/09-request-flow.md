# Request Flow

This document traces the complete lifecycle of a request through DIN Caddy Plugins, from arrival to response.

## High-Level Flow

```
┌──────────────────────────────────────────────────────────────────────────┐
│                              Client                                       │
│                    POST /ethereum-mainnet                                 │
│                    {"jsonrpc":"2.0","method":"eth_blockNumber",...}      │
└──────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌──────────────────────────────────────────────────────────────────────────┐
│  1. Caddy HTTP Server                                                     │
│     - TLS termination                                                     │
│     - HTTP parsing                                                        │
└──────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌──────────────────────────────────────────────────────────────────────────┐
│  2. DinMiddleware.ServeHTTP()                                            │
│     - Extract network from path                                           │
│     - Parse request body                                                  │
│     - Extract RPC method                                                  │
│     - Filter eligible providers                                           │
│     - Attach to context                                                   │
└──────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌──────────────────────────────────────────────────────────────────────────┐
│  3. Caddy Reverse Proxy                                                   │
│     - Calls DinUpstreams.GetUpstreams()                                  │
│     - Calls DinSelect.Select()                                           │
└──────────────────────────────────────────────────────────────────────────┘
                                    │
              ┌─────────────────────┴─────────────────────┐
              │                                           │
              ▼                                           ▼
┌─────────────────────────┐                 ┌─────────────────────────┐
│  4. DinUpstreams        │                 │  5. DinSelect           │
│     - Get from context  │                 │     - Session affinity  │
│     - Filter healthy    │                 │     - Score-based       │
│     - Return pool       │                 │     - Return upstream   │
└─────────────────────────┘                 └─────────────────────────┘
                                    │
                                    ▼
┌──────────────────────────────────────────────────────────────────────────┐
│  6. Request Preparation                                                   │
│     - Apply authentication                                                │
│     - Set headers                                                         │
│     - Configure path                                                      │
└──────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌──────────────────────────────────────────────────────────────────────────┐
│  7. Upstream Request                                                      │
│     - Forward to provider                                                 │
│     - Wait for response                                                   │
└──────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌──────────────────────────────────────────────────────────────────────────┐
│  8. Response Processing                                                   │
│     - Parse response                                                      │
│     - Record metrics                                                      │
│     - Return to client                                                    │
└──────────────────────────────────────────────────────────────────────────┘
```

---

## Detailed Step-by-Step

### Step 1: Request Arrival

Client sends request to Caddy:

```http
POST /ethereum-mainnet HTTP/1.1
Host: din-router.example.com
Content-Type: application/json
Din-Session-Id: abc123

{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}
```

Caddy handles:
- TLS termination
- HTTP/2 multiplexing
- Request parsing

### Step 2: DinMiddleware Processing

**File**: `modules/din_middleware.go`

```go
func (m *DinMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
    // 2a. Extract network name from path
    networkName := extractNetworkFromPath(r.URL.Path)
    // "/ethereum-mainnet/..." -> "ethereum-mainnet"

    // 2b. Validate network exists
    network, ok := m.Services[networkName]
    if !ok {
        return caddyhttp.Error(http.StatusNotFound, fmt.Errorf("network not found"))
    }

    // 2c. Read and buffer request body
    body, err := io.ReadAll(r.Body)
    if err != nil {
        return err
    }
    r.Body = io.NopCloser(bytes.NewReader(body))

    // 2d. Extract RPC method
    method, err := network.handler.ExtractMethod(body)
    // {"jsonrpc":"2.0","method":"eth_blockNumber",...} -> "eth_blockNumber"

    // 2e. Create provider filter
    filter := createProviderFilter(network, method)

    // 2f. Get eligible providers
    providers := filterProviders(network.Providers, filter)

    // 2g. Attach to request context
    ctx := context.WithValue(r.Context(), DinUpstreamsContextKey, providers)
    ctx = context.WithValue(ctx, RequestMethodKey, method)
    ctx = context.WithValue(ctx, RequestBodyKey, body)
    r = r.WithContext(ctx)

    // 2h. Continue to next handler
    return next.ServeHTTP(w, r)
}
```

### Step 3: Caddy Reverse Proxy

Caddy's built-in reverse proxy module takes over:

```go
// Internal to Caddy reverse_proxy module
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
    // Get available upstreams
    upstreams, err := h.Upstreams.GetUpstreams(r)

    // Select one upstream
    upstream := h.SelectionPolicy.Select(upstreams, r, w)

    // Forward request
    return h.proxyRequest(w, r, upstream)
}
```

### Step 4: DinUpstreams.GetUpstreams()

**File**: `modules/din_upstreams.go`

```go
func (u *DinUpstreams) GetUpstreams(r *http.Request) ([]*reverseproxy.Upstream, error) {
    // 4a. Get providers from context
    providers := r.Context().Value(DinUpstreamsContextKey).(map[string]*provider)

    // 4b. Filter to healthy providers
    var upstreams []*reverseproxy.Upstream
    for name, p := range providers {
        if p.Available() {  // Healthy or Warning status
            upstreams = append(upstreams, p.upstream)
        }
    }

    // 4c. Handle all unhealthy case
    if len(upstreams) == 0 {
        // Return all providers as last resort
        for _, p := range providers {
            upstreams = append(upstreams, p.upstream)
        }
    }

    return upstreams, nil
}
```

### Step 5: DinSelect.Select()

**File**: `modules/din_select.go`

```go
func (s *DinSelect) Select(pool reverseproxy.UpstreamPool, r *http.Request, rw http.ResponseWriter) *reverseproxy.Upstream {
    // 5a. Check for session affinity header
    sessionID := r.Header.Get("Din-Session-Id")

    if sessionID != "" {
        // 5b. Hash-based selection (sticky session)
        hash := fnv.New32a()
        hash.Write([]byte(sessionID))
        index := hash.Sum32() % uint32(len(pool))
        return pool[index]
    }

    // 5c. Fallback to score-based selection
    return s.Fallback.Select(pool, r, rw)
}
```

### Step 5b: DinScoreBasedSelector.Select()

**File**: `modules/din_scorebased_selector.go`

```go
func (s *DinScoreBasedSelector) Select(pool reverseproxy.UpstreamPool, r *http.Request, rw http.ResponseWriter) *reverseproxy.Upstream {
    // Get network from context
    network := getNetworkFromContext(r)

    // Build weighted distribution
    var total float64
    weights := make([]float64, len(pool))
    for i, upstream := range pool {
        score := network.GetProviderScore(upstream.Dial)
        weights[i] = score
        total += score
    }

    // Random selection based on weights
    random := rand.Float64() * total
    var cumulative float64
    for i, weight := range weights {
        cumulative += weight
        if random <= cumulative {
            return pool[i]
        }
    }

    return pool[len(pool)-1]
}
```

### Step 6: Request Preparation

Before forwarding, the request is prepared:

```go
func prepareUpstreamRequest(r *http.Request, provider *provider) {
    // 6a. Apply authentication
    if auth := provider.AuthClient(); auth != nil {
        auth.Sign(r)  // Adds JWT/auth headers
    }

    // 6b. Add custom headers
    for key, value := range provider.Headers {
        r.Header.Set(key, value)
    }

    // 6c. Configure request path
    handler.ConfigureRequestPath(r, provider.HttpUrl)
}
```

### Step 7: Upstream Request

Caddy forwards the request:

```go
// Request to upstream
POST / HTTP/1.1
Host: mainnet.infura.io
Content-Type: application/json
Authorization: Bearer eyJ...
X-Custom-Header: value

{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}
```

Provider responds:

```json
{"jsonrpc":"2.0","result":"0x10a3b5c","id":1}
```

### Step 8: Response Processing

**File**: `modules/response_writer_wrapper.go`

```go
func (m *DinMiddleware) processResponse(w http.ResponseWriter, r *http.Request, resp *http.Response) {
    // 8a. Record metrics
    m.prometheusClient.RecordRequest(
        network,
        provider,
        method,
        resp.StatusCode,
        duration,
    )

    // 8b. Copy response to client
    for key, values := range resp.Header {
        for _, value := range values {
            w.Header().Add(key, value)
        }
    }
    w.WriteHeader(resp.StatusCode)
    io.Copy(w, resp.Body)
}
```

---

## Context Data Flow

Data is passed through the request context:

```
┌─────────────────────────────────────────────────────────────┐
│                     Request Context                          │
├─────────────────────────────────────────────────────────────┤
│  DinUpstreamsContextKey    │  map[string]*provider          │
│  RequestMethodKey          │  "eth_blockNumber"              │
│  RequestBodyKey            │  []byte (original body)         │
│  RequestProviderKey        │  "infura" (selected)            │
│  RequestProviderPriorityKey│  0                              │
│  HealthStatusKey           │  Healthy                        │
│  BlockNumberKey            │  17480540                       │
└─────────────────────────────────────────────────────────────┘
```

---

## Provider Filtering

### Filter Criteria

```go
type providerFilter struct {
    // Method must be in provider's method list (if specified)
    method string

    // Maximum priority to consider
    maxPriority int

    // Require healthy status
    requireHealthy bool
}
```

### Filtering Logic

```go
func filterProviders(providers map[string]*provider, filter providerFilter) map[string]*provider {
    result := make(map[string]*provider)

    for name, p := range providers {
        // Check method support
        if filter.method != "" && len(p.Methods) > 0 {
            if !contains(p.Methods, filter.method) {
                continue
            }
        }

        // Check priority
        if p.Priority > filter.maxPriority {
            continue
        }

        // Check health
        if filter.requireHealthy && !p.Available() {
            continue
        }

        result[name] = p
    }

    return result
}
```

---

## Error Handling

### Network Not Found

```go
if network == nil {
    return caddyhttp.Error(http.StatusNotFound,
        fmt.Errorf("network '%s' not found", networkName))
}
// Returns: 404 Not Found
```

### No Healthy Providers

```go
if len(upstreams) == 0 {
    // Fallback: try all providers anyway
    for _, p := range providers {
        upstreams = append(upstreams, p.upstream)
    }
}
// Best-effort routing
```

### Upstream Failure

```go
if resp.StatusCode >= 500 {
    // Record error metric
    m.prometheusClient.RecordError(network, provider, "upstream_error")

    // Caddy may retry with different upstream
}
```

### Network Unreachable

```go
if err == context.DeadlineExceeded {
    return caddyhttp.Error(523,
        fmt.Errorf("origin unreachable"))
}
// Returns: 523 Origin Is Unreachable
```

---

## Request Types

### JSON-RPC (EVM, Bitcoin, Solana, StarkNet, Tron)

```
POST /network-name
Body: {"jsonrpc":"2.0","method":"...","params":[...],"id":1}
```

### REST API (Beacon Chain, Esplora)

```
GET /network-name/eth/v1/beacon/headers/head
# or
GET /network-name/api/blocks/tip/height
```

### Path Handling

| Handler Type | Path Behavior |
|--------------|---------------|
| JSON-RPC | Path stripped, body contains method |
| REST | Path forwarded to upstream |

---

## Timing Diagram

```
Client          DinMiddleware      DinUpstreams     DinSelect       Provider
  │                  │                  │               │               │
  │  POST /eth...    │                  │               │               │
  │─────────────────▶│                  │               │               │
  │                  │                  │               │               │
  │                  │ Extract network  │               │               │
  │                  │ Parse body       │               │               │
  │                  │ Filter providers │               │               │
  │                  │                  │               │               │
  │                  │ GetUpstreams()   │               │               │
  │                  │─────────────────▶│               │               │
  │                  │                  │ Filter healthy│               │
  │                  │  [upstreams]     │               │               │
  │                  │◀─────────────────│               │               │
  │                  │                  │               │               │
  │                  │ Select()         │               │               │
  │                  │─────────────────────────────────▶│               │
  │                  │                  │               │ Hash/Score    │
  │                  │  upstream        │               │               │
  │                  │◀─────────────────────────────────│               │
  │                  │                  │               │               │
  │                  │ Forward request  │               │               │
  │                  │─────────────────────────────────────────────────▶│
  │                  │                  │               │               │
  │                  │                  │               │   Response    │
  │                  │◀─────────────────────────────────────────────────│
  │                  │                  │               │               │
  │                  │ Record metrics   │               │               │
  │    Response      │                  │               │               │
  │◀─────────────────│                  │               │               │
  │                  │                  │               │               │
```

---

## Related Documentation

- [Core Modules](./02-core-modules.md) - Module details
- [Network Handlers](./04-network-handlers.md) - Handler-specific processing
- [Authentication](./05-authentication.md) - Auth flow
- [Dynamic Load Balancing](./07-dynamic-load-balancing.md) - Score-based selection
