# DIN Health Check System Guide

## Overview
The DIN health check system is a robust monitoring solution that continuously evaluates the health and performance of blockchain providers. Think of it as a smart watchdog that ensures your blockchain connections remain reliable and performant.

## Why Health Checks Matter
- Ensures reliable blockchain connectivity
- Prevents routing traffic to problematic providers
- Maintains service quality
- Detects network-wide issues vs individual provider problems
- Helps with load balancing decisions

## Health Status Levels
The system uses three status levels to classify provider health:

1. **Healthy (Green)**
   - Provider is fully operational
   - Block numbers are current
   - Chain ID is correct
   - All checks passing

2. **Warning (Yellow)**
   - Provider is operational but showing signs of issues
   - Slightly behind in block numbers
   - Can still serve traffic but being monitored closely

3. **Unhealthy (Red)**
   - Provider should not receive traffic
   - Significantly behind or ahead in blocks
   - Wrong chain ID
   - Failed critical checks

## What Gets Checked

### 1. Block Height Monitoring
The system tracks each provider's current block number and compares it with other providers in the network.

**Health Status Impact:**
- **Healthy**: Block height within normal range
- **Warning**: Minor block height discrepancies
- **Unhealthy**: Significant block height deviation

```go
// Example configuration in Caddyfile
din {
  services {
    ethereum {
      block_lag_limit 10        # Maximum blocks a provider can lag behind
      block_jump_limit 5        # Maximum blocks a provider can be ahead
      health_check_interval 15s # How often to check
    }
  }
}
```

### 2. Block Lag Detection
- Monitors how far behind a provider is compared to the network
- Triggers warning if provider falls behind by more than `block_lag_limit` blocks
- Useful for detecting slow-syncing nodes

**Health Status Impact:**
- **Healthy**: Block lag < `block_lag_limit`
- **Warning**: Block lag > `block_lag_limit`
- **Unhealthy**: Block lag + stalled state

#### Dynamic Block Lag Calculation

The system can automatically calculate optimal `block_lag_limit` values based on the blockchain's actual block time, making configuration network-agnostic and adaptive to each chain's characteristics.

**How It Works:**
1. **Timestamp-Based Measurement**: On startup, the system measures average block time by comparing timestamps from blocks that are 1024 blocks apart
2. **Deterministic Results**: Uses immutable blockchain timestamps, so all server instances calculate the same value
3. **Automatic Adjustment**: Calculates how many blocks fit within a 13-second tolerance period (configurable via `DefaultBlockLagPeriodMs`)
4. **Safe Defaults**: Enforces minimum limit of 5 blocks and rounds up to nearest interval of 5

**Formula:**
```
block_lag_limit = ceil(13000ms / avg_block_time_ms)
```

**Example Calculations:**
- **Ethereum (12s blocks)**: 13000ms / 12000ms = ~2 blocks → rounded to 5 blocks minimum
- **Polygon (2s blocks)**: 13000ms / 2000ms = ~7 blocks → rounded to 10 blocks
- **BSC (3s blocks)**: 13000ms / 3000ms = ~5 blocks → stays at 5 blocks
- **Monad (1s blocks)**: 13000ms / 1000ms = 13 blocks → rounded to 15 blocks

**Configuration:**
Dynamic block lag is automatically enabled for supported networks (EVM, Beacon Chain, Tron, Starknet). No manual configuration required.

```go
// The system uses these constants:
DefaultBlockLagPeriodMs = 13000          // 13 seconds tolerance
BlockLagCalculationLookback = 1024       // Blocks to look back for measurement
MinBlockLagLimit = 5                     // Minimum allowed limit
```

**Benefits:**
- **Network-Agnostic**: Same configuration works across all blockchain networks
- **Accurate**: Based on actual on-chain data, not estimates
- **Consistent**: All instances calculate identical values from blockchain timestamps
- **Adaptive**: Automatically adjusts to network conditions without manual tuning

**Supported Networks:**
- ✅ EVM chains (Ethereum, Polygon, BSC, Arbitrum, etc.)
- ✅ Beacon Chain (Ethereum consensus layer)
- ✅ Tron
- ✅ Starknet
- ❌ Bitcoin (uses default/configured value)
- ❌ Bitcoin Esplora (uses default/configured value)
- ❌ Solana (uses default/configured value)

**Fallback Behavior:**
If dynamic calculation fails (network too young, API errors, etc.), the system falls back to:
1. Manually configured `block_lag_limit` if set in Caddyfile
2. Default value of 15 blocks (`DefaultBlockLagLimit`)

### 3. Block Jump Detection
- Identifies providers reporting blocks too far ahead
- Helps detect potential chain forks or misconfigured nodes
- Marks providers as unhealthy if they exceed `block_jump_limit`
- Implements smart recovery mechanism during network-wide issues

**Health Status Impact:**
- **Healthy**: Block jump < `block_jump_limit` OR the provider is the first to recover during network-wide outage
- **Unhealthy**: Block jump > `block_jump_limit` (only if other healthy providers exist)

**Smart Recovery Logic:**
- Only marks a provider as unhealthy for block jumps if at least one other healthy provider exists
- If no other healthy providers are available, assumes the ahead provider might be the first to recover from a network outage
- Prevents all providers from being marked unhealthy during network recovery scenarios

### 4. Stall Detection
The system has smart logic to detect different types of stalls:
- Individual provider stalls (provider stopped processing blocks)
- Network-wide stalls (all providers stopped - possible network issue)
- Differential stalls (some providers moving, others stalled)

**Health Status Impact:**
- **Healthy**: No stall detected
- **Warning**: Temporary stall while others progress
- **Unhealthy**: 
  - Stalled + lagged state
  - Stalled while other providers progress
  - Exception: All providers stalled (potential network issue) remain in current state

### 5. Chain ID Verification
- Ensures providers are serving the correct blockchain network
- Critical for preventing cross-chain issues
- Immediate unhealthy status if chain ID mismatch detected

**Health Status Impact:**
- **Healthy**: Chain ID matches configured value
- **Unhealthy**: 
  - Chain ID mismatch
  - Unable to retrieve chain ID
  - Error in chain ID response

### 6. Archive Node Verification
For networks supporting archive mode:
- Verifies access to historical data
- Tests queries at 1/4 of the current block height
- Only applies to supported networks (excludes Bitcoin, Solana, Starknet)

**Health Status Impact:**
- **Healthy**: Successfully queries historical blocks
- **Unhealthy**:
  - Cannot access historical data
  - Error in historical block response
  - No block history available for testing

### Grace Period Behavior
The system implements a grace period for transitioning to unhealthy status:

**Health Status Transitions:**
- First failure → **Warning** status
- Consecutive failures up to threshold → remains in **Warning**
- Failures exceed threshold → transitions to **Unhealthy**
- Success at any point → resets to **Healthy**

```go
// Example of grace period configuration
din {
  services {
    ethereum {
      healthcheck_threshold 3    # Number of consecutive failures before unhealthy
    }
  }
}
```

This grace period helps prevent status flapping and provides stability during temporary network issues while still maintaining strict health monitoring for persistent problems.

## How Health Checks Work

### Check Frequency
1. Health checks run at configurable intervals (default: 15 seconds)
2. Each check includes multiple providers
3. Results are stored in a rolling history

### Retry Logic
The system implements smart retry logic:
- Multiple attempts per check (configurable)
- Handles transient failures gracefully
- Maintains history for trend analysis

### Grace Periods
To prevent flapping between states:
- Providers get a grace period before being marked unhealthy
- Multiple consecutive failures required for status change
- Helps maintain stability during minor network hiccups

## Configuration Options

```go
din {
  services {
    ethereum {
      # Health Check Settings
      block_lag_limit 10          # Maximum acceptable block lag (optional - auto-calculated for supported networks)
      block_jump_limit 5          # Maximum acceptable block jump
      block_history_size 10       # Number of historical entries to keep
      health_check_interval 15s   # Check frequency
      request_attempt_count 3     # Retry attempts per check
      archive_enabled true        # Enable archive node checks
    }
  }
}
```

**Note on `block_lag_limit`:**
- For EVM, Beacon Chain, Tron, and Starknet networks, this value is automatically calculated based on actual block times
- Manual configuration is optional and will be used as fallback if auto-calculation fails
- For Bitcoin and Solana, manual configuration is required (or defaults to 15 blocks)

## Troubleshooting Guide

### Common Issues and Solutions

#### 1. All Providers Show Unhealthy
**Possible Causes:**
- Network connectivity issues
- Wrong chain ID configuration
- Unsupported RPC methods

**Solutions:**
- Check network connectivity
- Verify chain ID configuration
- Confirm RPC method support
- Review logs for specific errors

#### 2. Frequent Health Status Changes
**Possible Causes:**
- Unstable connections
- Rate limiting
- Aggressive check settings

**Solutions:**
- Increase `request_attempt_count`
- Adjust `health_check_interval`
- Check for rate limiting
- Review provider stability

#### 3. High Provider Switch Latency
**Possible Causes:**
- Aggressive health settings
- Session management issues
- Network latency

**Solutions:**
- Adjust `block_lag_limit`
- Review session handling
- Check network performance

## Best Practices

1. **Configuration**
   - Set reasonable block lag limits based on network
   - Configure appropriate check intervals
   - Use sufficient retry attempts

2. **Monitoring**
   - Watch for patterns in health status changes
   - Monitor provider response times
   - Track block progression

3. **Maintenance**
   - Regularly review health check logs
   - Adjust thresholds based on network conditions
   - Remove consistently problematic providers

## Metrics and Logging
The system provides detailed metrics and logs:
- Health status changes
- Block number progression
- Response times
- Error conditions
- Provider availability

## Additional Resources
- [Main README](../README.md)
- [Configuration Guide](./configuration.md)
- [Provider Management](./providers.md)
