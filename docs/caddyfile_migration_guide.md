# Caddyfile Migration Guide: Legacy to Handler-Based Format

## Overview

This guide helps you migrate your Caddyfile configuration from the legacy format (with explicit method fields) to the new handler-based format that uses network-specific handlers for different blockchain types.

**What's New?** The DIN Gateway now uses a modular handler system where each blockchain network type (EVM, Solana, Starknet, etc.) has its own dedicated handler that automatically provides the correct methods and behavior. This eliminates the need to manually specify methods in your configuration.

**Learn More**: For details on how the handler system works and how to add new network types, see the [Adding New Networks Guide](./adding_new_networks.md).

## Migration Steps

### Step 1: Identify Your Current Configuration Format

**Legacy Format (Deprecated):**
```caddyfile
networks {
    solana-mainnet {
        providers {
            https://provider.example.com {
                priority 1
            }
        }
        healthcheck_method getBlockHeight        # X DEPRECATED
        chainid_method getGenesisHash            # X DEPRECATED  
        call_contract_method starknet_call       # X DEPRECATED
        get_block_by_number_method getBlock      # X DEPRECATED
        chain_id solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdpKuc147dw2N9d
    }
}
```

**New Format (Recommended):**
```caddyfile
networks {
    solana-mainnet {
        type solana                              # EXPLICIT TYPE
        providers {
            https://provider.example.com {
                priority 1
            }
        }
        chain_id solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdpKuc147dw2N9d
        # No method fields needed - provided by Solana handler
    }
}
```

### Step 2: Remove Deprecated Method Fields

Remove these deprecated fields from your network configurations:

- X `healthcheck_method`
- X `chainid_method` 
- X `call_contract_method`
- X `get_block_by_number_method`

### Step 3: Add Explicit Network Type

Add the `type` field to explicitly specify which handler to use:

| Network Name Pattern | Recommended Type | Auto-Detection |
|---------------------|------------------|----------------|
| `*solana*` | `solana` | Automatic |
| `*starknet*` | `starknet` | Automatic |
| `*bitcoin*` or `*btc*` | `bitcoin` | Automatic |
| `*beacon*` or `*consensus*` | `beacon-chain` | Automatic |
| Everything else | `evm` | Default |

**Note:** While auto-detection works, explicitly specifying the `type` is recommended for clarity.

## Network-Specific Migration Examples

### Solana Networks

**Before (Legacy):**
```caddyfile
solana-mainnet {
    providers {
        https://din-mainnet.rpc.extrnode.com/{Key}{
            priority 1
        }
        https://consensys.rpcpool.com/{Key} {
            priority 0
        }
    }
    healthcheck_method getBlockHeight           # X Remove
    chainid_method getGenesisHash              # X Remove
    get_block_by_number_method getBlock        # X Remove
    chain_id solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdpKuc147dw2N9d
}
```

**After (New Format):**
```caddyfile
solana-mainnet {
    type solana                                # Add explicit type
    providers {
        https://din-mainnet.rpc.extrnode.com/{Key} {
            priority 1
        }
        https://consensys.rpcpool.com/{Key} {
            priority 0
        }
    }
    chain_id solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdpKuc147dw2N9d
    # Methods are now provided automatically by Solana handler:
    # - healthcheck_method: getBlockHeight
    # - chainid_method: getGenesisHash  
    # - get_block_by_number_method: getBlock
}
```

### EVM Networks

**Before (Legacy):**
```caddyfile
ethereum {
    providers {
        https://eth-mainnet.g.alchemy.com/v2/your-api-key {
            priority 1
        }
    }
    healthcheck_method eth_blockNumber         # X Remove
    chainid_method eth_chainId                 # X Remove
    call_contract_method eth_call              # X Remove
    get_block_by_number_method eth_getBlockByNumber # X Remove
    chain_id eip155:0x1
}
```

**After (New Format):**
```caddyfile
ethereum {
    type evm                                   # Add explicit type (optional, auto-detected)
    providers {
        https://eth-mainnet.g.alchemy.com/v2/your-api-key {
            priority 1
        }
    }
    chain_id eip155:0x1
    # Methods are now provided automatically by EVM handler:
    # - healthcheck_method: eth_blockNumber
    # - chainid_method: eth_chainId
    # - call_contract_method: eth_call
    # - get_block_by_number_method: eth_getBlockByNumber
}
```

### Starknet Networks

**Before (Legacy):**
```caddyfile
starknet-mainnet {
    providers {
        https://starknet-mainnet.public.blastapi.io {
            priority 0
        }
    }
    healthcheck_method starknet_blockNumber    # X Remove
    chainid_method starknet_chainId            # X Remove
    call_contract_method starknet_call         # X Remove
    chain_id starknet:0x534e5f4d41494e
}
```

**After (New Format):**
```caddyfile
starknet-mainnet {
    type starknet                              # Add explicit type
    providers {
        https://starknet-mainnet.public.blastapi.io {
            priority 0
        }
    }
    chain_id starknet:0x534e5f4d41494e
    # Methods are now provided automatically by Starknet handler:
    # - healthcheck_method: starknet_blockNumber
    # - chainid_method: starknet_chainId
    # - call_contract_method: starknet_call
}
```

### Bitcoin Networks

**Before (Legacy):**
```caddyfile
bitcoin-mainnet {
    providers {
        https://bitcoin-rpc.example.com {
            priority 1
        }
    }
    healthcheck_method getblockcount           # X Remove
    chainid_method getblockchaininfo           # X Remove
    chain_id bip122:main
}
```

**After (New Format):**
```caddyfile
bitcoin-mainnet {
    type bitcoin                               # Add explicit type
    providers {
        https://bitcoin-rpc.example.com {
            priority 1
        }
    }
    chain_id bip122:main
    # Methods are now provided automatically by Bitcoin handler:
    # - healthcheck_method: getblockcount
    # - chainid_method: getblockchaininfo
}
```

### Beacon Chain Networks

**Before (Legacy):**
```caddyfile
ethereum-beacon {
    providers {
        https://beacon-api.example.com {
            priority 1
        }
    }
    healthcheck_method /eth/v1/beacon/headers/head # X Remove
    chainid_method /eth/v1/config/spec             # X Remove
    healthcheck_endpoint /eth/v1/beacon/headers/head # Keep for REST APIs
    chain_id beacon:0x00000000
}
```

**After (New Format):**
```caddyfile
ethereum-beacon {
    type beacon-chain                          # Add explicit type
    providers {
        https://beacon-api.example.com {
            priority 1
        }
    }
    healthcheck_endpoint /eth/v1/beacon/headers/head # Keep for REST APIs
    chain_id beacon:0x00000000
    # Methods are now provided automatically by Beacon Chain handler
}
```

## Configuration Validation

### What Configurations Still Work?

The following configurations remain **unchanged** and continue to work:

- `providers` block and provider URLs
- `priority` settings for providers  
- `chain_id` field
- `healthcheck_interval_seconds`
- `healthcheck_blocklag_limit`
- `healthcheck_blockjump_limit`
- `max_request_payload_size_kb`
- `request_attempt_count`
- `archive_enabled`
- `healthcheck_endpoint` (for REST APIs like Beacon Chain)

### Backward Compatibility

- **Deprecation warnings** are logged when legacy method fields are used
- **Legacy fields are ignored** during configuration parsing
- **No breaking changes** - old configurations continue to work with warnings

## Testing Your Migration

### Step 1: Update Configuration
1. Remove deprecated method fields
2. Add explicit `type` field
3. Verify `chain_id` format matches network type

### Step 2: Test Health Checks
```bash
# Check Caddy logs for deprecation warnings
tail -f /var/log/caddy/caddy.log | grep -i "deprecated"

# Test health check endpoint
curl -X POST http://localhost:8000/solana-mainnet \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"getBlockHeight","id":1}'
```

### Step 3: Verify Handler Methods

Each network type uses specific methods automatically:

| Network Type | Health Check Method | Chain ID Method | Block Method |
|-------------|-------------------|-----------------|-------------|
| `solana` | `getBlockHeight` | `getGenesisHash` | `getBlock` |
| `evm` | `eth_blockNumber` | `eth_chainId` | `eth_getBlockByNumber` |
| `starknet` | `starknet_blockNumber` | `starknet_chainId` | `starknet_getBlockWithTxHashes` |
| `bitcoin` | `getblockcount` | `getblockchaininfo` | `getblock` |
| `beacon-chain` | `/eth/v1/beacon/headers/head` | `/eth/v1/config/spec` | `/eth/v2/beacon/blocks/{block_id}` |

## Troubleshooting Common Issues

### Issue 1: "Method not found" errors for Solana

**Problem:** Using EVM methods (`eth_blockNumber`) on Solana network
```
"error": {"code": -32601, "message": "Method not found"}
"request_method": "eth_blockNumber"
```

**Solution:** Ensure network type is correctly set to `solana` or auto-detected:
```caddyfile
solana-mainnet {
    type solana  # Explicit type specification
    # ... rest of config
}
```

### Issue 2: "unsupported block number type" errors

**Problem:** Legacy response parsing trying to parse Solana integer responses as hex strings

**Solution:** Remove deprecated method fields and let the handler manage response parsing:
```caddyfile
# Remove these:
# healthcheck_method getBlockHeight
# chainid_method getGenesisHash
```

### Issue 3: Handler initialization failures

**Problem:** `failed to get handler for network type 'unknown'`

**Solution:** Add explicit `type` field or ensure network name follows naming conventions:
```caddyfile
my-custom-solana-network {
    type solana  # Explicit type when name doesn't contain "solana"
    # ... rest of config  
}
```

## Migration Checklist

- [ ] **Backup** your current Caddyfile
- [ ] **Remove** deprecated method fields:
  - [ ] `healthcheck_method`
  - [ ] `chainid_method`
  - [ ] `call_contract_method`
  - [ ] `get_block_by_number_method`
- [ ] **Add** explicit `type` field for each network
- [ ] **Verify** `chain_id` format matches network type
- [ ] **Test** configuration with `caddy validate`
- [ ] **Monitor** logs for deprecation warnings
- [ ] **Test** health checks and network functionality
- [ ] **Update** documentation and deployment scripts

## Need Help?

- Check the logs for specific error messages and deprecation warnings
- Verify your network type matches the expected handler
- Ensure chain ID format is correct for your network type
- Test individual networks by temporarily commenting out others
- See [Adding New Networks Guide](./adding_new_networks.md) for handler details
- Review [Multi-Network Type Support Guide](./multi_network_type_support.md) for system architecture

## Handler Method Reference

### Automatic Method Mappings

When you specify a network `type`, the following methods are automatically configured:

#### Solana Handler (`type: solana`)
- Health Check: `getBlockHeight`
- Chain ID: `getGenesisHash` 
- Block Method: `getBlock`
- Archive: Not supported
- Chain ID Format: `solana:{base58_genesis_hash}`

#### EVM Handler (`type: evm`)
- Health Check: `eth_blockNumber`
- Chain ID: `eth_chainId`
- Block Method: `eth_getBlockByNumber`
- Archive: `eth_call`
- Chain ID Format: `eip155:{hex_chain_id}`

#### Starknet Handler (`type: starknet`)
- Health Check: `starknet_blockNumber`
- Chain ID: `starknet_chainId`
- Block Method: `starknet_getBlockWithTxHashes`
- Archive: `starknet_call`
- Chain ID Format: `starknet:{hex_chain_id}`

#### Bitcoin Handler (`type: bitcoin`)
- Health Check: `getblockcount`
- Chain ID: `getblockchaininfo`
- Block Method: `getblock`
- Archive: Not supported
- Chain ID Format: `bip122:{network_name}`

#### Beacon Chain Handler (`type: beacon-chain`)
- Health Check: `/eth/v1/beacon/headers/head`
- Chain ID: `/eth/v1/config/spec`
- Block Method: `/eth/v2/beacon/blocks/{block_id}`
- Archive: Not supported
- Chain ID Format: `beacon:{hex_config_hash}` 