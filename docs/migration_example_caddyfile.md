# Configuration Migration Examples

This document shows examples of how to migrate your Caddyfile configurations from the old format (with deprecated fields) to the new format (using network handlers).

## Before Migration (Deprecated Format)

```caddyfile
{
    order din before reverse_proxy
}

localhost:8000 {
    din {
        networks {
            ethereum {
                methods eth_blockNumber eth_getBlockByNumber eth_call
                providers {
                    https://eth-mainnet.g.alchemy.com/v2/demo {
                        headers {
                            Content-Type application/json
                        }
                        priority 1
                    }
                    https://mainnet.infura.io/v3/demo {
                        headers {
                            Content-Type application/json
                        }
                        priority 2
                    }
                }
                chain_id eip155:0x1
                healthcheck_method eth_blockNumber        # DEPRECATED
                chainid_method eth_chainId               # DEPRECATED
                call_contract_method eth_call            # DEPRECATED
                get_block_by_number_method eth_getBlockByNumber  # DEPRECATED
                healthcheck_threshold 3
                healthcheck_interval 30
                healthcheck_blocklag_limit 10
            }
            
            ethereum-beacon {
                methods /eth/v1/beacon/headers /eth/v1/beacon/blocks
                providers {
                    https://beacon-nd-123-456-789.p2pify.com {
                        headers {
                            Content-Type application/json
                        }
                        priority 1
                    }
                }
                chain_id eip155:0x1
                healthcheck_method GET                   # DEPRECATED
                healthcheck_endpoint /eth/v1/beacon/headers/head
                healthcheck_threshold 2
                healthcheck_interval 15
            }
        }
    }
}
```

## After Migration (New Format)

```caddyfile
{
    order din before reverse_proxy
}

localhost:8000 {
    din {
        networks {
            ethereum {
                type evm                                 # NEW: Explicit network type
                methods eth_blockNumber eth_getBlockByNumber eth_call
                providers {
                    https://eth-mainnet.g.alchemy.com/v2/demo {
                        headers {
                            Content-Type application/json
                        }
                        priority 1
                    }
                    https://mainnet.infura.io/v3/demo {
                        headers {
                            Content-Type application/json
                        }
                        priority 2
                    }
                }
                chain_id eip155:0x1
                # Methods are now provided by the EVM handler automatically
                healthcheck_threshold 3
                healthcheck_interval 30
                healthcheck_blocklag_limit 10
            }
            
            ethereum-beacon {
                type beacon_chain                        # NEW: Explicit network type
                methods /eth/v1/beacon/headers /eth/v1/beacon/blocks
                providers {
                    https://beacon-nd-123-456-789.p2pify.com {
                        headers {
                            Content-Type application/json
                        }
                        priority 1
                    }
                }
                chain_id eip155:0x1
                healthcheck_endpoint /eth/v1/beacon/headers/head
                # Methods are now provided by the Beacon Chain handler automatically
                healthcheck_threshold 2
                healthcheck_interval 15
            }
            
            starknet-mainnet {
                type starknet                            # NEW: Explicit network type
                methods starknet_blockNumber starknet_getBlockWithTxHashes
                providers {
                    https://starknet-mainnet.g.alchemy.com/v2/demo {
                        headers {
                            Content-Type application/json
                        }
                        priority 1
                    }
                }
                chain_id sn_main
                # Methods are now provided by the Starknet handler automatically
                healthcheck_threshold 2
                healthcheck_interval 30
            }
            
            solana-mainnet {
                type solana                              # NEW: Explicit network type
                methods getSlot getBlockHeight getBlock
                providers {
                    https://api.mainnet-beta.solana.com {
                        headers {
                            Content-Type application/json
                        }
                        priority 1
                    }
                }
                chain_id solana:mainnet
                # Methods are now provided by the Solana handler automatically
                healthcheck_threshold 2
                healthcheck_interval 30
            }
        }
    }
}
```

## Key Changes

### 1. **Required `type` Field**
- **Before**: Network type was inferred from network name or method patterns
- **After**: Explicit `type` field is required for each network
- **Valid types**: `evm`, `beacon_chain`, `starknet`, `solana`, `bitcoin`

### 2. **Removed Deprecated Fields**
The following fields are **no longer needed** and will generate warnings if used:

- `healthcheck_method` → Provided by network handler
- `chainid_method` → Provided by network handler  
- `call_contract_method` → Provided by network handler
- `get_block_by_number_method` → Provided by network handler

### 3. **Automatic Method Selection**
- Health check methods are automatically selected based on network type
- Chain ID retrieval methods are automatically selected based on network type
- Contract call methods are automatically selected based on network type
- Block retrieval methods are automatically selected based on network type

### 4. **Backward Compatibility**
- Old configurations will continue to work but will log deprecation warnings
- The system will attempt to auto-detect network types if `type` field is missing
- Explicit `type` declaration is recommended for better performance and reliability

## Network Type Mapping

| Network Type | Handler | Default Health Check Method | Default Chain ID Method |
|-------------|---------|----------------------------|------------------------|
| `evm` | EVM Handler | `eth_blockNumber` | `eth_chainId` |
| `beacon_chain` | Beacon Chain Handler | `GET /eth/v1/beacon/headers/head` | `eth_chainId` |
| `starknet` | Starknet Handler | `starknet_blockNumber` | `starknet_chainId` |
| `solana` | Solana Handler | `getSlot` | `getGenesisHash` |
| `bitcoin` | Bitcoin Handler | `getblockcount` | `getblockchaininfo` |

## Migration Steps

1. **Add `type` field** to each network configuration
2. **Remove deprecated method fields** from your Caddyfile
3. **Test the configuration** to ensure it works correctly
4. **Monitor logs** for any deprecation warnings
5. **Update documentation** and deployment scripts

## Validation

The new configuration includes validation to ensure:
- Network type is supported by available handlers
- Required fields are present
- Configuration is internally consistent

If you specify an unsupported network type, you'll get a clear error message during startup. 