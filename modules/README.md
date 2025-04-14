# DIN Caddy Modules

This directory contains the core modules for the DIN Caddy proxy system. These modules handle provider management, health checks, request routing, and more.

# Healthchecks

The health check system is implemented primarily in `network.go` and `provider.go`. It provides a robust mechanism for monitoring provider health and making intelligent routing decisions.

## Health Status Levels

The system defines three health status levels in `consts.go`:

```go
const (
    Healthy HealthStatus = iota
    Warning
    Unhealthy
)
```

- **Healthy**: Provider is fully operational and should receive traffic
- **Warning**: Provider has issues but can still serve traffic (e.g., slightly lagging behind)
- **Unhealthy**: Provider should not receive traffic (e.g., stalled, incorrect chain ID)

## Health Check Process

The health check process runs in a background goroutine started in the `startHealthChecks` method of `network.go`. Here's how it works:

1. **Initialization**: 
   - Each network starts a health check goroutine that runs at configurable intervals
   - The goroutine continues until the network's `quit` channel is closed

2. **Provider Querying**:
   - For each provider, the system calls `getLatestBlockNumber` to retrieve the current block
   - This function includes retry logic to handle transient failures
   - The block number and initial health status are determined by `processBlockNumberResponse`

3. **Health Evaluation**:
   - Each provider's health is evaluated by `evaluateProviderHealth`, which considers:
     - Block lag compared to the network
     - Block jump (being too far ahead)
     - Stalled status (block number not changing)
     - Chain ID verification
     - Archive mode support (for applicable networks)

4. **Block History Tracking**:
   - Each provider maintains a history of block numbers and health statuses
   - This history is used to detect stalled providers and track trends

5. **Network-wide Awareness**:
   - The system calculates the latest healthy block across all providers
   - It can detect when all providers are stalled (potential network outage)
   - It distinguishes between network-wide issues and individual provider problems

## Key Health Check Components

### Block Lag Detection

```go
if latestNetworkBlock > 0 {
    blockLag = int64(latestNetworkBlock) - currentBlock
    if blockLag > n.BlockLagLimit {
        isLagged = true
        n.logProviderWarning("Provider is lagging behind network", provider,
            zap.Int64("block_lag", blockLag),
            zap.Int64("provider_block", currentBlock),
            zap.Int64("network_block", latestNetworkBlock))
        if Warning > worstStatus {
            worstStatus = Warning
        }
    }
}
```

Providers that lag behind the network by more than `BlockLagLimit` blocks are marked with a warning status.

### Block Jump Detection

```go
blockJump := currentBlock - int64(latestNetworkBlock)
if blockJump > n.BlockJumpLimit {
    n.logProviderWarning("Provider is too far ahead of network", provider,
        zap.Int64("block_jump", blockJump),
        zap.Int64("provider_block", currentBlock),
        zap.Int64("network_block", latestNetworkBlock))
    return Unhealthy
}
```

Providers that report blocks too far ahead of the network are marked as unhealthy, as this could indicate a fork or misconfiguration.

### Stalled Provider Detection

```go
if isLagged {
    // Provider is lagging behind
    n.logProviderWarning("Provider is lagging behind network", provider,
        zap.Int64("block_lag", blockLag),
        zap.Int64("provider_block", currentBlock),
        zap.Int64("network_block", latestNetworkBlock))
    if Warning > worstStatus {
        worstStatus = Warning
    }
    
    if isStalled {
        // Provider is both stalled and lagged - more serious issue
        n.logProviderWarning("Provider is stalled and lagged", provider)
        return Unhealthy
    }
} else if isStalled && !allStalled {
    // Edge case: Provider is stalled but not yet lagged, while others are making progress
    n.logProviderWarning("Provider is stalled while others are progressing", provider)
    return Unhealthy
}
```

The system intelligently handles stalled providers:
- If a provider is both stalled and lagged, it's marked as unhealthy (serious issue)
- If a provider is lagged but not stalled, it's marked with a warning
- If all providers are stalled (indicating a network outage), they remain available
- If a provider is stalled while others are progressing, it's marked as unhealthy

### Chain ID Verification

```go
chainId, err := n.getChainID(provider.HttpUrl, provider.Headers, provider.AuthClient())
if err != nil {
    n.logProviderWarning("Error getting chain ID", provider, zap.Error(err))
    return Unhealthy
}

if !n.verifyChainID(chainId) {
    n.logProviderWarning("Provider has incorrect chain ID", provider)
    return Unhealthy
}
```

Ensures that providers are serving the correct blockchain network by verifying the chain ID.

### Archive Mode Support

```go
if n.ArchiveEnabled && !strings.Contains(n.Name, "bitcoin") && !strings.Contains(n.Name, "solana") && !strings.Contains(n.Name, "starknet") {
    // check if the provider can return back block data from half of its block height
    currentBlock := provider.getLatestHealthyBlockEntry()
    if currentBlock == nil {
        // if the provider has no healthy block history, return unhealthy
        return Unhealthy
    }
    // get a quarter of the block height
    quarterBlockHeight := currentBlock.blockNumber / 4

    // convert quarterBlockHeight to hex string
    quarterBlockHeightHex := fmt.Sprintf("%#x", quarterBlockHeight)

    // call the network method
    err := n.archiveModeCheck(provider.HttpUrl, provider.Headers, provider.AuthClient(), quarterBlockHeightHex)
    if err != nil {
        n.logProviderWarning("Error testing archive mode", provider, zap.Error(err))
        return Unhealthy
    }
}
```

For networks that support archive mode, the system verifies that providers can access historical data by querying a block at 1/4 of the current height.

## Retry Logic

The health check system implements retry logic for all network requests to handle transient failures:

```go
func (n *network) getLatestBlockNumber(httpUrl string, headers map[string]string, ac auth.IAuthClient) (int64, HealthStatus, error) {
    var lastErr error
    var lastHealthStatus HealthStatus = Unhealthy

    // Layer 1: Handle attempts
    for attempt := 0; attempt < n.RequestAttemptCount; attempt++ {
        payload := []byte(fmt.Sprintf(`{"jsonrpc":"2.0","method": "%s","params":[],"id":1}`, n.HCMethod))
        resBytes, statusCode, err := n.HttpClient.Post(httpUrl, headers, payload, ac)
        if err != nil {
            lastErr = err
            continue
        }

        blockNumber, health, err := n.processBlockNumberResponse(resBytes, statusCode)
        if err != nil {
            lastErr = err
            lastHealthStatus = health
            continue
        }

        return blockNumber, health, nil
    }

    return 0, lastHealthStatus, errors.Wrap(lastErr, fmt.Sprintf("Failed after %d attempts", n.RequestAttemptCount))
}
```

Similar retry logic is implemented for `getChainID` and `archiveModeCheck` functions.

## Provider Health Status Management

The `provider.go` file manages the block history and health status for each provider:

```go
func (p *provider) AddBlockEntry(block int64, status HealthStatus, blockHistorySize int) {
    p.mu.Lock()
    defer p.mu.Unlock()

    now := time.Now()
    entry := blockHistoryEntry{
        blockNumber:  block,
        healthStatus: status,
        timestamp:    &now,
    }
    p.blockHistory = append(p.blockHistory, entry)
    if len(p.blockHistory) > blockHistorySize {
        p.blockHistory = p.blockHistory[1:]
    }
}
```

This maintains a rolling history of block numbers and health statuses, which is used to detect stalled providers and track trends.

For more information on the overall DIN proxy architecture, see the [main README.md](../README.md#healthchecks).

# Network-specific Routing Rules

The **din_provider_filter.go** offers an interface for filtering which providers are capable of handling specific requests. The current 
implementation only supports method based routing, but this may be extended in the future.

This routing can be activated by updating the network configuration with:

```
din {
  services {
    ethereum {
        # Method routing configuration
        routed_methods debug_traceBlockByNumber
        providers {
          https://debug.tracer/ {
            priority 0
            methods debug_traceBlockByNumber
          }
          https://other.provider/ {
            priority 0
          }
        }
    }
  }
}
```

In this case any methods other than `debug_traceBlockByNumber` may be routed to either provider, but `debug_traceBlockByNumber` calls will
only be routed to the `debug.tracer` provider.

## Consistency Notes

Normally the DIN router ensures consistency across requests by routing requests with the same Din-Session-Id header to the same provider. This
might mean that most requests for a given Din-Session-Id are routed to `other.provider`, but if they make a `debug_traceBlockByNumber` request,
it must be routed to `debug.tracer` as the only provider capable of handling the request. In this case it's possible that the `debug_traceBlockByNumber`
requests may go to a provider that is ahead or behind other requests in the same session, or perhaps even on a different side of a small reorg.

We have method based routing with more sophisticated consistency guards on the roadmap, but right now it should be clearly communicated to clients that
`routed_methods` may not share the consistency guarantees seen on other methods.

# Module Overview

The DIN Caddy plugin consists of several key files:

- **network.go**: Implements the health check system and provider management
- **provider.go**: Defines the provider structure and block history tracking
- **consts.go**: Contains constants used throughout the system
- **din_upstreams.go**: Implements the upstream selection logic
- **din_handler.go**: The main HTTP handler for the DIN proxy

# Configuration Examples

Health check behavior can be customized in the Caddyfile:

```
din {
  services {
    ethereum {
      # Health check configuration
      block_lag_limit 10        # Number of blocks a provider can lag before warning
      block_jump_limit 5        # Number of blocks a provider can be ahead before unhealthy
      block_history_size 10     # Number of blocks to keep in history for stall detection
      health_check_interval 15s # How often to run health checks
      request_attempt_count 3   # Number of retry attempts for health check requests
      
      # Other service configuration...
    }
  }
}
```

# Troubleshooting

## Common Issues

### All Providers Show as Unhealthy

This could indicate:
- Network connectivity issues
- Incorrect chain ID configuration
- RPC method incompatibility

**Solution**: Check logs for specific error messages. Verify that the configured health check method is supported by all providers.

### Providers Frequently Toggle Between Healthy and Unhealthy

This could indicate:
- Unstable network connections
- Provider rate limiting
- Insufficient retry attempts

**Solution**: Increase `request_attempt_count` and `health_check_interval` to be more tolerant of transient issues.

### High Latency When Switching Between Providers

This could indicate:
- Aggressive health check settings causing frequent provider switching
- Session consistency issues

**Solution**: Adjust `block_lag_limit` to be more tolerant of minor block differences between providers.
