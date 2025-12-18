# Pricing and Routing

## Overview

This document covers how providers set pricing and how the protocol routes requests to providers based on consumer preferences.

## Pricing Models

Two pricing models are available. **Consumers choose their payment mode upfront**, and the protocol only routes to providers that support that mode.

### CU-Based Pricing

Compute Unit (CU) pricing provides standardized pricing across providers.

```
CU-BASED PRICING
----------------
- Protocol sets: Price per CU (universal, e.g., $0.00008/CU)
- Providers set: Method/endpoint CU costs via methodCUs mapping
- Cost = methodCUs[method] x PROTOCOL_PRICE_PER_CU
- Provider must set: supportsCUPricing = true

Key insight: The protocol sets the CU price (universal).
Providers can only set how many CUs each method costs.
```

**Cost calculation example:**

```
Protocol Price: $0.00008/CU (set by governance)

eth_call on ethereum-mainnet:
  Provider A: 10 CUs x $0.00008/CU = $0.0008
  Provider B: 5 CUs x $0.00008/CU = $0.0004  <-- More efficient!

Providers compete by being more efficient (lower CU costs).
```

### Per-Request Pricing

Direct pricing per method - simpler but less standardized.

```
PER-REQUEST PRICING
-------------------
- Providers set: Direct price via methodPrices mapping
- Cost = methodPrices[method] (direct USDC price)
- Provider must set: supportsRequestPricing = true
```

**Cost calculation example:**

```
eth_call on ethereum-mainnet (Provider A):
  Cost = $0.0008 (provider's direct price)

/address/{address}/txs on bitcoin-esplora (Provider C):
  Cost = $0.0002 (provider's direct price)
```

### Comparison

| Aspect | CU-Based | Per-Request |
|--------|----------|-------------|
| Price standardization | Universal CU price | Provider sets prices |
| Consumer predictability | Know CU price, method CUs vary | Prices vary per provider |
| Provider flexibility | Set CU costs per method | Set prices per method |
| Best for | Complex methods, compute-intensive | Simple, flat-rate services |

### Consumer Payment Mode

When starting a session, consumers declare their payment mode:

```typescript
const session = await din.startSession({
  paymentMode: 'cu',      // or 'request'
  // ...
});
```

The protocol then **only routes to providers supporting that mode**.

### Provider Options

Providers can support one or both modes:

| Configuration | CU Pricing | Request Pricing | Consumer Pool |
|---------------|------------|-----------------|---------------|
| CU-only | true | false | CU-mode consumers |
| Request-only | false | true | Request-mode consumers |
| Both | true | true | All consumers |

**Recommendation:** Support both modes to maximize your potential consumer pool.

---

## Provider Rate Cards

### Rate Card Structure

Rate cards are stored in the DINProtocol contract:

```solidity
// In DINProtocol contract
struct ProviderServiceData {
    uint256 id;
    uint256 providerId;
    uint256 serviceId;              // e.g., "ethereum-mainnet"
    string endpointUrl;
    uint256 capabilities;           // Bitmask of supported methods

    // Payment rate card
    bool supportsCUPricing;         // Provider accepts CU-based payments
    bool supportsRequestPricing;    // Provider accepts per-request payments
    uint256 defaultCUCost;          // Default CUs for unlisted methods
    uint256 defaultPricePerRequest; // Default price for unlisted methods
}

// Method-specific pricing
mapping(uint256 => mapping(uint16 => uint256)) public methodCUCosts;
mapping(uint256 => mapping(uint16 => uint256)) public methodPrices;

// Protocol-level pricing (set by governance)
uint256 public pricePerCU;  // Universal CU price in USDC (6 decimals)
```

### Example Rate Cards

**Provider A (Supports both pricing modes):**

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
        "debug_traceCall": 500
      },
      "defaultPricePerRequest": 0.001,
      "methodPrices": {
        "eth_call": 0.0008,
        "eth_getLogs": 0.004,
        "debug_traceCall": 0.05
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
        "/tx/{txid}": 0.0001
      }
    }
  }
}
```

### Provider Competition

Providers compete on:

| Factor | How Providers Compete |
|--------|----------------------|
| **Efficiency** | Lower CU costs for CU-mode consumers |
| **Pricing** | Lower direct prices for request-mode consumers |
| **Quality** | Higher health scores, better uptime, lower latency |
| **Coverage** | More services and methods supported |
| **Mode support** | Supporting both modes captures more consumers |

---

## Request Routing

The protocol routes requests based on payment mode, service support, pricing, health scores, and consumer routing preferences.

### Routing Strategies

| Strategy | Description | Selection Method | Best For |
|----------|-------------|------------------|----------|
| `cost` | Prioritize lowest price | Cheapest provider | Price-sensitive apps, batch jobs |
| `health` | Prioritize reliability | Weighted random by health | Mission-critical, production |
| `balanced` | Best value (quality + price) | Weighted random by value score | Most users (default) |

### Minimum Health Threshold

**All strategies** enforce a minimum health threshold before any routing logic runs. Providers below this threshold are excluded regardless of price.

```
MINIMUM HEALTH THRESHOLD
------------------------

Default: 0.5 (configurable per consumer)

Before ANY routing strategy:
Provider A: health=0.98  --> Above threshold, included
Provider B: health=0.30  --> Below threshold, EXCLUDED
Provider C: health=0.80  --> Above threshold, included
Provider D: health=0.45  --> Below threshold, EXCLUDED

This protects consumers from routing to unhealthy providers
even when using "cost" strategy.
```

### Strategy: cost

Select the cheapest provider above the health threshold.

```
Remaining after health filter: [Provider A, Provider C]

Provider A: cost=$0.0008
Provider C: cost=$0.0004

Selection: Provider C (cheapest) - 100% of the time
```

Simple and deterministic. Best for high-volume, cost-sensitive workloads.

### Strategy: health

Weighted random selection based on health scores.

```
Remaining after health filter: [Provider A, Provider C]

Provider A: health=0.98 --> weight=0.98
Provider C: health=0.80 --> weight=0.80

Selection: Weighted random
  - Provider A selected ~55% of the time (0.98 / 1.78)
  - Provider C selected ~45% of the time (0.80 / 1.78)
```

Prioritizes reliability and distributes load proportionally to health.

### Strategy: balanced (Default)

Weighted random selection based on **value score** - rewards providers who offer both good health AND competitive pricing.

**Value Score Formula:**

```
valueScore = health x costEfficiency

where costEfficiency = cheapestCost / providerCost
```

**Example:**

```
BALANCED STRATEGY EXAMPLE
=========================

Providers after health filter (minHealth=0.5):

Provider A: health=0.98, cost=$0.0008
Provider B: health=0.30, cost=$0.0004  <-- excluded (below 0.5)
Provider C: health=0.80, cost=$0.0004

Cheapest cost among remaining: $0.0004

Value Score Calculation:
------------------------
Provider A: 0.98 x (0.0004 / 0.0008) = 0.98 x 0.5 = 0.49
Provider C: 0.80 x (0.0004 / 0.0004) = 0.80 x 1.0 = 0.80

Selection: Weighted random by value score
- Provider C selected ~62% of the time (0.80 / 1.29)
- Provider A selected ~38% of the time (0.49 / 1.29)

Result: Provider C wins most often because it offers
        good health (0.80) at the best price ($0.0004)
```

### Strategy Comparison

Given providers after filtering:
- Provider A: health=0.98, cost=$0.0008
- Provider B: health=0.85, cost=$0.0004

| Strategy | Winner | Reasoning |
|----------|--------|-----------|
| `cost` | Provider B (100%) | Cheapest price |
| `health` | Provider A (~54%) | Higher health score |
| `balanced` | Provider B (~63%) | Better value (health x cost efficiency) |

---

## Full Routing Algorithm

```
ROUTING DECISION FLOW
=====================

Input: Consumer request for "ethereum-mainnet", method "eth_call"
       Consumer paymentMode: "cu"
       Consumer routingStrategy: "balanced"
       Consumer minHealthThreshold: 0.5
       Consumer maxCUsPerRequest: 100

Step 1: Filter by payment mode
------------------------------
Provider A: supportsCUPricing=true   --> Include
Provider B: supportsCUPricing=true   --> Include
Provider C: supportsCUPricing=false  --> Exclude
Provider D: supportsCUPricing=true   --> Include
Remaining: [A, B, D]

Step 2: Filter by service + method support
------------------------------------------
Provider A: supports ethereum-mainnet + eth_call  --> Include
Provider B: supports ethereum-mainnet + eth_call  --> Include
Provider D: only supports polygon                 --> Exclude
Remaining: [A, B]

Note: If a provider doesn't list a method, it uses defaultCUCost.

Step 3: Filter by cost limit
----------------------------
Protocol CU price: $0.00008/CU (universal)

(CU mode) Filter by maxCUsPerRequest:
Provider A: eth_call costs 10 CUs  --> Under 100 max, include
Provider B: eth_call costs 5 CUs   --> Under 100 max, include
Remaining: [A, B]

Step 4: Filter by minimum health threshold
------------------------------------------
Provider A: health=0.98  --> Above 0.5, include
Provider B: health=0.85  --> Above 0.5, include
Remaining: [A, B]

Step 5: Apply routing strategy ("balanced")
-------------------------------------------
Cheapest cost: $0.0004 (Provider B)
Provider A: valueScore = 0.98 x (0.0004/0.0008) = 0.49
Provider B: valueScore = 0.85 x (0.0004/0.0004) = 0.85

Step 6: Select (weighted random by value score)
-----------------------------------------------
Provider B selected (~63% probability)
```

---

## Error Handling

### No Providers for Payment Mode

When no providers support the consumer's chosen payment mode:

```
Consumer                     Protocol
   |                            |
   |  Request (paymentMode: "request")
   |  for "solana-mainnet"
   |--------------------------->|
   |                            |
   |                            |  No providers have
   |                            |  supportsRequestPricing=true
   |                            |  for solana-mainnet
   |                            |
   |  400 / Error Response      |
   |  {                         |
   |    error: "no_providers_for_mode",
   |    service: "solana-mainnet",
   |    requestedMode: "request",
   |    availableModes: ["cu"],
   |    suggestion: "Switch to CU payment mode"
   |  }                         |
   |<---------------------------|
```

### Cost Limit Exceeded

When no providers meet the consumer's cost limit:

```
Consumer                     Protocol
   |                            |
   |  Request (paymentMode: "cu",
   |           maxCUsPerRequest: 5)
   |--------------------------->|
   |                            |
   |                            |  All providers > 5 CUs
   |                            |  for this method
   |                            |
   |  402 / Error Response      |
   |  {                         |
   |    error: "no_providers",
   |    cheapestCUs: 10,
   |    method: "eth_call",
   |    suggestion: "Increase maxCUsPerRequest"
   |  }                         |
   |<---------------------------|
```

---

## Supported Service Types

| Service Type | Request Type | Examples |
|--------------|--------------|----------|
| `evm` | JSON-RPC | ethereum-mainnet, base-mainnet, polygon-mainnet |
| `solana` | JSON-RPC | solana-mainnet |
| `starknet` | JSON-RPC | starknet-mainnet |
| `bitcoin` | JSON-RPC | bitcoin-mainnet |
| `tron-full-node` | JSON-RPC | tron-mainnet |
| `beacon-chain` | REST | ethereum-beacon |
| `bitcoin-esplora` | REST | bitcoin-esplora |

New service types can be added as the ecosystem grows.
