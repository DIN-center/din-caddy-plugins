# New Network Requirements Form

Please provide the following information to add support for your blockchain network.

## 1. Basic Information

**Network Name**: _____________________ (e.g., "Bitcoin", "Polygon")  
**Handler Type ID**: _____________________ (e.g., "bitcoin-esplora", "polygon") - lowercase with hyphens  
**API Documentation**: _____________________ (URL)

## 2. API Type

**Type** (check one):
- [ ] **JSON-RPC** (like Ethereum)
  - Version: [ ] 2.0 [ ] 1.0
  - Example method: `eth_blockNumber`
- [ ] **REST API** (like Bitcoin Esplora)
  - Base path: _____________________ (e.g., `/api/v1`)
- [ ] **GraphQL**

## 3. Authentication

**Auth Required** (check all that apply):
- [ ] **None** (public API)
- [ ] **API Key**
  - Header: _____________________ (e.g., `X-API-Key`)
- [ ] **OAuth2**
  - Token URL: _____________________
  - Grant type: [ ] client_credentials [ ] other: _____

## 4. Critical Endpoints

### 4.1 Health Check (Get Latest Block)
**Purpose**: Monitor provider health and get current block number

**Endpoint**: _____________________ (e.g., `/blocks/latest` or `eth_blockNumber`)  
**Method**: [ ] GET [ ] POST  

**Response Example**:
```json
// Show where the block number is located
{
  "height": 123456,  // <- block number here
  "hash": "0xabc..."
}
```

### 4.2 Get Block by Number
**Purpose**: Retrieve block details by height/number

**Endpoint**: _____________________ (e.g., `/blocks/{number}` or `eth_getBlockByNumber`)  
**Supported**: [ ] Yes [ ] No [ ] Requires 2 calls (number→hash→block)

**Response Example**:
```json
{
  "number": 123456,     // <- block number
  "hash": "0xabc...",   // <- block hash (required)
  "timestamp": 1234567  // <- if available
}
```

### 4.3 Get Chain ID (if applicable)
**Endpoint**: _____________________ (e.g., `eth_chainId`)  
**Static Chain IDs**: _____________________ (e.g., `bitcoin:mainnet`, `evm:1`)

## 5. Error Handling

### 5.1 Error Format
```json
// Provide your API's error format
{
  "error": {
    "code": -32000,
    "message": "Error description"
  }
}
```

### 5.2 Retry Logic
**Retry on** (check all that apply):
- [ ] HTTP 5xx errors
- [ ] HTTP 429 (rate limit)
- [ ] Timeout/connection errors
- [ ] Specific error codes: _____________________

**Don't retry on**:
- [ ] HTTP 4xx errors (except 429)
- [ ] Auth failures
- [ ] Invalid method errors

## 6. Special Requirements

### 6.1 Path Patterns (REST APIs only)
List dynamic paths that need normalization for metrics:
- `/tx/{txid}` - Transaction by ID
- `/address/{address}` - Address info
- `/block/{height}` - Block by height
- _____________________

### 6.2 Rate Limits
- Limit: _____ requests per _____
- Rate limit header: _____________________ (e.g., `X-RateLimit-Remaining`)

### 6.3 Unique Considerations
List any special behavior or limitations:
- _____________________
- _____________________

## 7. Test Data

### 7.1 Example Requests
Provide 2-3 real examples we can use for testing:

**Health check**:
```
GET https://api.example.com/blocks/latest
Response: {"height": 838530}
```

**Get block**:
```
POST https://api.example.com/
{"jsonrpc": "2.0", "method": "getBlock", "params": [123456], "id": 1}
Response: {"result": {"hash": "0xabc...", "number": 123456}}
```

<<<<<<< HEAD
### 7.2 Configuration Example
=======
#### For REST Networks:

```go
func (h *YourNetworkHandler) ProcessRequest(req *http.Request, provider Provider) error {
    if err := h.ValidateRequest(req); err != nil {
        return err
    }
    
    translatedPath, err := h.TranslatePath(req.URL.Path, provider)
    if err != nil {
        return err
    }
    
    req.URL.Path = translatedPath
    req.URL.RawPath = translatedPath
    
    return nil
}

func (h *YourNetworkHandler) ValidateRequest(req *http.Request) error {
    // Validate REST request
    allowedMethods := []string{"GET", "POST", "PUT", "DELETE"}
    methodAllowed := false
    for _, method := range allowedMethods {
        if req.Method == method {
            methodAllowed = true
            break
        }
    }
    
    if !methodAllowed {
        return fmt.Errorf("method %s not allowed", req.Method)
    }
    
    return nil
}

func (h *YourNetworkHandler) TranslatePath(gatewayPath string, provider Provider) (string, error) {
    // Remove network prefix
    relativePath := strings.TrimPrefix(gatewayPath, "/"+h.config.Name)
    
    // Apply provider-specific transformations
    if provider.GetPathPrefix() != "" {
        return provider.GetPathPrefix() + relativePath, nil
    }
    
    return relativePath, nil
}

func (h *YourNetworkHandler) NormalizeEndpoint(path string) string {
    // Implement path normalization for metrics
    // Example: /api/users/123 -> /api/users/{id}
    return h.normalizePathPatterns(path)
}

func (h *YourNetworkHandler) normalizePathPatterns(path string) string {
    // Add regex patterns to normalize dynamic paths
    patterns := map[string]string{
        `/users/\d+`:     "/users/{id}",
        `/blocks/0x[a-fA-F0-9]+`: "/blocks/{hash}",
    }
    
    for pattern, replacement := range patterns {
        if matched, _ := regexp.MatchString(pattern, path); matched {
            return regexp.MustCompile(pattern).ReplaceAllString(path, replacement)
        }
    }
    
    return path
}
```

### Step 5: Implement Health Check Methods

```go
func (h *YourNetworkHandler) GetLatestBlock(provider Provider) (*BlockInfo, error) {
    // For RPC networks, make health check RPC call
    resp, err := h.makeHealthCheckRequest(provider, "your_blockNumber")
    if err != nil {
        return nil, err
    }
    
    var result YourNetworkBlockResponse
    if err := json.Unmarshal(resp, &result); err != nil {
        return nil, fmt.Errorf("failed to parse block response: %w", err)
    }
    
    return &BlockInfo{
        Number:    result.BlockNumber,
        Hash:      result.BlockHash,
        Timestamp: time.Now(),
    }, nil
}

func (h *YourNetworkHandler) CheckHealth(provider Provider) (*HealthStatus, error) {
    start := time.Now()
    
    blockInfo, err := h.GetLatestBlock(provider)
    if err != nil {
        return &HealthStatus{
            Healthy:     false,
            BlockNumber: 0,
            Latency:     time.Since(start),
            Error:       err,
        }, err
    }
    
    return &HealthStatus{
        Healthy:     true,
        BlockNumber: blockInfo.Number,
        Latency:     time.Since(start),
        Error:       nil,
    }, nil
}
```

### Step 6: Implement Error Handling

```go
func (h *YourNetworkHandler) ParseResponse(body []byte, statusCode int) error {
    if statusCode >= 400 {
        return fmt.Errorf("HTTP error: %d", statusCode)
    }
    
    // For RPC networks, check for JSON-RPC errors
    if h.GetRequestType() == RequestTypeRPC {
        var response dinHttp.JSONRPCResponse
        if err := json.Unmarshal(body, &response); err != nil {
            return nil // Not a JSON-RPC response, that's ok
        }
        
        if response.Error != nil {
            return fmt.Errorf("RPC error: %s", response.Error.Message)
        }
    }
    
    return nil
}

func (h *YourNetworkHandler) IsRetryableError(err error, statusCode int) bool {
    // Define which errors should trigger retries
    if statusCode >= 500 {
        return true // Server errors are retryable
    }
    
    if statusCode == 429 {
        return true // Rate limit errors are retryable
    }
    
    // Network-specific error patterns
    if err != nil {
        message := strings.ToLower(err.Error())
        retryablePatterns := []string{
            "timeout",
            "connection",
            "temporary",
            "rate limit",
        }
        
        for _, pattern := range retryablePatterns {
            if strings.Contains(message, pattern) {
                return true
            }
        }
    }
    
    return false
}
```

### Step 7: Register Your Handler

Add your handler to `lib/network/handlers.go`:

```go
// Add to RegisterBuiltinHandlers function
func RegisterBuiltinHandlers() {
    // ... existing registrations ...
    
    // Register your new handler
    if err := DefaultRegistry.RegisterHandler("yournetwork", NewYourNetworkHandlerFactory()); err != nil {
        log.Printf("Failed to register YourNetwork handler: %v", err)
    }
}

// Add your factory function
func NewYourNetworkHandlerFactory() HandlerFactory {
    return func(config *NetworkConfig) (NetworkHandler, error) {
        return NewYourNetworkHandler(config), nil
    }
}
```

## Testing Your Handler

### Step 1: Create Unit Tests

Create `lib/network/yournetwork_handler_test.go`:

```go
package network

import (
    "net/http"
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestYourNetworkHandler_GetType(t *testing.T) {
    handler := NewYourNetworkHandler(&NetworkConfig{})
    assert.Equal(t, "yournetwork", handler.GetType())
}

func TestYourNetworkHandler_GetRequestType(t *testing.T) {
    handler := NewYourNetworkHandler(&NetworkConfig{})
    assert.Equal(t, RequestTypeRPC, handler.GetRequestType())
}

func TestYourNetworkHandler_ValidateRequest(t *testing.T) {
    handler := NewYourNetworkHandler(&NetworkConfig{})
    
    tests := []struct {
        name      string
        method    string
        headers   map[string]string
        expectErr bool
    }{
        {
            name:      "valid POST with JSON",
            method:    "POST",
            headers:   map[string]string{"Content-Type": "application/json"},
            expectErr: false,
        },
        {
            name:      "invalid GET method",
            method:    "GET",
            headers:   map[string]string{"Content-Type": "application/json"},
            expectErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            req := &http.Request{
                Method: tt.method,
                Header: make(http.Header),
            }
            
            for k, v := range tt.headers {
                req.Header.Set(k, v)
            }
            
            err := handler.ValidateRequest(req)
            if tt.expectErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}

func TestYourNetworkHandler_TranslatePath(t *testing.T) {
    handler := NewYourNetworkHandler(&NetworkConfig{Name: "yournetwork"})
    
    tests := []struct {
        name         string
        gatewayPath  string
        provider     Provider
        expected     string
    }{
        {
            name:        "basic path translation",
            gatewayPath: "/yournetwork/api/v1/status",
            provider:    &mockProvider{path: "/rpc"},
            expected:    "/rpc",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := handler.TranslatePath(tt.gatewayPath, tt.provider)
            assert.NoError(t, err)
            assert.Equal(t, tt.expected, result)
        })
    }
}

// Mock provider for testing
type mockProvider struct {
    path string
}

func (p *mockProvider) GetPath() string { return p.path }
func (p *mockProvider) GetPathPrefix() string { return "" }
```

### Step 2: Run Tests

```bash
cd lib/network
go test -v -run TestYourNetwork
```

### Step 3: Integration Tests

Add your handler to the registry test in `lib/network/handler_test.go`:

```go
func TestDefaultRegistry_Initialization(t *testing.T) {
    registry := DefaultRegistry
    
    expectedHandlers := []string{"evm", "beacon-chain", "starknet", "solana", "yournetwork"}
    registeredHandlers := registry.ListHandlers()
    
    for _, expected := range expectedHandlers {
        assert.Contains(t, registeredHandlers, expected)
    }
}
```

## Configuration Integration

### Step 1: Add Configuration Options

Your handler can accept custom configuration through the `NetworkConfig.Custom` field:

```go
func (h *YourNetworkHandler) Initialize(config *NetworkConfig) error {
    h.config = config
    
    // Parse custom configuration
    if config.Custom != nil {
        if apiVersion, ok := config.Custom["api_version"].(string); ok {
            h.apiVersion = apiVersion
        }
        
        if timeout, ok := config.Custom["timeout"].(int); ok {
            h.requestTimeout = time.Duration(timeout) * time.Second
        }
    }
    
    return nil
}
```

### Step 2: Document Configuration Schema

Create example configurations for your Caddyfile:

>>>>>>> origin/feat/support_rest_api_beacon
```caddyfile
network-mainnet {
    type your-network-type
    chain_id "network:mainnet"
    
    providers {
        https://api.example.com {
            priority 0
            # Add auth if needed
        }
    }
}
```

---

## Quick Reference: Required Handler Methods

Your handler will implement these key methods based on your API type:

**All handlers need**:
- Health check endpoint
- Block retrieval 
- Error handling
- Request validation

**JSON-RPC specific**:
- Method name mapping
- Request ID handling
- Batch support (optional)

**REST API specific**:
- Path normalization
- URL parameter handling
- Path stripping for routing

---

**Submission**: Email completed form to: _____________________ or create a GitHub issue