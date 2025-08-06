# Chain ID Migration Guide: Removing CAIP-2 Prefixes

## Overview

As of this update, the DIN Caddy plugins no longer require CAIP-2 format prefixes for chain IDs. This change simplifies configuration and removes redundancy since network types are now determined by explicit handler declarations.

**Note:** EVM networks maintain backwards compatibility and support both formats.

## What Changed?

### Before (Old Format)
Chain IDs required CAIP-2 prefixes to identify the network type:
```
eip155:0x1      # Ethereum Mainnet
solana:5eykt... # Solana Mainnet  
starknet:0x534... # Starknet Mainnet
beacon:1        # Beacon Chain
```

### After (New Format - Recommended)
Chain IDs now use their native format without prefixes:
```
0x1             # Ethereum Mainnet (both formats supported)
5eykt...        # Solana Mainnet
0x534...        # Starknet Mainnet
1               # Beacon Chain
```

### Backwards Compatibility

**EVM Networks Only:** For backwards compatibility, EVM networks (handler type `evm`) continue to support both formats:
- ✅ `0x1` (recommended)
- ✅ `eip155:0x1` (supported for backwards compatibility)

Other network types (Solana, Starknet, Beacon) require the new format without prefixes.

## Migration Steps

### 1. Update Your Caddyfile

#### EVM Networks (Ethereum, Polygon, Arbitrum, etc.)
**Old:**
```caddyfile
networks {
    eth {
        providers {
            https://eth-mainnet.example.com
        }
        chain_id eip155:0x1
    }
    polygon {
        providers {
            https://polygon-mainnet.example.com
        }
        chain_id eip155:0x89
    }
}
```

**New:**
```caddyfile
networks {
    eth {
        providers {
            https://eth-mainnet.example.com
        }
        chain_id 0x1
    }
    polygon {
        providers {
            https://polygon-mainnet.example.com
        }
        chain_id 0x89
    }
}
```

#### Solana Networks
**Old:**
```caddyfile
networks {
    solana-mainnet {
        handler solana
        providers {
            https://solana.example.com
        }
        chain_id solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdpKuc147dw2N9d
    }
}
```

**New:**
```caddyfile
networks {
    solana-mainnet {
        handler solana
        providers {
            https://solana.example.com
        }
        chain_id 5eykt4UsFv8P8NJdTREpY1vzqKqZKvdpKuc147dw2N9d
    }
}
```

#### Starknet Networks
**Old:**
```caddyfile
networks {
    starknet-mainnet {
        handler starknet
        providers {
            https://starknet.example.com
        }
        chain_id starknet:0x534e5f4d41494e
    }
}
```

**New:**
```caddyfile
networks {
    starknet-mainnet {
        handler starknet
        providers {
            https://starknet.example.com
        }
        chain_id 0x534e5f4d41494e
    }
}
```

#### Beacon Chain
**Old:**
```caddyfile
networks {
    eth-beacon {
        handler beacon-chain
        providers {
            https://beacon.example.com
        }
        chain_id beacon:1
    }
}
```

**New:**
```caddyfile
networks {
    eth-beacon {
        handler beacon-chain
        providers {
            https://beacon.example.com
        }
        chain_id 1
    }
}
```

## Common Chain IDs Reference

### EVM Networks
| Network | Old Format | New Format |
|---------|------------|------------|
| Ethereum Mainnet | `eip155:0x1` | `0x1` |
| Ethereum Goerli | `eip155:0x5` | `0x5` |
| Ethereum Sepolia | `eip155:0xaa36a7` | `0xaa36a7` |
| Ethereum Holesky | `eip155:0x4268` | `0x4268` |
| Polygon Mainnet | `eip155:0x89` | `0x89` |
| Polygon Amoy | `eip155:0x13882` | `0x13882` |
| Arbitrum One | `eip155:0xa4b1` | `0xa4b1` |
| Arbitrum Sepolia | `eip155:0x66eee` | `0x66eee` |
| Optimism Mainnet | `eip155:0xa` | `0xa` |
| Optimism Sepolia | `eip155:0xaa37dc` | `0xaa37dc` |
| Base Mainnet | `eip155:0x2105` | `0x2105` |
| Base Sepolia | `eip155:0x14a34` | `0x14a34` |
| BSC Mainnet | `eip155:0x38` | `0x38` |
| BSC Testnet | `eip155:0x61` | `0x61` |
| Avalanche C-Chain | `eip155:0xa86a` | `0xa86a` |
| zkSync Era | `eip155:0x144` | `0x144` |
| zkSync Sepolia | `eip155:0x12c` | `0x12c` |
| Scroll Mainnet | `eip155:0x82750` | `0x82750` |
| Scroll Sepolia | `eip155:0x8274f` | `0x8274f` |
| Mantle Mainnet | `eip155:0x1388` | `0x1388` |
| Mantle Sepolia | `eip155:0x138b` | `0x138b` |
| Blast Mainnet | `eip155:0x13e31` | `0x13e31` |
| Blast Sepolia | `eip155:0xa0c71fd` | `0xa0c71fd` |

### Solana Networks
| Network | Old Format | New Format |
|---------|------------|------------|
| Solana Mainnet | `solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdpKuc147dw2N9d` | `5eykt4UsFv8P8NJdTREpY1vzqKqZKvdpKuc147dw2N9d` |
| Solana Devnet | `solana:EtWTRABZaYq6iMfeYKouRu166VU2xqa1wcaWoxPkrZBG` | `EtWTRABZaYq6iMfeYKouRu166VU2xqa1wcaWoxPkrZBG` |
| Solana Testnet | `solana:4uhcVJyU9pJkvQyS88uRDiswHXSCkY3zQawwpjk2NsNY` | `4uhcVJyU9pJkvQyS88uRDiswHXSCkY3zQawwpjk2NsNY` |

### Starknet Networks
| Network | Old Format | New Format |
|---------|------------|------------|
| Starknet Mainnet | `starknet:0x534e5f4d41494e` | `0x534e5f4d41494e` |
| Starknet Sepolia | `starknet:0x534e5f5345504f4c4941` | `0x534e5f5345504f4c4941` |

### Beacon Chain
| Network | Old Format | New Format |
|---------|------------|------------|
| Ethereum Beacon Mainnet | `beacon:1` | `1` |
| Goerli Beacon | `beacon:5` | `5` |
| Sepolia Beacon | `beacon:11155111` | `11155111` |
| Holesky Beacon | `beacon:17000` | `17000` |

## Validation Changes

### What's Now Invalid
For non-EVM networks, the system will **reject** chain IDs containing colons (`:`):
- ❌ `solana:5eykt...` (Solana networks)
- ❌ `starknet:0x534...` (Starknet networks)
- ❌ `beacon:1` (Beacon Chain)

Error message examples:
```
invalid Solana chain ID format: solana:5eykt..., chain ID should not contain ':' (CAIP-2 prefix no longer required)
invalid Beacon Chain chain ID format: beacon:1, chain ID should not contain ':' (CAIP-2 prefix no longer required)
```

### What's Valid
- ✅ Hex format for EVM: `0x1`, `0x89`, `0xa4b1`
- ✅ Decimal format for EVM: `1`, `137`, `42161`
- ✅ **CAIP-2 format for EVM (backwards compatibility)**: `eip155:0x1`, `eip155:1`
- ✅ Base58 hashes for Solana: `5eykt4UsFv8P8NJdTREpY1vzqKqZKvdpKuc147dw2N9d`
- ✅ Hex format for Starknet: `0x534e5f4d41494e`
- ✅ Numeric format for Beacon: `1`, `5`, `11155111`

## Handler Types

Remember to explicitly set handler types for non-EVM networks:

```caddyfile
networks {
    # EVM networks - handler defaults to 'evm' if not specified
    ethereum {
        chain_id 0x1
        # handler evm  # Optional, this is the default
    }
    
    # Non-EVM networks - handler must be specified
    solana-mainnet {
        handler solana  # Required for Solana
        chain_id 5eykt4UsFv8P8NJdTREpY1vzqKqZKvdpKuc147dw2N9d
    }
    
    starknet-mainnet {
        handler starknet  # Required for Starknet
        chain_id 0x534e5f4d41494e
    }
    
    eth-beacon {
        handler beacon-chain  # Required for Beacon Chain
        chain_id 1
    }
}
```

## Why This Change?

The CAIP-2 (Chain Agnostic Improvement Proposal 2) format was originally designed to provide a universal way to identify blockchain networks across different ecosystems. However, in our implementation:

1. **Handler Types Already Differentiate Networks**: Each network in the Caddyfile can explicitly declare its handler type (`handler evm`, `handler solana`, etc.), making the prefix redundant.

2. **Simplified Configuration**: Removing prefixes makes configuration files cleaner and easier to read.

3. **Native Format Alignment**: Chain IDs now match what the blockchains themselves use internally.

4. **Reduced Complexity**: One less thing to remember and one less place for typos.

## Troubleshooting

### Error: "chain ID should not contain ':'"
**Cause:** Your configuration still uses the old CAIP-2 format with prefixes.

**Solution:** Remove the prefix (everything before and including the colon) from your chain_id values.

### Error: "invalid chain ID format"
**Cause:** The chain ID format doesn't match what the handler expects.

**Solution:** Ensure you're using the correct format for your network type:
- EVM: Hex (`0x1`) or decimal (`1`)
- Solana: Base58 hash
- Starknet: Hex with `0x` prefix
- Beacon: Numeric

### Error: "no handler registered for network type"
**Cause:** The handler type isn't specified for a non-EVM network.

**Solution:** Add the appropriate handler declaration to your network configuration.

### Error: "failed to parse beacon config response"
**Cause:** The beacon API response format varies between providers.

**Solution:** The beacon handler has been updated to handle multiple response formats. Ensure you're using the latest version.

## Complete Migration Example

Here's a complete before/after example showing multiple network types:

**Before:**
```caddyfile
{
    admin :2019
}
:8000 {
    route /* {
        din {
            networks {
                eth {
                    providers {
                        https://eth-mainnet.example.com
                    }
                    chain_id eip155:0x1
                }
                polygon {
                    providers {
                        https://polygon-mainnet.example.com
                    }
                    chain_id eip155:0x89
                }
                solana-mainnet {
                    handler solana
                    providers {
                        https://solana-mainnet.example.com
                    }
                    chain_id solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdpKuc147dw2N9d
                }
                starknet-mainnet {
                    handler starknet
                    providers {
                        https://starknet-mainnet.example.com
                    }
                    chain_id starknet:0x534e5f4d41494e
                }
            }
        }
        reverse_proxy {
            lb_policy din_reverse_proxy_policy
            dynamic din_reverse_proxy_policy
        }
    }
}
```

**After:**
```caddyfile
{
    admin :2019
}
:8000 {
    route /* {
        din {
            networks {
                eth {
                    providers {
                        https://eth-mainnet.example.com
                    }
                    chain_id 0x1
                }
                polygon {
                    providers {
                        https://polygon-mainnet.example.com
                    }
                    chain_id 0x89
                }
                solana-mainnet {
                    handler solana
                    providers {
                        https://solana-mainnet.example.com
                    }
                    chain_id 5eykt4UsFv8P8NJdTREpY1vzqKqZKvdpKuc147dw2N9d
                }
                starknet-mainnet {
                    handler starknet
                    providers {
                        https://starknet-mainnet.example.com
                    }
                    chain_id 0x534e5f4d41494e
                }
            }
        }
        reverse_proxy {
            lb_policy din_reverse_proxy_policy
            dynamic din_reverse_proxy_policy
        }
    }
}
```

## Benefits of This Change

1. **Simpler Configuration**: No need to remember CAIP-2 prefixes
2. **Clearer Intent**: Handler type explicitly declares the network type
3. **Reduced Errors**: Fewer chances for prefix typos
4. **Better Alignment**: Chain IDs now match their native blockchain format
5. **Easier Migration**: New networks can be added without worrying about prefix standards
6. **Improved Maintainability**: Less code complexity in validation logic

## Need Help?

If you encounter any issues during migration:
1. Check that all chain_id values have been updated
2. Verify handler types are set for non-EVM networks
3. Ensure your Caddy server has been restarted after configuration changes
4. Review the error logs for specific validation messages

For additional support, please open an issue on the [DIN Caddy Plugins GitHub repository](https://github.com/DIN-center/din-caddy-plugins/issues).