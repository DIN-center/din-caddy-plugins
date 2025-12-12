# Authentication Flows

## Overview

DIN supports multiple authentication patterns depending on the consumer's payment model. This document covers all auth flows and when to use each.

## Auth Pattern Decision Tree

```
                    ┌─────────────────────────┐
                    │   Incoming Request      │
                    └───────────┬─────────────┘
                                │
                    ┌───────────▼─────────────┐
                    │  Has X-PAYMENT-SESSION? │
                    └───────────┬─────────────┘
                                │
                ┌───────────────┼───────────────┐
                │ Yes           │               │ No
                ▼               │               ▼
    ┌───────────────────┐      │    ┌───────────────────┐
    │  Verify Session   │      │    │  Has X-DIN-AGREEMENT? │
    └─────────┬─────────┘      │    └─────────┬─────────┘
              │                │              │
              │ Valid          │    ┌─────────┼─────────┐
              ▼                │    │ Yes     │         │ No
    ┌───────────────────┐      │    ▼         │         ▼
    │   Serve Request   │      │  Verify     │    ┌───────────────┐
    └───────────────────┘      │  Agreement  │    │  Has JWT?     │
                               │    │        │    └───────┬───────┘
                               │    │ Valid  │            │
                               │    ▼        │    ┌───────┼───────┐
                               │  Issue     │    │ Yes   │       │ No
                               │  Session    │    ▼       │       ▼
                               │    │        │  Verify   │    Return 402
                               │    ▼        │  SIWE JWT │
                               │  Serve     │    │       │
                               │  Request    │    │ Valid │
                               │             │    ▼       │
                               │             │  Serve    │
                               │             │  (free)   │
                               └─────────────┴───────────┘
```

## Auth Flow 1: x402 v2 Session (Recommended)

For paying users (micropayments or subscriptions), x402 v2 sessions provide unified auth + payment.

### Flow Diagram

```
Consumer                           Provider
   │                                  │
   │  1. Request (no auth)            │
   │─────────────────────────────────▶│
   │                                  │
   │  2. 402 + X-PAYMENT-REQUIRED     │
   │◀─────────────────────────────────│
   │                                  │
   │  3. Sign payment (USDC)          │
   │                                  │
   │  4. Request + X-PAYMENT          │
   │─────────────────────────────────▶│
   │                                  │
   │  5. 200 + X-PAYMENT-SESSION      │
   │◀─────────────────────────────────│
   │                                  │
   │  6. Request + X-PAYMENT-SESSION  │
   │─────────────────────────────────▶│
   │                                  │
   │  7. 200 (no 402)                 │
   │◀─────────────────────────────────│
```

### Implementation

```typescript
// Consumer side
class X402SessionAuth {
  private session: Session | null = null;

  async makeRequest(url: string, options: RequestOptions): Promise<Response> {
    // If we have a valid session, use it
    if (this.session && !this.session.isExpired()) {
      const response = await this.requestWithSession(url, options);
      if (response.status !== 401) {
        return response;
      }
      // Session invalid - clear and retry
      this.session = null;
    }

    // No session - start payment flow
    const response = await fetch(url, options);

    if (response.status === 402) {
      // Get payment requirements
      const requirements = parsePaymentRequired(
        response.headers.get('X-PAYMENT-REQUIRED')
      );

      // Sign payment
      const payment = await this.signPayment(requirements);

      // Retry with payment
      const paymentResponse = await fetch(url, {
        ...options,
        headers: {
          ...options.headers,
          'X-PAYMENT': encodePayment(payment),
        },
      });

      // Extract session for future requests
      const sessionToken = paymentResponse.headers.get('X-PAYMENT-SESSION');
      if (sessionToken) {
        this.session = decodeSession(sessionToken);
      }

      return paymentResponse;
    }

    return response;
  }

  private async requestWithSession(url: string, options: RequestOptions) {
    return fetch(url, {
      ...options,
      headers: {
        ...options.headers,
        'X-PAYMENT-SESSION': this.session!.token,
      },
    });
  }
}
```

```go
// Provider side
func (h *X402Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Check for session
    sessionToken := r.Header.Get("X-PAYMENT-SESSION")
    if sessionToken != "" {
        session, err := h.verifySession(sessionToken)
        if err == nil && session.IsValid() {
            h.next.ServeHTTP(w, r)
            return
        }
    }

    // Check for payment
    payment := r.Header.Get("X-PAYMENT")
    if payment != "" {
        verified, err := h.verifyPayment(payment)
        if err == nil && verified {
            // Issue session token
            session := h.issueSession(r)
            w.Header().Set("X-PAYMENT-SESSION", session.Encode())
            h.next.ServeHTTP(w, r)
            return
        }
    }

    // No valid auth - return 402
    h.write402(w, r)
}
```

## Auth Flow 2: Agreement-Backed Session

For consumers with marketplace agreements, the agreement pre-authorizes access.

### Flow Diagram

```
Consumer                  Marketplace              Provider
   │                         │                        │
   │  1. Create agreement    │                        │
   │  (Pay USDC upfront)     │                        │
   │────────────────────────▶│                        │
   │                         │                        │
   │  2. Agreement created   │                        │
   │◀────────────────────────│                        │
   │                         │                        │
   │  3. Request + X-DIN-AGREEMENT                    │
   │─────────────────────────────────────────────────▶│
   │                         │                        │
   │                         │  4. Verify agreement   │
   │                         │◀───────────────────────│
   │                         │                        │
   │                         │  5. Agreement valid    │
   │                         │───────────────────────▶│
   │                         │                        │
   │  6. 200 + X-PAYMENT-SESSION                      │
   │◀─────────────────────────────────────────────────│
   │                         │                        │
   │  7. Subsequent: Use session                      │
   │─────────────────────────────────────────────────▶│
```

### Implementation

```typescript
// Consumer side
class AgreementAuth {
  private marketplace: MarketplaceClient;
  private session: Session | null = null;

  async makeRequest(
    network: string,
    options: RequestOptions
  ): Promise<Response> {
    // Check for existing session
    if (this.session && !this.session.isExpired()) {
      return this.requestWithSession(options);
    }

    // Get active agreement
    const agreement = await this.marketplace.getActiveAgreement(network);
    if (!agreement) {
      throw new Error('No active agreement');
    }

    // Request with agreement proof
    const response = await fetch(options.url, {
      ...options,
      headers: {
        ...options.headers,
        'X-DIN-AGREEMENT': agreement.id.toString(),
        'X-DIN-SIGNATURE': await this.signAgreementProof(agreement),
      },
    });

    // Extract session for future requests
    const sessionToken = response.headers.get('X-PAYMENT-SESSION');
    if (sessionToken) {
      this.session = decodeSession(sessionToken);
    }

    return response;
  }

  private async signAgreementProof(agreement: Agreement): Promise<string> {
    const message = `DIN Agreement Access\nAgreement: ${agreement.id}\nTimestamp: ${Date.now()}`;
    return this.wallet.signMessage(message);
  }
}
```

```go
// Provider side
func (h *AgreementHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Check for session first (fast path)
    sessionToken := r.Header.Get("X-PAYMENT-SESSION")
    if sessionToken != "" {
        session, err := h.verifySession(sessionToken)
        if err == nil && session.IsValid() {
            h.next.ServeHTTP(w, r)
            h.recordUsage(r.Context(), session)
            return
        }
    }

    // Check for agreement
    agreementId := r.Header.Get("X-DIN-AGREEMENT")
    signature := r.Header.Get("X-DIN-SIGNATURE")

    if agreementId != "" && signature != "" {
        agreement, err := h.verifyAgreement(agreementId, signature)
        if err == nil && agreement.IsValid() {
            // Issue session for future requests
            session := h.issueAgreementSession(agreement)
            w.Header().Set("X-PAYMENT-SESSION", session.Encode())

            h.next.ServeHTTP(w, r)
            h.recordUsage(r.Context(), agreement)
            return
        }
    }

    // Fall back to x402
    h.x402Handler.ServeHTTP(w, r)
}
```

## Auth Flow 3: SIWE (Legacy/Free Tier)

For users who need authentication without payment (free tier, admin access).

### Flow Diagram

```
Consumer                           Provider
   │                                  │
   │  1. GET /auth/nonce              │
   │─────────────────────────────────▶│
   │                                  │
   │  2. Nonce                        │
   │◀─────────────────────────────────│
   │                                  │
   │  3. Sign SIWE message            │
   │                                  │
   │  4. POST /auth + SIWE message    │
   │─────────────────────────────────▶│
   │                                  │
   │  5. JWT token                    │
   │◀─────────────────────────────────│
   │                                  │
   │  6. Request + Authorization JWT  │
   │─────────────────────────────────▶│
   │                                  │
   │  7. 200                          │
   │◀─────────────────────────────────│
```

### Implementation

```typescript
// Consumer side
class SIWEAuth {
  private jwt: string | null = null;
  private jwtExpiry: Date | null = null;

  async authenticate(providerUrl: string): Promise<void> {
    // Get nonce
    const nonceResponse = await fetch(`${providerUrl}/auth/nonce`);
    const { nonce } = await nonceResponse.json();

    // Create SIWE message
    const message = new SiweMessage({
      domain: new URL(providerUrl).host,
      address: this.walletAddress,
      statement: 'Sign in to DIN Provider',
      uri: `${providerUrl}/auth`,
      version: '1',
      chainId: 59144,  // Linea
      nonce,
    });

    // Sign message
    const signature = await this.wallet.signMessage(message.prepareMessage());

    // Exchange for JWT
    const authResponse = await fetch(`${providerUrl}/auth`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message: message.prepareMessage(), signature }),
    });

    const { token, expiresAt } = await authResponse.json();
    this.jwt = token;
    this.jwtExpiry = new Date(expiresAt);
  }

  async makeRequest(url: string, options: RequestOptions): Promise<Response> {
    if (!this.jwt || this.isExpired()) {
      await this.authenticate(new URL(url).origin);
    }

    return fetch(url, {
      ...options,
      headers: {
        ...options.headers,
        'Authorization': `Bearer ${this.jwt}`,
      },
    });
  }
}
```

## Auth Priority Order

Providers should check auth methods in this order:

```go
func (h *AuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // 1. x402 Session (fastest, most common for paying users)
    if session := h.checkX402Session(r); session != nil {
        h.serveWithSession(w, r, session)
        return
    }

    // 2. Agreement proof (for first request after agreement creation)
    if agreement := h.checkAgreement(r); agreement != nil {
        h.serveWithAgreement(w, r, agreement)
        return
    }

    // 3. x402 Payment (for micropayments)
    if payment := h.checkX402Payment(r); payment != nil {
        h.serveWithPayment(w, r, payment)
        return
    }

    // 4. SIWE JWT (for free tier / legacy)
    if jwt := h.checkSIWE(r); jwt != nil {
        h.serveWithJWT(w, r, jwt)
        return
    }

    // 5. No auth - return 402
    h.write402(w, r)
}
```

## Session Token Format

### JWT Structure

```json
{
  "header": {
    "alg": "ES256",
    "typ": "JWT"
  },
  "payload": {
    "sub": "0xConsumerAddress",
    "iss": "0xProviderAddress",
    "iat": 1704067200,
    "exp": 1704153600,
    "type": "x402-session",
    "agreement_id": "123",
    "network": "ethereum-mainnet",
    "max_requests": null,
    "used_requests": 0
  }
}
```

### Token Verification

```go
type SessionClaims struct {
    jwt.StandardClaims
    Type         string  `json:"type"`
    AgreementID  *uint64 `json:"agreement_id,omitempty"`
    Network      string  `json:"network"`
    MaxRequests  *uint64 `json:"max_requests,omitempty"`
    UsedRequests uint64  `json:"used_requests"`
}

func (h *AuthHandler) verifySession(tokenString string) (*Session, error) {
    token, err := jwt.ParseWithClaims(tokenString, &SessionClaims{}, func(token *jwt.Token) (interface{}, error) {
        return h.publicKey, nil
    })

    if err != nil {
        return nil, err
    }

    claims, ok := token.Claims.(*SessionClaims)
    if !ok || !token.Valid {
        return nil, ErrInvalidToken
    }

    // Check expiration
    if claims.ExpiresAt < time.Now().Unix() {
        return nil, ErrTokenExpired
    }

    // Check request limit (if applicable)
    if claims.MaxRequests != nil && claims.UsedRequests >= *claims.MaxRequests {
        return nil, ErrSessionExhausted
    }

    return &Session{
        Consumer:     claims.Subject,
        AgreementID:  claims.AgreementID,
        Network:      claims.Network,
        ExpiresAt:    time.Unix(claims.ExpiresAt, 0),
        MaxRequests:  claims.MaxRequests,
        UsedRequests: claims.UsedRequests,
    }, nil
}
```

## Error Responses

### 402 Payment Required

```http
HTTP/1.1 402 Payment Required
X-PAYMENT-REQUIRED: {
  "scheme": "exact",
  "network": "linea",
  "maxAmountRequired": "1000000000000000",
  "payTo": "0xProviderAddress",
  "sessionOptions": {
    "enabled": true,
    "maxDuration": 3600
  }
}
```

### 401 Session Invalid

```http
HTTP/1.1 401 Unauthorized
X-DIN-Error: session_expired
Content-Type: application/json

{
  "error": "Session expired",
  "code": "SESSION_EXPIRED",
  "action": "Re-authenticate or make new payment"
}
```

### 403 Agreement Invalid

```http
HTTP/1.1 403 Forbidden
X-DIN-Error: agreement_invalid
Content-Type: application/json

{
  "error": "Agreement not valid",
  "code": "AGREEMENT_INVALID",
  "reason": "expired",
  "action": "Create new agreement"
}
```

## Migration Guide

### From SIWE-only to x402 Sessions

1. Keep SIWE for backward compatibility
2. Add x402 session support
3. Return session token on successful payment
4. Accept both JWT and session headers
5. Deprecate SIWE for paying users over time

### Configuration

```caddyfile
:8080 {
    route /* {
        # New: x402 v2 session auth (recommended)
        din_x402_v2 {
            payment_address 0x...
            session_duration 3600
            session_max_requests 0  # unlimited
        }

        # New: Agreement auth
        din_agreement {
            marketplace_contract 0x...
            cache_ttl 30s
        }

        # Legacy: SIWE auth (optional, for free tier)
        din_siwe {
            secret {$AUTH_SECRET}
            allow_free_tier true
        }

        reverse_proxy localhost:8545
    }
}
```
