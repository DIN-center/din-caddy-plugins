# x402 v2 Integration

## Overview

x402 v2 introduces session-based payments that complement the DIN subscription model. This document covers how x402 v2 integrates with DIN Payments.

## x402 v2 Features

### Key Improvements Over v1

| Feature | v1 | v2 |
|---------|-----|-----|
| Payment per request | Yes | Optional |
| Sessions | No | Yes |
| Subscription support | No | Native |
| Auth integration | Separate | Built-in |

### Session Model

```
┌──────────────┐                              ┌──────────────┐
│   Client     │                              │   Server     │
└──────┬───────┘                              └──────┬───────┘
       │                                             │
       │  1. Initial request                         │
       │─────────────────────────────────────────────▶
       │                                             │
       │  2. 402 + X-PAYMENT-REQUIRED               │
       │     (includes session option)               │
       │◀─────────────────────────────────────────────
       │                                             │
       │  3. Pay for session (single payment)       │
       │─────────────────────────────────────────────▶
       │                                             │
       │  4. Session token                           │
       │◀─────────────────────────────────────────────
       │                                             │
       │  5. Subsequent requests + session token     │
       │     (no additional payment)                 │
       │─────────────────────────────────────────────▶
       │                                             │
       │  6. Response (no 402)                       │
       │◀─────────────────────────────────────────────
```

## Two-Layer Architecture

x402 v2 operates at the **payment layer**, while DIN Marketplace operates at the **business layer**:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Business Layer                                     │
│                    (DIN Marketplace Contract)                                │
│                                                                              │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │  Agreement Types                                                     │   │
│   │  - Unlimited (monthly subscription)                                  │   │
│   │  - Request Count (prepaid requests)                                  │   │
│   │  - Credit-Based (method pricing)                                     │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│   Handles: Pricing, terms, usage tracking, provider payouts                  │
└─────────────────────────────────────────────────────────────────────────────┘
                              │
                              │ Agreement funds subscription
                              ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Payment Layer                                      │
│                         (x402 v2 Sessions)                                   │
│                                                                              │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │  Session Management                                                  │   │
│   │  - Initial payment creates session                                   │   │
│   │  - Session token for subsequent requests                             │   │
│   │  - Session renewal on expiry                                         │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│   Handles: Payment signing, session auth, renewal                            │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Integration Patterns

### Pattern 1: Agreement-Funded Sessions

Consumer creates DIN agreement, which pre-funds x402 sessions:

```typescript
// 1. Consumer creates marketplace agreement
const agreement = await marketplace.subscribe({
  planId: plan.id,
  quantity: 1,  // 1 month unlimited
});

// 2. Agreement creates x402 session automatically
//    Provider issues session token for agreement duration

// 3. Subsequent requests use session token
const response = await din.request('ethereum-mainnet', {
  body: { jsonrpc: '2.0', method: 'eth_blockNumber', params: [], id: 1 },
  // Session token automatically included
});
```

### Pattern 2: Direct x402 Micropayments

For consumers without agreements, standard x402 v2 flow:

```typescript
// No agreement - x402 handles payment
const response = await din.request('ethereum-mainnet', {
  body: { jsonrpc: '2.0', method: 'eth_blockNumber', params: [], id: 1 },
});

// x402 interceptor:
// 1. Gets 402 response
// 2. Signs payment
// 3. Gets session token (for v2)
// 4. Retries with payment/session
// 5. Returns response

console.log(response.paymentInfo);  // { amount: '...', txHash: '...' }
```

## Provider Implementation

### Session Types

Providers can issue different session types based on consumer status:

```go
type SessionType string

const (
    // Agreement-backed session (from marketplace)
    SessionTypeAgreement SessionType = "agreement"

    // Micropayment session (from x402)
    SessionTypeMicropayment SessionType = "micropayment"
)

type Session struct {
    ID          string
    Type        SessionType
    ConsumerAddr string
    AgreementID *uint64      // Present for agreement sessions
    ExpiresAt   time.Time
    MaxRequests *uint64      // For usage-limited sessions
    UsedRequests uint64
}
```

### Session Verification

```go
type SessionVerifier struct {
    agreementCache *AgreementCache
    x402Verifier   *X402Verifier
}

func (v *SessionVerifier) Verify(r *http.Request) (*Session, error) {
    // Check for x402 v2 session token
    sessionToken := r.Header.Get("X-PAYMENT-SESSION")

    if sessionToken != "" {
        // Verify session token
        session, err := v.x402Verifier.VerifySession(sessionToken)
        if err != nil {
            return nil, err
        }

        // If agreement-backed, verify agreement still valid
        if session.Type == SessionTypeAgreement {
            if !v.agreementCache.IsValid(session.AgreementID) {
                return nil, ErrAgreementInvalid
            }
        }

        return session, nil
    }

    // No session - check for agreement directly
    consumerAddr := v.extractConsumer(r)
    network := v.extractNetwork(r)

    agreement, exists := v.agreementCache.Get(consumerAddr, network)
    if exists && v.isValid(agreement) {
        // Issue new session for agreement
        return v.issueAgreementSession(agreement)
    }

    // No agreement - require x402 payment
    return nil, Err402Required
}
```

### 402 Response with Session Option

```go
func (h *X402Handler) Write402Response(w http.ResponseWriter, r *http.Request) {
    requirements := X402Requirements{
        Scheme:            "exact",
        Network:           "linea",
        MaxAmountRequired: h.getPrice(r).String(),
        Resource:          r.URL.Path,
        PayTo:             h.paymentAddr,
        MaxTimeoutSeconds: 300,

        // v2 session options
        SessionOptions: &SessionOptions{
            Enabled:        true,
            MaxDuration:    3600,           // 1 hour
            MaxRequests:    1000,           // Or unlimited
            RenewalAllowed: true,
        },
    }

    w.Header().Set("X-PAYMENT-REQUIRED", requirements.Encode())
    w.WriteHeader(http.StatusPaymentRequired)
}
```

## SDK Implementation

### Session Management

```typescript
class X402V2Client {
  private sessions: Map<string, Session> = new Map();

  async makeRequest(url: string, options: RequestOptions): Promise<Response> {
    // Check for existing session
    const session = this.sessions.get(this.getSessionKey(url));

    if (session && !session.isExpired()) {
      // Use existing session
      return this.requestWithSession(url, options, session);
    }

    // No valid session - make payment to get one
    const response = await this.axios.request({ url, ...options });

    if (response.status === 402) {
      // Get session through payment
      const newSession = await this.handlePaymentRequired(response);
      this.sessions.set(this.getSessionKey(url), newSession);

      // Retry with session
      return this.requestWithSession(url, options, newSession);
    }

    return response;
  }

  private async requestWithSession(
    url: string,
    options: RequestOptions,
    session: Session
  ): Promise<Response> {
    return this.axios.request({
      url,
      ...options,
      headers: {
        ...options.headers,
        'X-PAYMENT-SESSION': session.token,
      },
    });
  }
}
```

### Agreement + Session Integration

```typescript
class DinClient {
  private marketplace: MarketplaceClient;
  private x402Client: X402V2Client;
  private agreements: Map<string, Agreement> = new Map();

  async request(network: string, options: RequestOptions): Promise<DinResponse> {
    // Check for marketplace agreement first
    const agreement = await this.getActiveAgreement(network);

    if (agreement) {
      // Agreement exists - may already have session
      const provider = this.selectProvider(network, agreement.provider);

      try {
        // Try with agreement's session
        return await this.requestWithAgreement(provider, options, agreement);
      } catch (err) {
        if (err.code === 'SESSION_EXPIRED') {
          // Session expired but agreement valid - get new session
          const session = await this.renewAgreementSession(agreement, provider);
          return await this.requestWithSession(provider, options, session);
        }
        throw err;
      }
    }

    // No agreement - use x402 micropayments
    return this.x402Client.makeRequest(provider.url, options);
  }

  private async renewAgreementSession(
    agreement: Agreement,
    provider: Provider
  ): Promise<Session> {
    // Call provider's session endpoint with agreement proof
    const response = await this.axios.post(provider.sessionUrl, {
      agreementId: agreement.id.toString(),
      signature: await this.signAgreementProof(agreement),
    });

    return response.data.session;
  }
}
```

## Session vs SIWE

### Recommendation: Use x402 v2 Sessions

For consumers who pay (micropayments or subscriptions), x402 v2 sessions replace SIWE:

| Aspect | SIWE | x402 v2 Session |
|--------|------|-----------------|
| Auth | Wallet signature | Payment receipt |
| Session mgmt | Manual | Built-in |
| Payment | Separate | Unified |
| Complexity | Higher | Lower |

### When SIWE is Still Useful

- **Free tier access** - Consumers who don't pay but need auth
- **Provider dashboard** - Web UI authentication
- **Admin operations** - Non-payment API access

### Hybrid Support

```go
type AuthHandler struct {
    x402Verifier  *X402V2Verifier
    siweVerifier  *SIWEVerifier
}

func (h *AuthHandler) Verify(r *http.Request) (*AuthResult, error) {
    // Check for x402 session first (paying users)
    if session := r.Header.Get("X-PAYMENT-SESSION"); session != "" {
        return h.x402Verifier.Verify(session)
    }

    // Check for SIWE JWT (non-paying authenticated users)
    if jwt := r.Header.Get("Authorization"); jwt != "" {
        return h.siweVerifier.Verify(jwt)
    }

    // No auth - return 402 for payment
    return nil, Err402Required
}
```

## Migration Path

### From x402 v1 to v2

```typescript
// v1: Payment every request
const response = await x402V1Client.request(url, options);
// Each request: request -> 402 -> sign -> retry -> response

// v2: Session-based
const response = await x402V2Client.request(url, options);
// First request: request -> 402 -> sign -> session -> response
// Subsequent: request + session -> response (no 402)
```

### Provider Migration

1. Add session support to 402 handler
2. Accept both payment and session headers
3. Issue sessions on valid payment
4. Verify sessions on subsequent requests

### Consumer Migration

1. Update x402 client to v2
2. Store and reuse sessions
3. Handle session renewal
4. Fall back to payment on session expiry

## Performance Benefits

### Latency Reduction

| Scenario | v1 Latency | v2 Latency |
|----------|------------|------------|
| First request | ~500ms (402 round-trip) | ~500ms |
| Subsequent | ~500ms (402 round-trip) | ~100ms (direct) |
| With agreement | N/A | ~100ms (direct) |

### Cost Reduction

- Fewer payment transactions
- Lower gas costs (batched)
- Reduced signature operations
