# Caddyfile Configuration

This document provides a complete reference for configuring DIN Caddy Plugins via Caddyfile.

## Basic Structure

```caddyfile
{
    # Global options
    admin :2019
    log {
        level INFO
    }
}

:8080 {
    # Route configuration
    route /* {
        din {
            # DIN middleware configuration
        }
        reverse_proxy {
            dynamic din
            lb_policy din
        }
    }
}
```

---

## DIN Middleware Block

### Complete Example

```caddyfile
din {
    # Network services
    services {
        ethereum-mainnet {
            handler_type evm
            chain_id eip155:1
            hc_interval 5
            hc_threshold 2
            hc_timeout 5000
            block_lag_limit 5
            block_jump_limit 100
            archive_enabled false

            methods {
                eth_blockNumber
                eth_getBalance
                eth_call
                eth_estimateGas
                eth_sendRawTransaction
            }

            providers {
                https://mainnet.infura.io/v3/{$INFURA_KEY} {
                    priority 0
                    headers {
                        X-Custom-Header value
                    }
                    auth siwe {
                        private_key {$SIWE_PRIVATE_KEY}
                        auth_endpoint /auth
                    }
                }
                https://eth-mainnet.alchemyapi.io/v2/{$ALCHEMY_KEY} {
                    priority 1
                }
            }
        }
    }

    # Registry configuration
    din_registry {
        endpoint {$REGISTRY_ENDPOINT}
        contract_address {$REGISTRY_CONTRACT}
        block_epoch 2000
        check_interval 60
    }

    # Dynamic load balancing
    dynamic_load_balancing {
        enabled true
        watcher_url {$WATCHER_URL}
        sync_interval 30
        grace_period 60
        convergence_period 120
        weights {
            block_consistency 0.4
            state_consistency 0.4
            latency 0.2
        }
    }
}
```

---

## Network Configuration

### Network Block

```caddyfile
services {
    <network-name> {
        handler_type <type>
        chain_id <caip-2-id>
        hc_interval <seconds>
        hc_threshold <count>
        hc_timeout <milliseconds>
        block_lag_limit <blocks>
        block_jump_limit <blocks>
        archive_enabled <true|false>

        methods {
            <method1>
            <method2>
        }

        providers {
            # provider blocks
        }
    }
}
```

### Handler Types

| Type | Networks |
|------|----------|
| `evm` | Ethereum, Optimism, Arbitrum, Polygon, Base, Linea |
| `beacon-chain` | Ethereum Beacon Chain |
| `bitcoin` | Bitcoin (JSON-RPC) |
| `bitcoin-esplora` | Bitcoin (Esplora API) |
| `solana` | Solana |
| `starknet` | StarkNet |
| `tron-full-node` | Tron |

### Chain ID Format (CAIP-2)

| Network | Format | Example |
|---------|--------|---------|
| EVM | `eip155:<chainId>` | `eip155:1` |
| Bitcoin | `bip122:<genesisHash>` | `bip122:000000000019d6...` |
| Solana | `solana:<network>` | `solana:mainnet` |
| StarkNet | `starknet:<network>` | `starknet:mainnet` |

### Health Check Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `hc_interval` | int | 5 | Seconds between checks |
| `hc_threshold` | int | 2 | Failures before unhealthy |
| `hc_timeout` | int | 5000 | Timeout in milliseconds |
| `block_lag_limit` | int | 5 | Max blocks behind |
| `block_jump_limit` | int | 100 | Max block jump |
| `archive_enabled` | bool | false | Verify archive capability |

---

## Provider Configuration

### Provider Block

```caddyfile
providers {
    <url> {
        priority <number>
        headers {
            <Header-Name> <value>
        }
        methods {
            <method1>
            <method2>
        }
        auth siwe {
            private_key <key>
            auth_endpoint <path>
            pool_size <count>
            expiration_buffer <seconds>
        }
        # OR
        oidc {
            client_id <id>
            client_secret <secret>
            token_url <url>
            scopes <scope1> <scope2>
        }
    }
}
```

### Provider Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `priority` | int | 0 | Priority tier (lower = higher) |
| `headers` | block | - | Custom headers to include |
| `methods` | block | all | Supported methods |
| `auth` | block | - | SIWE authentication |
| `oidc` | block | - | OIDC authentication |

### SIWE Auth Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `private_key` | string | required | Ethereum private key |
| `auth_endpoint` | string | `/auth` | Auth endpoint path |
| `pool_size` | int | 5 | Session pool size |
| `expiration_buffer` | int | 60 | Renew before expiry (seconds) |

### OIDC Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `client_id` | string | required | OAuth client ID |
| `client_secret` | string | required | OAuth client secret |
| `token_url` | string | required | Token endpoint URL |
| `scopes` | list | - | OAuth scopes |

---

## Registry Configuration

```caddyfile
din_registry {
    endpoint <rpc-url>
    contract_address <address>
    block_epoch <blocks>
    check_interval <seconds>
    retry_attempts <count>
    retry_delay <seconds>
}
```

### Registry Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `endpoint` | string | required | RPC endpoint for registry |
| `contract_address` | string | required | Registry contract address |
| `block_epoch` | int | 2000 | Sync every N blocks |
| `check_interval` | int | 60 | Check interval (seconds) |
| `retry_attempts` | int | 3 | Retry count on failure |
| `retry_delay` | int | 2 | Delay between retries |

---

## Dynamic Load Balancing Configuration

```caddyfile
dynamic_load_balancing {
    enabled <true|false>
    watcher_url <url>
    sync_interval <seconds>
    grace_period <seconds>
    convergence_period <seconds>
    weights {
        block_consistency <weight>
        state_consistency <weight>
        latency <weight>
    }
}
```

### Dynamic LB Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | false | Enable dynamic LB |
| `watcher_url` | string | - | Watcher API URL |
| `sync_interval` | int | 30 | Score sync interval |
| `grace_period` | int | 60 | Stale score grace period |
| `convergence_period` | int | 120 | Recovery smoothing period |
| `weights` | block | - | Metric weights |

---

## SIWE Auth Server (Provider Side)

For providers that want to accept SIWE authentication:

```caddyfile
:8545 {
    route /auth {
        din_auth {
            secret {$AUTH_SECRET}
            token_duration 3600
            max_token_uses 100
            allowed_addresses {
                0x1234...
                0x5678...
            }
        }
    }

    route /* {
        din_auth {
            secret {$AUTH_SECRET}
        }
        reverse_proxy localhost:8546
    }
}
```

### din_auth Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `secret` | string | required | JWT signing secret |
| `token_duration` | int | 3600 | Token validity (seconds) |
| `max_token_uses` | int | 0 | Max uses (0 = unlimited) |
| `allowed_addresses` | list | all | Allowed wallet addresses |

---

## Reverse Proxy Integration

```caddyfile
reverse_proxy {
    # Use DIN for upstream discovery
    dynamic din

    # Use DIN for load balancing
    lb_policy din

    # Optional: session header for affinity
    header_up Din-Session-Id {http.request.header.Din-Session-Id}

    # Timeouts
    transport http {
        dial_timeout 5s
        response_header_timeout 30s
    }
}
```

---

## Environment Variables

Use `{$VAR_NAME}` syntax for environment variables:

```caddyfile
din {
    services {
        ethereum-mainnet {
            providers {
                https://mainnet.infura.io/v3/{$INFURA_KEY} {
                    priority 0
                }
            }
        }
    }
}
```

### Required Environment Variables

```bash
# Provider keys
INFURA_KEY=your-infura-key
ALCHEMY_KEY=your-alchemy-key

# SIWE authentication
SIWE_PRIVATE_KEY=0x...

# Registry (optional)
REGISTRY_ENDPOINT=https://mainnet.infura.io/v3/...
REGISTRY_CONTRACT=0x...

# Watcher (optional)
WATCHER_URL=https://watcher.din.network/api/v1

# Auth server (optional)
AUTH_SECRET=your-jwt-secret
```

---

## Complete Production Example

```caddyfile
{
    admin :2019
    log {
        level INFO
        format json
    }
}

:8080 {
    log {
        output stdout
        format json
    }

    route /* {
        din {
            services {
                ethereum-mainnet {
                    handler_type evm
                    chain_id eip155:1
                    hc_interval 5
                    hc_threshold 2
                    hc_timeout 5000
                    block_lag_limit 5

                    providers {
                        https://mainnet.infura.io/v3/{$INFURA_KEY} {
                            priority 0
                        }
                        https://eth-mainnet.g.alchemy.com/v2/{$ALCHEMY_KEY} {
                            priority 0
                        }
                        https://rpc.ankr.com/eth {
                            priority 1
                        }
                    }
                }

                optimism-mainnet {
                    handler_type evm
                    chain_id eip155:10
                    hc_interval 3
                    hc_threshold 2

                    providers {
                        https://optimism-mainnet.infura.io/v3/{$INFURA_KEY} {
                            priority 0
                        }
                    }
                }
            }

            din_registry {
                endpoint https://mainnet.infura.io/v3/{$INFURA_KEY}
                contract_address {$REGISTRY_CONTRACT}
                block_epoch 2000
            }

            dynamic_load_balancing {
                enabled true
                watcher_url {$WATCHER_URL}
                sync_interval 30
            }
        }

        reverse_proxy {
            dynamic din
            lb_policy din

            transport http {
                dial_timeout 5s
                response_header_timeout 30s
            }
        }
    }
}
```

---

## Validation

Validate your Caddyfile:

```bash
make validate-config
# or
./build/din-caddy validate --config Caddyfile
```

---

## Related Documentation

- [Network Configuration](./03-network-configuration.md) - Network options
- [Authentication](./05-authentication.md) - Auth setup
- [Registry Sync](./08-registry-sync.md) - Registry options
- [Dynamic Load Balancing](./07-dynamic-load-balancing.md) - DLB options
