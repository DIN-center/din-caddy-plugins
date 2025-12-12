# Network Configuration

This document covers the configuration structures for networks and providers in DIN Caddy Plugins.

## Network Structure

**File**: `modules/network.go` (869 lines)

A network represents a blockchain network configuration with its providers, health check settings, and runtime state.

### Core Fields

```go
type network struct {
    // Identity
    Name        string      `json:"name,omitempty"`
    HandlerType HandlerType `json:"handler_type,omitempty"`
    ChainId     string      `json:"chain_id,omitempty"`

    // Providers
    Providers map[string]*provider `json:"providers,omitempty"`

    // Supported methods
    Methods []string `json:"methods,omitempty"`

    // Health check configuration
    HCInterval  int `json:"hc_interval,omitempty"`   // Seconds between checks
    HCThreshold int `json:"hc_threshold,omitempty"` // Failures before unhealthy
    HCTimeout   int `json:"hc_timeout,omitempty"`   // Timeout per check (ms)

    // Block deviation limits
    BlockLagLimit  int64 `json:"block_lag_limit,omitempty"`  // Max blocks behind
    BlockJumpLimit int64 `json:"block_jump_limit,omitempty"` // Max block jump

    // Archive mode
    ArchiveEnabled bool `json:"archive_enabled,omitempty"`

    // Internal state (not serialized)
    handler          networkLib.NetworkHandler
    blockHistory     *ring.Ring
    healthCheckStop  chan struct{}
    watcherScoreMgr  watcherscore.IWatcherScoreManager
    CaddyfileFlags   CaddyfileFlags
}
```

### Configuration Options

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | required | Network identifier (e.g., "ethereum-mainnet") |
| `handler_type` | HandlerType | required | Handler type (evm, bitcoin, beacon-chain, etc.) |
| `chain_id` | string | optional | CAIP-2 chain ID for validation |
| `providers` | map | required | Provider configurations |
| `methods` | []string | optional | Allowed RPC methods |
| `hc_interval` | int | 5 | Health check interval in seconds |
| `hc_threshold` | int | 2 | Consecutive failures before unhealthy |
| `hc_timeout` | int | 5000 | Health check timeout in milliseconds |
| `block_lag_limit` | int64 | 5 | Maximum blocks behind consensus |
| `block_jump_limit` | int64 | 100 | Maximum block number jump |
| `archive_enabled` | bool | false | Enable archive node verification |

### Handler Types

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

### Chain ID Formats (CAIP-2)

| Network Type | Format | Example |
|--------------|--------|---------|
| EVM | `eip155:{chainId}` | `eip155:1` (Ethereum) |
| Bitcoin | `bip122:{genesisHash}` | `bip122:000000000019d6...` |
| Solana | `solana:{network}` | `solana:mainnet` |
| StarkNet | `starknet:{network}` | `starknet:mainnet` |

### Block History

Each network maintains a ring buffer of recent block numbers:

```go
const networkBlockHistorySize = 128

// blockHistory is a ring buffer storing recent block numbers
// Used for detecting stalls and jumps
```

### Key Methods

```go
// Create a new network instance
func NewNetwork(name string) *network

// Set the network handler
func (n *network) SetHandler(h networkLib.NetworkHandler)

// Get latest block from history
func (n *network) GetLatestBlock() uint64

// Add block to history
func (n *network) AddBlockToHistory(block uint64)

// Check if network is stalled
func (n *network) IsStalled() bool
```

---

## Provider Structure

**File**: `modules/provider.go`

A provider represents a single blockchain endpoint with its configuration, authentication, and health state.

### Core Fields

```go
type provider struct {
    // Endpoint configuration
    HttpUrl  string            `json:"http_url,omitempty"`
    Priority int               `json:"priority,omitempty"`
    Headers  map[string]string `json:"headers,omitempty"`

    // Authentication (mutually exclusive)
    Auth       *siwe.SIWEClient `json:"auth,omitempty"`
    OIDCClient *oidc.Client     `json:"oidc,omitempty"`

    // Method support
    Methods []string `json:"methods,omitempty"`

    // Watcher score
    Score float64 `json:"score,omitempty"`

    // Internal state
    upstream     *reverseproxy.Upstream
    healthStatus HealthStatus
    blockHistory *ring.Ring
    failureCount int
}
```

### Configuration Options

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `http_url` | string | required | Provider endpoint URL |
| `priority` | int | 0 | Priority tier (lower = higher priority) |
| `headers` | map | optional | Custom headers to include |
| `auth` | SIWEClient | optional | SIWE authentication config |
| `oidc` | Client | optional | OIDC authentication config |
| `methods` | []string | optional | Methods this provider supports |
| `score` | float64 | 1.0 | Watcher score for selection |

### Priority System

Providers are grouped by priority tier:
- **Priority 0**: Primary providers (highest priority)
- **Priority 1**: Secondary providers
- **Priority 2+**: Fallback providers

Lower priority numbers are preferred. Within the same tier, selection uses score-based weighting.

### Health Status

```go
type HealthStatus int

const (
    Healthy   HealthStatus = iota  // Fully operational
    Warning                        // Issues but serving traffic
    Unhealthy                      // Should not receive traffic
)
```

### Availability Methods

```go
// Available returns true if provider should receive traffic
func (p *provider) Available() bool {
    return p.healthStatus == Healthy || p.healthStatus == Warning
}

// IsAvailableWithWarning returns true if provider is in warning state
func (p *provider) IsAvailableWithWarning() bool {
    return p.healthStatus == Warning
}
```

### Authentication

Providers can use either SIWE or OIDC authentication (mutually exclusive):

```go
// Get the appropriate auth client
func (p *provider) AuthClient() auth.IAuthClient {
    if p.Auth != nil {
        return p.Auth
    }
    if p.OIDCClient != nil {
        return p.OIDCClient
    }
    return nil
}
```

### Block History

Each provider maintains its own block history:

```go
const providerBlockHistorySize = 10

// blockHistory tracks recent block numbers for this provider
// Used for lag/jump detection relative to network consensus
```

---

## Caddyfile Flags

Tracks which fields were configured via Caddyfile to prevent registry sync from overriding them.

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

This allows manual Caddyfile configuration to take precedence over DIN Registry settings.

---

## Configuration Examples

### Minimal Network Configuration

```json
{
    "name": "ethereum-mainnet",
    "handler_type": "evm",
    "providers": {
        "infura": {
            "http_url": "https://mainnet.infura.io/v3/YOUR_KEY"
        }
    }
}
```

### Full Network Configuration

```json
{
    "name": "ethereum-mainnet",
    "handler_type": "evm",
    "chain_id": "eip155:1",
    "hc_interval": 5,
    "hc_threshold": 3,
    "hc_timeout": 5000,
    "block_lag_limit": 5,
    "block_jump_limit": 100,
    "archive_enabled": true,
    "methods": [
        "eth_blockNumber",
        "eth_getBalance",
        "eth_call"
    ],
    "providers": {
        "primary": {
            "http_url": "https://primary.example.com",
            "priority": 0,
            "headers": {
                "X-API-Key": "secret"
            }
        },
        "secondary": {
            "http_url": "https://secondary.example.com",
            "priority": 1
        },
        "fallback": {
            "http_url": "https://fallback.example.com",
            "priority": 2,
            "methods": ["eth_blockNumber"]
        }
    }
}
```

### Provider with SIWE Authentication

```json
{
    "http_url": "https://protected.example.com",
    "priority": 0,
    "auth": {
        "private_key": "0x...",
        "auth_endpoint": "/auth"
    }
}
```

---

## Caddyfile Configuration

### Network Block

```caddyfile
din {
    services {
        ethereum-mainnet {
            handler_type evm
            chain_id eip155:1
            hc_interval 5
            hc_threshold 3
            hc_timeout 5000
            block_lag_limit 5
            block_jump_limit 100
            archive_enabled true

            methods {
                eth_blockNumber
                eth_getBalance
                eth_call
            }

            providers {
                https://primary.example.com {
                    priority 0
                    headers {
                        X-API-Key secret
                    }
                }
                https://secondary.example.com {
                    priority 1
                }
            }
        }
    }
}
```

---

## Default Values

| Parameter | Default | Notes |
|-----------|---------|-------|
| `hc_interval` | 5 | 5 seconds between checks |
| `hc_threshold` | 2 | 2 failures = unhealthy |
| `hc_timeout` | 5000 | 5 second timeout |
| `block_lag_limit` | 5 | 5 blocks behind = warning |
| `block_jump_limit` | 100 | 100 block jump = reset |
| `archive_enabled` | false | No archive verification |
| `priority` | 0 | Highest priority |
| `score` | 1.0 | Full weight |

---

## Related Documentation

- [Network Handlers](./04-network-handlers.md) - Handler implementations
- [Health Checks](./06-health-checks.md) - Health monitoring details
- [Caddyfile Configuration](./11-caddyfile-configuration.md) - Full configuration reference
- [Authentication](./05-authentication.md) - SIWE and OIDC setup
