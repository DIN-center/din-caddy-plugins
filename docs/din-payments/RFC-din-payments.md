# RFC: DIN Protocol-Mediated Payment System

**Status:** Draft
**Authors:** DIN Team
**Date:** December 2025

---

## Table of Contents

1. [Summary](#summary)
2. [Problem Statement](#problem-statement)
3. [Proposed Solution](#proposed-solution)
4. [System Architecture](#system-architecture)
   - [High-Level Overview](#high-level-overview)
   - [Network Topology](#network-topology)
5. [Consumer Payment Options](#consumer-payment-options)
   - [Deposit Model](#deposit-model)
6. [Provider Rate Cards](#provider-rate-cards)
   - [Pricing Models](#pricing-models)
   - [Service Types](#service-types-as-of-dec-17-2025)
   - [Rate Card Structure](#rate-card-structure)
   - [Example Rate Cards](#example-rate-cards)
   - [Cost Calculation](#cost-calculation)
7. [Request Routing](#request-routing)
   - [Routing Strategies](#routing-strategies)
   - [Minimum Health Threshold](#minimum-health-threshold-safety-floor)
   - [Strategy: cost](#strategy-cost)
   - [Strategy: health](#strategy-health)
   - [Strategy: balanced (Default)](#strategy-balanced-default)
   - [Full Routing Algorithm](#full-routing-algorithm)
   - [Strategy Comparison](#strategy-comparison)
   - [No Providers for Payment Mode](#no-providers-for-payment-mode)
   - [Price Mismatch Handling](#price-mismatch-handling)
8. [Session-Based Authorization](#session-based-authorization)
   - [Overview](#overview)
   - [Gas Costs (Base Chain)](#gas-costs-base-chain)
   - [Session Flow](#session-flow)
   - [On-Chain Fund Locking](#on-chain-fund-locking)
   - [Consumer-Signed Session Proof (Decentralized)](#consumer-signed-session-proof-decentralized)
   - [Provider Sidecar Validation](#provider-sidecar-validation)
   - [Shared Sessions Across SDK Instances](#shared-sessions-across-sdk-instances)
   - [Checkpoints for Long Sessions](#checkpoints-for-long-sessions)
   - [Smart Contract: Full Session Lifecycle](#smart-contract-full-session-lifecycle)
   - [Consumer SDK Usage](#consumer-sdk-usage)
   - [Session Configuration Options](#session-configuration-options)
9. [Usage Tracking](#usage-tracking)
   - [Aggregated Tracking (Not Per-Request)](#aggregated-tracking-not-per-request)
   - [Local Storage (JSON File)](#local-storage-json-file)
   - [Usage Update Flow](#usage-update-flow)
10. [Bilateral Reconciliation](#bilateral-reconciliation)
    - [Why Bilateral Reconciliation?](#why-bilateral-reconciliation)
    - [How It Works](#how-it-works)
    - [Reconciliation Rules](#reconciliation-rules)
    - [Trust Model](#trust-model)
    - [Why Not Single-Party Attestation?](#why-not-single-party-attestation)
11. [Settlement Coordinator](#settlement-coordinator)
    - [Coordinator Responsibilities](#coordinator-responsibilities)
    - [Off-Chain Reconciliation Flow](#off-chain-reconciliation-flow)
    - [Why Off-Chain?](#why-off-chain)
12. [Settlement Flow](#settlement-flow)
    - [Session End / Checkpoint](#session-end--checkpoint)
    - [Provider Confirmation](#provider-confirmation)
    - [On-Chain Batch Settlement](#on-chain-batch-settlement)
13. [Provider Earnings](#provider-earnings)
    - [Provider SDK (Stats Only)](#provider-sdk-stats-only)
14. [Failure Handling](#failure-handling)
    - [SDK/Router Crash Recovery](#sdkrouter-crash-recovery)
15. [Dispute Resolution](#dispute-resolution)
    - [Auto-Average Settlement (Default)](#auto-average-settlement-default)
    - [Escalation for Malicious Actors](#escalation-for-malicious-actors)
    - [Escalation Fee](#escalation-fee)
    - [Identity Verification (Anti-Abuse)](#identity-verification-anti-abuse)
16. [Smart Contract Architecture](#smart-contract-architecture)
    - [Protocol Contract](#protocol-contract)
    - [Events](#events)
17. [SDK Implementation](#sdk-implementation)
    - [Consumer SDK](#consumer-sdk)
    - [Provider SDK](#provider-sdk)
18. [x402 Alternative Payment Path](#x402-alternative-payment-path)
19. [Protocol Fee (Ideation)](#protocol-fee-ideation)
20. [Success Metrics](#success-metrics)
21. [Open Questions](#open-questions)
22. [Next Steps](#next-steps)
23. [Related Documents](#related-documents)

---

## Summary

A protocol-mediated payment system where the DIN Protocol acts as an intermediary between consumers and providers. Consumers deposit funds into the protocol and set spending preferences. The protocol routes requests to providers based on health scores, pricing, and consumer constraints, then settles payments to providers based on actual usage.

This protocol-as-clearing-house model enables automatic failover, competitive pricing, and simplified consumer experience.

---

## Problem Statement

Building a decentralized RPC network requires solving several payment challenges:

1. **Consumer complexity** - Consumers shouldn't need to discover, evaluate, and manage relationships with individual providers
2. **Automatic failover** - If a provider goes down, routing should automatically switch without consumer intervention
3. **Unified liquidity** - Consumer funds should work across any provider in the network
4. **Provider discovery** - Consumers shouldn't need to understand which providers serve which services
5. **Competitive pricing** - Providers should compete on price and quality, benefiting consumers

The network supports multiple entry points and providers:
- **Many SDKs** - Direct client libraries used by developers
- **Many Routers** - Gateway instances run by different parties
- **Many Providers** - RPC node operators serving requests

The protocol must coordinate usage tracking and settlement across all these participants, ensuring accurate attestation and fair payment distribution.

---

## Proposed Solution

A protocol-mediated system where:

1. **Providers publish rate cards** to the protocol (not individual plans to consumers)
2. **Consumers deposit into the protocol** (or sign spending authorizations)
3. **Protocol routes requests** based on health, price, and consumer preferences
4. **Protocol settles payments** to providers based on verified usage

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                              │
│                      DIN Protocol-Mediated Payment Model                     │
│                                                                              │
│         Consumer ─────── deposits into ─────── DIN Protocol                  │
│                                                     │                        │
│                                                     │ routes requests        │
│                                                     │ settles payments       │
│                                                     ▼                        │
│                                                 Providers                    │
│                              (protocol handles routing & payments)           │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## System Architecture

### High-Level Overview

```
┌──────────────────────────────────────────────────────────────────┐
│                       DIN Protocol Layer                         │
│                                                                  │
│  ┌────────────────┐  ┌────────────────┐  ┌────────────────┐     │
│  │ Consumer       │  │ Provider Rate  │  │ Settlement     │     │
│  │ Accounts       │  │ Registry       │  │ Engine         │     │
│  │                │  │                │  │                │     │
│  │ - Deposits     │  │ - Rate cards   │  │ - Verify usage │     │
│  │ - Locked       │  │ - CU costs     │  │ - Batch settle │     │
│  │ - Max prices   │  │ - Volume tiers │  │ - Disputes     │     │
│  └───────┬────────┘  └───────┬────────┘  └───────┬────────┘     │
│          │                   │                   │               │
│          └───────────────────┼───────────────────┘               │
│                              │                                   │
│                  ┌───────────▼───────────┐                       │
│                  │    Routing Engine     │                       │
│                  │                       │                       │
│                  │  - Health scores      │                       │
│                  │  - Price filtering    │                       │
│                  │  - Load balancing     │                       │
│                  └───────────────────────┘                       │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
         ▲                                           │
         │ deposits                                  │ routes requests
         │ session auth                              │ settles payments
         │                                           ▼
┌─────────────────────┐                   ┌─────────────────────┐
│     Consumers       │                   │     Providers       │
│                     │                   │                     │
│  ┌─────┐  ┌─────┐   │                   │  ┌───┐ ┌───┐ ┌───┐  │
│  │ SDK │  │ SDK │   │                   │  │ P │ │ P │ │ P │  │
│  └─────┘  └─────┘   │                   │  └───┘ └───┘ └───┘  │
│                     │                   │                     │
│  ┌─────┐  ┌─────┐   │                   │  Publish rate cards │
│  │ RTR │  │ RTR │   │                   │  Serve requests     │
│  └─────┘  └─────┘   │                   │  Report usage       │
│                     │                   │  Withdraw earnings  │
└─────────────────────┘                   └─────────────────────┘
```

### Network Topology

The system supports many consumers (SDKs, Routers) and many providers:

```
┌────────────────────────────────────────────────────────────────┐
│                     DIN Network Topology                       │
│                                                                │
│   SDK #1 ───┐                                 ┌─── Provider A  │
│   SDK #2 ───┼──┐                         ┌───┼─── Provider B  │
│   SDK #N ───┘  │                         │   └─── Provider C  │
│                │                         │                     │
│                ▼                         ▼                     │
│        ┌─────────────────────────────────────────┐            │
│        │            DIN PROTOCOL                  │            │
│        │                                          │            │
│        │  - Receives requests from any SDK/Router │            │
│        │  - Routes to best available provider     │            │
│        │  - Tracks usage per consumer per provider│            │
│        │  - Settles payments periodically         │            │
│        └─────────────────────────────────────────┘            │
│                ▲                         ▲                     │
│                │                         │                     │
│   Router #1 ───┼──┘                         └───┼─── Provider D  │
│   Router #2 ───┘                               └─── Provider N  │
│                                                                │
└────────────────────────────────────────────────────────────────┘
```

---

## Consumer Payment Options

### Deposit Model

Consumer deposits USDC into the protocol contract. On Base chain, this costs ~$0.001-0.01 per transaction — negligible.

```
Consumer                     Protocol Contract
   │                              │
   │  1. approve(protocol, $500)  │
   │     (one-time or per-deposit)│
   │─────────────────────────────▶│
   │                              │
   │  2. deposit($500)            │
   │─────────────────────────────▶│
   │                              │
   │  [Money held in contract]    │
   │  [Can withdraw anytime]      │
   │  [Use for many sessions]     │
   │                              │
```

**Why not EIP-2612 Permit?**

We considered a permit-based approach where consumers sign off-chain and funds stay in their wallet. However:

1. **Security risk** - Consumer can move funds after signing permit but before settlement
2. **Marginal benefit** - On Base, saving one transaction saves ~$0.002-0.02
3. **Added complexity** - Permit handling, EIP-712 domains, deadline management
4. **Deposit is better UX** - Deposit once, start many sessions without re-approving

The deposit model is simpler, more secure, and costs nearly nothing on Base.

**Consumer Account Structure:**

```solidity
struct ConsumerAccount {
    uint256 balance;           // USDC deposited (not locked)
    uint256 locked;            // USDC locked in active sessions
    uint256 totalSpent;        // Historical spend
    bool active;               // Account active
}
```

---

## Provider Rate Cards

Providers publish their pricing to the protocol. Rate cards support multiple services and pricing models.

### Pricing Models

Two pricing models are available. **Consumers choose their payment mode upfront**, and the protocol only routes to providers that support that mode.

```
                          Pricing Models & Routing

    CU-BASED PRICING
    ────────────────
    • Protocol sets: Price per CU (universal, e.g., $0.00008/CU)
    • Providers set: Method/endpoint CU costs via methodCUs mapping
    • Cost = methodCUs[method] × PROTOCOL_PRICE_PER_CU
    • Provider must set: supportsCUPricing = true

    PRICE-PER-REQUEST
    ─────────────────
    • Providers set: Direct price via methodPrices mapping
    • Cost = methodPrices[method] (direct USDC price)
    • Provider must set: supportsRequestPricing = true

    CONSUMER PAYMENT MODE
    ─────────────────────
    • Consumer declares payment mode when starting session: "cu" or "request"
    • Protocol filters routing pool to ONLY providers supporting that mode
    • Consumer pays consistently in their chosen mode for entire session

    PROVIDER OPTIONS
    ────────────────
    • Support CU-only:      supportsCUPricing=true,  supportsRequestPricing=false
    • Support Request-only: supportsCUPricing=false, supportsRequestPricing=true
    • Support Both:         supportsCUPricing=true,  supportsRequestPricing=true
```

**Routing Example:**
```
Consumer starts session with paymentMode: "cu"
                    │
                    ▼
┌─────────────────────────────────────────────────────────────────┐
│  Filter providers for "ethereum-mainnet" that support CU pricing │
│                                                                  │
│  Provider A: supportsCUPricing=true  ✓ Include                   │
│  Provider B: supportsCUPricing=true  ✓ Include                   │
│  Provider C: supportsCUPricing=false ✗ Skip (request-only)       │
└─────────────────────────────────────────────────────────────────┘
                    │
                    ▼
         Route to Provider A or B based on health/price
```

### Service Types (as of Dec 17, 2025)

| Service Type | Request Type | Examples |
|--------------|--------------|----------|
| `evm` | RPC | ethereum-mainnet, base-mainnet, polygon-mainnet |
| `solana` | RPC | solana-mainnet |
| `starknet` | RPC | starknet-mainnet |
| `bitcoin` | RPC | bitcoin-mainnet |
| `tron-full-node` | RPC | tron-mainnet |
| `beacon-chain` | REST | ethereum-beacon |
| `bitcoin-esplora` | REST | bitcoin-esplora |

### Rate Card Structure

Rate cards are stored in the **DINProtocol** contract, which unifies the registry and payment system into a single protocol. This provides O(1) lookups and a single source of truth for service capabilities, pricing, deposits, and settlements.

```solidity
// In DINProtocol contract
struct ProviderServiceData {
    // Provider service configuration
    uint256 id;
    uint256 providerId;
    uint256 serviceId;              // References Service (e.g., "ethereum-mainnet")
    string endpointUrl;
    uint256 capabilities;           // Bitmask of supported methods

    // PAYMENT RATE CARD (NEW)
    bool supportsCUPricing;         // Provider accepts CU-based payments
    bool supportsRequestPricing;    // Provider accepts per-request payments
    uint256 defaultCUCost;          // Default CUs for unlisted methods
    uint256 defaultPricePerRequest; // Default price for unlisted methods (6 decimals)
}

// Method-specific pricing uses bit positions for efficient lookup
// This leverages the existing bit mapping system (uint16 per method)
mapping(uint256 => mapping(uint16 => uint256)) public methodCUCosts;    // providerServiceId => methodBit => CU cost
mapping(uint256 => mapping(uint16 => uint256)) public methodPrices;     // providerServiceId => methodBit => direct price

// Protocol-level pricing (set by governance)
uint256 public pricePerCU;  // Universal CU price in USDC (6 decimals)
```

**Benefits of Unified Protocol:**
- **Single source of truth** - Registry, pricing, deposits, and settlements in one contract
- **Efficient lookups** - O(1) via `methodCUCosts[providerServiceId][methodBit]`
- **Batch reads** - `getProtocolSnapshot()` includes pricing data for SDK caching
- **Consistent bit mapping** - Same method bits used for capabilities and pricing

// Cost calculation (based on consumer's payment mode):
//
// If consumer paymentMode = "cu":
//   Cost = methodCUs[method] × PROTOCOL_PRICE_PER_CU
//   (falls back to defaultCUCost if method not listed)
//
// If consumer paymentMode = "request":
//   Cost = methodPrices[method]
//   (falls back to defaultPricePerRequest if method not listed)
```

### Example Rate Cards

Providers declare which payment modes they support per service.

**Provider A (Supports both CU and per-request pricing):**
```json
{
  "provider": "0xProviderA",
  "services": {
    "ethereum-mainnet": {
      "serviceType": "evm",
      "supportsCUPricing": true,
      "supportsRequestPricing": true,
      "defaultCUCost": 5,
      "methodCUs": {
        "eth_call": 10,
        "eth_getLogs": 50,
        "eth_getBalance": 5,
        "eth_getBlockByNumber": 10,
        "debug_traceCall": 500
      },
      "defaultPricePerRequest": 0.001,
      "methodPrices": {
        "eth_call": 0.0008,
        "eth_getLogs": 0.004,
        "debug_traceCall": 0.05
      }
    },
    "ethereum-beacon": {
      "serviceType": "beacon-chain",
      "supportsCUPricing": true,
      "supportsRequestPricing": true,
      "defaultCUCost": 1,
      "methodCUs": {
        "/eth/v1/beacon/genesis": 1,
        "/eth/v1/beacon/states/{state_id}/root": 2,
        "/eth/v1/beacon/states/{state_id}/finality_checkpoints": 3,
        "/eth/v1/beacon/states/{state_id}/committees": 10,
        "/eth/v1/beacon/states/{state_id}/validators": 100
      },
      "defaultPricePerRequest": 0.0001,
      "methodPrices": {
        "/eth/v1/beacon/genesis": 0.00008,
        "/eth/v1/beacon/states/{state_id}/validators": 0.001
      }
    }
  }
}
```

**Provider B (CU-only - efficient infrastructure):**
```json
{
  "provider": "0xProviderB",
  "services": {
    "ethereum-mainnet": {
      "serviceType": "evm",
      "supportsCUPricing": true,
      "supportsRequestPricing": false,
      "defaultCUCost": 1,
      "methodCUs": {
        "eth_call": 5,
        "eth_getLogs": 25,
        "debug_traceCall": 250
      }
    }
  }
}
```

**Provider C (Per-request only - simple flat pricing):**
```json
{
  "provider": "0xProviderC",
  "services": {
    "bitcoin-esplora": {
      "serviceType": "bitcoin-esplora",
      "supportsCUPricing": false,
      "supportsRequestPricing": true,
      "defaultPricePerRequest": 0.0001,
      "methodPrices": {
        "/address/{address}": 0.0001,
        "/address/{address}/txs": 0.0002,
        "/tx/{txid}": 0.0001,
        "/block/{hash}": 0.0005
      }
    }
  }
}
```

**Provider D (Multi-chain, CU-only):**
```json
{
  "provider": "0xProviderD",
  "services": {
    "solana-mainnet": {
      "serviceType": "solana",
      "supportsCUPricing": true,
      "supportsRequestPricing": false,
      "defaultCUCost": 1,
      "methodCUs": {
        "getAccountInfo": 2,
        "getBalance": 1,
        "getTransaction": 3,
        "getSlot": 1
      }
    },
    "starknet-mainnet": {
      "serviceType": "starknet",
      "supportsCUPricing": true,
      "supportsRequestPricing": false,
      "defaultCUCost": 2,
      "methodCUs": {
        "starknet_call": 5,
        "starknet_getBlockWithTxs": 10
      }
    }
  }
}
```

### Cost Calculation

Cost depends on the **consumer's chosen payment mode**:

**Consumer with paymentMode: "cu"** (routed only to providers with supportsCUPricing=true)
```
Cost = methodCUs[method] × PROTOCOL_PRICE_PER_CU

Protocol Price: $0.00008/CU (set by governance)

Examples:

eth_call on ethereum-mainnet (Provider A or B):
  Provider A: 10 CUs × $0.00008/CU = $0.0008
  Provider B: 5 CUs × $0.00008/CU = $0.0004  ← More efficient!

/eth/v1/beacon/states/{state_id}/committees on ethereum-beacon (Provider A):
  Cost = 10 CUs × $0.00008/CU = $0.0008
```

**Consumer with paymentMode: "request"** (routed only to providers with supportsRequestPricing=true)
```
Cost = methodPrices[method]

Examples:

eth_call on ethereum-mainnet (Provider A):
  Cost = $0.0008 (provider's direct price)

/eth/v1/beacon/states/{state_id}/validators on ethereum-beacon (Provider A):
  Cost = $0.001 (provider's direct price)

/address/{address}/txs on bitcoin-esplora (Provider C):
  Cost = $0.0002 (provider's direct price)
```

**Provider Competition:**

Providers compete on:
- **Efficiency** (lower CU costs for CU-mode consumers)
- **Competitive pricing** (lower direct prices for request-mode consumers)
- **Quality** (health scores, uptime, latency)
- **Coverage** (services and methods supported)
- **Mode support** (supporting both modes captures more consumers)

---

## Request Routing

The protocol routes requests based on payment mode, service support, pricing, health scores, and consumer routing preferences.

### Routing Strategies

Consumers can choose how the protocol prioritizes providers:

| Strategy | Description | Selection Method | Best For |
|----------|-------------|------------------|----------|
| `cost` | Prioritize lowest price | Cheapest provider | Price-sensitive apps, batch jobs |
| `health` | Prioritize reliability | Weighted random by health | Mission-critical, production |
| `balanced` | Best value (quality + price) | Weighted random by value score | Most users (default) |

### Minimum Health Threshold (Safety Floor)

**All strategies** enforce a minimum health threshold before any routing logic runs. Providers below this threshold are excluded regardless of price.

```
                         Minimum Health Threshold

    Default: 0.5 (configurable per consumer)

    Before ANY routing strategy:
    Provider A: health=0.98 ✓ Above threshold
    Provider B: health=0.30 ✗ Below threshold (excluded)
    Provider C: health=0.80 ✓ Above threshold
    Provider D: health=0.45 ✗ Below threshold (excluded)

    This protects consumers from routing to unhealthy providers
    even when using "cost" strategy.
```

### Strategy: `cost`

Select the cheapest provider above the health threshold.

```
Remaining after health filter: [Provider A, Provider C]

Provider A: cost=$0.0008
Provider C: cost=$0.0004

Selection: Provider C (cheapest)
```

Simple and deterministic. Best for high-volume, cost-sensitive workloads where any healthy provider is acceptable.

### Strategy: `health`

Weighted random selection based on health scores (current DIN behavior).

```
Remaining after health filter: [Provider A, Provider C]

Provider A: health=0.98 → weight=0.98
Provider C: health=0.80 → weight=0.80

Selection: Weighted random
  - Provider A selected ~55% of the time (0.98 / 1.78)
  - Provider C selected ~45% of the time (0.80 / 1.78)
```

Prioritizes reliability and distributes load proportionally to health. Best for mission-critical applications.

### Strategy: `balanced` (Default)

Weighted random selection based on **value score** - rewards providers who offer both good health AND competitive pricing.

**Value Score Formula:**

```
valueScore = health × costEfficiency

where costEfficiency = cheapestCost / providerCost
```

**Example:**

```
                         Balanced Strategy Example

    Providers after health filter (minHealth=0.5):

    Provider A: health=0.98, cost=$0.0008
    Provider B: health=0.30, cost=$0.0004  ← excluded (below 0.5)
    Provider C: health=0.80, cost=$0.0004

    Cheapest cost among remaining: $0.0004

    Value Score Calculation:
    ─────────────────────────
    Provider A: 0.98 × (0.0004 / 0.0008) = 0.98 × 0.5 = 0.49
    Provider C: 0.80 × (0.0004 / 0.0004) = 0.80 × 1.0 = 0.80

    Selection: Weighted random by value score
    - Provider C selected ~62% of the time (0.80 / 1.29)
    - Provider A selected ~38% of the time (0.49 / 1.29)

    Result: Provider C wins most often because it offers
            good health (0.80) at the best price ($0.0004)

    Provider A loses despite higher health because it costs 2x
    for only marginally better reliability.
```

### Full Routing Algorithm

```
                              Routing Decision

    Input: Consumer request for "ethereum-mainnet", method "eth_call"
           Consumer paymentMode: "cu"
           Consumer routingStrategy: "balanced"
           Consumer minHealthThreshold: 0.5
           Consumer maxCUsPerRequest: 100

    Step 1: Filter by payment mode
    ──────────────────────────────
    Provider A: supportsCUPricing=true  ✓
    Provider B: supportsCUPricing=true  ✓
    Provider C: supportsCUPricing=false ✗ Skip
    Provider D: supportsCUPricing=true  ✓
    Remaining: [A, B, D]

    Step 2: Filter by service + method support
    ──────────────────────────────────────────
    Provider A: ✓ supports ethereum-mainnet + eth_call
    Provider B: ✓ supports ethereum-mainnet + eth_call
    Provider D: ✗ only supports polygon
    Remaining: [A, B]

    Note: Provider must support both the service AND the specific method.
    If a provider doesn't list a method, it uses defaultCUCost/defaultPrice.

    Step 3: Filter by cost limit
    ────────────────────────────
    Protocol CU price: $0.00008/CU (universal, set by governance)

    (CU mode) Filter by maxCUsPerRequest:
    Provider A: eth_call costs 10 CUs ✓ under 100 max
    Provider B: eth_call costs 5 CUs ✓ under 100 max
    Remaining: [A, B]

    (Per-request mode would filter by maxPricePerMethod instead)

    Step 4: Filter by minimum health threshold
    ──────────────────────────────────────────
    Provider A: health=0.98 ✓ above 0.5
    Provider B: health=0.85 ✓ above 0.5
    Remaining: [A, B]

    Step 5: Apply routing strategy ("balanced")
    ───────────────────────────────────────────
    Cheapest: $0.0004
    Provider A: valueScore = 0.98 × (0.0004/0.0008) = 0.49
    Provider B: valueScore = 0.85 × (0.0004/0.0004) = 0.85

    Step 6: Select (weighted random by value score)
    ───────────────────────────────────────────────
    Provider B selected (~63% probability)
```

### Strategy Comparison

Given providers after filtering:
- Provider A: health=0.98, cost=$0.0008
- Provider B: health=0.85, cost=$0.0004

| Strategy | Winner | Reasoning |
|----------|--------|-----------|
| `cost` | Provider B (100%) | Cheapest price |
| `health` | Provider A (~54%) | Higher health score |
| `balanced` | Provider B (~63%) | Better value (health × cost efficiency) |

### No Providers for Payment Mode

When no providers support the consumer's chosen payment mode:

```
Consumer                     Protocol
   │                            │
   │  Request (paymentMode: "request")
   │  for "solana-mainnet"
   │───────────────────────────▶│
   │                            │
   │                            │  No providers have
   │                            │  supportsRequestPricing=true
   │                            │  for solana-mainnet
   │                            │
   │  400 / Error Response      │
   │  {                         │
   │    error: "no_providers_for_mode",  │
   │    service: "solana-mainnet",       │
   │    requestedMode: "request",        │
   │    availableModes: ["cu"],          │
   │    suggestion: "Switch to CU        │
   │      payment mode for this service" │
   │  }                         │
   │◀───────────────────────────│
```

### Cost Limit Mismatch Handling

When no providers meet the consumer's cost limit:

```
Consumer                     Protocol
   │                            │
   │  Request (paymentMode: "cu",
   │           maxCUsPerRequest: 5)
   │───────────────────────────▶│
   │                            │
   │                            │  All providers > 5 CUs
   │                            │  for this method
   │                            │
   │  402 / Error Response      │
   │  {                         │
   │    error: "no_providers",  │
   │    cheapestCUs: 10,        │
   │    method: "eth_call",     │
   │    suggestion: "Increase   │
   │      maxCUsPerRequest"     │
   │  }                         │
   │◀───────────────────────────│
```

Note: In CU mode, the protocol sets a universal CU price (e.g., $0.00008/CU).
Providers cannot change this - they can only set how many CUs each method costs.
The consumer's cost limit controls the maximum CUs (in CU mode) or maximum
price per method (in per-request mode) they're willing to spend.

---

## Session-Based Authorization

Sessions allow consumers to make many requests without signing each one. The system uses **on-chain fund locking** for security and **consumer-signed session proofs** verified by provider sidecars checking on-chain state.

### Overview

```
                              Session Lifecycle

        ┌─────────────────────────────────────────────────────────┐
        │                       1. DEPOSIT                        │
        │─────────────────────────────────────────────────────────│
        │  Consumer deposits USDC into the protocol contract.     │
        │  Funds are held but not locked - available for use.     │
        │                                                         │
        │  Transaction: On-chain (consumer pays ~$0.001-0.01)     │
        └────────────────────────────┬────────────────────────────┘
                                     │
                                     ▼
        ┌─────────────────────────────────────────────────────────┐
        │                    2. START SESSION                     │
        │─────────────────────────────────────────────────────────│
        │  Consumer locks a portion of deposited funds on-chain.  │
        │  Consumer signs a session proof with their wallet.      │
        │                                                         │
        │  Transaction: On-chain (consumer pays ~$0.001-0.01)     │
        └────────────────────────────┬────────────────────────────┘
                                     │
                                     ▼
        ┌─────────────────────────────────────────────────────────┐
        │                        3. USE                           │
        │─────────────────────────────────────────────────────────│
        │  Consumer sends requests with signed session proof.     │
        │  Provider verifies on-chain (once), then caches.        │
        │                                                         │
        │  Transaction: Off-chain (no gas costs)                  │
        └────────────────────────────┬────────────────────────────┘
                                     │
                                     ▼
        ┌─────────────────────────────────────────────────────────┐
        │                    4. CHECKPOINT                        │◀──────┐
        │─────────────────────────────────────────────────────────│       │
        │  Protocol settles usage for the period. Both consumer   │       │
        │  and provider report usage for bilateral reconciliation.│ repeat│
        │                                                         │       │
        │  Transaction: On-chain (protocol pays, batched)         │───────┘
        └────────────────────────────┬────────────────────────────┘
                                     │
                                     ▼
        ┌─────────────────────────────────────────────────────────┐
        │                   5. FINAL SETTLE                       │
        │─────────────────────────────────────────────────────────│
        │  Protocol settles remaining usage and unlocks unused    │
        │  funds back to consumer's available balance.            │
        │                                                         │
        │  Transaction: On-chain (protocol pays, batched)         │
        └─────────────────────────────────────────────────────────┘


        Checkpoint Model (for long-running sessions)
        ────────────────────────────────────────────
        • Lock small amount ($50), not entire session spend
        • Checkpoint triggers: time-based (30min) or spend-based (60%)
        • At checkpoint: settle actual usage, keep session alive
        • Session can run indefinitely with small locked amount
        • Consumer tops up deposit if balance runs low
```

### Gas Costs (Base Chain)

Base chain offers extremely low transaction costs, making the two-step process (deposit + start session) very affordable:

| Action | Who Pays | Estimated Cost | What It Does |
|--------|----------|----------------|--------------|
| Deposit USDC | Consumer | ~$0.001-0.01 | Transfer USDC from wallet INTO contract |
| Start session (lock funds) | Consumer | ~$0.001-0.01 | Reserve deposited funds for session |
| Make RPC requests | - | $0 (off-chain) | Consumer-signed proof, provider verifies |
| Checkpoint settlement | Protocol (batched) | Amortized ~$0.001/session | Settle usage, keep session alive |
| Final settlement | Protocol (batched) | Amortized ~$0.001/session | Close session, unlock unused |
| Withdraw USDC | Consumer | ~$0.001-0.01 | Transfer USDC from contract to wallet |

**Why Two Steps (Deposit vs Start Session)?**

```
                       Deposit vs Start Session

    DEPOSIT ($0.001-0.01 gas)
    ─────────────────────────
    • Transfers USDC from wallet → contract
    • Adds funds to your account balance
    • One-time or whenever you need more funds
    • Funds remain available (not locked)

    START SESSION ($0.001-0.01 gas)
    ────────────────────────────────
    • No USDC transfer - just internal accounting
    • Locks portion of deposited funds for this session
    • Every time you start a new usage session
    • Locked funds can't be withdrawn until session ends

    Example Flow:
    ─────────────
    1. Consumer deposits $100 USDC (one transaction)
       → Balance: deposited=$100, locked=$0, available=$100

    2. Consumer starts session, locks $50
       → Balance: deposited=$100, locked=$50, available=$50

    3. Consumer makes requests, spends $35 over time
       → (tracked off-chain, settled at checkpoints)

    4. Session ends, settled for $35, unlocks remaining $15
       → Balance: deposited=$65, locked=$0, available=$65

    5. Consumer starts new session, locks $30
       → Balance: deposited=$65, locked=$30, available=$35

    Benefits:
    • Deposit once, start many sessions (no repeated deposits)
    • Small sessions don't require large upfront deposits
    • Unused funds immediately available for new sessions
    • Security: locked funds guaranteed for provider payment
```

### Session Flow

```
                            Full Session Flow

    Consumer                   Smart Contract             Provider Sidecar
        │                          │                          │
        │  1. startSession($50)    │                          │
        │     (on-chain tx)        │                          │
        │─────────────────────────▶│                          │
        │                          │                          │
        │  2. SessionStarted event │                          │
        │     (sessionId, expiry)  │                          │
        │◀─────────────────────────│                          │
        │                          │                          │
        │  3. Sign session proof   │                          │
        │     with wallet          │                          │
        │     (off-chain, no gas)  │                          │
        │                          │                          │
        │  4. First RPC Request + Signed Proof                │
        │────────────────────────────────────────────────────▶│
        │                          │                          │
        │                          │     Verify signature     │
        │                          │     Query chain:         │
        │                          │◀───── session valid? ────│
        │                          │────── yes, active ──────▶│
        │                          │     Cache session        │
        │                          │                          │
        │  5. Response                                        │
        │◀────────────────────────────────────────────────────│
        │                          │                          │
        │  6. Subsequent requests (cache hit - no chain query)│
        │────────────────────────────────────────────────────▶│
        │◀────────────────────────────────────────────────────│
        │  ... many more requests (fast cache lookup) ...     │
        │                          │                          │
        │  7. Checkpoint           │                          │
        │     (periodic)           │                          │
        │                          │◀──── Report usage ───────│
        │────── Report usage ─────▶│                          │
        │                          │                          │
        │                          │  Reconcile + settle      │
        │                          │  (on-chain, batched)     │
        │                          │                          │
        │  8. Session continues    │                          │
        │     or ends              │                          │
        │                          │                          │
```

**Key difference from centralized model:** There is no Protocol API for token issuance. The consumer signs their own session proof, and provider sidecars verify it by checking on-chain state directly. This is fully decentralized.

### On-Chain Fund Locking

When a consumer starts a session, funds are **locked in the smart contract**. This prevents the consumer from withdrawing funds that are committed to a session.

```solidity
contract DINProtocol {
    mapping(address => uint256) public deposits;
    mapping(address => uint256) public locked;
    mapping(bytes32 => Session) public sessions;
    mapping(address => bytes32) public activeSession;  // One active session per consumer

    struct Session {
        address consumer;
        uint256 maxSpend;
        uint256 lockedAmount;
        uint64 validUntil;
        uint256 snapshotBlock;     // Block at which rate card prices were snapshotted
        bool active;
    }

    function deposit(uint256 amount) external {
        USDC.transferFrom(msg.sender, address(this), amount);
        deposits[msg.sender] += amount;
    }

    function startSession(
        uint256 maxSpend,
        uint64 duration,
        string calldata paymentMode,
        string calldata routingStrategy
    ) external returns (bytes32 sessionId) {
        // Enforce one active session per consumer
        require(activeSession[msg.sender] == bytes32(0), "Session already active");

        // Check available balance
        uint256 available = deposits[msg.sender] - locked[msg.sender];
        require(available >= maxSpend, "Insufficient unlocked balance");

        // Lock funds
        locked[msg.sender] += maxSpend;

        // Create session ID (unique per consumer + timestamp + amount)
        sessionId = keccak256(abi.encodePacked(msg.sender, block.timestamp, maxSpend));

        // Track active session
        activeSession[msg.sender] = sessionId;

        sessions[sessionId] = Session({
            consumer: msg.sender,
            maxSpend: maxSpend,
            lockedAmount: maxSpend,
            validUntil: uint64(block.timestamp) + duration,
            snapshotBlock: block.number,  // Snapshot prices at session start
            active: true
        });

        emit SessionStarted(sessionId, msg.sender, maxSpend, duration);
    }

    function withdraw(uint256 amount) external {
        uint256 available = deposits[msg.sender] - locked[msg.sender];
        require(available >= amount, "Insufficient unlocked balance");

        deposits[msg.sender] -= amount;
        USDC.transfer(msg.sender, amount);
    }
}
```

**Consumer's view:**

```
    Your DIN Balance
    ───────────────────

    Total Deposited:    $100.00
    Locked (sessions):  $ 50.00
    Available:          $ 50.00

    Active Sessions:
    • Session #abc123: $50 locked
      Expires: Dec 18, 3:00 PM
```

### Consumer-Signed Session Proof (Decentralized)

Unlike centralized systems that require a Protocol API to issue tokens, DIN uses a **fully decentralized** approach: consumers sign their own session proofs, and providers verify them by checking on-chain state directly.

**Why decentralized?**

| Centralized (Protocol API) | Decentralized (Consumer Self-Sign) |
|---------------------------|-----------------------------------|
| Single point of failure | No central dependency |
| Protocol must be online | Works with just the chain |
| Trust in Protocol service | Trust in smart contract only |
| Protocol can censor | Censorship resistant |

**How it works:**

```
                    Decentralized Session Proof Flow

    1. CONSUMER CREATES SESSION ON-CHAIN
    ─────────────────────────────────────
    Consumer calls startSession() → Contract locks funds
                                  → Emits SessionStarted event
                                  → Returns sessionId

    2. CONSUMER SIGNS SESSION PROOF (off-chain, no gas)
    ───────────────────────────────────────────────────
    Consumer's wallet signs: {
      sessionId: "0x123...",
      timestamp: 1702857600
    }
    This signed message IS the "token" - no Protocol API needed.

    3. PROVIDER VERIFIES ON-CHAIN (first request only)
    ──────────────────────────────────────────────────
    Sidecar receives request + signed proof:
    a) Verify signature → confirms consumer identity
    b) Query smart contract → confirms session is active
    c) Cache result → skip chain query for subsequent requests

    4. SUBSEQUENT REQUESTS (cache hit)
    ───────────────────────────────────
    Consumer sends sessionId with requests
    Sidecar checks cache → processes immediately
```

**Session proof structure:**

```typescript
interface SessionProof {
  // Session reference (matches on-chain)
  sessionId: string;          // e.g. "0x7a3b...f9c2" (bytes32 hash from contract)
  consumer: string;           // e.g. "0x1234567890abcdef1234567890abcdef12345678"

  // Timestamp for replay protection
  timestamp: number;          // e.g. 1702857600 (Unix timestamp in seconds)

  // Consumer's signature (EIP-191 personal_sign)
  signature: string;          // e.g. "0x3a4b5c...d8e9f0" (signed by consumer's wallet)
}

// Message format for signing (EIP-191)
const message = `DIN Session Proof
Session: ${sessionId}
Timestamp: ${timestamp}`;
```

**Request format:**

```typescript
// Default request format - sessionId only (provider has session cached)
{
  sessionId: "0x7a3b...f9c2",
  request: {
    method: "eth_blockNumber",
    params: [],
    id: 1
  }
}

// If provider returns 401 + X-DIN-PROOF, retry with full proof
{
  proof: {
    sessionId: "0x7a3b...f9c2",
    consumer: "0x1234...5678",
    timestamp: 1702857600,
    signature: "0x3a4b5c...d8e9f0"
  },
  request: {
    method: "eth_blockNumber",
    params: [],
    id: 1
  }
}
```

**Provider-driven proof flow:**

```
SDK                              Provider
 │                                  │
 │  Request (sessionId only)        │
 │─────────────────────────────────▶│
 │                                  │  Cache miss
 │  401 + X-DIN-PROOF      │
 │◀─────────────────────────────────│
 │                                  │
 │  Retry with full proof           │
 │─────────────────────────────────▶│
 │                                  │  Verify + cache
 │  200 OK                          │
 │◀─────────────────────────────────│
 │                                  │
 │  Next request (sessionId only)   │
 │─────────────────────────────────▶│
 │                                  │  Cache hit
 │  200 OK                          │
 │◀─────────────────────────────────│
```

This approach:
- Saves ~200 bytes/request after first request to each provider
- SDK doesn't need to track which providers have been verified
- Provider controls when proof is needed

**SDK handles proof-on-demand automatically:**

```typescript
class DinSDK {
  private sessionId: string;

  async request(providerUrl: string, rpcRequest: RPCRequest): Promise<RPCResponse> {
    // First attempt - sessionId only
    const response = await fetch(providerUrl, {
      method: 'POST',
      body: JSON.stringify({
        sessionId: this.sessionId,
        request: rpcRequest
      })
    });

    // If provider needs proof, retry with it
    if (response.status === 401 && response.headers.get('X-DIN-PROOF')) {
      const proof = await this.createSessionProof();

      const retryResponse = await fetch(providerUrl, {
        method: 'POST',
        body: JSON.stringify({
          sessionId: this.sessionId,
          proof,
          request: rpcRequest
        })
      });

      return retryResponse.json();
    }

    return response.json();
  }

  private async createSessionProof(): Promise<SessionProof> {
    const timestamp = Math.floor(Date.now() / 1000);
    const message = `DIN Session Proof\nSession: ${this.sessionId}\nTimestamp: ${timestamp}`;

    // Consumer signs with their existing wallet - no new keys needed
    const signature = await this.wallet.signMessage(message);

    return {
      sessionId: this.sessionId,
      consumer: this.wallet.address,
      timestamp,
      signature
    };
  }
}
```

### Provider Sidecar Validation

The provider sidecar validates the session proof **once per session** by:
1. Verifying the consumer's signature (EIP-191)
2. Verifying the session exists on-chain with sufficient locked funds
3. Caching the verified session for subsequent requests

```go
type Sidecar struct {
    contractClient    *DINPaymentsClient  // On-chain verification
    sessionCache      map[string]*CachedSession  // sessionId -> cached session
    mu                sync.RWMutex
}

type CachedSession struct {
    Consumer         common.Address
    LockedAmount     *big.Int
    SpentAmount      *big.Int     // On-chain spent amount at time of caching
    LocalSpentAmount *big.Int     // Provider's locally tracked spend (for staleness detection)
    Services         []string
    ValidUntil       time.Time    // Cache expiry (30 min, aligned with checkpoint)
}

// Request can have just sessionId, or full proof
type Request struct {
    SessionId string        `json:"sessionId"`        // Always present
    Proof     *SessionProof `json:"proof,omitempty"`  // Only on retry after 401
    Request   json.RawMessage `json:"request"`
}

// SessionProof - only sent when provider requests it
type SessionProof struct {
    SessionId string `json:"sessionId"`   // e.g. "0x7a3b...f9c2"
    Consumer  string `json:"consumer"`    // e.g. "0x1234...5678"
    Timestamp int64  `json:"timestamp"`   // e.g. 1702857600
    Signature string `json:"signature"`   // e.g. "0x3a4b5c...d8e9f0"
}

// ErrProofRequired signals SDK to retry with full proof
// Returns: 401 + X-DIN-PROOF header
var ErrProofRequired = errors.New("proof required")

func (s *Sidecar) ValidateRequest(r *http.Request, requestCost *big.Int) (*CachedSession, error) {
    var req Request
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        return nil, ErrInvalidRequest
    }

    // Fast path: check cache first
    s.mu.RLock()
    cached, exists := s.sessionCache[req.SessionId]
    s.mu.RUnlock()

    if exists && time.Now().Before(cached.ValidUntil) {
        // Cache hit - check if local spend is approaching limit
        remaining := new(big.Int).Sub(cached.LockedAmount, cached.SpentAmount)
        remaining.Sub(remaining, cached.LocalSpentAmount)

        if remaining.Cmp(requestCost) < 0 {
            // Local spend tracking suggests session may be exhausted
            // Force re-verification to get fresh on-chain state
            exists = false
        } else {
            // Cache hit - no proof needed, no signature verification
            // Update local spend tracking
            cached.LocalSpentAmount.Add(cached.LocalSpentAmount, requestCost)
            return cached, nil
        }
    }

    // Cache miss or local spend limit reached - need proof to verify
    if req.Proof == nil {
        // Return 401 + X-DIN-PROOF, SDK will retry with proof
        return nil, ErrProofRequired
    }

    // Verify the proof
    proof := req.Proof

    // Step 1: Verify consumer's signature (EIP-191)
    if err := s.verifySignature(proof); err != nil {
        return nil, err
    }

    // Step 2: Verify session on-chain (~200-500ms)
    session, err := s.contractClient.GetSession(proof.SessionId)
    if err != nil {
        return nil, ErrSessionNotFound
    }

    // Verify consumer matches and session is active
    if session.Consumer != common.HexToAddress(proof.Consumer) {
        return nil, ErrConsumerMismatch
    }
    if session.Status != SessionStatusActive {
        return nil, ErrSessionNotActive
    }
    if session.LockedAmount.Cmp(session.SpentAmount) <= 0 {
        return nil, ErrInsufficientFunds
    }

    // Cache the verified session
    cached = &CachedSession{
        Consumer:         session.Consumer,
        LockedAmount:     session.LockedAmount,
        SpentAmount:      session.SpentAmount,
        LocalSpentAmount: big.NewInt(0),  // Reset local tracking
        Services:         session.Services,
        ValidUntil:       time.Now().Add(30 * time.Minute),  // 30 min, aligned with checkpoint
    }

    s.mu.Lock()
    s.sessionCache[req.SessionId] = cached
    s.mu.Unlock()

    return cached, nil
}

func (s *Sidecar) verifySignature(proof *SessionProof) error {
    // Check timestamp freshness (prevent replay)
    if time.Now().Unix() - proof.Timestamp > 300 { // 5 minute window
        return ErrStaleProof
    }

    // Verify consumer's signature (EIP-191)
    message := fmt.Sprintf("DIN Session Proof\nSession: %s\nTimestamp: %d",
        proof.SessionId, proof.Timestamp)

    recoveredAddr, err := ecrecover(message, proof.Signature)
    if err != nil || recoveredAddr != common.HexToAddress(proof.Consumer) {
        return ErrInvalidSignature
    }
    return nil
}

// HTTP handler returns appropriate response
func (s *Sidecar) handleValidationError(w http.ResponseWriter, err error) {
    switch err {
    case ErrProofRequired:
        w.Header().Set("X-DIN-PROOF", "required")
        w.WriteHeader(http.StatusUnauthorized)  // 401
    case ErrInvalidSignature, ErrStaleProof:
        w.WriteHeader(http.StatusUnauthorized)  // 401
    case ErrSessionNotFound, ErrSessionNotActive, ErrInsufficientFunds:
        w.WriteHeader(http.StatusForbidden)     // 403
    default:
        w.WriteHeader(http.StatusInternalServerError)
    }
}
```

**Performance characteristics:**

| Request Type | Latency | What Happens |
|--------------|---------|--------------|
| Cache hit | ~0.01ms | Cache lookup only, no crypto |
| Cache miss (first request) | ~1ms + retry | 401 response, SDK retries with proof |
| Retry with proof | ~200-500ms | Signature verify + on-chain lookup |
| After cache expires (30 min) | ~200-500ms | Re-verify on-chain at checkpoint boundary |
| Local spend limit reached | ~200-500ms | Re-verify to get fresh on-chain state |

**Cache Staleness Protection:**

Providers track their own served requests locally (`LocalSpentAmount`) to detect when a session may be exhausted:

```
Provider's local tracking:
- Cached LockedAmount: $50
- Cached SpentAmount (on-chain): $10
- LocalSpentAmount (this provider): $35

Remaining = $50 - $10 - $35 = $5

If next request costs $6:
→ Force re-verification to get fresh on-chain state
→ Catches case where other providers have also served this session
```

**Sidecar needs to:**
1. Check session cache for incoming sessionId
2. Track local spend and re-verify if approaching limit
3. Return `401 + X-DIN-PROOF` if cache miss and no proof provided
4. Verify consumer's EIP-191 signature when proof is provided
5. Query on-chain session state to verify funds are locked
6. Cache verified sessions for fast subsequent lookups (30 min TTL)

**No centralized token issuer needed** - the blockchain is the source of truth.

### Shared Sessions Across SDK Instances

One session can be used across multiple SDK instances **if they share the same wallet private key**:

```
                    One Wallet, Multiple SDK Instances

                        Consumer Wallet: 0xABC
                                  │
                                  │  One session, same proof
                                  │  $50 locked on-chain
                                  │
                ┌─────────────────┼─────────────────┐
                │                 │                 │
                ▼                 ▼                 ▼
        ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
        │  SDK Instance│  │  SDK Instance│  │  SDK Instance│
        │  (Laptop)    │  │  (Server)    │  │  (Lambda)    │
        │  same key    │  │  same key    │  │  same key    │
        └──────┬───────┘  └──────┬───────┘  └──────┬───────┘
               │                 │                 │
               │  Same proof     │  Same proof     │  Same proof
               │                 │                 │
               └─────────────────┼─────────────────┘
                                 │
                                 ▼
                        Provider Sidecars

        All usage aggregated under one session.
        Total spend across all instances capped at $50 locked amount.
```

**Important:** All instances must use the same private key to sign proofs. If you need different keys (e.g., for security isolation), use separate sessions:

```
                    Different Keys = Different Sessions

        Laptop Key: 0xAAA              Server Key: 0xBBB
              │                               │
              │  Session 1                    │  Session 2
              │  $25 locked                   │  $50 locked
              │                               │
              ▼                               ▼
        ┌──────────────┐              ┌──────────────┐
        │  SDK Instance│              │  SDK Instance│
        │  (Laptop)    │              │  (Server)    │
        └──────────────┘              └──────────────┘

        Each key has its own session with its own locked funds.
        More secure (keys isolated) but requires more locked capital.
```

**Recommendation:** For high-security environments, use separate sessions per deployment. For development or trusted environments, share the wallet key across instances.

### Checkpoints for Long Sessions

Checkpoints allow sessions to run indefinitely without locking large amounts upfront.

```
                         Session with Checkpoints

    Without checkpoints:
    └─ Lock $500 for 1-week session (ties up funds)

    With checkpoints:
    └─ Lock $50, checkpoint every 30 min or when 60% spent


    Timeline:
    ─────────────────────────────────────────────────────────────────────

    Session       Checkpoint     Checkpoint     Checkpoint        Session
    Start         #1             #2             #3          ...   End
    │             │              │              │                  │
    ▼             ▼              ▼              ▼                  ▼
    Lock $50      Settle $40     Settle $45     Settle $30        Final
                  (usage         (usage         (usage            settle
                  reported       reported       reported
                  bilaterally)   bilaterally)   bilaterally)


    After each checkpoint:
    • Actual usage settled on-chain (batched)
    • Unused funds remain locked for next period
    • If balance runs low, consumer tops up or session ends
```

**Checkpoint triggers (whichever comes first):**

| Trigger | Threshold | Use Case |
|---------|-----------|----------|
| Time-based | Every 30 minutes | Predictable settlement, cache refresh |
| Spend-based | When 60% of locked amount used | High-volume apps, multi-provider protection |

**Checkpoint flow with bilateral reconciliation:**

```
                         Checkpoint Reconciliation

    Consumer (all SDK instances)              Provider Sidecars
              │                                       │
              │  Report: "I used:"                    │  Report: "I served:"
              │  • $20 to Provider A                  │  • $20 from Consumer 0xABC
              │  • $15 to Provider B                  │  • ... from other consumers
              │                                       │
              └───────────────┬───────────────────────┘
                              │
                              ▼
                         Protocol
                              │
                              │  Compare claims:
                              │  • Consumer says $20 to Provider A
                              │  • Provider A says $20 from Consumer ✓
                              │
                              │  If mismatch < 5%: average and settle
                              │  If mismatch ≥ 5%: hold 48 hours for
                              │    escalation, then settle by average
                              │
                              ▼
                   Batch Settlement (on-chain)
                              │
                              │  • Deduct actual spend from locked
                              │  • Pay providers
                              │  • Remaining stays locked for next period
                              │
                              ▼
                      Session Continues
```

### Smart Contract: Full Session Lifecycle

```solidity
contract DINProtocol {
    // ... deposit and locked mappings ...
    mapping(address => bytes32) public activeSession;  // One active session per consumer

    struct Session {
        address consumer;
        uint256 maxSpend;
        uint256 lockedAmount;
        uint256 totalSettled;      // Running total settled at checkpoints
        uint64 validUntil;
        uint256 snapshotBlock;     // Block at which rate card prices were snapshotted
        bool active;
    }

    // Start session - consumer pays gas
    function startSession(uint256 maxSpend, uint64 duration) external returns (bytes32 sessionId) {
        // Enforce one active session per consumer
        require(activeSession[msg.sender] == bytes32(0), "Session already active");

        uint256 available = deposits[msg.sender] - locked[msg.sender];
        require(available >= maxSpend, "Insufficient balance");

        locked[msg.sender] += maxSpend;

        // Create session ID (unique per consumer + timestamp)
        sessionId = keccak256(abi.encodePacked(msg.sender, block.timestamp));

        // Track active session
        activeSession[msg.sender] = sessionId;

        sessions[sessionId] = Session({
            consumer: msg.sender,
            maxSpend: maxSpend,
            lockedAmount: maxSpend,
            totalSettled: 0,
            validUntil: uint64(block.timestamp) + duration,
            active: true
        });

        emit SessionStarted(sessionId, msg.sender, maxSpend);
    }

    // Checkpoint settlement - protocol pays gas (batched)
    function settleCheckpoint(
        bytes32 sessionId,
        uint256 periodSpend,
        ProviderPayment[] calldata payments
    ) external onlyProtocol {
        Session storage session = sessions[sessionId];
        require(session.active, "Session not active");

        // Verify spend doesn't exceed locked
        require(session.totalSettled + periodSpend <= session.lockedAmount, "Exceeds locked");

        // Update totals
        session.totalSettled += periodSpend;

        // Deduct from consumer deposit
        deposits[session.consumer] -= periodSpend;
        locked[session.consumer] -= periodSpend;

        // Pay providers directly
        for (uint i = 0; i < payments.length; i++) {
            USDC.transfer(payments[i].provider, payments[i].amount);
        }

        emit CheckpointSettled(sessionId, periodSpend, session.totalSettled);
    }

    // End session - protocol pays gas
    function endSession(bytes32 sessionId, uint256 finalSpend, ProviderPayment[] calldata payments) external onlyProtocol {
        Session storage session = sessions[sessionId];
        require(session.active, "Session not active");

        // Final settlement
        uint256 totalSpend = session.totalSettled + finalSpend;
        require(totalSpend <= session.lockedAmount, "Exceeds locked");

        // Unlock unused funds
        uint256 unused = session.lockedAmount - totalSpend;
        locked[session.consumer] -= unused;

        // Deduct final spend
        deposits[session.consumer] -= finalSpend;
        locked[session.consumer] -= finalSpend;

        // Pay providers directly
        for (uint i = 0; i < payments.length; i++) {
            USDC.transfer(payments[i].provider, payments[i].amount);
        }

        session.active = false;

        // Clear active session tracker (allows consumer to start new session)
        activeSession[session.consumer] = bytes32(0);

        emit SessionEnded(sessionId, totalSpend, unused);
    }

    // Reclaim funds from expired session - anyone can call (cleanup function)
    // This prevents consumer funds from being stuck forever if session expires without settlement
    uint64 public constant GRACE_PERIOD = 24 hours;

    function reclaimExpiredSession(bytes32 sessionId) external {
        Session storage session = sessions[sessionId];

        require(session.active, "Session not active");
        require(block.timestamp > session.validUntil + GRACE_PERIOD, "Session not expired");

        // Calculate unsettled amount (what was locked but never settled)
        uint256 unsettled = session.lockedAmount - session.totalSettled;

        // Unlock remaining funds back to consumer
        locked[session.consumer] -= unsettled;

        // Mark session as inactive
        session.active = false;

        // Clear active session tracker (allows consumer to start new session)
        activeSession[session.consumer] = bytes32(0);

        emit SessionReclaimed(sessionId, session.consumer, unsettled);
    }

    event SessionReclaimed(bytes32 indexed sessionId, address indexed consumer, uint256 amount);
}
```

### Consumer SDK Usage

```typescript
import { DinClient } from '@din-center/sdk';

const din = new DinClient({ privateKey: process.env.PRIVATE_KEY });

// 1. Deposit (one-time, on-chain)
await din.deposit(100);  // $100 USDC

// 2. Start session (on-chain, locks funds)
const session = await din.startSession({
  maxSpend: 50,                    // Lock $50
  paymentMode: 'cu',
  routingStrategy: 'balanced',
  checkpointInterval: '30m',       // Checkpoint every 30 minutes
});

// 3. Make requests (off-chain, uses session proof)
const result = await din.request('ethereum-mainnet', {
  method: 'eth_blockNumber',
});

// Proof automatically included in request
// Works from any SDK instance with same wallet

// 4. Check session status
const status = await din.getSessionStatus();
// {
//   sessionId: "0xabc...",
//   locked: 50,
//   spent: 12.50,
//   remaining: 37.50,
//   nextCheckpoint: "2025-12-18T14:00:00Z"
// }

// 5. End session (or let it expire)
await din.endSession();
// Unused funds automatically available again
```

### Session Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `maxSpend` | Amount to lock for session | Required |
| `paymentMode` | `"cu"` or `"request"` | `"cu"` |
| `routingStrategy` | `"cost"`, `"health"`, `"balanced"` | `"balanced"` |
| `minHealthThreshold` | Minimum provider health (0-1) | `0.5` |
| `duration` | Session validity period | `1h` |
| `checkpointInterval` | Time between checkpoints | `30m` |
| `services` | Allowed services | All |

---

## Usage Tracking

### Aggregated Tracking (Not Per-Request)

To avoid massive logs, usage is aggregated by key dimensions:

```
                        Aggregated Usage Tracking

    Instead of storing every request:

        { requestId: 1, provider: A, method: eth_call, CUs: 2, price: 0.00008 }
        { requestId: 2, provider: A, method: eth_call, CUs: 2, price: 0.00008 }
        { requestId: 3, provider: A, method: eth_call, CUs: 2, price: 0.00008 }
        ... millions of rows ...


    We aggregate by (provider, method, pricePerCU):

        {
          key: "0xProviderA:eth_call:0.00008",
          provider: "0xProviderA",
          method: "eth_call",
          pricePerCU: 0.00008,
          totalCUs: 6000,           // Aggregated
          requestCount: 3000,       // Number of requests
          totalCost: 0.48           // 6000 × 0.00008
        }
```

### Local Storage (JSON File)

For SDK/Router persistence, a simple JSON file:

```json
// ~/.din/session-log.json
{
  "sessionId": "0x7a3b...f9c2",
  "consumer": "0x1234...5678",
  "lockedAmount": 50000000,
  "spentAmount": 12500000,
  "lastCheckpoint": 1702857600,
  "usage": {
    "0xProviderA": {
      "eth_call": { "cus": 3000, "requests": 1000, "cost": 240000 },
      "eth_blockNumber": { "cus": 500, "requests": 500, "cost": 40000 }
    },
    "0xProviderB": {
      "eth_getLogs": { "cus": 2500, "requests": 100, "cost": 200000 }
    }
  }
}
```

**Simple and recoverable** - if SDK crashes, read JSON file on restart to resume session.

### Usage Update Flow

```
                            Usage Update Flow

    Request arrives
          │
          ▼
    1. Route to provider, get response

    2. Calculate usage:
       provider = "0xProviderA"
       method = "eth_call"
       CUs = 2
       pricePerCU = 0.00008
       cost = 0.00016

    3. Aggregate key = "0xProviderA:eth_call:0.00008"

    4. UPSERT into usage_aggregates:
       INSERT INTO usage_aggregates (...)
       ON CONFLICT (session_id, provider_address, method, price_per_cu)
       DO UPDATE SET
         total_cus = total_cus + 2,
         request_count = request_count + 1,
         total_cost = total_cost + 0.00016,
         last_updated = NOW()

    5. Update session totals:
       UPDATE sessions SET
         used_cus = used_cus + 2,
         used_spend = used_spend + 0.00016
       WHERE session_id = ?
```

---

## Bilateral Reconciliation

The protocol uses **Bilateral Reconciliation** for usage attestation - a two-party verification model where both consumers and providers independently track and report usage, then reconcile any differences before settlement.

### Why Bilateral Reconciliation?

With many SDKs, Routers, and Providers in the network, no single party can be fully trusted to report usage accurately:

| Party | Incentive to Lie |
|-------|------------------|
| Consumer | Under-report to pay less |
| Provider | Over-report to earn more |

By requiring **both parties to attest**, neither can unilaterally manipulate the system:

- Consumer submits usage claim
- Provider independently verifies against their logs
- Protocol compares and reconciles

### How It Works

```
                      Bilateral Reconciliation Flow

    CONSUMER SIDE                                  PROVIDER SIDE
    ─────────────                                  ─────────────

    SDK/Router tracks usage           ←─────────→ Provider logs requests
    locally during session                        served for session

          │                                             │
          │  Session ends or checkpoint                 │
          ▼                                             ▼

    Consumer submits:                            Provider submits:
    "I used 3000 CUs from                        "I served 3000 CUs to
     Provider A"                                  Consumer X"

          │                                             │
          └─────────────────┬───────────────────────────┘
                            │
                            ▼
                 ┌─────────────────────┐
                 │  PROTOCOL COMPARES  │
                 └──────────┬──────────┘
                            │
            ┌───────────────┼───────────────┐
            │               │               │
            ▼               ▼               ▼
         MATCH         MINOR DIFF       MAJOR DIFF
       (3000 = 3000)  (3000 vs 3100)  (3000 vs 4000)
            │               │               │
            ▼               ▼               ▼
         Settle         Average &      Hold 48 hours
       immediately       settle        for escalation
                       (3050 CUs)           │
                                            ▼
                                      No escalation?
                                      Settle by average
```

### Reconciliation Rules

| Scenario | Threshold | Action |
|----------|-----------|--------|
| Exact match | 0% difference | Settle immediately |
| Minor discrepancy | < 5% difference | Average and settle |
| Major discrepancy | ≥ 5% difference | Hold for 48 hours; if no escalation, settle by average |

### Trust Model

```
                              Trust Guarantees

    Consumer protected from:
    ─────────────────────────
    • Provider over-reporting (consumer's claim caps the charge)
    • Paying more than session max (signed authorization enforced)

    Provider protected from:
    ─────────────────────────
    • Consumer under-reporting (provider can dispute with logs)
    • Non-payment (consumer pre-deposited funds are locked)

    Protocol guarantees:
    ────────────────────
    • Settlement only occurs when both parties agree (or dispute resolved)
    • Neither party can unilaterally extract funds
    • Transparent reconciliation process
```

### Why Not Single-Party Attestation?

| Model | Problem |
|-------|---------|
| Trust consumer only | Consumer under-reports, providers don't get paid fairly |
| Trust provider only | Provider over-reports, consumers get overcharged |
| Trust protocol/router only | Single point of failure, centralization risk |
| **Bilateral reconciliation** | Neither party can cheat, disputes are resolvable |

---

## Settlement Coordinator

The Settlement Coordinator is an off-chain service (hosted by DIN initially) that orchestrates checkpoints and settlement. This is simpler than on-chain averaging while maintaining bilateral verification.

### Coordinator Responsibilities

```
                    Settlement Coordinator

    ┌─────────────────────────────────────────────────────────────────┐
    │  1. TRIGGER CHECKPOINTS                                         │
    │     - Emit checkpoint events every 30 minutes (time-based)      │
    │     - OR when any session reports 60% spend (spend-based)       │
    │     - Whichever comes first                                     │
    │                                                                 │
    │  2. COLLECT CLAIMS                                              │
    │     - Accept usage reports from all SDK instances               │
    │     - Accept usage reports from all providers                   │
    │     - Aggregate multiple SDK instance reports per session       │
    │                                                                 │
    │  3. RECONCILE                                                   │
    │     - Match consumer ↔ provider claims                          │
    │     - Apply averaging for mismatches (auto-settle)              │
    │     - Flag large mismatches (>10%) for manual review            │
    │                                                                 │
    │  4. SUBMIT ON-CHAIN                                             │
    │     - Batch multiple settlements into one transaction           │
    │     - Coordinator pays gas (funded by protocol fee)             │
    │     - Can batch daily to reduce transfers if desired            │
    └─────────────────────────────────────────────────────────────────┘
```

### Off-Chain Reconciliation Flow

```
                         Off-Chain Reconciliation

    SDK Instance 1    SDK Instance 2    Provider A       Provider B
         │                  │                │                │
         │  Report usage    │  Report usage  │  Report usage  │  Report usage
         │  for session X   │  for session X │  from session X│  from session X
         │                  │                │                │
         └────────┬─────────┴────────┬───────┴────────┬───────┘
                  │                  │                │
                  ▼                  ▼                ▼
         ┌──────────────────────────────────────────────────────────┐
         │              SETTLEMENT COORDINATOR                       │
         │                                                           │
         │  1. Aggregate SDK instances:                              │
         │     Instance 1: $7.00 to Provider A, $3.00 to Provider B  │
         │     Instance 2: $5.00 to Provider A, $2.00 to Provider B  │
         │     Total consumer claim: $12.00 to A, $5.00 to B         │
         │                                                           │
         │  2. Compare with provider claims:                         │
         │     Provider A claims: $12.50 from session X              │
         │     Provider B claims: $5.00 from session X               │
         │                                                           │
         │  3. Reconcile:                                            │
         │     Provider A: Mismatch $0.50 (4%) → Average: $12.25     │
         │     Provider B: Exact match → Settle: $5.00               │
         │                                                           │
         │  4. Get signatures from both parties on agreed amounts    │
         │                                                           │
         └──────────────────────────┬────────────────────────────────┘
                                    │
                                    ▼
                          Submit to contract
                          (both signatures attached)
```

### Checkpoint Timing

Checkpoints are triggered by **whichever comes first**:

| Trigger | Threshold | Rationale |
|---------|-----------|-----------|
| Time-based | Every 30 minutes | Predictable settlement for all sessions |
| Spend-based | When 60% of locked amount used | Protect providers from session exhaustion |

**Why 30 minutes?**
- Multiple providers may serve the same session
- Each provider's cache becomes stale faster with concurrent usage
- 30-minute checkpoints ensure providers can re-verify on-chain state regularly

### Who Runs the Coordinator?

**Phase 1 (Launch):** DIN team hosts the coordinator service
- Simple, centralized, fast iteration
- Coordinator has no custody of funds (just orchestrates signatures)
- If coordinator goes offline, sessions continue but settlement is delayed

**Phase 2 (Decentralization):** Multiple coordinator nodes
- Keeper network or dedicated nodes
- Any node can trigger checkpoints and collect claims
- Redundancy eliminates single point of failure

### Why Off-Chain?

Off-chain reconciliation provides significant advantages over fully on-chain approaches:

| Aspect | On-Chain | Off-Chain (Current) |
|--------|----------|---------------------|
| Gas costs | High (every claim on-chain) | Low (only final settlement) |
| Complexity | Complex on-chain averaging logic | Simple off-chain aggregation |
| Flexibility | Hard to update | Easy to iterate |
| Speed | Limited by block times | Instant reconciliation |
| Privacy | All claims public | Only settlements public |

**Key insight:** The coordinator has no custody of funds - it only orchestrates the collection of signed claims. The smart contract remains the source of truth for fund locking and settlement. If the coordinator is compromised or offline:
- Existing sessions continue working (providers verify on-chain)
- Settlement is delayed but not lost
- Funds remain secure in the contract

This provides the security of on-chain while reducing costs and complexity.

---

## Settlement Flow

### Session End / Checkpoint

When a session ends or at periodic checkpoints:

```
                              Settlement Flow

    Consumer (SDK/Router)             Protocol                  Providers
          │                               │                         │
          │  1. Session ends or           │                         │
          │     checkpoint interval       │                         │
          │                               │                         │
          │  2. Submit UsageClaim:        │                         │
          │  {                            │                         │
          │    sessionId: "abc123",       │                         │
          │    aggregates: [              │                         │
          │      { provider: "0xProvA",   │                         │
          │        method: "eth_call",    │                         │
          │        totalCUs: 3000,        │                         │
          │        totalCost: 0.24 },     │                         │
          │      { provider: "0xProvB",   │                         │
          │        method: "eth_getLogs", │                         │
          │        totalCUs: 2000,        │                         │
          │        totalCost: 0.20 }      │                         │
          │    ]                          │                         │
          │  }                            │                         │
          │───────────────────────────────▶│                         │
          │                               │                         │
          │                               │  3. Request confirmation│
          │                               │     from providers      │
          │                               │────────────────────────▶│
          │                               │                         │
          │                               │  4. Providers confirm   │
          │                               │     or dispute          │
          │                               │◀────────────────────────│
          │                               │                         │
          │                               │  5. Reconcile & settle  │
          │                               │                         │
```

### Provider Confirmation

Providers track their own usage and confirm or dispute:

```
                          Provider Confirmation

    Protocol                                       Provider
        │                                              │
        │  "Session abc123 claims                      │
        │   3000 CUs from you at $0.00008/CU"          │
        │─────────────────────────────────────────────▶│
        │                                              │
        │                                              │  Check local logs...
        │                                              │
        │  Option A: Confirm                           │
        │  { status: "confirmed",                      │
        │    sessionId: "abc123",                      │
        │    agreedCUs: 3000 }                         │
        │◀─────────────────────────────────────────────│
        │                                              │
        │  Option B: Dispute                           │
        │  { status: "disputed",                       │
        │    sessionId: "abc123",                      │
        │    claimedCUs: 3000,                         │
        │    actualCUs: 3200,                          │
        │    evidence: "..." }                         │
        │◀─────────────────────────────────────────────│
```

### On-Chain Batch Settlement

Settlement happens at checkpoints (every 30 minutes) or session end. Both consumer and provider submit signed usage claims, and the contract reconciles them.

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/utils/cryptography/ECDSA.sol";

contract DINProtocol {
    using ECDSA for bytes32;

    IERC20 public immutable usdc;

    // Consumer locked funds per session
    mapping(bytes32 => Session) public sessions;

    struct Session {
        address consumer;
        uint256 lockedAmount;
        uint256 spentAmount;
        bool active;
    }

    struct UsageClaim {
        bytes32 sessionId;
        address provider;
        uint256 amount;          // USDC amount (6 decimals)
        uint256 timestamp;
    }

    struct SignedClaim {
        UsageClaim claim;
        bytes consumerSig;       // Consumer signs their claim
        bytes providerSig;       // Provider signs their claim
    }

    // Settlement with bilateral reconciliation - pays provider directly
    function settleBatch(SignedClaim[] calldata claims) external {
        for (uint i = 0; i < claims.length; i++) {
            SignedClaim calldata sc = claims[i];
            Session storage session = sessions[sc.claim.sessionId];

            require(session.active, "Session not active");

            // Verify consumer signature
            bytes32 claimHash = keccak256(abi.encode(
                sc.claim.sessionId,
                sc.claim.provider,
                sc.claim.amount,
                sc.claim.timestamp
            ));
            address recoveredConsumer = claimHash.toEthSignedMessageHash().recover(sc.consumerSig);
            require(recoveredConsumer == session.consumer, "Invalid consumer signature");

            // Verify provider signature
            address recoveredProvider = claimHash.toEthSignedMessageHash().recover(sc.providerSig);
            require(recoveredProvider == sc.claim.provider, "Invalid provider signature");

            // Transfer from consumer's locked funds directly to provider wallet
            uint256 amount = sc.claim.amount;
            require(session.lockedAmount - session.spentAmount >= amount, "Insufficient locked funds");

            session.spentAmount += amount;

            // Direct transfer to provider - no withdrawal step needed
            usdc.transfer(sc.claim.provider, amount);

            emit UsageSettled(sc.claim.sessionId, sc.claim.provider, amount);
        }
    }

    event UsageSettled(bytes32 indexed sessionId, address indexed provider, uint256 amount);
}
```

**Settlement flow:**

```
                         Checkpoint Settlement

    Consumer SDK                Protocol Contract              Provider Sidecar
         │                            │                              │
         │                            │                              │
         │  1. Sign usage claim       │       1. Sign usage claim    │
         │     (I used $12 at         │          (I served $12 to    │
         │      Provider X)           │           Consumer Y)        │
         │                            │                              │
         └────────────┬───────────────┴──────────────┬───────────────┘
                      │                              │
                      │     2. Submit both signed    │
                      │        claims to contract    │
                      │                              │
                      └──────────────┬───────────────┘
                                     │
                                     ▼
                      ┌──────────────────────────────┐
                      │  Contract verifies:          │
                      │  • Consumer signature ✓      │
                      │  • Provider signature ✓      │
                      │  • Amounts match ✓           │
                      │  • Funds available ✓         │
                      │                              │
                      │  Then:                       │
                      │  • Debit consumer locked     │
                      │  • Transfer USDC directly    │
                      │    to provider wallet        │
                      └──────────────────────────────┘
```

**Who submits settlements?**
- Either consumer or provider can submit the transaction
- Both signatures are required, so neither can cheat

---

## Provider Earnings

Providers receive USDC directly to their wallet at settlement - no withdrawal step needed.

```
                         Direct Payment Flow

    Settlement                              Provider Wallet
        │                                        │
        │  settleBatch() executes                │
        │                                        │
        │  usdc.transfer(provider, amount)       │
        │───────────────────────────────────────▶│
        │                                        │
        │                                        │  USDC received
        │                                        │  immediately
```

### Settlement Frequency Options

The Settlement Coordinator can batch transfers to reduce transaction count:

| Mode | Transfer Frequency | Use Case |
|------|-------------------|----------|
| **Immediate** | Every 30-min checkpoint | Real-time earnings, higher gas cost |
| **Daily batch** | Once per day | Reduced gas, 24 aggregated settlements per provider |

**Daily batching** sums all of a provider's earnings across the day and transfers once:

```
Provider A earnings:
- Checkpoint 1: $5.00
- Checkpoint 2: $8.00
- Checkpoint 3: $3.00
- ... (48 checkpoints/day)

Daily batch: Single $250 transfer instead of 48 transfers
```

**Trade-off:** Providers wait up to 24 hours for earnings vs. receiving every 30 minutes. Configurable per provider or protocol-wide.

### Provider SDK (Stats Only)

```typescript
const provider = new DinProvider({
  privateKey: process.env.PROVIDER_KEY,
});

// Get usage stats (earnings are in wallet, not contract)
const stats = await provider.getStats({ period: '7d' });
// {
//   requestsServed: 2_500_000,
//   cusServed: 5_200_000,
//   revenue: 416.00,
//   uniqueConsumers: 342
// }
```

---

## Failure Handling

### SDK/Router Crash Recovery

With JSON file persistence, usage survives crashes:

```
                           Crash Recovery Flow

    Normal Operation:
    1. SDK tracks usage in ~/.din/session-log.json
    2. JSON file updated after each request (or batched)
    3. Periodic checkpoints (every 30 minutes)
    4. On graceful shutdown, submit final claim

    After Crash:
    1. SDK restarts, reads ~/.din/session-log.json
    2. Finds incomplete session with usage
    3. Resume session if still valid
    4. Submit checkpoint claim with recovered usage data
```

**JSON file is simple and sufficient** - no database needed for tracking session state.

---

## Dispute Resolution

### Auto-Average Settlement (Default)

All claim mismatches are resolved by averaging - no manual intervention needed:

```
Consumer claims: $12.00
Provider claims: $15.00
Settlement: $13.50 (average)
```

**Why this works:**
- Neither party gains significantly by lying
- If consumer lies low: saves at most half the difference
- If provider lies high: gains at most half the difference
- Small incentive to lie, not worth reputation damage
- Fully automated, no arbitration overhead

```
                         Auto-Average Settlement

    Consumer                 Contract                  Provider
        │                       │                          │
        │  Claim: $12.00        │         Claim: $15.00    │
        │──────────────────────▶│◀─────────────────────────│
        │                       │                          │
        │                       │  Calculate average:      │
        │                       │  ($12 + $15) / 2 = $13.50│
        │                       │                          │
        │                       │  Settle at $13.50        │
        │                       │──────────────────────────▶│
        │                       │                          │
        │  Debit $13.50         │                          │
        │◀──────────────────────│                          │
```

### Escalation for Malicious Actors

If a consumer suspects a provider is consistently inflating claims, they can escalate. The timeline differs based on discrepancy size.

**Time Windows:**

| Phase | Duration | Action Required |
|-------|----------|-----------------|
| Major discrepancy hold | 48 hours after detection | Settlement held; either party can escalate |
| Post-settlement window | 7 days after settlement | Consumer can still escalate after auto-average |
| Provider response | 7 days after escalation | Provider must submit proof |
| DIN team review | 14 days after response | Team makes decision |
| Penalty applied (if guilty) | Immediate | AVS reward deducted + reputation penalty |

**Escalation Flow:**

```
                         Escalation Flow with Time Bounds

    FOR MAJOR DISCREPANCIES (≥5%):
    ──────────────────────────────

    Hour 0: Discrepancy detected
        │
        │  Settlement HELD (not processed)
        │  48-hour window for escalation
        │
        ├─── Escalation filed during hold?
        │       │
        │       ▼
        │     Skip to "Escalation Process" below
        │
        └─── No escalation within 48 hours?
                │
                ▼
              Settle by average (auto-averaged)
              Consumer still has 7 days to escalate after settlement


    FOR MINOR DISCREPANCIES (<5%) OR POST-SETTLEMENT:
    ──────────────────────────────────────────────────

    Day 0: Settlement occurs (auto-averaged)
        │
        │  Consumer has 7 days to escalate
        │
    Day 7: Reporting window closes


    ESCALATION PROCESS:
    ───────────────────

    Day 0: Consumer files escalation
        │  - Provides session IDs
        │  - Provides local usage logs (JSON file)
        │  - Pays escalation fee ($25, refunded if valid)
        │  - Must complete identity verification (see below)
        │
        │  Provider notified, has 7 days to respond
        │
    Day 7: Provider response deadline
        │
        ├─── Provider submits proof (server logs)
        │       │
        │       ▼
        │    DIN team reviews (14 days)
        │       │
        │       ├─── Provider at fault
        │       │       • 25% AVS reward deducted (1st strike)
        │       │       • Consumer refunded from deduction
        │       │       • Strike added to provider record
        │       │       • Health score reduced
        │       │
        │       └─── Consumer at fault
        │               • Consumer loses escalation fee
        │               • Warning added to consumer record
        │
        └─── Provider fails to respond
                │
                ▼
             Provider penalized via AVS rewards
                │
                • 25% AVS reward deducted (1st offense)
                • Strike added to record
                • 3rd no-response → Removed + slashed
```

**Consequences:**

Penalties are deducted from the provider's AVS staking rewards - no separate fee payment required.

| Offense | 1st Strike | 2nd Strike | 3rd Strike |
|---------|------------|------------|------------|
| Provider guilty or no-response | 25% reward reduction | 50% reward reduction | 100% reward reduction + removed from protocol |
| Consumer false claim | Lose escalation fee | 2x fee | Banned from escalations |

### Escalation Fee

The escalation fee prevents spam escalations while protecting legitimate claims:

| Outcome | Fee Handling |
|---------|--------------|
| Consumer's claim validated | Fee refunded in full ($25 returned) |
| Provider found at fault | Fee refunded + consumer compensated from provider's AVS reward deduction |
| Consumer's claim invalid | Fee forfeited ($25 kept by protocol) |
| Consumer's claim malicious | Fee forfeited + consumer banned from future escalations |

**How it works:**

```
                         Escalation Fee Flow

    Consumer                  Protocol Treasury              Provider
        │                            │                          │
        │  Pay $25 escrow            │                          │
        │───────────────────────────▶│                          │
        │                            │                          │
        │                    Fee held in escrow                 │
        │                            │                          │
        │  ... investigation ...     │                          │
        │                            │                          │
        │                            │                          │
    Outcome A: Consumer valid        │                          │
        │                            │                          │
        │  Refund $25                │  Deduct from AVS rewards │
        │◀───────────────────────────│─────────────────────────▶│
        │                            │                          │
        │  + Compensation from       │                          │
        │    provider's reward       │                          │
        │                            │                          │
    Outcome B: Consumer invalid      │                          │
        │                            │                          │
        │  Fee forfeited             │                          │
        │  (stays in treasury)       │                          │
```

**Why $25?**
- High enough to deter frivolous escalations
- Low enough to not discourage legitimate claims
- Covers administrative cost of manual review

### Identity Verification (Anti-Abuse)

To prevent consumers from creating new wallets to spam escalations, identity verification is required:

**Requirements:**

| Verification Type | Description | When Required |
|-------------------|-------------|---------------|
| Wallet linking | Consumer must have minimum $100 lifetime spend | Always (automated) |
| Email verification | Consumer provides email, receives confirmation | First escalation |
| KYC (optional) | Full identity verification | After 2 escalations |

**How it prevents abuse:**

```
                    Anti-Abuse Mechanism

    Attack: Create new wallet → Spam escalation → Create another wallet

    Defense:
    ┌─────────────────────────────────────────────────────────────┐
    │  1. Minimum spend requirement ($100 lifetime)               │
    │     New wallets can't escalate until they've spent $100     │
    │     Cost to spam: $100 + $25 fee per fake account          │
    │                                                             │
    │  2. Email verification (linked to consumer identity)        │
    │     Same email can't be used for multiple escalations       │
    │     across different wallets within 90 days                 │
    │                                                             │
    │  3. Progressive KYC                                         │
    │     After 2 escalations: KYC required                       │
    │     Creates legal accountability for false claims           │
    └─────────────────────────────────────────────────────────────┘
```

**Verified consumer definition:**

A consumer is considered "verified" when they meet ALL of the following:

| Requirement | How It's Checked |
|-------------|------------------|
| $100+ lifetime spend | Automated - protocol tracks total settled usage per wallet |
| Email verified | Consumer submits email, clicks confirmation link |
| No active ban | No 2+ false claims on record for this identity |

The $100 spend requirement is checked automatically from on-chain settlement history. Email verification happens once when filing first escalation - the email becomes permanently linked to that wallet address.

**Rate limits:**

| Consumer Status | Escalation Limit |
|-----------------|------------------|
| Unverified (< $100 spend OR no email) | Cannot escalate |
| Verified (meets all requirements) | 3 escalations per 90 days |
| Verified + 1 prior false claim | 1 escalation per 180 days |
| Verified + 2+ false claims | Banned from escalations |

**Escalation tracking:**

```typescript
// Protocol tracks escalations per verified identity, not per wallet
interface EscalationRecord {
  identityHash: string;           // SHA256(email) or KYC reference
  linkedWallets: string[];        // All wallets this identity has used
  escalationCount: number;        // Total escalations filed
  validatedCount: number;         // Escalations that were valid
  falseClaimCount: number;        // Escalations that were false
  lastEscalation: Date;
  banned: boolean;
}
```

This ensures that:
- Creating new wallets doesn't reset escalation history
- Genuine consumers can escalate legitimate concerns
- Bad actors are identified and banned across all their wallets

**AVS Reward Deduction Examples:**

```
Provider monthly AVS reward: $1,000

1st Strike (25% reduction):
  - Deducted: $250
  - Provider receives: $750
  - Consumer refunded from deduction

2nd Strike (50% reduction):
  - Deducted: $500
  - Provider receives: $500
  - Consumer refunded from deduction

3rd Strike (100% reduction + removal):
  - Deducted: $1,000
  - Provider receives: $0
  - Provider removed from protocol
  - Consumer refunded from deduction
```

**Additional Reputation Penalties:**
- **Health score reduced** - Watcher marks provider as less reliable
- **Less traffic routed** - Routing algorithms deprioritize low-reputation providers

```
                    Reputation Impact on Provider

    Before dispute:                  After guilty verdict:
    ┌────────────────────┐           ┌────────────────────┐
    │  Health Score: 95  │           │  Health Score: 75  │
    │  AVS Reward: 100%  │           │  AVS Reward: 75%   │
    │  Traffic Share: 25%│           │  Traffic Share: 15%│
    └────────────────────┘           └────────────────────┘

    Result: Provider earns less until they rebuild reputation
```

**Escalation is rare** - auto-averaging handles 99% of cases. Escalation is only for repeated, significant abuse.

### Edge Case Resolution

**Provider Offline During Settlement:**

If a provider is unresponsive during settlement:

```
Provider Offline Scenario
─────────────────────────

Checkpoint triggered (30 min or 60% spend)
    │
    │  Consumer submits usage claim
    │  Provider fails to respond within 15 minutes
    │
    ▼
Fallback: Settle based on consumer's claim
    │
    │  Provider can dispute later (within 7 days)
    │  if they have evidence of higher usage
    │
    ▼
Session continues with consumer's claimed amount
```

The session log (`~/.din/session-log.json`) is authoritative for the consumer's view.

**Network Congestion Handling:**

When network congestion prevents timely settlement:

| Scenario | Action |
|----------|--------|
| Settlement tx pending > 10 min | Extend session validity by 30 min, retry with priority gas |
| Settlement tx failed | Retry with increased gas, notify consumer |
| Prolonged congestion (>1 hour) | Pause new sessions, complete existing |

**Ambiguous Evidence Cases:**

When both parties provide seemingly valid evidence and fault cannot be determined:
- Neither party is penalized
- Settlement at midpoint (split the difference)
- Flag for monitoring (future sessions tracked closely)

### Future Dispute Enhancements

The following functionality is planned for future releases:

| Feature | Description | Status |
|---------|-------------|--------|
| **Session Pause** | Temporarily pause a session during investigation | Planned |
| **Provider Suspension** | Temporarily suspend a provider pending review | Planned |
| **Consumer Blacklisting** | Block repeat bad actors across all wallets | Planned |
| **Delegated Signing** | Allow consumers to delegate session signing to a hot wallet | Planned |
| **Automated Pattern Detection** | ML-based detection of suspicious claim patterns | Future |

---

## Smart Contract Architecture

### Protocol Contract

```solidity
contract DINProtocol {
    // Token
    IERC20 public immutable usdc;

    // Consumer accounts
    mapping(address => ConsumerAccount) public consumers;

    // Provider rate cards
    mapping(address => ProviderRateCard) public providerRates;

    // Session tracking
    mapping(bytes32 => bool) public sessionSettled;

    // Consumer functions
    function deposit(uint256 amount) external;
    function withdraw(uint256 amount) external;

    // Provider functions
    function registerRateCard(RateCardParams calldata params) external;
    function updateRateCard(RateCardParams calldata params) external;
    // Note: No withdrawEarnings - providers receive USDC directly at settlement

    // Settlement functions (protocol operator)
    function settleBatch(Settlement[] calldata settlements) external;  // Pays providers directly
    function settleDispute(bytes32 sessionId, DisputeResolution resolution) external;

    // View functions
    function getConsumerBalance(address consumer) external view returns (uint256);
    function getProviderRateCard(address provider) external view returns (RateCard);
}
```

### Events

```solidity
event ConsumerDeposited(address indexed consumer, uint256 amount);
event ConsumerWithdrawn(address indexed consumer, uint256 amount);
event ProviderRateCardUpdated(address indexed provider);
event SessionSettled(bytes32 indexed sessionId, address indexed consumer, uint256 totalCost);
event ProviderPaid(address indexed provider, uint256 amount);  // Direct payment at settlement
event DisputeRaised(bytes32 indexed sessionId, address indexed raiser);
event DisputeResolved(bytes32 indexed sessionId, uint256 settledAmount);
```

---

## SDK Implementation

### Consumer SDK

```typescript
import { DinClient } from '@din-center/sdk';

// Initialize
const din = new DinClient({
  privateKey: process.env.PRIVATE_KEY,
  persistencePath: '~/.din/session-log.json',  // JSON file for crash recovery
});

// Deposit funds (Phase 1)
await din.deposit({ amount: 100 }); // $100 USDC

// Configure service preferences (optional - restricts which services can be used)
await din.setPreferences({
  services: ['ethereum-mainnet', 'polygon-mainnet', 'ethereum-beacon'],
});

// Start session with PAYMENT MODE and ROUTING STRATEGY (signs once)
const session = await din.startSession({
  // Payment preferences
  paymentMode: 'cu',         // 'cu' or 'request' - determines provider pool
  maxCUsPerRequest: 100,     // Max CUs per request (filter providers in CU mode)
  maxSpend: 10,              // Max USDC spend for entire session

  // Routing preferences
  routingStrategy: 'balanced',  // 'cost', 'health', or 'balanced' (default)
  minHealthThreshold: 0.5,      // Minimum provider health (0-1, default 0.5)

  // Session bounds
  duration: 3600,         // 1 hour
});

// routingStrategy options:
// - 'cost': Always pick cheapest provider (deterministic)
// - 'health': Weighted random by health score (current DIN behavior)
// - 'balanced': Weighted random by value score (health × cost efficiency)

// Make requests (no additional signing)
// Protocol routes to providers supporting your paymentMode
const result = await din.request('ethereum-mainnet', {
  method: 'eth_call',
  params: [{ to: '0x...', data: '0x...' }, 'latest'],
});

// Check session usage
const usage = await din.getSessionUsage();
// {
//   sessionId: "abc123",
//   paymentMode: "cu",
//   usedCUs: 5000,
//   usedSpend: 0.40,          // 5000 CUs × $0.00008/CU
//   remainingCUs: 95000,
//   remainingSpend: 9.60,
//   byProvider: {
//     "0xProviderA": { CUs: 3000, cost: 0.24 },
//     "0xProviderB": { CUs: 2000, cost: 0.16 }
//   }
// }

// End session (submits claim)
await din.endSession();

// Check balance
const balance = await din.getBalance();
// { available: 99.60, pending: 0, totalSpent: 0.40 }
```

**Per-Request Payment Mode Example:**

```typescript
// For simpler pricing or REST APIs, use per-request mode
const session = await din.startSession({
  // Payment preferences
  paymentMode: 'request',  // Pay per-request
  maxSpend: 10,            // Max USDC spend (no CU limits in request mode)

  // Routing preferences
  routingStrategy: 'cost', // Use cheapest provider (great for batch jobs)
  minHealthThreshold: 0.6, // Slightly higher quality floor

  // Session bounds
  duration: 3600,
});

// Requests route to providers with supportsRequestPricing=true
const beaconResult = await din.request('ethereum-beacon', {
  endpoint: '/eth/v1/beacon/states/head/validators',
});

// Usage tracked by direct cost
const usage = await din.getSessionUsage();
// {
//   sessionId: "xyz789",
//   paymentMode: "request",
//   usedSpend: 0.001,        // Provider's direct price
//   remainingSpend: 9.999,
//   byProvider: {
//     "0xProviderA": { requests: 1, cost: 0.001 }
//   }
// }
```

### Provider SDK

```typescript
import { DinProvider } from '@din-center/provider-sdk';

// Initialize
const provider = new DinProvider({
  privateKey: process.env.PROVIDER_KEY,
  rpcEndpoint: 'http://localhost:8545',
});

// Register rate card with PRICING MODE FLAGS
// Set which payment modes you accept - this determines which consumers can be routed to you
await provider.registerRateCard({
  services: {
    'ethereum-mainnet': {
      serviceType: 'evm',

      // PRICING MODE FLAGS (at least one must be true)
      supportsCUPricing: true,       // Accept CU-based payments
      supportsRequestPricing: true,  // Accept per-request payments

      // CU-BASED PRICING (used when consumer paymentMode='cu')
      defaultCUCost: 5,
      methodCUs: {
        'eth_call': 10,
        'eth_getLogs': 50,
        'debug_traceCall': 500,
      },

      // PER-REQUEST PRICING (used when consumer paymentMode='request')
      defaultPricePerRequest: 0.001,  // $0.001 USDC
      methodPrices: {
        'eth_call': 0.0008,
        'eth_getLogs': 0.004,
        'debug_traceCall': 0.05,
      },
    },

    'ethereum-beacon': {
      serviceType: 'beacon-chain',

      // This service supports BOTH payment modes
      supportsCUPricing: true,
      supportsRequestPricing: true,

      // CU costs
      defaultCUCost: 1,
      methodCUs: {
        '/eth/v1/beacon/genesis': 1,
        '/eth/v1/beacon/states/{state_id}/root': 2,
        '/eth/v1/beacon/states/{state_id}/finality_checkpoints': 3,
        '/eth/v1/beacon/states/{state_id}/committees': 10,
      },

      // Direct prices
      defaultPricePerRequest: 0.0001,
      methodPrices: {
        '/eth/v1/beacon/states/{state_id}/validators': 0.001,
        '/eth/v1/beacon/states/{state_id}/validator_balances': 0.0005,
      },
    },

    'bitcoin-esplora': {
      serviceType: 'bitcoin-esplora',

      // This service ONLY supports per-request pricing
      supportsCUPricing: false,
      supportsRequestPricing: true,

      // Only methodPrices needed
      defaultPricePerRequest: 0.0001,
      methodPrices: {
        '/address/{address}': 0.0001,
        '/address/{address}/txs': 0.0002,
        '/tx/{txid}': 0.0001,
      },
    },
  },
});

// Provider serves requests through DIN routing
// Consumers with paymentMode='cu' will be routed to services with supportsCUPricing=true
// Consumers with paymentMode='request' will be routed to services with supportsRequestPricing=true

// Check earnings stats (USDC is paid directly to wallet at settlement)
const stats = await provider.getStats();
// {
//   pending: 45.50,        // Current period, not yet settled
//   totalEarned: 1530.30,  // Lifetime earnings (paid directly to wallet)
//   byPaymentMode: {
//     cu: { revenue: 180.00, requests: 2_000_000 },
//     request: { revenue: 54.80, requests: 500_000 }
//   }
// }
// Note: No withdraw needed - USDC is transferred directly to provider wallet at each settlement
```

---

## x402 Alternative Payment Path

For users without a deposit, x402 micropayments provide an alternative entry point:

```
                    Payment Path Selection

    New request arrives
           │
           │  Has active session with locked funds?
           │
           ├── YES ──▶ Use Session-Based Payment (this RFC)
           │           • Lower overhead per request
           │           • Faster (no payment per request)
           │           • Better for regular users
           │
           └── NO ───▶ Use x402 Micropayments
                       • No deposit required
                       • Pay per request
                       • Higher per-request cost
                       • Good for first-time/low-volume users
```

**When to use x402:**

| Use Case | Recommendation |
|----------|----------------|
| First-time user | x402 (try before depositing) |
| One-off request | x402 (not worth depositing) |
| Low volume (< 100 requests/month) | x402 (simpler) |
| Regular usage | Session-based (cheaper) |
| High volume | Session-based (much cheaper) |

**Onboarding flow:**

```
1. New user makes request with no session
2. Provider returns 402 with x402 payment requirements
3. User pays per-request via x402 ($0.001-0.01 per request)
4. After $10+ spent via x402, SDK prompts: "Deposit $50 and save 30%?"
5. User deposits, enjoys session-based rates going forward
```

x402 is the **onboarding ramp** - easy to start, graduate to sessions for savings.

---

## Protocol Fee (Ideation)

> **Note:** This section is for ideation purposes. Protocol fee is **not necessarily planned** for the initial launch. If implemented, the fee would primarily fund Settlement Coordinator operations.

A protocol fee could support sustainable operations:

```
                    Protocol Fee Flow (Example: 5%)

    Consumer spends $100 during session
                │
                ▼
    Settlement breakdown:
    ┌─────────────────────────────────┐
    │  Gross spend:        $100.00    │
    │  Protocol fee (5%):  -$5.00     │
    │  Net to providers:   $95.00     │
    └─────────────────────────────────┘
                │
                ▼
    Provider distribution:
    ┌─────────────────────────────────┐
    │  Provider A (60%):   $57.00     │
    │  Provider B (40%):   $38.00     │
    └─────────────────────────────────┘
```

**Potential fee usage:**

| Use | Description |
|-----|-------------|
| Settlement Coordinator gas | Covers transaction costs for batched settlements |
| Infrastructure | Coordinator service hosting, monitoring |
| Protocol development | Ongoing improvements, audits |
| Ecosystem grants | Developer incentives, integrations |

**Fee scenarios at different rates:**

| Fee Rate | Consumer Spend | Protocol Revenue | Provider Revenue |
|----------|----------------|------------------|------------------|
| 2% | $1,000 | $20 | $980 |
| 5% | $1,000 | $50 | $950 |
| 10% | $1,000 | $100 | $900 |

**Contract implementation (if implemented):**

```solidity
uint256 public protocolFeeBps = 500;  // 5% = 500 basis points (0 for no fee)

function settleBatch(SignedClaim[] calldata claims) external {
    uint256 totalFees = 0;

    for (uint i = 0; i < claims.length; i++) {
        uint256 grossAmount = claims[i].amount;
        uint256 fee = (grossAmount * protocolFeeBps) / 10000;
        uint256 netAmount = grossAmount - fee;

        totalFees += fee;

        // Pay provider net amount
        usdc.transfer(claims[i].provider, netAmount);
    }

    // Protocol fee to treasury (if any)
    if (totalFees > 0) {
        usdc.transfer(protocolTreasury, totalFees);
    }
}
```

---

## Success Metrics

1. **Consumer UX** - Time to first request (< 30 seconds)
2. **Routing efficiency** - Requests served by optimal provider (> 90%)
3. **Settlement accuracy** - Disputes rate (< 1%)
4. **Provider earnings** - Average time to payment (< 24 hours)
5. **Service health** - Automatic failover success rate (> 99%)

---

## Open Questions

1. **Governance** - Who operates the protocol initially? Path to decentralization?
2. **Staking** - Should providers stake for quality guarantees?
3. **Cross-chain** - Settlement on Base, providers on multiple chains?

---

## Next Steps

1. Design and deploy Protocol contract on Base testnet (Sepolia)
2. Implement session-based authorization in SDK
3. Build aggregated usage tracking with JSON file persistence
4. Create provider rate card registration system
5. Implement settlement engine with dispute resolution
6. Integration testing with multiple providers
7. Mainnet deployment

---

## Related Documents

- [x402 Payment Protocol](https://github.com/coinbase/x402) - Underlying payment protocol
- [DIN Router SDK](/docs/din-router-sdk/) - Router architecture
- [DIN Protocol](/docs/din-protocol/) - Unified registry and payment system
