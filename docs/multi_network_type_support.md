# Multi-Network Type Support in DIN Gateway

## Overview

The DIN Gateway provides a unified, extensible system for supporting multiple blockchain networks regardless of their underlying API protocol. Through a modular handler architecture, the gateway can seamlessly route requests between different types of blockchain APIs while maintaining consistent behavior, monitoring, and reliability.

This document explains how the multi-network type support system works from top to bottom, covering the architecture, implementation details, and operational aspects.

## Table of Contents

1. [System Architecture](#system-architecture)
2. [Supported Network Types](#supported-network-types)
3. [Request Flow](#request-flow)
4. [Handler System](#handler-system)
5. [Configuration](#configuration)
6. [Health Monitoring](#health-monitoring)
7. [Metrics and Observability](#metrics-and-observability)
8. [Adding New Network Types](#adding-new-network-types)
9. [Migration from Legacy System](#migration-from-legacy-system)

## System Architecture

The multi-network type support is built on a **handler registry pattern** that decouples network-specific logic from the core middleware:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                             DIN Gateway Architecture                         │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌─────────────┐    ┌──────────────┐    ┌─────────────────┐               │
│  │   Client    │───▶│   Caddy      │───▶│ DIN Middleware  │               │
│  │  (dApp/SDK) │    │  Web Server  │    │                 │               │
│  └─────────────┘    └──────────────┘    └────────┬────────┘               │
│                                                   │                         │
│                                          ┌────────▼────────┐                │
│                                          │ Handler Registry│                │
│                                          └────────┬────────┘                │
│                                                   │                         │
│        ┌──────────────┬──────────────┬───────────┴────────┬───────────┐   │
│        │              │              │                     │           │   │
│   ┌────▼─────┐  ┌────▼─────┐  ┌────▼─────┐  ┌───────────▼────┐  ┌───▼──┐│
│   │   EVM    │  │  Solana  │  │ Starknet │  │  Beacon Chain  │  │ ...  ││
│   │ Handler  │  │ Handler  │  │ Handler  │  │    Handler     │  │      ││
│   └────┬─────┘  └────┬─────┘  └────┬─────┘  └───────┬────────┘  └───┬──┘│
│        │              │              │                │               │    │
│   ┌────▼─────────────▼──────────────▼────────────────▼───────────────▼──┐ │
│   │                         Provider Pool                                │ │
│   │  (Infura, Alchemy, QuickNode, Blast, PublicNode, Custom, etc.)     │ │
│   └──────────────────────────────────────────────────────────────────────┘ │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

### Key Components

1. **DIN Middleware**: Core routing and orchestration layer
2. **Handler Registry**: Manages network type handlers and their lifecycle
3. **Network Handlers**: Protocol-specific implementations
4. **Provider Pool**: Backend RPC/API endpoints for each network

## Supported Network Types

### Currently Implemented

| Network Type | Handler | API Protocol | Example Networks |
|-------------|---------|--------------|------------------|
| **EVM** | `EVMHandler` | JSON-RPC 2.0 | Ethereum, Polygon, BSC, Arbitrum |
| **Solana** | `SolanaHandler` | JSON-RPC 2.0 | Solana Mainnet, Devnet |
| **Starknet** | `StarknetHandler` | JSON-RPC 2.0 | Starknet Mainnet, Testnet |
| **Beacon Chain** | `BeaconHandler` | REST API | Ethereum Consensus Layer |
| **Bitcoin** | `BitcoinHandler` | JSON-RPC/REST | Bitcoin, Bitcoin Testnet |

### Request Type Classification

The system supports three fundamental request types:

```go
type RequestType string

const (
    RequestTypeRPC     RequestType = "rpc"      // JSON-RPC 2.0
    RequestTypeREST    RequestType = "rest"     // RESTful HTTP
    RequestTypeGraphQL RequestType = "graphql"  // GraphQL (future)
)
```

## Request Flow

### 1. Client Request

A client sends a request to the gateway:

```bash
# JSON-RPC Example (Ethereum)
curl -X POST http://gateway.example.com/ethereum \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'

# REST API Example (Beacon Chain)  
curl http://gateway.example.com/beacon/eth/v1/beacon/headers/head
```

### 2. Middleware Processing

The DIN middleware intercepts the request and:

1. **Extracts Network**: Parses the URL path to identify the target network
2. **Loads Configuration**: Retrieves network configuration from memory
3. **Gets Handler**: Requests appropriate handler from the registry
4. **Validates Request**: Uses handler to validate protocol-specific requirements

### 3. Handler Processing

The network-specific handler:

1. **Validates Request**: Checks method, headers, and payload format
2. **Translates Path**: Converts gateway paths to provider-specific paths
3. **Processes Request**: Applies any network-specific transformations
4. **Handles Response**: Parses and validates the provider response

### 4. Provider Selection

The system selects a healthy provider based on:

- Provider health status
- Priority configuration
- Load balancing strategy
- Recent performance metrics

## Handler System

### Handler Interface

All network handlers implement a common interface:

```go
type NetworkHandler interface {
    // Metadata
    GetType() string              // Handler type identifier
    GetName() string              // Human-readable name
    GetVersion() string           // Handler version
    GetRequestType() RequestType  // RPC, REST, or GraphQL
    
    // Lifecycle
    Initialize(config *NetworkConfig) error
    Shutdown() error
    
    // Request Processing
    ProcessRequest(req *http.Request, provider Provider) error
    ValidateRequest(req *http.Request) error
    TranslatePath(gatewayPath string, provider Provider) (string, error)
    NormalizeEndpoint(path string) string
    
    // Health Monitoring
    GetLatestBlock(provider Provider) (*BlockInfo, error)
    CheckHealth(provider Provider) (*HealthStatus, error)
    GetHealthCheckMethod() string
    GetBlockByNumberMethod() string
    
    // Error Handling
    ParseResponse(body []byte, statusCode int) error
    IsRetryableError(err error, statusCode int) bool
}
```

### Handler Registration

Handlers are registered at startup:

```go
func RegisterBuiltinHandlers() {
    DefaultRegistry.RegisterHandler("evm", NewEVMHandlerFactory())
    DefaultRegistry.RegisterHandler("solana", NewSolanaHandlerFactory())
    DefaultRegistry.RegisterHandler("starknet", NewStarknetHandlerFactory())
    DefaultRegistry.RegisterHandler("beacon_chain", NewBeaconHandlerFactory())
    // Additional handlers...
}
```

### Protocol-Specific Behavior

Each handler implements protocol-specific logic:

#### JSON-RPC Handlers (EVM, Solana, Starknet)
- Validate JSON-RPC 2.0 structure
- Extract method from request body
- Handle batch requests
- Parse JSON-RPC errors

#### REST Handlers (Beacon Chain, Bitcoin)
- Support multiple HTTP methods
- Translate URL paths
- Handle query parameters
- Parse REST-style responses

## Configuration

### Caddyfile Configuration

Networks are configured in the Caddyfile with explicit type specification:

```caddyfile
din {
    networks {
        # EVM Network
        ethereum {
            type evm
            chain_id eip155:0x1
            providers {
                https://mainnet.infura.io/v3/YOUR-KEY {
                    priority 0
                }
            }
        }
        
        # Solana Network
        solana-mainnet {
            type solana
            chain_id solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdpKuc147dw2N9d
            providers {
                https://api.mainnet-beta.solana.com {
                    priority 0
                }
            }
        }
        
        # Beacon Chain (REST API)
        ethereum-beacon {
            type beacon_chain
            chain_id beacon:mainnet
            healthcheck_endpoint "/eth/v1/beacon/headers/head"
            providers {
                https://beacon-api.example.com {
                    priority 0
                }
            }
        }
    }
}
```

### Network Type Auto-Detection

If `type` is not specified, the system attempts auto-detection based on network name:

| Name Pattern | Auto-Detected Type |
|-------------|-------------------|
| Contains "solana" | `solana` |
| Contains "starknet" | `starknet` |
| Contains "bitcoin" or "btc" | `bitcoin` |
| Contains "beacon" or "consensus" | `beacon_chain` |
| All others | `evm` (default) |

**Best Practice**: Always specify `type` explicitly for clarity.

## Health Monitoring

### Unified Health Check System

Each handler implements network-specific health checks:

```go
// EVM Health Check
func (h *EVMHandler) GetLatestBlock(provider Provider) (*BlockInfo, error) {
    // Calls eth_blockNumber
}

// Solana Health Check  
func (h *SolanaHandler) GetLatestBlock(provider Provider) (*BlockInfo, error) {
    // Calls getBlockHeight
}

// Beacon Chain Health Check
func (h *BeaconHandler) GetLatestBlock(provider Provider) (*BlockInfo, error) {
    // GET /eth/v1/beacon/headers/head
}
```

### Health Status States

Providers can be in one of three states:

1. **Healthy**: Responding normally with recent blocks
2. **Warning**: Experiencing issues but still usable
3. **Unhealthy**: Not responding or severely lagged

### Grace Period and Recovery

The system provides grace periods for recovering providers:
- Unhealthy providers get multiple chances before being marked down
- Warning status allows gradual recovery
- Automatic re-integration when health improves

## Metrics and Observability

### Unified Metrics Across Protocols

The gateway exposes consistent metrics regardless of network type:

```prometheus
# Request metrics by network and method
din_requests_total{network="ethereum", method="eth_blockNumber", provider="infura", status="200"}
din_requests_total{network="solana", method="getBlockHeight", provider="quicknode", status="200"}
din_requests_total{network="beacon", endpoint="/eth/v1/beacon/headers/head", provider="publicnode", status="200"}

# Provider health metrics
din_provider_health{network="ethereum", provider="infura", health="healthy"}
din_provider_block_number{network="solana", provider="quicknode"} 150234567

# Latency metrics
din_request_duration_seconds{network="ethereum", method="eth_call", provider="alchemy"}
```

### Logging

Structured logging provides visibility into request flow:

```json
{
  "level": "info",
  "ts": 1642531200,
  "msg": "Request processed",
  "network": "ethereum",
  "handler": "evm",
  "method": "eth_blockNumber",
  "provider": "infura",
  "status": 200,
  "duration_ms": 45
}
```

## Adding New Network Types

To add support for a new blockchain network:

1. **Create Handler**: Implement the `NetworkHandler` interface
2. **Register Handler**: Add to the handler registry
3. **Add Tests**: Create comprehensive unit tests
4. **Update Configuration**: Document Caddyfile options
5. **Test Integration**: Verify end-to-end functionality

**Detailed Guide**: See [Adding New Networks Guide](./adding_new_networks.md) for step-by-step instructions.

## Migration from Legacy System

The multi-network type support maintains backward compatibility while providing enhanced functionality:

### What Changed?

**Before**: Manual method configuration in Caddyfile
```caddyfile
solana-mainnet {
    healthcheck_method getBlockHeight
    chainid_method getGenesisHash
    # ... manual method mappings
}
```

**After**: Handler-based automatic configuration
```caddyfile
solana-mainnet {
    type solana  # Handler provides all methods
}
```

### Benefits

1. **Automatic Method Configuration**: Handlers know their network's methods
2. **Protocol-Aware Processing**: Proper handling of different API styles
3. **Better Error Handling**: Protocol-specific retry logic
4. **Unified Monitoring**: Consistent metrics across all network types

**Migration Guide**: See [Caddyfile Migration Guide](./caddyfile_migration_guide.md) for detailed migration instructions.

## Advanced Features

### Request Context Propagation

The system maintains request context throughout the pipeline:

```go
type RequestContext struct {
    Network      string
    Provider     string
    Method       string
    RequestID    string
    StartTime    time.Time
    // ... additional context
}
```

### Dynamic Handler Loading (Future)

The architecture supports dynamic handler loading:
- Plugin-based handlers
- Runtime handler updates
- Custom handler implementations

### Protocol Extensions

Easy addition of new protocols:
- WebSocket support
- GraphQL implementation
- Custom binary protocols

## Troubleshooting

### Common Issues

1. **Handler Not Found**
   - Ensure handler is registered
   - Check network type spelling
   - Verify handler initialization

2. **Method Not Supported**
   - Confirm correct handler type
   - Check method name spelling
   - Verify network configuration

3. **Health Check Failures**
   - Review handler-specific health check methods
   - Check provider endpoints
   - Monitor network-specific logs

### Debug Mode

Enable debug logging for detailed handler operations:

```caddyfile
{
    debug
}
```

## Summary

The multi-network type support system provides:

1. **Universal Gateway**: Single entry point for all blockchain networks
2. **Protocol Flexibility**: Support for JSON-RPC, REST, GraphQL, and more
3. **Consistent Operations**: Unified monitoring, health checks, and metrics
4. **Easy Extensibility**: Simple process to add new networks
5. **Production Ready**: Battle-tested with major blockchain networks

This architecture enables the DIN Gateway to adapt to the evolving blockchain ecosystem while maintaining operational simplicity and reliability. 