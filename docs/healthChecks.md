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

### 3. Block Jump Detection
- Identifies providers reporting blocks too far ahead
- Helps detect potential chain forks or misconfigured nodes
- Marks providers as unhealthy if they exceed `block_jump_limit`

**Health Status Impact:**
- **Healthy**: Block jump < `block_jump_limit`
- **Unhealthy**: Block jump > `block_jump_limit`

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
      block_lag_limit 10          # Maximum acceptable block lag
      block_jump_limit 5          # Maximum acceptable block jump
      block_history_size 10       # Number of historical entries to keep
      health_check_interval 15s   # Check frequency
      request_attempt_count 3     # Retry attempts per check
      archive_enabled true        # Enable archive node checks
    }
  }
}
```

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
