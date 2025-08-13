# DIN Registry Synchronization Guide

## Overview
The DIN Registry Sync system is a feature that automatically synchronizes your DIN proxy with a central registry of blockchain networks and providers. Think of it as an auto-updating address book that keeps your proxy's network configurations current and consistent.

## Why Registry Sync Matters
- **Automated Updates**: No manual configuration needed for new networks and providers
- **Consistency**: Ensures all DIN proxies use the same configurations
- **Reliability**: Automatically removes inactive or problematic providers
- **Scalability**: Easy addition of new networks and providers
- **Maintenance**: Reduces operational overhead

## How Registry Sync Works

### 1. Initialization Process
When the DIN proxy starts up, it:
1. Connects to the configured registry endpoint
2. Pulls initial network and provider data
3. Sets up periodic sync checks
4. Starts monitoring for updates

```go
// Example Registry Configuration in Caddyfile
din {
  din_registry {
    registry_enabled true
    registry_endpoint_url "https://registry.example.com"
    registry_contract_address "0x1234..."
    registry_block_epoch 10
    registry_block_check_interval_sec 60
    registry_priority 1
  }
}
```

### 2. Sync Schedule
The system uses a block-based scheduling mechanism:

- **Block Epoch**: Defines how many blocks to wait between syncs
- **Check Interval**: How often to check for new blocks (in seconds)
- **Priority**: Determines provider priority in the network

Example: With `registry_block_epoch = 10` and current block = 105:
- Next sync will occur at block 110 (next multiple of 10)
- System checks block numbers every `registry_block_check_interval_sec`

### 3. What Gets Synchronized

#### Network Configuration
- Network names and identifiers  
- Handler type (e.g., "evm", "solana", "starknet")
- Chain IDs (now in simple hex format, e.g., "0x1")
- Health check parameters:
  - Interval (seconds)
  - Threshold
  - Timeout
  - Block lag limit
  - Block jump limit
- Block history sizes:
  - Provider block history size
  - Network block history size
- Request configuration:
  - Max request payload size (KB)
  - Request attempt count
- Archive mode settings

#### Provider Information
- Provider URLs
- Authentication configurations
- Status (active/inactive)
- Priority levels
- Supported methods

#### Configuration Priority System
**Important:** Caddyfile configurations always take precedence over registry values. The system follows this priority order:
1. **Caddyfile (Highest Priority)**: Values explicitly set in your Caddyfile are never overwritten by registry updates
2. **Registry**: Values from the registry are only applied if not set in Caddyfile
3. **Defaults (Lowest Priority)**: Built-in default values are used if neither Caddyfile nor registry provides a value

This ensures that your local configurations remain stable and predictable, while still benefiting from registry updates for unspecified values.

## Sync Process Details

### 1. Block Number Check
```go
// Pseudocode example of sync timing
currentBlock = 55
blockEpoch = 10
lastSyncBlock = 50

nextSyncBlock = currentBlock - (currentBlock % blockEpoch) // = 50
if nextSyncBlock > lastSyncBlock {
    performSync()
}
```

### 2. Network Updates
The system handles three scenarios:

#### New Networks
- Detects previously unknown networks
- Creates new network configurations
- Initializes health checks
- Adds configured providers

#### Existing Networks
- Updates configuration if changed
- Syncs provider lists
- Maintains health check history
- Preserves existing metrics

#### Inactive Networks
- Removes networks marked as inactive
- Gracefully terminates connections
- Cleans up resources

### 3. Provider Updates
For each network, the system manages providers:

#### Active Providers
- Adds new providers to the network
- Updates existing provider configurations
- Maintains authentication settings
- Preserves health check history

#### Inactive Providers
- Removes providers marked as inactive
- Gracefully closes connections
- Updates routing tables

## Configuration Options

### Basic Configuration
```go
din {
  din_registry {
    # Required Settings
    registry_enabled true                                    # Enable/disable registry sync
    registry_endpoint_url "https://registry.example.com"     # Registry API endpoint
    registry_contract_address "0x1234..."                    # Registry contract address

    # Optional Settings
    registry_block_epoch 10                                  # Blocks between syncs
    registry_block_check_interval_sec 60                     # Check frequency
    registry_priority 1                                      # Provider priority
  }
}
```

### Advanced Settings
- **Block Epoch**: Adjust based on network block time
  - Lower values: More frequent updates
  - Higher values: Reduced sync overhead

- **Check Interval**: Balance between responsiveness and resource usage
  - Lower values: Faster update detection
  - Higher values: Reduced API calls

## Troubleshooting Guide

### Common Issues and Solutions

#### 1. Registry Connection Failed
**Symptoms:**
- No new networks appearing
- Configuration not updating

**Solutions:**
- Check registry endpoint URL
- Verify contract address
- Check network connectivity
- Review authentication settings

#### 2. Sync Not Occurring
**Symptoms:**
- Old configurations persist
- Updates not reflecting

**Solutions:**
- Verify `registry_enabled` is true
- Check block epoch settings
- Review check interval
- Monitor block progression

#### 3. Inconsistent Updates
**Symptoms:**
- Some networks update, others don't
- Intermittent synchronization

**Solutions:**
- Adjust block epoch
- Increase check interval
- Check network stability
- Review provider health

## Best Practices

### 1. Configuration
- Set reasonable block epochs based on network speed
- Use appropriate check intervals
- Configure fallback providers
- Maintain backup configurations

### 2. Monitoring
- Watch sync logs regularly
- Monitor update frequencies
- Track provider changes
- Observe network additions/removals

### 3. Maintenance
- Regular configuration reviews
- Update endpoint URLs as needed
- Adjust sync parameters based on needs
- Keep fallback configurations current

## Metrics and Logging
The system provides detailed sync metrics:
- Sync attempts and success rates
- Network update counts
- Provider changes
- Block progression
- Error rates and types

## Migration Guide

### Migrating from Old Method Bit Fields to Handler-Based System

The registry sync system has been updated to use a handler-based approach instead of method bit fields:

#### What Changed:
- **Removed**: `healthcheck_method_bit`, `chain_id_method_bit`, `get_block_by_number_method_bit`, `call_contract_method_bit`
- **Added**: `handler` field that specifies the network type (e.g., "evm", "solana", "starknet")
- **New Fields**: 
  - `healthcheck_threshold`: Number of consecutive failures before marking unhealthy
  - `healthcheck_timeout`: Timeout for health check requests (seconds)
  - `provider_block_history_size`: Number of blocks to track per provider
  - `network_block_history_size`: Number of blocks to track for the network

#### Chain ID Format:
- **Old Format**: CAIP-2 format (e.g., `eip155:0x1`)
- **New Format**: Simple hex format (e.g., `0x1`)

#### How Methods Are Determined:
Methods are now determined by the handler type rather than explicit bit fields. Each handler knows its appropriate methods:
- **EVM Handler**: Uses `eth_blockNumber`, `eth_chainId`, `eth_getBlockByNumber`, `eth_call`
- **Solana Handler**: Uses Solana-specific RPC methods
- **Starknet Handler**: Uses Starknet-specific RPC methods

#### Configuration Examples:

**Old Configuration:**
```yaml
networks:
  ethereum:
    healthcheck_method_bit: 1
    chain_id_method_bit: 1
    chain_id: "eip155:0x1"
```

**New Configuration:**
```yaml
networks:
  ethereum:
    handler: "evm"
    chain_id: "0x1"
    healthcheck_threshold: 2
    healthcheck_timeout: 5
    provider_block_history_size: 10
    network_block_history_size: 128
```

## Additional Resources
- [Main README](../README.md)
- [Health Checks Guide](./healthChecks.md)
- [Configuration Guide](./configuration.md)
