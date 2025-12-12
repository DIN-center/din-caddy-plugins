# Extending DIN Caddy Plugins

This document provides guides for extending the system with new network handlers, authentication methods, and other customizations.

## Adding a New Network Handler

### Overview

To add support for a new blockchain network:

1. Create a new handler file
2. Implement the `NetworkHandler` interface
3. Register the handler type
4. Add Caddyfile parsing support
5. Write tests

### Step 1: Create Handler File

Create `lib/network/<network>_handler.go`:

```go
package network

import (
    "context"
    "encoding/json"
    "net/http"
)

// CosmosHandler handles Cosmos SDK-based chains
type CosmosHandler struct {
    BaseHandler
}

// NewCosmosHandler creates a new Cosmos handler
func NewCosmosHandler() *CosmosHandler {
    return &CosmosHandler{}
}
```

### Step 2: Implement NetworkHandler Interface

```go
// Identity methods
func (h *CosmosHandler) GetType() HandlerType {
    return CosmosHandler
}

func (h *CosmosHandler) GetName() string {
    return "Cosmos"
}

func (h *CosmosHandler) GetRequestType() RequestType {
    return RequestTypeREST  // Cosmos uses REST API
}

// Request processing
func (h *CosmosHandler) ProcessRequest(r *http.Request) error {
    // Add any request modifications
    return nil
}

func (h *CosmosHandler) ExtractMethod(body []byte) (string, error) {
    // For REST, extract from path
    return "", nil
}

func (h *CosmosHandler) ConfigureRequestPath(r *http.Request, basePath string) error {
    // Configure the upstream path
    return nil
}

// Block operations
func (h *CosmosHandler) GetLatestBlockNumber(
    ctx context.Context,
    client IHTTPClient,
    url string,
    headers map[string]string,
) (uint64, error) {
    // GET /cosmos/base/tendermint/v1beta1/blocks/latest
    req, _ := http.NewRequestWithContext(ctx, "GET",
        url+"/cosmos/base/tendermint/v1beta1/blocks/latest", nil)

    for k, v := range headers {
        req.Header.Set(k, v)
    }

    resp, err := client.DoWithContext(ctx, req)
    if err != nil {
        return 0, err
    }
    defer resp.Body.Close()

    var result struct {
        Block struct {
            Header struct {
                Height string `json:"height"`
            } `json:"header"`
        } `json:"block"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return 0, err
    }

    height, _ := strconv.ParseUint(result.Block.Header.Height, 10, 64)
    return height, nil
}

// Health check methods
func (h *CosmosHandler) GetHealthCheckMethod() string {
    return "/cosmos/base/tendermint/v1beta1/blocks/latest"
}

func (h *CosmosHandler) CreateHealthCheckPayload() ([]byte, error) {
    return nil, nil  // REST uses GET, no payload
}

func (h *CosmosHandler) ParseHealthCheckResponse(body []byte) (uint64, error) {
    var result struct {
        Block struct {
            Header struct {
                Height string `json:"height"`
            } `json:"header"`
        } `json:"block"`
    }

    if err := json.Unmarshal(body, &result); err != nil {
        return 0, err
    }

    return strconv.ParseUint(result.Block.Header.Height, 10, 64)
}

// Chain ID methods
func (h *CosmosHandler) GetChainID() string {
    return ""  // Set during validation
}

func (h *CosmosHandler) ValidateChainID(body []byte, expected string) (bool, error) {
    var result struct {
        Block struct {
            Header struct {
                ChainID string `json:"chain_id"`
            } `json:"header"`
        } `json:"block"`
    }

    if err := json.Unmarshal(body, &result); err != nil {
        return false, err
    }

    return result.Block.Header.ChainID == expected, nil
}

// Archive mode (optional)
func (h *CosmosHandler) SupportsArchiveMode() bool {
    return false
}

func (h *CosmosHandler) PerformArchiveCheck(
    ctx context.Context,
    client IHTTPClient,
    url string,
    headers map[string]string,
) (bool, error) {
    return false, nil
}

// Lifecycle
func (h *CosmosHandler) Initialize(ctx context.Context) error {
    return nil
}
```

### Step 3: Register Handler Type

In `modules/consts.go`:

```go
const (
    EVMHandler            HandlerType = "evm"
    BeaconHandler         HandlerType = "beacon-chain"
    // ... existing handlers
    CosmosHandler         HandlerType = "cosmos"  // Add new type
)
```

In `lib/network/registry.go`:

```go
func init() {
    RegisterHandler(CosmosHandler, func() NetworkHandler {
        return NewCosmosHandler()
    })
}
```

### Step 4: Add Caddyfile Support

In `modules/caddy_unmarshaller.go`, add parsing for the new handler type:

```go
case "cosmos":
    n.HandlerType = CosmosHandler
```

### Step 5: Write Tests

Create `lib/network/cosmos_handler_test.go`:

```go
func TestCosmosHandler_GetLatestBlockNumber(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(map[string]interface{}{
            "block": map[string]interface{}{
                "header": map[string]interface{}{
                    "height": "12345678",
                },
            },
        })
    }))
    defer server.Close()

    handler := NewCosmosHandler()
    blockNum, err := handler.GetLatestBlockNumber(
        context.Background(),
        http.DefaultClient,
        server.URL,
        nil,
    )

    assert.NoError(t, err)
    assert.Equal(t, uint64(12345678), blockNum)
}
```

---

## Adding a New Authentication Method

### Step 1: Implement IAuthClient

Create `lib/auth/apikey/client.go`:

```go
package apikey

import (
    "net/http"

    "github.com/DIN-center/din-caddy-plugins/lib/auth"
)

type Client struct {
    APIKey     string `json:"api_key,omitempty"`
    HeaderName string `json:"header_name,omitempty"`
}

func NewClient(apiKey, headerName string) *Client {
    if headerName == "" {
        headerName = "X-API-Key"
    }
    return &Client{
        APIKey:     apiKey,
        HeaderName: headerName,
    }
}

func (c *Client) Start() error {
    return nil  // No initialization needed
}

func (c *Client) Error() error {
    return nil
}

func (c *Client) GetToken() (*auth.AuthToken, error) {
    return &auth.AuthToken{
        Headers: map[string]string{
            c.HeaderName: c.APIKey,
        },
    }, nil
}

func (c *Client) Sign(r *http.Request) error {
    r.Header.Set(c.HeaderName, c.APIKey)
    return nil
}

func (c *Client) Stop() {
    // No cleanup needed
}
```

### Step 2: Add Provider Support

In `modules/provider.go`:

```go
type provider struct {
    // ... existing fields
    APIKeyClient *apikey.Client `json:"api_key,omitempty"`
}

func (p *provider) AuthClient() auth.IAuthClient {
    if p.Auth != nil {
        return p.Auth
    }
    if p.OIDCClient != nil {
        return p.OIDCClient
    }
    if p.APIKeyClient != nil {
        return p.APIKeyClient
    }
    return nil
}
```

### Step 3: Add Caddyfile Parsing

```go
case "api_key":
    d.Next()  // consume 'api_key'
    provider.APIKeyClient = &apikey.Client{
        APIKey:     d.Val(),
        HeaderName: "X-API-Key",
    }
```

---

## Adding Custom Metrics

### Step 1: Define Metrics

In `lib/prometheus/prometheus.go`:

```go
var (
    customMetric = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "din_custom_metric_total",
            Help: "Custom metric description",
        },
        []string{"network", "label"},
    )
)

func init() {
    prometheus.MustRegister(customMetric)
}
```

### Step 2: Add to Interface

In `lib/prometheus/interface.go`:

```go
type IPrometheusClient interface {
    // ... existing methods
    RecordCustomMetric(network, label string)
}
```

### Step 3: Implement Method

```go
func (c *PrometheusClient) RecordCustomMetric(network, label string) {
    customMetric.WithLabelValues(network, label).Inc()
}
```

---

## Adding Custom Provider Filters

### Step 1: Define Filter

In `modules/din_provider_filter.go`:

```go
type CustomFilter struct {
    // Filter criteria
    MinScore float64
}

func (f *CustomFilter) Matches(p *provider) bool {
    return p.Score >= f.MinScore
}
```

### Step 2: Integrate with Middleware

```go
func (m *DinMiddleware) createFilters(network *network, method string) []ProviderFilter {
    filters := []ProviderFilter{
        &MethodFilter{Method: method},
        &HealthFilter{},
    }

    // Add custom filter
    if m.CustomFilterConfig != nil {
        filters = append(filters, &CustomFilter{
            MinScore: m.CustomFilterConfig.MinScore,
        })
    }

    return filters
}
```

---

## Adding Custom Selectors

### Step 1: Implement Selector Interface

```go
// modules/din_custom_selector.go
package modules

import (
    "net/http"

    "github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
)

type DinCustomSelector struct {
    // Configuration
}

func (DinCustomSelector) CaddyModule() caddy.ModuleInfo {
    return caddy.ModuleInfo{
        ID:  "http.reverse_proxy.selection_policies.din_custom",
        New: func() caddy.Module { return new(DinCustomSelector) },
    }
}

func (s *DinCustomSelector) Select(
    pool reverseproxy.UpstreamPool,
    r *http.Request,
    rw http.ResponseWriter,
) *reverseproxy.Upstream {
    // Custom selection logic
    return pool[0]
}
```

### Step 2: Register Module

In `module.go`:

```go
func init() {
    caddy.RegisterModule(modules.DinCustomSelector{})
}
```

---

## Adding Caddyfile Directives

### Step 1: Define Directive

In `modules/caddy_unmarshaller.go`:

```go
func (m *DinMiddleware) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
    for d.Next() {
        for d.NextBlock(0) {
            switch d.Val() {
            case "custom_option":
                if !d.NextArg() {
                    return d.ArgErr()
                }
                m.CustomOption = d.Val()
            // ... other cases
            }
        }
    }
    return nil
}
```

### Step 2: Register Directive Order

In `module.go`:

```go
func init() {
    httpcaddyfile.RegisterDirective("din", parseDin)
    httpcaddyfile.RegisterDirectiveOrder("din", "before", "reverse_proxy")
}
```

---

## Best Practices

### Handler Development

1. **Follow existing patterns** - Look at `evm_handler.go` as reference
2. **Handle errors gracefully** - Return meaningful errors
3. **Support context cancellation** - Respect context timeouts
4. **Write comprehensive tests** - Cover all interface methods

### Authentication

1. **Never log credentials** - Use structured logging without secrets
2. **Support token refresh** - Handle expiration gracefully
3. **Thread safety** - Use proper synchronization

### Metrics

1. **Use appropriate types** - Counter for counts, Gauge for values, Histogram for distributions
2. **Label carefully** - Too many labels = high cardinality
3. **Document metrics** - Clear Help text

### Configuration

1. **Provide defaults** - Sensible defaults for optional fields
2. **Validate early** - Check configuration in Provision()
3. **Document options** - Update Caddyfile configuration docs

---

## Related Documentation

- [Network Handlers](./04-network-handlers.md) - Existing handlers
- [Authentication](./05-authentication.md) - Auth implementations
- [Interfaces & Types](./12-interfaces-types.md) - Interface definitions
- [Testing](./13-testing.md) - Testing patterns
