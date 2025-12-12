# Network Handlers

Network handlers provide blockchain-specific logic for request processing, health checks, and response parsing. This document covers the handler interface and all implementations.

## NetworkHandler Interface

**File**: `lib/network/handlers.go`

The `NetworkHandler` interface defines 92 methods across 9 categories that each blockchain handler must implement.

### Interface Categories

#### 1. Core Identification

```go
// GetType returns the handler type identifier
GetType() HandlerType

// GetName returns a human-readable name
GetName() string

// GetRequestType returns the request format (RPC, REST, GraphQL)
GetRequestType() RequestType
```

#### 2. Request Processing

```go
// ProcessRequest prepares the request for forwarding
ProcessRequest(r *http.Request) error

// ExtractMethod extracts the RPC method name from request
ExtractMethod(body []byte) (string, error)

// ConfigureRequestPath modifies the request path if needed
ConfigureRequestPath(r *http.Request, path string) error
```

#### 3. Response Handling

```go
// ParseResponse parses the upstream response
ParseResponse(body []byte) (*JSONRPCResponse, error)

// IsRetryableError determines if error should trigger retry
IsRetryableError(err error) bool
```

#### 4. Block Operations

```go
// GetLatestBlockNumber returns the current block number
GetLatestBlockNumber(ctx context.Context, httpClient IHTTPClient, url string, headers map[string]string) (uint64, error)

// GetBlockByNumber returns block data for a given number
GetBlockByNumber(ctx context.Context, httpClient IHTTPClient, url string, blockNumber uint64, headers map[string]string) (*Block, error)

// FormatBlockHeight formats block number for display
FormatBlockHeight(height uint64) string
```

#### 5. Health Checks

```go
// GetHealthCheckMethod returns the RPC method for health checks
GetHealthCheckMethod() string

// CreateHealthCheckPayload creates the health check request body
CreateHealthCheckPayload() ([]byte, error)

// ParseHealthCheckResponse extracts block number from response
ParseHealthCheckResponse(body []byte) (uint64, error)
```

#### 6. Chain ID Validation

```go
// GetChainID returns the expected chain ID
GetChainID() string

// ValidateChainID checks if response chain ID matches expected
ValidateChainID(body []byte, expectedChainID string) (bool, error)

// ParseChainIDResponse extracts chain ID from response
ParseChainIDResponse(body []byte) (string, error)
```

#### 7. Archive Mode

```go
// SupportsArchiveMode returns true if handler supports archive verification
SupportsArchiveMode() bool

// PerformArchiveCheck verifies archive node capability
PerformArchiveCheck(ctx context.Context, httpClient IHTTPClient, url string, headers map[string]string) (bool, error)
```

#### 8. Separate Block Info

```go
// RequiresSeparateBlockInfoCall returns true if block info needs separate call
RequiresSeparateBlockInfoCall() bool

// GetBlockInfoMethod returns the method for block info
GetBlockInfoMethod() string

// CreateBlockInfoPayload creates the block info request body
CreateBlockInfoPayload(blockNumber uint64) ([]byte, error)
```

#### 9. Lifecycle

```go
// Initialize sets up the handler
Initialize(ctx context.Context) error
```

---

## Request Types

```go
type RequestType int

const (
    RequestTypeRPC     RequestType = iota  // JSON-RPC (POST)
    RequestTypeREST                        // REST API (GET/POST)
    RequestTypeGraphQL                     // GraphQL
)
```

---

## Handler Implementations

### 1. EVM Handler

**File**: `lib/network/evm_handler.go`

Handles all EVM-compatible networks (Ethereum, Optimism, Arbitrum, Polygon, Base, Linea, etc.)

| Property | Value |
|----------|-------|
| Type | `evm` |
| Request Type | JSON-RPC |
| Health Check Method | `eth_blockNumber` |
| Chain ID Method | `eth_chainId` |
| Archive Check | `eth_getBalance` at genesis block |

**Key Methods**:
- `eth_blockNumber` - Get latest block
- `eth_chainId` - Validate chain ID
- `eth_getBalance` - Archive verification

**Block Number Parsing**:
```go
// EVM returns hex-encoded block numbers
// "0x10a3b5c" -> 17480540
```

---

### 2. Beacon Chain Handler

**File**: `lib/network/beacon_handler.go`

Handles Ethereum Beacon Chain (Consensus Layer) REST API.

| Property | Value |
|----------|-------|
| Type | `beacon-chain` |
| Request Type | REST |
| Health Check Endpoint | `/eth/v1/beacon/headers/head` |
| Chain ID | Not applicable (uses genesis validators root) |

**Key Endpoints**:
- `/eth/v1/beacon/headers/head` - Latest slot
- `/eth/v1/beacon/genesis` - Genesis info
- `/eth/v1/node/health` - Node health

**Slot vs Block**:
Beacon chain uses slots instead of block numbers. Handler converts slots to comparable values.

---

### 3. Bitcoin Handler

**File**: `lib/network/bitcoin_handler.go`

Handles Bitcoin Core JSON-RPC API.

| Property | Value |
|----------|-------|
| Type | `bitcoin` |
| Request Type | JSON-RPC |
| Health Check Method | `getblockcount` |
| Chain ID | Uses `getblockhash(0)` (genesis hash) |

**Key Methods**:
- `getblockcount` - Get latest block height
- `getblockhash` - Get block hash by height
- `getblock` - Get block data

**Authentication**:
Bitcoin nodes often require HTTP Basic Auth, configured via headers.

---

### 4. Bitcoin Esplora Handler

**File**: `lib/network/bitcoin_esplora_handler.go`

Handles Bitcoin Esplora REST API (block explorer).

| Property | Value |
|----------|-------|
| Type | `bitcoin-esplora` |
| Request Type | REST |
| Health Check Endpoint | `/api/blocks/tip/height` |
| Chain ID | Uses `/api/block-height/0` |

**Key Endpoints**:
- `/api/blocks/tip/height` - Latest block height
- `/api/block/:hash` - Block by hash
- `/api/tx/:txid` - Transaction by ID

---

### 5. Solana Handler

**File**: `lib/network/solana_handler.go`

Handles Solana JSON-RPC API.

| Property | Value |
|----------|-------|
| Type | `solana` |
| Request Type | JSON-RPC |
| Health Check Method | `getSlot` |
| Chain ID | Uses `getGenesisHash` |

**Key Methods**:
- `getSlot` - Get latest slot
- `getGenesisHash` - Validate network
- `getHealth` - Node health check

**Slot Numbers**:
Solana uses slots, which increment faster than traditional block numbers.

---

### 6. StarkNet Handler

**File**: `lib/network/starknet_handler.go`

Handles StarkNet JSON-RPC API.

| Property | Value |
|----------|-------|
| Type | `starknet` |
| Request Type | JSON-RPC |
| Health Check Method | `starknet_blockNumber` |
| Chain ID Method | `starknet_chainId` |

**Key Methods**:
- `starknet_blockNumber` - Get latest block
- `starknet_chainId` - Validate chain
- `starknet_getBlockWithTxHashes` - Block data

---

### 7. Tron Handler

**File**: `lib/network/tron_handler.go`

Handles Tron Full Node JSON-RPC API.

| Property | Value |
|----------|-------|
| Type | `tron-full-node` |
| Request Type | JSON-RPC |
| Health Check Method | `eth_blockNumber` |
| Chain ID Method | `eth_chainId` |

Tron Full Node exposes an EVM-compatible JSON-RPC interface.

---

## Handler Registry

**File**: `lib/network/registry.go`

Handlers are registered and retrieved through a registry:

```go
// Register a handler type
func RegisterHandler(handlerType HandlerType, factory HandlerFactory)

// Get handler for type
func GetHandler(handlerType HandlerType) (NetworkHandler, error)
```

### Handler Factory

```go
type HandlerFactory func() NetworkHandler
```

Each handler type has a factory function that creates new instances.

---

## Request Path Configuration

**File**: `lib/network/request_path_config.go`

Configures how request paths are handled for different network types:

```go
type RequestPathConfig struct {
    // Whether to append original path to upstream
    AppendPath bool

    // Base path for the upstream
    BasePath string

    // Path transformations
    Transforms []PathTransform
}
```

### Path Handling by Type

| Handler | Path Behavior |
|---------|---------------|
| EVM | Path ignored (JSON-RPC uses POST body) |
| Beacon | Path forwarded (REST endpoints) |
| Bitcoin | Path ignored (JSON-RPC) |
| Esplora | Path forwarded (REST endpoints) |
| Solana | Path ignored (JSON-RPC) |
| StarkNet | Path ignored (JSON-RPC) |
| Tron | Path ignored (JSON-RPC) |

---

## JSON-RPC Utilities

### JSON-RPC Client

**File**: `lib/network/json_rpc_client.go`

```go
// Make a JSON-RPC call
func (c *JSONRPCClient) Call(ctx context.Context, method string, params interface{}) (*JSONRPCResponse, error)
```

### JSON-RPC Types

**File**: `lib/network/json_rpc_types.go`

```go
type JSONRPCRequest struct {
    JSONRPC string      `json:"jsonrpc"`
    Method  string      `json:"method"`
    Params  interface{} `json:"params"`
    ID      interface{} `json:"id"`
}

type JSONRPCResponse struct {
    JSONRPC string          `json:"jsonrpc"`
    Result  json.RawMessage `json:"result,omitempty"`
    Error   *JSONRPCError   `json:"error,omitempty"`
    ID      interface{}     `json:"id"`
}

type JSONRPCError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Data    string `json:"data,omitempty"`
}
```

### JSON-RPC Parser

**File**: `lib/network/json_rpc_parser.go`

Utilities for parsing JSON-RPC responses:
- Extract result from response
- Parse error codes
- Handle batch requests

---

## Handler Comparison

| Handler | Request Type | Block Field | Chain ID | Archive |
|---------|--------------|-------------|----------|---------|
| EVM | JSON-RPC | `eth_blockNumber` | `eth_chainId` | Yes |
| Beacon | REST | `/headers/head` | Genesis root | No |
| Bitcoin | JSON-RPC | `getblockcount` | Genesis hash | No |
| Esplora | REST | `/blocks/tip/height` | Block 0 hash | No |
| Solana | JSON-RPC | `getSlot` | `getGenesisHash` | No |
| StarkNet | JSON-RPC | `starknet_blockNumber` | `starknet_chainId` | No |
| Tron | JSON-RPC | `eth_blockNumber` | `eth_chainId` | No |

---

## Adding New Handlers

See [Extending](./16-extending.md) for instructions on adding new network handlers.

## Related Documentation

- [Network Configuration](./03-network-configuration.md) - Network setup
- [Health Checks](./06-health-checks.md) - Health monitoring
- [Interfaces & Types](./12-interfaces-types.md) - Type definitions
- [Extending](./16-extending.md) - Adding new handlers
