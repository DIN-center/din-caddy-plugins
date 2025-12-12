# Health Checks

The health check system continuously monitors provider endpoints to ensure requests are only routed to functioning nodes. This document covers the health check architecture, algorithms, and configuration.

## Overview

Health checks run as background goroutines for each network, periodically querying all providers and updating their health status based on response quality.

```
┌─────────────────────────────────────────────────────────────┐
│                    Health Check Goroutine                    │
│                     (per network)                            │
└─────────────────────────────────────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
        ▼                     ▼                     ▼
┌───────────────┐    ┌───────────────┐    ┌───────────────┐
│  Provider A   │    │  Provider B   │    │  Provider C   │
│ eth_blockNum  │    │ eth_blockNum  │    │ eth_blockNum  │
└───────────────┘    └───────────────┘    └───────────────┘
        │                     │                     │
        ▼                     ▼                     ▼
┌─────────────────────────────────────────────────────────────┐
│                  Block Number Comparison                     │
│           (determine consensus, detect lag/stalls)           │
└─────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────┐
│                   Update Health Status                       │
│              (Healthy / Warning / Unhealthy)                 │
└─────────────────────────────────────────────────────────────┘
```

## Health Status

```go
type HealthStatus int

const (
    Healthy   HealthStatus = iota  // Provider is fully operational
    Warning                        // Provider has issues but can serve traffic
    Unhealthy                      // Provider should not receive traffic
)
```

### Status Transitions

```
                    ┌─────────────────┐
         success   │                 │   lag detected
    ┌─────────────▶│    Healthy      │◀────────────────┐
    │              │                 │                  │
    │              └────────┬────────┘                  │
    │                       │                           │
    │            lag/stall  │                           │ recovery
    │              detected │                           │
    │                       ▼                           │
    │              ┌─────────────────┐                  │
    │              │                 │                  │
    │              │    Warning      │──────────────────┘
    │              │                 │
    │              └────────┬────────┘
    │                       │
    │      consecutive      │
    │        failures       │
    │                       ▼
    │              ┌─────────────────┐
    │              │                 │
    └──────────────│   Unhealthy    │
        success    │                 │
                   └─────────────────┘
```

## Health Check Configuration

### Network-Level Settings

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `hc_interval` | int | 5 | Seconds between health checks |
| `hc_threshold` | int | 2 | Consecutive failures before Unhealthy |
| `hc_timeout` | int | 5000 | Timeout per check in milliseconds |
| `block_lag_limit` | int64 | 5 | Max blocks behind consensus |
| `block_jump_limit` | int64 | 100 | Max acceptable block number jump |

### Caddyfile Example

```caddyfile
din {
    services {
        ethereum-mainnet {
            hc_interval 5
            hc_threshold 3
            hc_timeout 5000
            block_lag_limit 10
            block_jump_limit 50
        }
    }
}
```

## Health Check Cycle

### 1. Trigger Health Check

Health checks run at the configured interval:

```go
ticker := time.NewTicker(time.Duration(network.HCInterval) * time.Second)
for {
    select {
    case <-ticker.C:
        runHealthCheck(network)
    case <-stopCh:
        return
    }
}
```

### 2. Query All Providers

Each provider is queried concurrently:

```go
for name, provider := range network.Providers {
    go func(p *provider) {
        blockNum, err := handler.GetLatestBlockNumber(ctx, httpClient, p.HttpUrl, p.Headers)
        results <- healthCheckResult{provider: p, blockNum: blockNum, err: err}
    }(provider)
}
```

### 3. Determine Network Consensus

The highest block number from healthy providers becomes the consensus:

```go
func determineConsensus(results []healthCheckResult) uint64 {
    var maxBlock uint64
    for _, r := range results {
        if r.err == nil && r.blockNum > maxBlock {
            maxBlock = r.blockNum
        }
    }
    return maxBlock
}
```

### 4. Evaluate Each Provider

Each provider is evaluated against the consensus:

```go
func evaluateProvider(provider *provider, blockNum uint64, consensus uint64, config *network) HealthStatus {
    if blockNum == 0 {
        return Unhealthy
    }

    lag := consensus - blockNum
    if lag > config.BlockLagLimit {
        return Warning
    }

    return Healthy
}
```

### 5. Update Provider Status

```go
provider.healthStatus = newStatus
provider.failureCount = 0  // Reset on success
```

## Block Tracking

### Provider Block History

Each provider maintains a ring buffer of recent block numbers:

```go
const providerBlockHistorySize = 10

// Tracks last 10 block numbers from this provider
provider.blockHistory.Value = blockNumber
provider.blockHistory = provider.blockHistory.Next()
```

### Network Block History

Networks maintain a larger history for stall detection:

```go
const networkBlockHistorySize = 128

// Tracks last 128 blocks across all providers
network.blockHistory.Value = consensusBlock
network.blockHistory = network.blockHistory.Next()
```

## Detection Algorithms

### Lag Detection

Provider is behind the network consensus:

```go
lag := consensus - providerBlock
if lag > network.BlockLagLimit {
    return Warning  // Provider is lagging
}
```

**Default**: 5 blocks behind = Warning

### Stall Detection

Network hasn't produced new blocks:

```go
func (n *network) IsStalled() bool {
    // Check if block number unchanged for too long
    // Uses network block history
}
```

A stall affects all providers equally, so status isn't changed individually.

### Jump Detection

Large block number increase (possible chain reorg or provider issue):

```go
jump := newBlock - previousBlock
if jump > network.BlockJumpLimit {
    // Log warning, may indicate reorg
}
```

**Default**: Jump of 100+ blocks triggers warning log.

## Chain ID Validation

Periodically validates that providers are on the correct chain:

```go
func validateChainID(provider *provider, expectedChainID string) bool {
    actualChainID, err := handler.GetChainID(ctx, httpClient, provider.HttpUrl)
    if err != nil {
        return false
    }
    return actualChainID == expectedChainID
}
```

Chain ID format follows CAIP-2:
- EVM: `eip155:1` (Ethereum mainnet)
- Bitcoin: `bip122:000000000019d6...`
- Solana: `solana:mainnet`

## Archive Node Verification

For networks with `archive_enabled: true`:

```go
func verifyArchiveCapability(provider *provider) bool {
    // Query historical state (e.g., balance at genesis block)
    isArchive, err := handler.PerformArchiveCheck(ctx, httpClient, provider.HttpUrl)
    return err == nil && isArchive
}
```

Archive verification uses `eth_getBalance` at block 0 for EVM networks.

## Failure Counting

Consecutive failures are tracked:

```go
if err != nil {
    provider.failureCount++
    if provider.failureCount >= network.HCThreshold {
        provider.healthStatus = Unhealthy
    }
} else {
    provider.failureCount = 0
    provider.healthStatus = Healthy
}
```

## Health Check Methods by Handler

| Handler | Method | Response |
|---------|--------|----------|
| EVM | `eth_blockNumber` | Hex block number |
| Beacon | `GET /eth/v1/beacon/headers/head` | Slot in JSON |
| Bitcoin | `getblockcount` | Integer block height |
| Esplora | `GET /api/blocks/tip/height` | Integer block height |
| Solana | `getSlot` | Integer slot number |
| StarkNet | `starknet_blockNumber` | Integer block number |
| Tron | `eth_blockNumber` | Hex block number |

## Metrics Emitted

Health checks emit Prometheus metrics:

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `din_health_check_duration_seconds` | Histogram | network, provider | Check duration |
| `din_provider_health_status` | Gauge | network, provider | Current status (0/1/2) |
| `din_provider_block_number` | Gauge | network, provider | Latest block |
| `din_provider_block_lag` | Gauge | network, provider | Blocks behind consensus |

## Graceful Degradation

### All Providers Unhealthy

If all providers become unhealthy, requests are still routed:

```go
func (u *DinUpstreams) GetUpstreams(r *http.Request) ([]*reverseproxy.Upstream, error) {
    available := filterAvailable(providers)
    if len(available) == 0 {
        // Return all providers anyway (best effort)
        return allProviders, nil
    }
    return available, nil
}
```

### Network Stall

If the network itself is stalled:
- Providers aren't marked Unhealthy for lack of new blocks
- Status remains as last evaluated
- Requests continue to route

### Provider Recovery

Providers recover immediately on first successful health check:

```go
if err == nil {
    provider.healthStatus = Healthy
    provider.failureCount = 0
}
```

## Configuration Best Practices

### High-Frequency Networks (Ethereum)

```caddyfile
hc_interval 5
hc_threshold 2
block_lag_limit 5
```

### Slower Networks (Bitcoin)

```caddyfile
hc_interval 30
hc_threshold 3
block_lag_limit 2
```

### Latency-Sensitive

```caddyfile
hc_timeout 2000    # 2 second timeout
hc_threshold 1     # Mark unhealthy after 1 failure
```

### Reliability-Focused

```caddyfile
hc_timeout 10000   # 10 second timeout
hc_threshold 5     # 5 failures before unhealthy
```

## Related Documentation

- [Network Configuration](./03-network-configuration.md) - Health check settings
- [Network Handlers](./04-network-handlers.md) - Handler-specific health checks
- [Metrics & Monitoring](./10-metrics-monitoring.md) - Health check metrics
- [Dynamic Load Balancing](./07-dynamic-load-balancing.md) - How health affects routing
