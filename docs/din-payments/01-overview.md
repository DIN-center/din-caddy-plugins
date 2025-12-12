# DIN Payments System Overview

## Introduction

The DIN Payments System provides a flexible subscription and prepaid payment model for accessing RPC providers on the DIN network. It sits on top of x402 micropayments, enabling consumers to establish payment agreements that eliminate per-request payment overhead.

## System Goals

1. **Zero-latency access** - Subscribed users experience no payment negotiation delay
2. **Flexible pricing** - Support multiple pricing models for different use cases
3. **Provider autonomy** - Providers set their own prices and terms
4. **Consumer choice** - Consumers choose between micropayments and subscriptions
5. **On-chain transparency** - Agreements stored on Linea for verifiability

## Architecture Layers

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Application Layer                                  │
│                                                                              │
│   ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐            │
│   │  DIN SDK        │  │  DIN Router     │  │  Provider UI    │            │
│   │  (TypeScript)   │  │  (Go/Caddy)     │  │  (Dashboard)    │            │
│   └────────┬────────┘  └────────┬────────┘  └────────┬────────┘            │
└────────────┼────────────────────┼────────────────────┼──────────────────────┘
             │                    │                    │
             ▼                    ▼                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Business Layer                                     │
│                    (DIN Marketplace Contract - Linea)                        │
│                                                                              │
│   ┌─────────────────────────────────────────────────────────────┐           │
│   │  Pricing Models                                              │           │
│   │  ┌───────────┐  ┌───────────┐  ┌───────────┐                │           │
│   │  │ Unlimited │  │ Request   │  │ Credit    │                │           │
│   │  │ (Monthly) │  │ Count     │  │ Based     │                │           │
│   │  └───────────┘  └───────────┘  └───────────┘                │           │
│   └─────────────────────────────────────────────────────────────┘           │
│                                                                              │
│   ┌─────────────────────────────────────────────────────────────┐           │
│   │  Agreement Management                                        │           │
│   │  - Create/Cancel agreements                                  │           │
│   │  - Track usage (request count, credits)                      │           │
│   │  - Escrow funds                                              │           │
│   └─────────────────────────────────────────────────────────────┘           │
└─────────────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Payment Layer                                      │
│                         (x402 v2 Sessions)                                   │
│                                                                              │
│   - Session-based authentication                                             │
│   - One-time payment per session                                             │
│   - Native subscription support                                              │
│   - USDC on Linea                                                            │
└─────────────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Blockchain Layer                                   │
│                              (Linea)                                         │
│                                                                              │
│   ┌───────────────────┐  ┌───────────────────┐  ┌───────────────────┐      │
│   │ DIN Registry      │  │ DIN Marketplace   │  │ USDC Token        │      │
│   │ (Networks/Provs)  │  │ (Agreements)      │  │ (Payments)        │      │
│   └───────────────────┘  └───────────────────┘  └───────────────────┘      │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. DIN Marketplace Contract

Smart contract on Linea that manages:
- Provider pricing plans
- Consumer agreements
- Escrow and fund management
- Usage tracking (for usage-based models)

### 2. Provider Integration

Providers need to:
- Register pricing plans in marketplace
- Verify agreements before serving requests
- Track and report usage (for usage-based)
- Cache agreement data for fast lookups

### 3. Consumer Integration

Consumers (SDK, Router) need to:
- Browse available plans
- Create agreements (pay upfront)
- Include agreement proof in requests
- Monitor usage and renewals

### 4. x402 v2 Sessions

The payment layer that handles:
- Initial agreement payments
- Session token generation
- Request authentication
- Subscription renewals

## Data Flow

### Agreement Creation

```
Consumer                    Marketplace                 Provider
   │                        Contract                       │
   │  1. getProviderPlans()     │                          │
   │───────────────────────────▶│                          │
   │                            │                          │
   │  2. plans[]                │                          │
   │◀───────────────────────────│                          │
   │                            │                          │
   │  3. createAgreement()      │                          │
   │  + USDC payment            │                          │
   │───────────────────────────▶│                          │
   │                            │                          │
   │                            │  4. AgreementCreated     │
   │                            │     event                │
   │                            │─────────────────────────▶│
   │                            │                          │
   │  5. agreementId            │                          │
   │◀───────────────────────────│                          │
```

### Request Flow (Subscribed User)

```
Consumer                     Provider                    Marketplace
   │                            │                        (Cache)
   │  1. RPC Request            │                           │
   │  + x402 session token      │                           │
   │───────────────────────────▶│                           │
   │                            │                           │
   │                            │  2. Verify agreement      │
   │                            │     (local cache)         │
   │                            │───────────────────────────│
   │                            │                           │
   │                            │  3. Agreement valid       │
   │                            │◀──────────────────────────│
   │                            │                           │
   │  4. RPC Response           │                           │
   │◀───────────────────────────│                           │
   │                            │                           │
   │                            │  5. Update usage          │
   │                            │     (async)               │
   │                            │───────────────────────────▶
```

## Pricing Models Summary

| Model | Payment | Tracking | Latency | Use Case |
|-------|---------|----------|---------|----------|
| Unlimited | Fixed monthly | None | Zero | Heavy, predictable usage |
| Request Count | Per-request bulk | Counter | Zero* | Light, simple usage |
| Credit-Based | Deposit | Per-method | Zero* | Variable, method-specific |

*Zero latency achieved via optimistic deduction

## Key Benefits

### For Providers
- Predictable revenue (monthly subscriptions)
- Reduced payment processing overhead
- Flexible pricing options
- On-chain payment guarantees

### For Consumers
- Predictable costs (budgeting)
- Zero-latency access
- Choice of pricing models
- No per-request payment overhead

## Related Documentation

- [02-pricing-models.md](./02-pricing-models.md) - Detailed pricing model specs
- [03-marketplace-contract.md](./03-marketplace-contract.md) - Smart contract design
- [04-provider-integration.md](./04-provider-integration.md) - Provider implementation guide
- [05-consumer-integration.md](./05-consumer-integration.md) - SDK/Router integration
- [06-x402-v2-integration.md](./06-x402-v2-integration.md) - Payment session handling
- [07-auth-flow.md](./07-auth-flow.md) - Authentication patterns
