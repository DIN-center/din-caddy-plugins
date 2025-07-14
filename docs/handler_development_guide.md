# Network Handler Development Guide

## Overview

This guide provides step-by-step instructions for adding new network type handlers to the DIN Caddy middleware. The handler registry system allows you to add support for any blockchain network or API type without modifying the core middleware logic.

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Quick Start](#quick-start)
3. [Handler Interface](#handler-interface)
4. [Step-by-Step Implementation](#step-by-step-implementation)
5. [Testing Your Handler](#testing-your-handler)
6. [Configuration Integration](#configuration-integration)
7. [Backward Compatibility](#backward-compatibility)
8. [Real-World Examples](#real-world-examples)
9. [Best Practices](#best-practices)
10. [Troubleshooting](#troubleshooting)

## Architecture Overview

The DIN Caddy middleware uses a **handler registry pattern** that allows different network types to be supported through pluggable handlers:

```
Request → Middleware → Handler Registry → Specific Handler → Provider
```

### Key Components

1. **NetworkHandler Interface**: Defines the contract all handlers must implement
2. **HandlerRegistry**: Manages handler registration and instantiation
3. **HandlerFactory**: Creates handler instances with specific configurations
4. **Network Handlers**: Type-specific implementations (EVM, Beacon Chain, etc.)

### Supported Request Types

- **RequestTypeRPC**: JSON-RPC based APIs (Ethereum, Starknet, Solana)
- **RequestTypeREST**: RESTful APIs (Beacon Chain, Bitcoin Esplora)
- **RequestTypeGraphQL**: GraphQL APIs (future support)

## Quick Start

To add a new network type handler, follow these 5 steps:

1. **Create Handler Implementation**: Implement the `NetworkHandler` interface
2. **Create Handler Factory**: Create a factory function for your handler
3. **Register Handler**: Add your handler to the registry
4. **Add Tests**: Create unit tests for your handler
5. **Update Documentation**: Document the configuration options

### Minimal Example

```go
// lib/network/mynetwork_handler.go
package network

type MyNetworkHandler struct {
    config  *NetworkConfig
    version string
}

func NewMyNetworkHandler(config *NetworkConfig) *MyNetworkHandler {
    return &MyNetworkHandler{
        config:  config,
        version: "1.0.0",
    }
}

func (h *MyNetworkHandler) GetType() string { return "mynetwork" }
func (h *MyNetworkHandler) GetName() string { return "My Network Handler" }
func (h *MyNetworkHandler) GetVersion() string { return h.version }
func (h *MyNetworkHandler) GetRequestType() RequestType { return RequestTypeRPC }

// ... implement remaining interface methods
```

## Handler Interface

All network handlers must implement the `NetworkHandler` interface:

```go
type NetworkHandler interface {
    // Metadata - Required for registry identification
    GetType() string                    // Handler type identifier
    GetName() string                    // Human-readable name
    GetVersion() string                 // Handler version
    GetRequestType() RequestType        // RPC, REST, or GraphQL
    
    // Lifecycle - Called by registry
    Initialize(config *NetworkConfig) error  // Setup handler
    Shutdown() error                         // Cleanup resources
    
    // Request Processing - Core functionality
    ProcessRequest(req *http.Request, provider Provider) error
    ValidateRequest(req *http.Request) error
    TranslatePath(gatewayPath string, provider Provider) (string, error)
    NormalizeEndpoint(path string) string
    
    // Health & Monitoring - For provider health checks
    GetLatestBlock(provider Provider) (*BlockInfo, error)
    CheckHealth(provider Provider) (*HealthStatus, error)
    
    // Error Handling - For retry logic
    ParseResponse(body []byte, statusCode int) error
    IsRetryableError(err error, statusCode int) bool
}
```

## Step-by-Step Implementation

### Step 1: Create Your Handler File

Create a new file `lib/network/{your_network}_handler.go`:

```go
package network

import (
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
    "time"
    dinHttp "github.com/DIN-center/din-caddy-plugins/lib/http"
)

type YourNetworkHandler struct {
    config  *NetworkConfig
    version string
    // Add any network-specific fields here
    apiVersion    string
    baseEndpoint  string
}

func NewYourNetworkHandler(config *NetworkConfig) *YourNetworkHandler {
    return &YourNetworkHandler{
        config:      config,
        version:     "1.0.0",
        apiVersion:  "v1", // Example network-specific field
        baseEndpoint: "/api/v1", // Example base path
    }
}
```

### Step 2: Implement Metadata Methods

```go
// Metadata methods for registry identification
func (h *YourNetworkHandler) GetType() string {
    return "yournetwork" // Must be unique across all handlers
}

func (h *YourNetworkHandler) GetName() string {
    return "Your Network Protocol Handler"
}

func (h *YourNetworkHandler) GetVersion() string {
    return h.version
}

func (h *YourNetworkHandler) GetRequestType() RequestType {
    // Choose based on your network's API style:
    return RequestTypeRPC   // For JSON-RPC APIs
    // return RequestTypeREST  // For RESTful APIs
    // return RequestTypeGraphQL // For GraphQL APIs
}
```

### Step 3: Implement Lifecycle Methods

```go
func (h *YourNetworkHandler) Initialize(config *NetworkConfig) error {
    h.config = config
    
    // Network-specific initialization
    if config.Custom != nil {
        if apiVer, ok := config.Custom["api_version"].(string); ok {
            h.apiVersion = apiVer
        }
    }
    
    // Validate required configuration
    if h.config.ChainID == "" {
        return fmt.Errorf("chain_id is required for %s networks", h.GetType())
    }
    
    return nil
}

func (h *YourNetworkHandler) Shutdown() error {
    // Cleanup any resources (connections, goroutines, etc.)
    return nil
}
```

### Step 4: Implement Request Processing

#### For JSON-RPC Networks:

```go
func (h *YourNetworkHandler) ProcessRequest(req *http.Request, provider Provider) error {
    if err := h.ValidateRequest(req); err != nil {
        return err
    }
    
    // Translate path for the provider
    translatedPath, err := h.TranslatePath(req.URL.Path, provider)
    if err != nil {
        return err
    }
    
    req.URL.Path = translatedPath
    req.URL.RawPath = translatedPath
    
    return nil
}

func (h *YourNetworkHandler) ValidateRequest(req *http.Request) error {
    // Validate JSON-RPC request
    if req.Method != "POST" {
        return fmt.Errorf("your network requires POST method, got %s", req.Method)
    }
    
    contentType := req.Header.Get("Content-Type")
    if !strings.Contains(contentType, "application/json") {
        return fmt.Errorf("invalid content type: %s", contentType)
    }
    
    // Add network-specific validation here
    return nil
}

func (h *YourNetworkHandler) TranslatePath(gatewayPath string, provider Provider) (string, error) {
    // For RPC, typically just use provider's configured path
    if provider != nil && provider.GetPath() != "" {
        return provider.GetPath(), nil
    }
    return gatewayPath, nil
}

func (h *YourNetworkHandler) NormalizeEndpoint(path string) string {
    // For RPC, endpoint is the method name from request body
    return path
}
```

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
    
    expectedHandlers := []string{"evm", "beacon_chain", "starknet", "solana", "yournetwork"}
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

```caddyfile
# Basic configuration
yournetwork-mainnet {
    type yournetwork
    chain_id "yournetwork:mainnet"
    healthcheck_method "your_blockNumber"
    
    providers {
        https://api.yournetwork.com/rpc {
            priority 0
        }
    }
}

# Advanced configuration with custom options
yournetwork-testnet {
    type yournetwork
    chain_id "yournetwork:testnet"
    healthcheck_method "your_blockNumber"
    healthcheck_endpoint "/health"  # For REST APIs
    
    # Custom configuration (passed to handler via NetworkConfig.Custom)
    custom_config {
        api_version "v2"
        timeout 30
        rate_limit 100
    }
    
    providers {
        https://testnet.yournetwork.com {
            path_prefix "/api/v2"
            priority 0
            headers {
                Authorization "Bearer token123"
                X-API-Key "your-api-key"
            }
        }
    }
}
```

## Backward Compatibility

When developing handlers, you may need to support different versions of dependencies or struct definitions. This is especially important when your local development environment uses newer versions than what's available in CI/CD or remote builds.

### Understanding Version Mismatches

Version mismatches typically occur when:

1. **Local Development**: Uses newer upstream dependencies with additional struct fields
2. **CI/CD Builds**: Use pinned versions that may lack newer fields
3. **Remote Dependencies**: May not have the latest struct definitions

### Safe Field Access Pattern

To handle version mismatches, use **reflection-based field access** instead of direct field access:

#### The Problem: Direct Field Access

```go
// ❌ This breaks when ChainId field doesn't exist in older versions
if regNetwork.NetworkConfig.ChainId != "" {
    network.ChainId = regNetwork.NetworkConfig.ChainId
}
```

#### The Solution: Reflection-Based Access

```go
// ✅ This safely handles missing fields across versions
if chainId := getNetworkConfigStringField(regNetwork.NetworkConfig, "ChainId"); chainId != "" {
    network.ChainId = chainId
}
```

### Helper Functions for Safe Access

Add these helper functions to your handler files:

```go
import "reflect"

// getNetworkConfigUint8Field safely gets a uint8 field using reflection
func getNetworkConfigUint8Field(config *dinreg.NetworkConfig, fieldName string) uint8 {
    if config == nil {
        return 0
    }
    v := reflect.ValueOf(config).Elem()
    field := v.FieldByName(fieldName)
    if !field.IsValid() || field.Kind() != reflect.Uint8 {
        return 0
    }
    return uint8(field.Uint())
}

// getNetworkConfigStringField safely gets a string field using reflection
func getNetworkConfigStringField(config *dinreg.NetworkConfig, fieldName string) string {
    if config == nil {
        return ""
    }
    v := reflect.ValueOf(config).Elem()
    field := v.FieldByName(fieldName)
    if !field.IsValid() || field.Kind() != reflect.String {
        return ""
    }
    return field.String()
}

// getNetworkConfigBoolField safely gets a bool field using reflection
func getNetworkConfigBoolField(config *dinreg.NetworkConfig, fieldName string) bool {
    if config == nil {
        return false
    }
    v := reflect.ValueOf(config).Elem()
    field := v.FieldByName(fieldName)
    if !field.IsValid() || field.Kind() != reflect.Bool {
        return false
    }
    return field.Bool()
}

// getNetworkConfigUint16Field safely gets a uint16 field using reflection
func getNetworkConfigUint16Field(config *dinreg.NetworkConfig, fieldName string) uint16 {
    if config == nil {
        return 0
    }
    v := reflect.ValueOf(config).Elem()
    field := v.FieldByName(fieldName)
    if !field.IsValid() || field.Kind() != reflect.Uint16 {
        return 0
    }
    return uint16(field.Uint())
}
```

### Backward Compatible Implementation Example

Here's how to safely access potentially missing fields:

```go
func (d *DinMiddleware) syncNetworkConfig(regNetwork *din.Network, network *network) (*network, error) {
    // Always available fields - direct access is safe
    registryHCMethod, err := d.DingoClient.GetNetworkMethodNameByBit(
        regNetwork.Name, 
        regNetwork.NetworkConfig.HealthcheckMethodBit,
    )
    if err != nil {
        return nil, err
    }

    // Potentially missing fields - use reflection-based access
    var registryChainIdMethod, registryCallContractMethod string

    // Safe access to ChainIdMethodBit (may not exist in older versions)
    if chainIdBit := getNetworkConfigUint8Field(regNetwork.NetworkConfig, "ChainIdMethodBit"); chainIdBit > 0 {
        registryChainIdMethod, err = d.DingoClient.GetNetworkMethodNameByBit(regNetwork.Name, chainIdBit)
        if err != nil {
            d.logger.Debug("Failed to get chain ID method", zap.Error(err))
        }
    }

    // Safe access to CallContractMethodBit
    if callContractBit := getNetworkConfigUint8Field(regNetwork.NetworkConfig, "CallContractMethodBit"); callContractBit > 0 {
        registryCallContractMethod, err = d.DingoClient.GetNetworkMethodNameByBit(regNetwork.Name, callContractBit)
        if err != nil {
            d.logger.Debug("Failed to get call contract method", zap.Error(err))
        }
    }

    // Safe access to string fields
    if chainId := getNetworkConfigStringField(regNetwork.NetworkConfig, "ChainId"); chainId != "" && chainId != network.ChainId {
        d.logger.Debug("Setting network chain ID", zap.String("chain_id", chainId))
        network.ChainId = chainId
    }

    // Safe access to numeric fields with type conversion
    if blockJumpLimit := int64(getNetworkConfigUint8Field(regNetwork.NetworkConfig, "BlockJumpLimit")); blockJumpLimit != 0 {
        network.BlockJumpLimit = blockJumpLimit
    }

    // Safe access to boolean fields
    if archiveEnabled := getNetworkConfigBoolField(regNetwork.NetworkConfig, "ArchiveEnabled"); archiveEnabled != network.ArchiveEnabled {
        network.ArchiveEnabled = archiveEnabled
    }

    return network, nil
}
```

### Best Practices for Backward Compatibility

#### 1. Defensive Programming

```go
// ✅ Good: Check for nil pointers and field existence
func (h *YourHandler) processConfig(config *ExternalConfig) error {
    if config == nil {
        return fmt.Errorf("config cannot be nil")
    }

    // Use reflection for potentially missing fields
    if timeout := getConfigDurationField(config, "RequestTimeout"); timeout > 0 {
        h.requestTimeout = timeout
    } else {
        h.requestTimeout = 30 * time.Second // fallback default
    }

    return nil
}

// ❌ Bad: Direct access without version checks
func (h *YourHandler) processConfig(config *ExternalConfig) error {
    h.requestTimeout = config.RequestTimeout // May not exist!
    return nil
}
```

#### 2. Graceful Degradation

```go
// ✅ Good: Provide fallback behavior when features aren't available
func (h *YourHandler) Initialize(config *NetworkConfig) error {
    // Try to use advanced features if available
    if apiVersion := getNetworkConfigStringField(config, "APIVersion"); apiVersion != "" {
        h.apiVersion = apiVersion
        h.logger.Info("Using advanced API version", zap.String("version", apiVersion))
    } else {
        h.apiVersion = "v1" // fallback to basic version
        h.logger.Info("Using basic API version (advanced features unavailable)")
    }

    return nil
}
```

#### 3. Version Detection

```go
// Create a helper to detect struct version capabilities
func (h *YourHandler) detectConfigVersion(config *NetworkConfig) string {
    if config == nil {
        return "unknown"
    }

    v := reflect.ValueOf(config).Elem()
    
    // Check for presence of newer fields
    if v.FieldByName("ArchiveEnabled").IsValid() && 
       v.FieldByName("ChainId").IsValid() {
        return "v2"
    }
    
    if v.FieldByName("HealthcheckMethodBit").IsValid() {
        return "v1"
    }

    return "legacy"
}

func (h *YourHandler) Initialize(config *NetworkConfig) error {
    version := h.detectConfigVersion(config)
    h.logger.Info("Detected config version", zap.String("version", version))

    switch version {
    case "v2":
        return h.initializeV2(config)
    case "v1":
        return h.initializeV1(config)
    default:
        return h.initializeLegacy(config)
    }
}
```

### Testing Backward Compatibility

#### 1. Test Multiple Struct Versions

```go
func TestYourHandler_BackwardCompatibility(t *testing.T) {
    tests := []struct {
        name         string
        createConfig func() *NetworkConfig
        expectError  bool
    }{
        {
            name: "new config with all fields",
            createConfig: func() *NetworkConfig {
                return &NetworkConfig{
                    HealthcheckMethodBit: 1,
                    // Set newer fields using reflection to simulate newer version
                }
            },
            expectError: false,
        },
        {
            name: "legacy config missing newer fields",
            createConfig: func() *NetworkConfig {
                return &NetworkConfig{
                    HealthcheckMethodBit: 1,
                    // Deliberately omit newer fields to simulate older version
                }
            },
            expectError: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            handler := NewYourHandler(tt.createConfig())
            err := handler.Initialize(tt.createConfig())
            
            if tt.expectError {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

#### 2. Test Field Access Safety

```go
func TestSafeFieldAccess(t *testing.T) {
    tests := []struct {
        name      string
        config    *NetworkConfig
        fieldName string
        expected  string
    }{
        {
            name:      "existing field",
            config:    &NetworkConfig{/* with ChainId field */},
            fieldName: "ChainId",
            expected:  "test-chain",
        },
        {
            name:      "missing field",
            config:    &NetworkConfig{/* without newer fields */},
            fieldName: "NonExistentField",
            expected:  "",
        },
        {
            name:      "nil config",
            config:    nil,
            fieldName: "ChainId",
            expected:  "",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := getNetworkConfigStringField(tt.config, tt.fieldName)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

### Dependency Version Management

#### 1. Go Module Best Practices

```go
// go.mod - Use replace directives for local development
module github.com/your-org/your-project

go 1.21

require (
    github.com/external/dependency v1.2.3
)

// Use replace for local development
replace github.com/external/dependency => ./upstream/external/dependency
```

#### 2. CI/CD Considerations

For continuous integration, ensure your build process:

1. **Tests Compatibility**: Run tests against both local and remote dependencies
2. **Version Pinning**: Use specific commit hashes rather than version ranges
3. **Fallback Support**: Ensure code gracefully handles missing features

```yaml
# GitHub Actions example
- name: Test with remote dependencies
  run: |
    # Temporarily disable replace directive for CI builds
    sed -i 's/^replace /# replace /' go.mod
    go mod tidy
    go test ./...
    
- name: Test with local dependencies  
  run: |
    # Restore replace directive for local testing
    sed -i 's/^# replace /replace /' go.mod
    go mod tidy
    go test ./...
```

## Real-World Examples

### Example 1: Bitcoin Esplora Handler

```go
type BitcoinEsploraHandler struct {
    config         *NetworkConfig
    pathNormalizer *BitcoinPathNormalizer
    version        string
}

func (h *BitcoinEsploraHandler) GetType() string { return "bitcoin" }
func (h *BitcoinEsploraHandler) GetRequestType() RequestType { return RequestTypeREST }

func (h *BitcoinEsploraHandler) NormalizeEndpoint(path string) string {
    // /api/tx/abc123... -> /api/tx/{txid}
    // /api/address/1A1zP1... -> /api/address/{address}
    return h.pathNormalizer.NormalizePath(path)
}

func (h *BitcoinEsploraHandler) GetLatestBlock(provider Provider) (*BlockInfo, error) {
    // Make request to /api/blocks/tip/height
    resp, err := h.makeRESTRequest(provider, "/api/blocks/tip/height")
    if err != nil {
        return nil, err
    }
    
    var height int64
    if err := json.Unmarshal(resp, &height); err != nil {
        return nil, err
    }
    
    return &BlockInfo{
        Number:    height,
        Hash:      "", // Get hash with separate call if needed
        Timestamp: time.Now(),
    }, nil
}
```

### Example 2: Polygon JSON-RPC Handler

```go
type PolygonHandler struct {
    config  *NetworkConfig
    version string
}

func (h *PolygonHandler) GetType() string { return "polygon" }
func (h *PolygonHandler) GetRequestType() RequestType { return RequestTypeRPC }

func (h *PolygonHandler) ValidateRequest(req *http.Request) error {
    // Polygon-specific validation
    if req.Method != "POST" {
        return fmt.Errorf("polygon requires POST method")
    }
    
    // Check for Polygon-specific headers if needed
    return nil
}

func (h *PolygonHandler) IsRetryableError(err error, statusCode int) bool {
    // Polygon-specific retry logic
    if statusCode == 429 {
        return true // Polygon rate limiting
    }
    
    // Check for Polygon-specific error messages
    if err != nil && strings.Contains(err.Error(), "polygon node overloaded") {
        return true
    }
    
    return statusCode >= 500
}
```

## Best Practices

### 1. Handler Design Principles

- **Single Responsibility**: Each handler should focus on one network type
- **Stateless**: Handlers should not maintain state between requests
- **Thread-Safe**: Multiple goroutines may call handler methods concurrently
- **Error Handling**: Provide clear, actionable error messages
- **Backward Compatibility**: Use reflection-based field access for external dependencies
- **Graceful Degradation**: Provide fallback behavior when advanced features aren't available

### 2. Configuration Best Practices

```go
// Good: Validate configuration early
func (h *YourHandler) Initialize(config *NetworkConfig) error {
    if config.ChainID == "" {
        return fmt.Errorf("chain_id is required")
    }
    
    // Validate custom configuration
    if config.Custom != nil {
        if apiKey, ok := config.Custom["api_key"].(string); ok && apiKey == "" {
            return fmt.Errorf("api_key cannot be empty when specified")
        }
    }
    
    return nil
}

// Good: Provide sensible defaults
func NewYourHandler(config *NetworkConfig) *YourHandler {
    handler := &YourHandler{
        config:         config,
        version:        "1.0.0",
        requestTimeout: 30 * time.Second, // Default timeout
        maxRetries:     3,                 // Default retry count
    }
    
    return handler
}

// Good: Use reflection for accessing potentially missing fields
func (h *YourHandler) Initialize(config *NetworkConfig) error {
    // Safe access to fields that may not exist in all versions
    if chainId := getNetworkConfigStringField(config, "ChainId"); chainId != "" {
        h.chainId = chainId
    } else {
        h.chainId = "default-chain" // fallback value
    }
    
    return nil
}
```

### 3. Path Normalization Best Practices

```go
// Good: Use efficient regex compilation
type YourPathNormalizer struct {
    patterns []compiledPattern
}

type compiledPattern struct {
    regex       *regexp.Regexp
    replacement string
}

func NewYourPathNormalizer() *YourPathNormalizer {
    patterns := []compiledPattern{
        {
            regex:       regexp.MustCompile(`/users/\d+`),
            replacement: "/users/{id}",
        },
        {
            regex:       regexp.MustCompile(`/blocks/0x[a-fA-F0-9]+`),
            replacement: "/blocks/{hash}",
        },
    }
    
    return &YourPathNormalizer{patterns: patterns}
}

// Good: Use longest match for overlapping patterns
func (n *YourPathNormalizer) NormalizePath(path string) string {
    var bestMatch compiledPattern
    var bestMatchLength int
    
    for _, pattern := range n.patterns {
        if pattern.regex.MatchString(path) {
            matchLength := len(pattern.regex.FindString(path))
            if matchLength > bestMatchLength {
                bestMatch = pattern
                bestMatchLength = matchLength
            }
        }
    }
    
    if bestMatchLength > 0 {
        return bestMatch.regex.ReplaceAllString(path, bestMatch.replacement)
    }
    
    return path
}
```

### 4. Health Check Best Practices

```go
// Good: Implement timeout and error handling
func (h *YourHandler) GetLatestBlock(provider Provider) (*BlockInfo, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    resp, err := h.makeRequestWithContext(ctx, provider, h.getHealthCheckEndpoint())
    if err != nil {
        return nil, fmt.Errorf("health check request failed: %w", err)
    }
    
    blockInfo, err := h.parseBlockResponse(resp)
    if err != nil {
        return nil, fmt.Errorf("failed to parse block response: %w", err)
    }
    
    return blockInfo, nil
}

// Good: Implement circuit breaker pattern for failing providers
func (h *YourHandler) CheckHealth(provider Provider) (*HealthStatus, error) {
    if h.isProviderInCircuitBreaker(provider) {
        return &HealthStatus{
            Healthy: false,
            Error:   fmt.Errorf("provider in circuit breaker state"),
        }, nil
    }
    
    // ... perform health check ...
}
```

### 5. Testing Best Practices

```go
// Good: Use table-driven tests
func TestYourHandler_ValidateRequest(t *testing.T) {
    handler := NewYourHandler(&NetworkConfig{})
    
    tests := []struct {
        name        string
        request     *http.Request
        expectError bool
        errorMsg    string
    }{
        {
            name:        "valid request",
            request:     createValidRequest(),
            expectError: false,
        },
        {
            name:        "invalid method",
            request:     createInvalidMethodRequest(),
            expectError: true,
            errorMsg:    "invalid method",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := handler.ValidateRequest(tt.request)
            
            if tt.expectError {
                assert.Error(t, err)
                if tt.errorMsg != "" {
                    assert.Contains(t, err.Error(), tt.errorMsg)
                }
            } else {
                assert.NoError(t, err)
            }
        })
    }
}

// Good: Use mocks for external dependencies
func TestYourHandler_GetLatestBlock(t *testing.T) {
    mockProvider := &MockProvider{
        responses: map[string][]byte{
            "/health": []byte(`{"block_number": 12345}`),
        },
    }
    
    handler := NewYourHandler(&NetworkConfig{})
    
    blockInfo, err := handler.GetLatestBlock(mockProvider)
    assert.NoError(t, err)
    assert.Equal(t, int64(12345), blockInfo.Number)
}
```

## Troubleshooting

### Common Issues and Solutions

#### 1. Dependency Version Mismatch Errors

**Error**: `regNetwork.NetworkConfig.ChainId undefined (type *dinregistry.NetworkConfig has no field or method ChainId)`

**Root Cause**: Your local code uses newer struct definitions than what's available in the remote dependency version.

**Solution**: Implement backward compatibility using reflection-based field access:

```go
// ❌ Breaks with version mismatches
if regNetwork.NetworkConfig.ChainId != "" {
    network.ChainId = regNetwork.NetworkConfig.ChainId
}

// ✅ Safe with all versions
if chainId := getNetworkConfigStringField(regNetwork.NetworkConfig, "ChainId"); chainId != "" {
    network.ChainId = chainId
}
```

**Prevention**: Always use the helper functions for accessing external struct fields that may vary between versions.

#### 2. CI/CD Build Failures with Local Success

**Error**: Builds pass locally but fail in GitHub Actions with undefined field errors.

**Root Cause**: Local development uses `replace` directive in `go.mod`, but CI uses remote dependencies.

**Solution**: 
1. Implement backward compatibility patterns (see [Backward Compatibility](#backward-compatibility))
2. Test with both local and remote dependencies before pushing:

```bash
# Test with remote dependencies (like CI does)
sed -i 's/^replace /# replace /' go.mod
go mod tidy
go build ./...

# Restore local development setup
sed -i 's/^# replace /replace /' go.mod
go mod tidy
```

#### 3. Handler Not Found Error

**Error**: `no handler registered for network type 'yournetwork'`

**Solution**: Ensure your handler is registered in `RegisterBuiltinHandlers()`:

```go
func RegisterBuiltinHandlers() {
    // ... other handlers ...
    
    if err := DefaultRegistry.RegisterHandler("yournetwork", NewYourNetworkHandlerFactory()); err != nil {
        log.Printf("Failed to register YourNetwork handler: %v", err)
    }
}
```

#### 4. Configuration Validation Errors

**Error**: `failed to initialize handler: chain_id is required`

**Solution**: Ensure required configuration is provided in Caddyfile:

```caddyfile
yournetwork-mainnet {
    type yournetwork
    chain_id "yournetwork:mainnet"  # Required field
    # ... other config
}
```

#### 5. Path Translation Issues

**Error**: Requests not reaching the correct provider endpoints

**Solution**: Debug path translation logic:

```go
func (h *YourHandler) TranslatePath(gatewayPath string, provider Provider) (string, error) {
    log.Printf("Translating path: %s for provider: %s", gatewayPath, provider.GetHost())
    
    translatedPath := gatewayPath // Your translation logic here
    
    log.Printf("Translated to: %s", translatedPath)
    return translatedPath, nil
}
```

#### 6. Health Check Failures

**Error**: Providers showing as unhealthy when they should be healthy

**Solution**: Verify your health check endpoint and response parsing:

```go
func (h *YourHandler) GetLatestBlock(provider Provider) (*BlockInfo, error) {
    endpoint := h.getHealthCheckEndpoint()
    log.Printf("Making health check request to: %s", endpoint)
    
    resp, err := h.makeHealthCheckRequest(provider)
    if err != nil {
        log.Printf("Health check request failed: %v", err)
        return nil, err
    }
    
    log.Printf("Health check response: %s", string(resp))
    
    // Parse and return block info...
}
```

### Debug Tips

1. **Enable Debug Logging**: Add debug logs to your handler methods
2. **Use Test Providers**: Set up local test servers to verify path translation
3. **Check Handler Registration**: Verify your handler appears in `registry.ListHandlers()`
4. **Validate Configuration**: Test with minimal configuration first, then add complexity

### Performance Considerations

1. **Regex Compilation**: Compile regex patterns once in constructor, not per request
2. **Connection Pooling**: Reuse HTTP clients where possible
3. **Caching**: Cache health check results appropriately
4. **Timeouts**: Set reasonable timeouts for all external requests

## Conclusion

The handler registry pattern makes it easy to add support for any blockchain network or API type. By following this guide, you can:

1. **Implement** a complete network handler in ~200 lines of code
2. **Test** your handler thoroughly with unit and integration tests
3. **Configure** your network through standard Caddyfile syntax
4. **Extend** the system without modifying core middleware logic

The pattern is proven to work with diverse network types including EVM (Ethereum), Starknet, Solana, and Beacon Chain APIs, demonstrating its flexibility and power.

For additional help or questions, refer to the existing handler implementations in `lib/network/` for real-world examples. 