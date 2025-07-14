# Generic API Support in DIN Gateway

## Overview

The DIN Gateway now provides unified support for multiple API protocols through a generic, extensible handler system. This allows the gateway to route and process requests for different blockchain networks regardless of their underlying API protocol (JSON-RPC, REST, GraphQL).

## Supported API Types

### 1. JSON-RPC APIs
**Protocol**: JSON-RPC 2.0 over HTTP  
**Request Type**: `RequestTypeRPC`  
**Method**: POST with JSON payload  
**Examples**: Ethereum, Starknet, Solana, Polygon

```json
{
  "jsonrpc": "2.0",
  "method": "eth_blockNumber",
  "params": [],
  "id": 1
}
```

**Supported Networks**:
- **Ethereum** (`evm` type): Full EVM-compatible JSON-RPC support
- **Starknet** (`starknet` type): Starknet-specific JSON-RPC methods
- **Solana** (`solana` type): Solana JSON-RPC API support

### 2. REST APIs
**Protocol**: RESTful HTTP  
**Request Type**: `RequestTypeREST`  
**Methods**: GET, POST, PUT, DELETE  
**Examples**: Beacon Chain, Bitcoin Esplora

```http
GET /eth/v1/beacon/headers/head
GET /api/tx/abc123456789
POST /api/mempool
```

**Supported Networks**:
- **Beacon Chain** (`beacon_chain` type): Ethereum 2.0 REST API
- **Bitcoin** (`bitcoin` type): Esplora-style REST API (planned)

### 3. GraphQL APIs (Future Support)
**Protocol**: GraphQL over HTTP  
**Request Type**: `RequestTypeGraphQL`  
**Method**: POST with GraphQL query  
**Examples**: The Graph Protocol, Subgraph APIs

## Architecture

### Unified Request Processing

The gateway uses a **handler registry pattern** that abstracts away protocol differences:

```
Client Request → DIN Middleware → Handler Registry → Protocol Handler → Provider
```

### Handler Interface

All protocol handlers implement the same `NetworkHandler` interface:

```go
type NetworkHandler interface {
    // Protocol identification
    GetRequestType() RequestType  // RPC, REST, or GraphQL
    
    // Request processing
    ProcessRequest(req *http.Request, provider Provider) error
    ValidateRequest(req *http.Request) error
    TranslatePath(gatewayPath string, provider Provider) (string, error)
    
    // Health monitoring
    GetLatestBlock(provider Provider) (*BlockInfo, error)
    CheckHealth(provider Provider) (*HealthStatus, error)
}
```

### Request Type Detection

The middleware automatically detects the request type based on:

1. **Network Configuration**: Each network specifies its handler type
2. **Handler Registry**: Maps network types to specific handlers
3. **Protocol Validation**: Handlers validate protocol-specific requirements

## Configuration Examples

### JSON-RPC Network (Ethereum)

```caddyfile
eth-mainnet {
    type evm                    # Uses EVM handler (JSON-RPC)
    chain_id eip155:0x1
    healthcheck_method eth_blockNumber
    
    providers {
        https://mainnet.infura.io/v3/your-key {
            priority 0
        }
        https://eth-mainnet.public.blastapi.io {
            priority 1
        }
    }
}
```

### REST Network (Beacon Chain)

```caddyfile
ethereum-beacon {
    type beacon_chain           # Uses Beacon Chain handler (REST)
    chain_id "mainnet"
    healthcheck_endpoint "/eth/v1/beacon/headers/head"
    
    providers {
        https://ethereum-beacon-api.publicnode.com {
            priority 0
        }
        https://beaconcha.in {
            path_prefix "/api/v1"
            priority 1
        }
    }
}
```

## Request Flow Examples

### JSON-RPC Request Flow

1. **Client Request**:
   ```bash
   curl -X POST http://localhost:8000/eth-mainnet \
     -H "Content-Type: application/json" \
     -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'
   ```

2. **Gateway Processing**:
   - Middleware extracts network name (`eth-mainnet`)
   - Handler registry returns EVM handler
   - EVM handler validates JSON-RPC structure
   - Request forwarded to configured provider

3. **Provider Response**:
   ```json
   {"jsonrpc":"2.0","id":1,"result":"0x1234567"}
   ```

### REST Request Flow

1. **Client Request**:
   ```bash
   curl http://localhost:8000/ethereum-beacon/eth/v1/beacon/headers/head
   ```

2. **Gateway Processing**:
   - Middleware extracts network name (`ethereum-beacon`)
   - Handler registry returns Beacon Chain handler
   - Handler translates path: `/eth/v1/beacon/headers/head`
   - Request forwarded to configured provider

3. **Provider Response**:
   ```json
   {
     "data": {
       "root": "0xabc123...",
       "canonical": true,
       "header": { ... }
     }
   }
   ```

## Protocol-Specific Features

### JSON-RPC Features

- **Method Validation**: Validates JSON-RPC 2.0 structure
- **Error Handling**: Parses JSON-RPC error responses
- **Batch Requests**: Support for JSON-RPC batch requests (future)
- **Method Routing**: Routes based on JSON-RPC method names

### REST Features

- **Path Translation**: Converts gateway paths to provider paths
- **HTTP Method Support**: GET, POST, PUT, DELETE operations
- **Path Normalization**: Normalizes dynamic paths for metrics
- **Query Parameter Handling**: Preserves query parameters

### Common Features

- **Provider Failover**: Automatic failover between providers
- **Health Monitoring**: Protocol-specific health checks
- **Load Balancing**: Round-robin and priority-based routing
- **Request Metrics**: Detailed metrics per protocol and network
- **Error Retry Logic**: Protocol-aware retry mechanisms

## Health Check Implementation

### JSON-RPC Health Checks

```go
func (h *EVMHandler) GetLatestBlock(provider Provider) (*BlockInfo, error) {
    // Make JSON-RPC call to eth_blockNumber
    request := JSONRPCRequest{
        JSONRPC: "2.0",
        Method:  "eth_blockNumber",
        Params:  []interface{}{},
        ID:      1,
    }
    
    response, err := h.makeJSONRPCRequest(provider, request)
    // Parse response and return block info
}
```

### REST Health Checks

```go
func (h *BeaconHandler) GetLatestBlock(provider Provider) (*BlockInfo, error) {
    // Make REST call to beacon headers endpoint
    resp, err := h.makeRESTRequest(provider, "/eth/v1/beacon/headers/head")
    // Parse JSON response and return block info
}
```

## Error Handling

### Protocol-Specific Error Parsing

The gateway handles errors differently based on the protocol:

#### JSON-RPC Errors
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32600,
    "message": "Invalid Request"
  }
}
```

#### REST Errors
```json
{
  "code": 404,
  "message": "Block not found",
  "data": null
}
```

### Retry Logic

The gateway implements protocol-aware retry logic:

- **JSON-RPC**: Retries on network errors and specific JSON-RPC error codes
- **REST**: Retries on 5xx status codes and network timeouts
- **Common**: Circuit breaker pattern for failing providers

## Monitoring and Metrics

### Protocol Metrics

The gateway exposes metrics for each protocol type:

```
din_requests_total{network="eth-mainnet", protocol="rpc", method="eth_blockNumber"}
din_requests_total{network="ethereum-beacon", protocol="rest", endpoint="/eth/v1/beacon/headers/head"}
```

### Health Metrics

```
din_provider_health{network="eth-mainnet", provider="infura", protocol="rpc"}
din_provider_latency_seconds{network="ethereum-beacon", provider="publicnode", protocol="rest"}
```

## Adding New Protocols

To add support for a new protocol (e.g., GraphQL):

1. **Define Request Type**:
   ```go
   const RequestTypeGraphQL RequestType = "graphql"
   ```

2. **Implement Handler**:
   ```go
   type GraphQLHandler struct {
       // Implementation
   }
   
   func (h *GraphQLHandler) GetRequestType() RequestType {
       return RequestTypeGraphQL
   }
   ```

3. **Register Handler**:
   ```go
   DefaultRegistry.RegisterHandler("graphql", NewGraphQLHandlerFactory())
   ```

## Benefits of Generic API Support

1. **Protocol Agnostic**: Add any network regardless of its API style
2. **Unified Configuration**: Same Caddyfile syntax for all protocols
3. **Consistent Monitoring**: Common metrics across all protocols
4. **Shared Infrastructure**: Provider management, failover, and health checks
5. **Future-Proof**: Easy to add new protocols and API styles

## Migration from Protocol-Specific Systems

The generic API support maintains backward compatibility while providing a path forward:

- **Existing Configurations**: Continue to work without changes
- **New Features**: Available to all protocol types
- **Gradual Migration**: Upgrade networks one at a time
- **Protocol Extensions**: Add new methods/endpoints without core changes

This unified approach enables the DIN Gateway to support any blockchain network or API type while maintaining consistent behavior, monitoring, and operational characteristics across all supported protocols. 