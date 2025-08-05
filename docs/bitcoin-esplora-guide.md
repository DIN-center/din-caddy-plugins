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

### Configuration with OAuth2

For APIs requiring OAuth2 authentication (like Blockstream Enterprise):

```caddyfile
:8000 {
    route /* {
        din {
            networks {
                bitcoin-esplora-mainnet {
                    type bitcoin-esplora
                    chain_id bitcoin:mainnet
                    
                    # OAuth2 configuration
                    custom_config {
                        oauth2_client_id "your-client-id"
                        oauth2_client_secret "your-client-secret"
                        oauth2_token_url "https://auth.provider.com/oauth/token"
                        oauth2_refresh_interval 180  # seconds (optional, default: 240)
                    }
                    
                    providers {
                        https://enterprise.blockstream.info {
                            priority 0
                            auth_type oauth2  # Enable OAuth2 for this provider
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

#### OAuth2 Parameters (in custom_config)

- `oauth2_client_id`: OAuth2 client identifier
- `oauth2_client_secret`: OAuth2 client secret
- `oauth2_token_url`: Token endpoint URL for obtaining access tokens
- `oauth2_refresh_interval`: Token refresh interval in seconds (optional, default: 240)
  - Should be set lower than the token's actual expiration time
  - Recommended: Set to 60-80% of token lifetime

#### Provider Parameters

- `auth_type oauth2`: Enable OAuth2 authentication for the provider
- Standard provider options (priority, headers, etc.) are also supported

## Supported Endpoints

The handler supports standard Esplora API endpoints with **GET requests only**:

### Block Endpoints
- `/api/blocks/tip/height` - Latest block height (used for health checks)
- `/api/blocks/tip/hash` - Latest block hash
- `/api/block/{hash}` - Block details by hash
- `/api/block-height/{height}` - Block hash at specific height

### Transaction Endpoints
- `/api/tx/{txid}` - Transaction details (GET only)

### Address Endpoints
- `/api/address/{address}` - Address information
- `/api/address/{address}/txs` - Address transactions
- `/api/address/{address}/utxo` - Unspent outputs

### Other Endpoints
- `/api/mempool` - Mempool statistics
- `/api/fee-estimates` - Fee estimates

### HTTP Method Restrictions
- **POST requests are blocked**: All POST requests return `405 Method Not Allowed`
- **Transaction broadcasting is not supported**: The `/api/tx` POST endpoint is blocked
- Only GET requests are allowed for all endpoints

## OAuth2 Authentication Flow

When OAuth2 is configured:

1. **Initial Token Request**: On startup, the handler requests an access token using the client credentials grant:
   ```
   POST /oauth/token
   Content-Type: application/x-www-form-urlencoded
   
   client_id=<client_id>&client_secret=<client_secret>&grant_type=client_credentials&scope=openid
   ```

2. **Automatic Refresh**: A background goroutine refreshes the token at the configured interval (minus 30 seconds buffer)

3. **Request Authentication**: Every request to the provider automatically includes:
   ```
   Authorization: Bearer <current_access_token>
   ```

4. **Error Handling**: If token refresh fails, the error is logged and subsequent requests will fail until successful refresh

## Request Routing

The handler uses REST API routing with automatic path stripping:

- Client request: `https://localhost:8000/bitcoin-esplora-mainnet/api/blocks/tip/hash`
- After path stripping: `/api/blocks/tip/hash`
- Provider request: `https://enterprise.blockstream.info/api/blocks/tip/hash`

## Health Checks

The handler uses the `/api/blocks/tip/height` endpoint for health checks:

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
                    
                    custom_config {
                        oauth2_client_id "***REMOVED***"
                        oauth2_client_secret "***REMOVED***"
                        oauth2_token_url "https://login.blockstream.com/realms/blockstream-public/protocol/openid-connect/token"
                        oauth2_refresh_interval 180
                    }
                    
                    providers {
                        # Primary provider with OAuth2
                        https://enterprise.blockstream.info {
                            priority 0
                            auth_type oauth2
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

### OAuth2 Logs

Monitor OAuth2 token rotation:
- Success: `"OAuth2 token refreshed successfully"`
- Failure: `"failed to refresh OAuth2 token"`

### Common Issues

1. **Token Refresh Failures**
   - Check network connectivity to token endpoint
   - Verify client credentials are correct
   - Ensure token endpoint URL is correct
   - Check logs for: `"failed to refresh OAuth2 token"`

2. **Authentication Errors (401 Unauthorized)**
   - Check logs for token refresh errors
   - Verify refresh interval is less than token lifetime
   - Ensure provider has `auth_type oauth2` set
   - Verify OAuth2 configuration in `custom_config` block
   - Check that all required OAuth2 fields are present:
     - `oauth2_client_id`
     - `oauth2_client_secret`
     - `oauth2_token_url`

3. **Invalid API Paths**
   - All paths must contain `/api/`
   - Use correct endpoint format per Esplora API specification

4. **405 Method Not Allowed Errors**
   - Only GET requests are supported
   - POST requests (including transaction broadcasting) are blocked
   - Use alternative services for transaction broadcasting

5. **Nil Pointer Errors in Health Checks**
   - Usually indicates network connectivity issues
   - Check provider URL is accessible
   - Verify network configuration

### Debug Tips

1. Check provider configuration:
   ```bash
   curl http://localhost:8000/bitcoin-esplora-mainnet/api/blocks/tip/height
   ```

2. Verify OAuth2 headers are being added (check provider logs)

3. Test without OAuth2 first to isolate authentication issues

## Security Considerations

1. **Credential Storage**: Never commit OAuth2 credentials to version control
2. **Network Security**: Ensure all endpoints use HTTPS
3. **Token Security**: OAuth2 tokens are stored in memory only
4. **Minimal Scope**: Request only necessary OAuth2 scopes (default: openid)

## API Compatibility

The handler is compatible with:
- Blockstream Esplora API
- BTCPay Server API (Esplora endpoints)
- Any Esplora-compatible Bitcoin explorer

## Performance Considerations

- OAuth2 token refresh runs in background (non-blocking)
- Minimal request overhead with direct path forwarding
- Supports connection pooling via the HTTP client
- Health checks run at configurable intervals
- No regex processing on request paths for optimal performance