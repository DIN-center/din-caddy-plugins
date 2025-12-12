# Authentication

DIN Caddy Plugins supports two authentication mechanisms for connecting to protected provider endpoints: SIWE (Sign-In with Ethereum) and OIDC (OAuth2/OpenID Connect).

## IAuthClient Interface

**File**: `lib/auth/interface.go`

All authentication implementations conform to this interface:

```go
type IAuthClient interface {
    // Start establishes sessions with the provider
    Start() error

    // Error returns the client's error state
    Error() error

    // GetToken retrieves the current auth token
    GetToken() (*AuthToken, error)

    // Sign adds authentication to an HTTP request
    Sign(r *http.Request) error

    // Stop cleans up the client
    Stop()
}
```

### AuthToken Structure

```go
type AuthToken struct {
    // Headers to include in requests
    Headers map[string]string `json:"headers,omitempty"`

    // Token expiration time
    ExpiresAt UnixTime `json:"expires_at,omitempty"`

    // Maximum number of uses
    MaxUses int `json:"max_uses,omitempty"`

    // Current usage count
    Uses int `json:"uses,omitempty"`
}
```

---

## SIWE (Sign-In with Ethereum)

SIWE provides cryptographic authentication using Ethereum wallets. It's the primary authentication mechanism for DIN.

### Architecture

```
┌──────────────────┐     1. SIWE Message      ┌──────────────────┐
│                  │ ────────────────────────▶│                  │
│   DIN Router     │                          │    Provider      │
│   (SIWE Client)  │     2. JWT Token         │   (SIWE Server)  │
│                  │ ◀────────────────────────│                  │
└──────────────────┘                          └──────────────────┘
         │
         │ 3. Signed Request (JWT in header)
         ▼
┌──────────────────────────────────────────────────────────────────┐
│                        Provider Endpoint                          │
└──────────────────────────────────────────────────────────────────┘
```

### SIWE Client

**File**: `lib/auth/siwe/client.go` (304 lines)

The SIWE client manages authentication sessions with provider endpoints.

#### Configuration

```go
type SIWEClient struct {
    // Ethereum private key (hex string)
    PrivateKey string `json:"private_key,omitempty"`

    // Authentication endpoint path
    AuthEndpoint string `json:"auth_endpoint,omitempty"`

    // Session pool size
    PoolSize int `json:"pool_size,omitempty"`

    // Session expiration buffer (seconds)
    ExpirationBuffer int `json:"expiration_buffer,omitempty"`
}
```

#### Default Values

| Parameter | Default | Description |
|-----------|---------|-------------|
| `auth_endpoint` | `/auth` | Path for SIWE authentication |
| `pool_size` | 5 | Number of concurrent sessions |
| `expiration_buffer` | 60 | Renew sessions 60s before expiry |

#### Session Lifecycle

1. **Initialization**: Client starts with configured pool size
2. **Session Creation**: Creates SIWE message, signs with private key
3. **Token Exchange**: Sends signed message to provider's auth endpoint
4. **Token Storage**: Stores JWT tokens in session pool
5. **Auto-Renewal**: Renews sessions before expiration
6. **Request Signing**: Attaches JWT to outgoing requests

#### SIWE Message Format

```
example.com wants you to sign in with your Ethereum account:
0x1234...abcd

Sign in to access protected resources

URI: https://example.com/auth
Version: 1
Chain ID: 1
Nonce: abc123
Issued At: 2024-01-01T00:00:00Z
Expiration Time: 2024-01-01T01:00:00Z
```

#### Request Signing

```go
func (c *SIWEClient) Sign(r *http.Request) error {
    token, err := c.GetToken()
    if err != nil {
        return err
    }

    for key, value := range token.Headers {
        r.Header.Set(key, value)
    }
    return nil
}
```

### SIWE Server

**File**: `lib/auth/siwe/server.go` (210 lines)

The SIWE server provides authentication middleware for providers.

#### Caddy Module Registration

```go
func (SIWEAuthServer) CaddyModule() caddy.ModuleInfo {
    return caddy.ModuleInfo{
        ID:  "http.handlers.din_auth",
        New: func() caddy.Module { return new(SIWEAuthServer) },
    }
}
```

#### Configuration

```go
type SIWEAuthServer struct {
    // JWT signing secret
    Secret string `json:"secret,omitempty"`

    // Token validity duration (seconds)
    TokenDuration int `json:"token_duration,omitempty"`

    // Maximum token uses
    MaxTokenUses int `json:"max_token_uses,omitempty"`

    // Allowed wallet addresses (optional)
    AllowedAddresses []string `json:"allowed_addresses,omitempty"`
}
```

#### Authentication Flow

```
1. Client sends POST /auth with SIWE message + signature
2. Server validates signature against message
3. Server verifies wallet address is allowed (if configured)
4. Server issues JWT token with claims
5. Client includes JWT in subsequent requests
6. Server validates JWT on each request
```

#### JWT Token Structure

```json
{
    "sub": "0x1234...abcd",
    "iat": 1704067200,
    "exp": 1704070800,
    "nonce": "abc123",
    "uses": 0,
    "max_uses": 100
}
```

#### Middleware Behavior

- **Pass-through for /auth**: Auth endpoint doesn't require authentication
- **JWT Validation**: All other requests require valid JWT
- **Usage Tracking**: Tracks token usage against max_uses
- **Expiration Check**: Rejects expired tokens

---

## OIDC (OAuth2/OpenID Connect)

**File**: `lib/auth/oidc/client.go`

OIDC provides OAuth2-based authentication for providers that use standard identity providers.

### Configuration

```go
type Client struct {
    // OAuth2 client ID
    ClientID string `json:"client_id,omitempty"`

    // OAuth2 client secret
    ClientSecret string `json:"client_secret,omitempty"`

    // Token endpoint URL
    TokenURL string `json:"token_url,omitempty"`

    // Additional scopes
    Scopes []string `json:"scopes,omitempty"`
}
```

### Authentication Flow

```
1. Client requests token from OIDC provider (client credentials flow)
2. OIDC provider returns access token
3. Client includes access token in Authorization header
4. Provider validates token with OIDC provider
```

### Token Management

- Automatic token refresh before expiration
- Caches tokens to minimize auth requests
- Supports multiple concurrent requests

---

## Caddyfile Configuration

### SIWE Client (Router Side)

```caddyfile
din {
    services {
        ethereum-mainnet {
            providers {
                https://protected.example.com {
                    auth siwe {
                        private_key {$SIWE_PRIVATE_KEY}
                        auth_endpoint /auth
                        pool_size 5
                        expiration_buffer 60
                    }
                }
            }
        }
    }
}
```

### SIWE Server (Provider Side)

```caddyfile
:8080 {
    route /auth {
        din_auth {
            secret {$AUTH_SECRET}
            token_duration 3600
            max_token_uses 100
        }
    }

    route /* {
        din_auth {
            secret {$AUTH_SECRET}
        }
        reverse_proxy upstream:8545
    }
}
```

### OIDC Client

```caddyfile
din {
    services {
        ethereum-mainnet {
            providers {
                https://oauth-protected.example.com {
                    oidc {
                        client_id {$OIDC_CLIENT_ID}
                        client_secret {$OIDC_CLIENT_SECRET}
                        token_url https://auth.example.com/oauth/token
                        scopes api:read api:write
                    }
                }
            }
        }
    }
}
```

---

## Authentication Selection

Providers can use either SIWE or OIDC (mutually exclusive):

```go
func (p *provider) AuthClient() auth.IAuthClient {
    if p.Auth != nil {
        return p.Auth  // SIWE
    }
    if p.OIDCClient != nil {
        return p.OIDCClient  // OIDC
    }
    return nil  // No auth
}
```

---

## Security Considerations

### Private Key Management

- **Never commit private keys to version control**
- Use environment variables: `{$SIWE_PRIVATE_KEY}`
- Use secret management systems in production
- Rotate keys periodically

### JWT Security

- Use strong secrets (32+ bytes)
- Set reasonable token durations
- Limit max token uses
- Use HTTPS for all auth endpoints

### OIDC Security

- Store client secrets securely
- Use minimal required scopes
- Validate tokens on every request

---

## SIWE Token CLI Tool

**File**: `cmd/siwe-token/main.go`

Generates SIWE tokens for testing:

```bash
# Generate a SIWE token
go run cmd/siwe-token/main.go \
    --private-key 0x... \
    --domain example.com \
    --uri https://example.com/auth
```

---

## Error Handling

### Common Auth Errors

| Error | Cause | Solution |
|-------|-------|----------|
| `token expired` | JWT past expiration | Client auto-renews |
| `invalid signature` | Wrong private key | Check key configuration |
| `address not allowed` | Wallet not in allowlist | Add address to server config |
| `max uses exceeded` | Token usage limit hit | Get new token |

### Client Recovery

The SIWE client automatically:
- Retries failed auth requests
- Renews tokens before expiration
- Maintains session pool health

---

## Related Documentation

- [Network Configuration](./03-network-configuration.md) - Provider setup
- [Caddyfile Configuration](./11-caddyfile-configuration.md) - Full config reference
- [Request Flow](./09-request-flow.md) - How auth is applied
