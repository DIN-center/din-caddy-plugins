# Bitcoin Esplora Handler Guide

This guide provides comprehensive documentation for the Bitcoin Esplora handler in DIN Caddy plugins, including configuration, OAuth2 authentication, and implementation details.

## Overview

The Bitcoin Esplora handler enables integration with Bitcoin block explorers that implement the [Esplora REST API](https://github.com/blockstream/esplora/blob/master/API.md). This handler supports:

- REST API endpoints for Bitcoin blockchain data
- OAuth2 authentication with automatic token rotation
- Compatible with Blockstream Enterprise API and other Esplora implementations
- Full support for mainnet, testnet, and regtest networks

## Configuration

### Basic Configuration

```caddyfile
:8000 {
    route /* {
        din {
            networks {
                bitcoin-esplora-mainnet {
                    type bitcoin-esplora
                    chain_id bitcoin:mainnet
                    
                    providers {
                        https://blockstream.info {
                            priority 0
                        }
                    }
                }
            }
        }
    }
}
```

### Configuration with OIDC/OAuth2

For APIs requiring OIDC/OAuth2 authentication (like Blockstream Enterprise):

```caddyfile
:8000 {
    route /* {
        din {
            networks {
                bitcoin-esplora-mainnet {
                    type bitcoin-esplora
                    chain_id bitcoin:mainnet
                    
                    providers {
                        https://enterprise.blockstream.info {
                            priority 0
                            auth {
                                type oidc
                                url "https://auth.provider.com/oauth/token"
                                client_id "your-client-id"
                                client_secret "your-client-secret"
                                # duration_seconds 240  # Optional - if not set, uses token's expires_in
                            }
                        }
                    }
                }
            }
        }
    }
}
```

### Configuration Parameters

#### Required Parameters

- `type`: Must be `bitcoin-esplora`
- `chain_id`: Network identifier (e.g., `bitcoin:mainnet`, `bitcoin:testnet`, `bitcoin:regtest`)

#### OIDC/OAuth2 Parameters (in auth block)

- `type`: Must be `oidc` for OAuth2/OIDC authentication
- `url`: Token endpoint URL for obtaining access tokens
- `client_id`: OAuth2 client identifier
- `client_secret`: OAuth2 client secret
- `duration_seconds`: Token refresh interval in seconds (optional)
  - If not set, uses the token's `expires_in` value minus 1 minute
  - If set, must be less than the token's actual expiration time
  - Minimum refresh interval is 30 seconds

#### Provider Parameters

- Standard provider options (priority, headers, etc.) are also supported

## Supported Endpoints

The handler supports standard Esplora API endpoints with **GET requests only**:

### Block Endpoints
- `/blocks/tip/height` - Latest block height (used for health checks)
- `/blocks/tip/hash` - Latest block hash
- `/block/{hash}` - Block details by hash
- `/block-height/{height}` - Block hash at specific height

### Transaction Endpoints
- `/tx/{txid}` - Transaction details (GET only)

### Address Endpoints
- `/address/{address}` - Address information
- `/address/{address}/txs` - Address transactions
- `/address/{address}/utxo` - Unspent outputs

### Other Endpoints
- `/mempool` - Mempool statistics
- `/fee-estimates` - Fee estimates

### HTTP Method Restrictions
- **POST requests are blocked**: All POST requests return `405 Method Not Allowed`
- **Transaction broadcasting is not supported**: The `/tx` POST endpoint is blocked
- Only GET requests are allowed for all endpoints

## OIDC/OAuth2 Authentication Flow

When OIDC authentication is configured:

1. **Initial Token Request**: On startup, the handler requests an access token using the client credentials grant:
   ```
   POST /oauth/token
   Content-Type: application/x-www-form-urlencoded
   
   client_id=<client_id>&client_secret=<client_secret>&grant_type=client_credentials&scope=openid
   ```

2. **Automatic Refresh**: A background goroutine refreshes the token:
   - Default: Uses token's `expires_in` value minus 1 minute
   - Custom: Uses `duration_seconds` if configured and valid
   - Minimum refresh interval is 30 seconds

3. **Request Authentication**: Every request to the provider automatically includes:
   ```
   Authorization: Bearer <current_access_token>
   ```

4. **Error Handling**: If token refresh fails, the error is logged and subsequent requests will fail until successful refresh

## Request Routing

The handler uses REST API routing with automatic path stripping:

- Client request: `https://localhost:8000/bitcoin-esplora-mainnet/blocks/tip/hash`
- After path stripping: `/blocks/tip/hash`
- Provider request: `https://enterprise.blockstream.info/api/blocks/tip/hash`

## Health Checks

The handler uses the `/blocks/tip/height` endpoint for health checks:

- Returns the current block height
- Used to monitor provider availability
- Supports provider failover based on health status

## Implementation Details

### Handler Registration

The handler is automatically registered when the module loads:

```go
func init() {
    DefaultRegistry.RegisterHandler("bitcoin-esplora", func(config *NetworkConfig) NetworkHandler {
        return NewBitcoinEsploraHandler(config)
    })
}
```

### Path Handling

The handler passes paths directly to the provider without normalization:

- Paths are forwarded as-is to maintain exact API compatibility
- No parameter substitution or normalization is performed
- This ensures complete transparency between client requests and provider API

### Error Handling

Retryable errors include:
- HTTP 5xx server errors
- HTTP 429 rate limit errors
- Network timeout errors
- Connection errors

Non-retryable errors:
- HTTP 4xx client errors (except 429)
- Invalid request format

## Example: Complete Configuration

```caddyfile
:8000 {
    route /* {
        din {
            networks {
                bitcoin-esplora-mainnet {
                    type bitcoin-esplora
                    chain_id bitcoin:mainnet
                    
                    providers {
                        # Primary provider with OIDC/OAuth2
                        https://enterprise.blockstream.info {
                            priority 0
                            auth {
                                type oidc
                                url "https://login.blockstream.com/realms/blockstream-public/protocol/openid-connect/token"
                                client_id "client_id"
                                client_secret "secret"
                                # duration_seconds 180  # Optional
                            }
                        }
                        
                        # Backup provider without auth
                        https://blockstream.info {
                            priority 1
                        }
                    }
                }
                
                bitcoin-esplora-testnet {
                    type bitcoin-esplora
                    chain_id bitcoin:testnet
                    
                    providers {
                        https://blockstream.info/testnet {
                            priority 0
                        }
                    }
                }
            }
        }
    }
}
```

## Monitoring and Troubleshooting

### OIDC/OAuth2 Logs

Monitor OIDC token rotation:
- Success: `"OIDC token refreshed successfully"`
- Failure: `"failed to refresh OIDC token"`

### Common Issues

1. **Token Refresh Failures**
   - Check network connectivity to token endpoint
   - Verify client credentials are correct
   - Ensure token endpoint URL is correct
   - Check logs for: `"failed to refresh OAuth2 token"`

2. **Authentication Errors (401 Unauthorized)**
   - Check logs for token refresh errors
   - Verify `duration_seconds` (if set) is less than token lifetime
   - Ensure provider has `auth` block with `type oidc`
   - Check that all required OIDC fields are present in auth block:
     - `type` (must be `oidc`)
     - `url` (token endpoint)
     - `client_id`
     - `client_secret`

3. **405 Method Not Allowed Errors**
   - Only GET requests are supported
   - POST requests (including transaction broadcasting) are blocked
   - Use alternative services for transaction broadcasting

4. **Nil Pointer Errors in Health Checks**
   - Usually indicates network connectivity issues
   - Check provider URL is accessible
   - Verify network configuration

### Debug Tips

1. Check provider configuration:
   ```bash
   curl http://localhost:8000/bitcoin-esplora-mainnet/blocks/tip/height
   ```

2. Verify OAuth2 headers are being added (check provider logs)

3. Test without OAuth2 first to isolate authentication issues

## Security Considerations

1. **Credential Storage**: Never commit OIDC/OAuth2 credentials to version control
2. **Network Security**: Ensure all endpoints use HTTPS
3. **Token Security**: OIDC tokens are stored in memory only
4. **Minimal Scope**: Request only necessary OAuth2 scopes (default: openid)

## API Compatibility

The handler is compatible with:
- Blockstream Esplora API
- BTCPay Server API (Esplora endpoints)
- Any Esplora-compatible Bitcoin explorer

## Performance Considerations

- OIDC token refresh runs in background (non-blocking)
- Minimal request overhead with direct path forwarding
- Supports connection pooling via the HTTP client
- Health checks run at configurable intervals
- No regex processing on request paths for optimal performance