# RFC: DIN Subscription & Payment System

**Status:** Draft
**Authors:** DIN Team
**Date:** December 2024

---

## Summary

A subscription and prepaid payment system for the DIN network that enables consumers (routers, SDK developers) to establish payment agreements with providers, avoiding the latency overhead of per-request micropayments while maintaining the flexibility of x402.

---

## Problem Statement

Currently, DIN uses x402 micropayments for every RPC request. While this provides fine-grained billing, it introduces:

1. **Latency overhead** - Each request requires payment negotiation (402 → sign → retry)
2. **No predictable pricing** - Consumers can't budget for fixed monthly costs
3. **Provider complexity** - Providers must handle micropayments for every request

Large consumers (enterprise apps, routers) need predictable pricing and zero-latency access.

---

## Proposed Solution

A two-layer payment system that separates business logic (subscriptions, pricing) from payment execution (x402):

```
┌─────────────────────────────────────────────────────────────────┐
│                      Business Layer                              │
│            (DIN Marketplace Contract - Linea)                    │
│                                                                  │
│   ┌─────────────┐  ┌─────────────┐  ┌─────────────┐             │
│   │  Unlimited  │  │  Request    │  │   Credit    │             │
│   │  (Monthly)  │  │  Count      │  │   Based     │             │
│   └─────────────┘  └─────────────┘  └─────────────┘             │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Payment Layer                               │
│                   (x402 v2 Sessions)                             │
│                                                                  │
│   - Session-based authentication                                 │
│   - One-time payment per session                                 │
│   - Native subscription support                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## Pricing Models

### 1. Unlimited (Fixed Monthly)

**Use case:** Predictable budgeting, heavy usage

- Consumer pays fixed monthly fee upfront
- Unlimited requests during subscription period
- Provider tracks active subscriptions
- Zero per-request overhead

**Example:** $500/month for unlimited Ethereum mainnet RPC

### 2. Request Count (Simple Usage)

**Use case:** Light/variable usage, simple tracking

- Consumer purchases X requests (e.g., 1M requests)
- Each request decrements counter by 1
- Simple to understand and implement
- Optimistic deduction (no latency added)

**Example:** $50 for 1,000,000 requests

### 3. Credit-Based (Method Pricing)

**Use case:** Different methods have different costs

- Consumer deposits credits (e.g., 10,000 credits)
- Each method costs different credits
- Provider defines credit costs per method
- Optimistic deduction with async balance updates

**Example:** `eth_call` = 1 credit, `eth_getLogs` = 5 credits, `debug_traceCall` = 50 credits

---

## System Architecture

### Component Overview

```
┌──────────────────────────────────────────────────────────────────────────────────────┐
│                                Consumer (SDK / Router)                                │
│                                                                                       │
│   ┌─────────────────┐   ┌─────────────────┐   ┌─────────────────┐                    │
│   │ Marketplace     │   │ Agreement       │   │ x402 Payment    │                    │
│   │ Client          │   │ Manager         │   │ Client (v2)     │                    │
│   │                 │   │                 │   │                 │                    │
│   │ - Browse plans  │   │ - Track active  │   │ - Sign payments │                    │
│   │ - Subscribe     │   │   agreements    │   │ - Manage        │                    │
│   │ - Monitor usage │   │ - Cache locally │   │   sessions      │                    │
│   └────────┬────────┘   └────────┬────────┘   └────────┬────────┘                    │
└────────────┼────────────────────┼────────────────────┼───────────────────────────────┘
             │                    │                    │
             │                    │                    │
             ▼                    ▼                    ▼
┌──────────────────────────────────────────────────────────────────────────────────────┐
│                           DIN Marketplace Contract (Linea)                            │
│                                                                                       │
│   Storage:                              Functions:                                    │
│   ┌─────────────────────────────────┐   ┌─────────────────────────────────────┐      │
│   │ Plans[]                         │   │ registerPlan()                       │      │
│   │  - id, provider, network        │   │ createAgreement() + USDC transfer    │      │
│   │  - planType, price, units       │   │ recordUsage() (batch)                │      │
│   │  - validityDays                 │   │ getActiveAgreement()                 │      │
│   │                                 │   │ withdrawFunds()                      │      │
│   │ Agreements[]                    │   └─────────────────────────────────────┘      │
│   │  - id, planId, consumer         │                                                 │
│   │  - provider, start/end time     │   Events:                                       │
│   │  - totalUnits, usedUnits        │   ┌─────────────────────────────────────┐      │
│   │  - status                       │   │ PlanRegistered                       │      │
│   │                                 │   │ AgreementCreated                     │      │
│   │ activeAgreementsByKey           │   │ UsageRecorded                        │      │
│   │  [consumer:provider:network]    │   │ AgreementExhausted                   │      │
│   │  → agreementId (O(1) lookup)    │   └─────────────────────────────────────┘      │
│   └─────────────────────────────────┘                                                 │
└──────────────────────────────────────────────────────────────────────────────────────┘
             │                                          ▲
             │ Contract Events                          │ Usage Updates (Batched)
             ▼                                          │
┌──────────────────────────────────────────────────────────────────────────────────────┐
│                                    Provider Stack                                     │
│                                                                                       │
│   ┌─────────────────┐   ┌─────────────────┐   ┌─────────────────┐                    │
│   │ Agreement       │   │ Usage           │   │ x402 Handler    │                    │
│   │ Verifier        │   │ Tracker         │   │ (Fallback)      │                    │
│   │                 │   │                 │   │                 │                    │
│   │ - Local cache   │   │ - Buffer usage  │   │ - Return 402    │                    │
│   │ - Sync from     │   │   locally       │   │   for non-      │                    │
│   │   events        │   │ - Batch write   │   │   subscribers   │                    │
│   │ - O(1) verify   │   │   to chain      │   │ - Issue session │                    │
│   └────────┬────────┘   └────────┬────────┘   └────────┬────────┘                    │
│            │                     │                     │                              │
│            └─────────────────────┴─────────────────────┘                              │
│                                  │                                                    │
│                                  ▼                                                    │
│                          ┌─────────────────┐                                          │
│                          │ RPC Backend     │                                          │
│                          │ (Geth, etc.)    │                                          │
│                          └─────────────────┘                                          │
└──────────────────────────────────────────────────────────────────────────────────────┘
```

---

## Detailed System Flows

### Flow 1: Plan Registration (Provider Setup)

Provider registers pricing plans before accepting subscriptions.

```
Provider Admin                    Marketplace Contract
      │                                   │
      │  1. registerPlan(                 │
      │       network: "ethereum-mainnet" │
      │       planType: Unlimited         │
      │       price: 500_000_000          │   (500 USDC)
      │       validityDays: 30            │
      │     )                             │
      │──────────────────────────────────▶│
      │                                   │
      │                                   │  2. Validate:
      │                                   │     - price > 0
      │                                   │     - validityDays > 0
      │                                   │
      │                                   │  3. Store plan:
      │                                   │     plans[nextPlanId] = Plan{...}
      │                                   │     providerPlans[provider].push(planId)
      │                                   │     networkPlans[network].push(planId)
      │                                   │
      │                                   │  4. Emit PlanRegistered
      │                                   │
      │  5. Return planId                 │
      │◀──────────────────────────────────│
      │                                   │
      │  [For Credit-Based plans only]    │
      │                                   │
      │  6. setMethodCosts(               │
      │       planId: 123                 │
      │       methods: ["eth_call",       │
      │                 "eth_getLogs",    │
      │                 "debug_traceCall"]│
      │       credits: [2, 5, 50]         │
      │     )                             │
      │──────────────────────────────────▶│
      │                                   │
      │                                   │  7. Store method costs
      │                                   │
```

**Step-by-step breakdown:**

1. **Plan Registration Call**: Provider calls `registerPlan()` with network name (e.g., "ethereum-mainnet"), plan type (Unlimited/RequestCount/CreditBased), price in USDC (6 decimals), and validity period in days.

2. **Validation**: Contract validates that price > 0 and validityDays > 0. For RequestCount/CreditBased plans, units must also be > 0.

3. **Storage**: The plan is stored in the `plans` mapping. Two additional indexes are maintained: `providerPlans[provider]` for provider dashboards, and `networkPlans[network]` for consumer discovery.

4. **Event Emission**: `PlanRegistered(planId, provider, network, planType)` event is emitted. Consumers and indexers can listen to this to update their local catalogs.

5. **Method Costs (Credit-Based only)**: For plans with method-specific pricing, provider makes a second call to `setMethodCosts()`. This stores a mapping of `methodCosts[planId][methodName] → creditCost`. A `defaultMethodCost` is also set for methods not explicitly listed.

**Key considerations:**
- Plans are immutable once created (no price changes). To update pricing, providers create a new plan and deactivate the old one.
- Method costs can be updated for existing credit-based plans, but changes only affect new agreements.

---

### Flow 2: Agreement Creation (Consumer Subscription)

Consumer subscribes to a provider's plan.

```
Consumer                      USDC Token               Marketplace Contract
   │                              │                           │
   │  1. approve(                 │                           │
   │       marketplace,           │                           │
   │       500_000_000)           │                           │
   │─────────────────────────────▶│                           │
   │                              │                           │
   │  2. Allowance set            │                           │
   │◀─────────────────────────────│                           │
   │                              │                           │
   │  3. createAgreement(planId: 123, quantity: 1)            │
   │─────────────────────────────────────────────────────────▶│
   │                              │                           │
   │                              │  4. transferFrom(         │
   │                              │       consumer,           │
   │                              │       marketplace,        │
   │                              │       500_000_000)        │
   │                              │◀──────────────────────────│
   │                              │                           │
   │                              │  5. Transfer success      │
   │                              │──────────────────────────▶│
   │                              │                           │
   │                              │                           │  6. Create agreement:
   │                              │                           │     agreements[nextId] = Agreement{
   │                              │                           │       planId: 123
   │                              │                           │       consumer: 0xConsumer
   │                              │                           │       provider: 0xProvider
   │                              │                           │       startTime: now
   │                              │                           │       endTime: now + 30 days
   │                              │                           │       paidAmount: 500 USDC
   │                              │                           │       status: Active
   │                              │                           │     }
   │                              │                           │
   │                              │                           │  7. Index for fast lookup:
   │                              │                           │     key = keccak256(consumer, provider, network)
   │                              │                           │     activeAgreementsByKey[key] = agreementId
   │                              │                           │
   │                              │                           │  8. Emit AgreementCreated
   │                              │                           │
   │  9. Return agreementId                                   │
   │◀─────────────────────────────────────────────────────────│
```

**Step-by-step breakdown:**

1. **USDC Approval**: Consumer first approves the marketplace contract to spend USDC on their behalf. This is a standard ERC-20 `approve()` call. The amount should be at least `plan.price * quantity`.

2. **Create Agreement**: Consumer calls `createAgreement(planId, quantity)`. Quantity determines the scope: for Unlimited plans, it's number of months; for RequestCount/CreditBased, it's the number of unit packages.

3. **Atomic Transfer**: The contract calls `usdc.transferFrom(consumer, address(this), totalAmount)`. If this fails (insufficient balance, insufficient allowance), the entire transaction reverts.

4. **Agreement Storage**: On successful payment, the contract creates an Agreement struct with:
   - `startTime`: current block timestamp
   - `endTime`: startTime + (validityDays * quantity * 1 day)
   - `totalUnits`: plan.units * quantity (for usage-based plans)
   - `usedUnits`: 0
   - `status`: Active

5. **Fast Lookup Index**: The contract maintains `activeAgreementsByKey` mapping where `key = keccak256(consumer, provider, network)`. This enables O(1) lookups during request verification. Only one active agreement per consumer-provider-network tuple is allowed.

6. **Event Emission**: `AgreementCreated(agreementId, consumer, provider, planId, startTime, endTime)` event is emitted. Providers listen to this to update their local agreement cache.

**Key considerations:**
- The consumer pays the full amount upfront. There's no billing cycle or recurring charges—consumers must manually renew.
- If an active agreement already exists for the same consumer-provider-network tuple, `createAgreement` reverts. The consumer must wait for expiration or create an agreement with a different provider.
- Provider funds are held in the marketplace contract until withdrawal (providers can call `withdrawFunds()` at any time).

---

### Flow 3: Request Handling (Zero-Latency Path)

Provider verifies agreement and serves request without 402 negotiation.

```
Consumer                           Provider                    Agreement Cache
   │                                  │                              │
   │  1. POST /rpc                    │                              │
   │     Headers:                     │                              │
   │       X-DIN-AGREEMENT: 456       │                              │
   │       X-DIN-SIGNATURE: 0x...     │                              │
   │     Body:                        │                              │
   │       {jsonrpc: "2.0",           │                              │
   │        method: "eth_call", ...}  │                              │
   │─────────────────────────────────▶│                              │
   │                                  │                              │
   │                                  │  2. Extract consumer address │
   │                                  │     from signature           │
   │                                  │                              │
   │                                  │  3. Cache lookup:            │
   │                                  │     key = consumer:provider: │
   │                                  │           ethereum-mainnet   │
   │                                  │────────────────────────────▶│
   │                                  │                              │
   │                                  │  4. Return cached agreement  │
   │                                  │◀────────────────────────────│
   │                                  │                              │
   │                                  │  5. Verify:                  │
   │                                  │     - status == Active       │
   │                                  │     - endTime > now          │
   │                                  │     - [usage-based]:         │
   │                                  │       remaining > cost       │
   │                                  │                              │
   │                                  │  6. Execute RPC call         │
   │                                  │                              │
   │  7. 200 OK                       │                              │
   │     Body: {result: ...}          │                              │
   │     Headers:                     │                              │
   │       X-PAYMENT-SESSION: <jwt>   │  (for future requests)       │
   │◀─────────────────────────────────│                              │
   │                                  │                              │
   │                                  │  8. [Async, non-blocking]    │
   │                                  │     Increment local usage    │
   │                                  │     buffer for this consumer │
   │                                  │                              │
```

**Step-by-step breakdown:**

1. **Request with Agreement Headers**: Consumer includes `X-DIN-AGREEMENT` (agreement ID) and `X-DIN-SIGNATURE` (signature proving ownership of the consumer address). The signature is over a message containing the agreement ID and a timestamp to prevent replay attacks.

2. **Consumer Extraction**: Provider recovers the consumer's Ethereum address from the signature using `ecrecover`. This proves the request is authorized by the agreement holder.

3. **Cache Lookup**: Provider looks up the agreement in their local cache using the composite key `consumer:provider:network`. This is an O(1) hash map lookup, adding negligible latency (~1μs).

4. **Cache Hit Path**: If found in cache, the provider uses the cached agreement data. Cache misses trigger a contract call (see Cache Strategy section).

5. **Validity Checks**: Provider verifies:
   - `status == Active` (not cancelled or exhausted)
   - `endTime > now` (not expired)
   - For usage-based: `remaining >= methodCost` where `remaining = totalUnits - usedUnits - localBuffer`

6. **Request Execution**: If all checks pass, the request is forwarded to the RPC backend (Geth, etc.) for execution. No 402 negotiation occurs.

7. **Session Token Issuance**: Provider returns an `X-PAYMENT-SESSION` JWT in the response. Subsequent requests can use this session token instead of the agreement signature, further reducing verification overhead.

8. **Async Usage Tracking**: After the response is sent, the provider asynchronously increments a local usage buffer. This happens off the critical path and adds zero latency to the request.

**Key considerations:**
- The entire verification path (steps 2-5) adds <1ms to request latency, compared to 100-500ms for x402 payment negotiation.
- Session tokens are optional but recommended. They contain the consumer address and agreement ID, signed by the provider, and are valid for a configurable duration (e.g., 1 hour).
- If the cached agreement is stale (e.g., consumer exhausted their balance via another provider), the worst case is serving a few extra requests before the cache syncs. This is acceptable given the UX benefit.

---

### Flow 4: Usage Tracking (Optimistic Deduction)

For usage-based plans, providers track usage asynchronously to avoid latency.

```
                    Request Path (Blocking)              Background Sync (Non-Blocking)
                           │                                      │
   Request arrives         │                                      │
         │                 │                                      │
         ▼                 │                                      │
┌─────────────────────┐    │                                      │
│ Read from cache:    │    │                                      │
│                     │    │                                      │
│ agreement.remaining │    │                                      │
│ = totalUnits        │    │                                      │
│   - usedUnits       │    │                                      │
│   - localBuffer     │    │                                      │
│                     │    │                                      │
│ (Single atomic read)│    │                                      │
└──────────┬──────────┘    │                                      │
           │               │                                      │
           │               │                                      │
     ┌─────┴─────┐         │                                      │
     │           │         │                                      │
remaining > 0?   │         │                                      │
     │           │         │                                      │
   Yes          No         │                                      │
     │           │         │                                      │
     ▼           ▼         │                                      │
  Serve      Return        │                                      │
  Request    402           │                                      │
     │                     │                                      │
     │                     │                                      │
     ▼                     │                                      │
┌─────────────────────┐    │                                      │
│ Async: Increment    │    │                                      │
│ localBuffer[consumer]    │                                      │
│ += methodCost       │    │                                      │
│                     │    │                                      │
│ (Non-blocking)      │    │                                      │
└─────────────────────┘    │                                      │
                           │                                      │
                           │               ┌──────────────────────┴───────┐
                           │               │                              │
                           │               │  Every 60 seconds:           │
                           │               │                              │
                           │               │  1. Collect all buffered     │
                           │               │     usage per agreement      │
                           │               │                              │
                           │               │  2. Batch call to contract:  │
                           │               │     batchRecordUsage(        │
                           │               │       agreementIds[],        │
                           │               │       usages[]               │
                           │               │     )                        │
                           │               │                              │
                           │               │  3. Reset local buffers      │
                           │               │                              │
                           │               │  4. Refresh agreement cache  │
                           │               │     from contract state      │
                           │               │                              │
                           │               └──────────────────────────────┘
```

**Step-by-step breakdown:**

**Request Path (Blocking, <1ms):**

1. **Balance Check**: On each request, provider reads `remaining = totalUnits - usedUnits - localBuffer` from the cached agreement. This is a single atomic read from memory—no locks, no I/O.

2. **Decision Point**: If `remaining > methodCost`, serve the request. Otherwise, return 402 to prompt micropayment or subscription renewal.

3. **Serve Request**: Forward to RPC backend, return response to consumer.

4. **Local Buffer Increment**: After sending response, atomically increment `localBuffer[agreementId] += methodCost`. This uses `atomic.AddUint64()` or equivalent—lock-free and non-blocking.

**Background Sync (Non-blocking, every 60s):**

1. **Collect Buffered Usage**: Iterate through all agreements with `localBuffer > 0`. Swap buffer to 0 and collect the value.

2. **Batch Contract Call**: Call `batchRecordUsage(agreementIds[], usages[])` with all collected usage. This is a single transaction recording usage for multiple agreements, amortizing gas costs.

3. **Reset Buffers**: Local buffers were already atomically swapped to 0 in step 1.

4. **Cache Refresh**: Re-fetch agreement state from contract to get updated `usedUnits` values. This reconciles any usage recorded by other providers (if consumer uses multiple providers).

**Key considerations:**

- **Why optimistic?**: We serve the request before recording usage on-chain. This eliminates chain latency from the request path. The tradeoff is that consumers might get a few extra requests if they exhaust their balance, but this is rare and the overage is minimal.

- **Local buffer vs chain state**: The local buffer tracks usage since the last sync. The formula `remaining = totalUnits - usedUnits - localBuffer` combines chain state (`usedUnits`) with local state (`localBuffer`) for accurate balance checking.

- **Sync interval tradeoffs**:
  - Shorter intervals (10s): More accurate balances, higher gas costs
  - Longer intervals (5min): Lower gas costs, higher risk of overage
  - 60s is a reasonable default, but configurable per provider.

- **Failure handling**: If `batchRecordUsage` fails (gas spike, network issue), the local buffer is restored and retried on next sync. Usage is never lost.

---

### Flow 5: x402 Fallback (Micropayment Path)

For consumers without agreements, fall back to standard x402 micropayments.

```
Consumer                           Provider                    x402 Verifier
   │                                  │                              │
   │  1. POST /rpc                    │                              │
   │     (No agreement headers)       │                              │
   │─────────────────────────────────▶│                              │
   │                                  │                              │
   │                                  │  2. Check for agreement      │
   │                                  │     → Not found              │
   │                                  │                              │
   │  3. 402 Payment Required         │                              │
   │     X-PAYMENT-REQUIRED: {        │                              │
   │       scheme: "exact",           │                              │
   │       network: "linea",          │                              │
   │       maxAmountRequired: "...",  │                              │
   │       payTo: "0xProvider",       │                              │
   │       sessionOptions: {          │                              │
   │         enabled: true,           │                              │
   │         maxDuration: 3600        │                              │
   │       }                          │                              │
   │     }                            │                              │
   │◀─────────────────────────────────│                              │
   │                                  │                              │
   │  4. Sign USDC payment            │                              │
   │     (viem/ethers)                │                              │
   │                                  │                              │
   │  5. POST /rpc                    │                              │
   │     X-PAYMENT: <signed payment>  │                              │
   │─────────────────────────────────▶│                              │
   │                                  │                              │
   │                                  │  6. Verify payment           │
   │                                  │─────────────────────────────▶│
   │                                  │                              │
   │                                  │  7. Payment valid            │
   │                                  │◀─────────────────────────────│
   │                                  │                              │
   │                                  │  8. Issue session token      │
   │                                  │                              │
   │  9. 200 OK                       │                              │
   │     X-PAYMENT-SESSION: <jwt>     │                              │
   │     X-PAYMENT-RESPONSE: {        │                              │
   │       success: true,             │                              │
   │       amount: "...",             │                              │
   │       txHash: "0x..."            │                              │
   │     }                            │                              │
   │◀─────────────────────────────────│                              │
   │                                  │                              │
   │  [Subsequent requests use        │                              │
   │   X-PAYMENT-SESSION header       │                              │
   │   until session expires]         │                              │
```

**Step-by-step breakdown:**

1. **Request Without Agreement**: Consumer sends RPC request without `X-DIN-AGREEMENT` or `X-PAYMENT-SESSION` headers. This could be a new user, a user whose subscription expired, or a user who prefers pay-per-use.

2. **Agreement Check Fails**: Provider checks local cache for an active agreement for this consumer. None found.

3. **402 Response**: Provider returns HTTP 402 with `X-PAYMENT-REQUIRED` header containing:
   - `scheme`: "exact" (x402 payment scheme)
   - `network`: "linea" (payment network)
   - `maxAmountRequired`: Price in wei for this request
   - `payTo`: Provider's payment address
   - `sessionOptions`: x402 v2 session configuration (duration, max requests)

4. **Payment Signing**: Consumer's SDK receives 402, extracts payment requirements, and signs a USDC transfer authorization using their wallet (viem, ethers.js, etc.). The signature authorizes the provider to claim USDC from the consumer.

5. **Retry with Payment**: Consumer retries the same request with `X-PAYMENT` header containing the signed payment authorization.

6. **Payment Verification**: Provider's x402 verifier validates the signature, checks that amount >= required, and submits the USDC transfer on-chain (or queues it for batch settlement).

7. **Payment Confirmed**: Once verified, provider knows they'll receive payment.

8. **Session Token Issuance**: Provider issues an x402 v2 session token (JWT) valid for the configured duration. This token encodes the consumer's address and session expiry.

9. **Response with Session**: Consumer receives the RPC response along with the session token and payment confirmation.

10. **Session Reuse**: Subsequent requests include `X-PAYMENT-SESSION` header. The provider verifies the JWT signature and serves requests without additional payment—until the session expires, at which point the 402 flow repeats.

**Key considerations:**

- **x402 v2 vs v1**: v1 required payment for every request. v2 introduces sessions, so payment is only required once per session (e.g., hourly). This dramatically reduces the micropayment overhead.

- **When to use x402 vs agreements**: x402 is ideal for low-volume or sporadic usage. For predictable, high-volume usage, agreements (subscriptions) are more cost-effective and lower latency.

- **Session pricing**: Providers can price sessions based on expected usage. A 1-hour session might cost $0.10 for light users or $1.00 for heavy users. This is configured in `sessionOptions`.

- **Fallback guarantee**: x402 ensures that any consumer can always access the network, even without a subscription. This maintains the permissionless nature of DIN.

---

## Agreement Cache Strategy

Providers maintain a local cache of agreements for O(1) verification.

### Cache Structure

```
┌──────────────────────────────────────────────────────────────────┐
│                    Provider Agreement Cache                       │
│                                                                   │
│   Primary Index (for request verification):                       │
│   ┌─────────────────────────────────────────────────────────┐    │
│   │  Map<string, CachedAgreement>                            │    │
│   │                                                          │    │
│   │  Key: "consumer:provider:network"                        │    │
│   │  Value: {                                                │    │
│   │    agreement: Agreement,                                 │    │
│   │    localUsage: uint64,      // Buffered usage           │    │
│   │    lastSync: timestamp,     // Last chain sync          │    │
│   │    planType: PlanType,                                   │    │
│   │    methodCosts: Map<string, uint64>  // For credit-based│    │
│   │  }                                                       │    │
│   └─────────────────────────────────────────────────────────┘    │
│                                                                   │
│   Sync Strategy:                                                  │
│   ┌─────────────────────────────────────────────────────────┐    │
│   │  1. On startup: Full sync from contract                  │    │
│   │  2. Ongoing: Listen to AgreementCreated events           │    │
│   │  3. Every 30s: Refresh active agreements from contract   │    │
│   │  4. On cache miss: Fetch from contract, cache result     │    │
│   └─────────────────────────────────────────────────────────┘    │
│                                                                   │
└──────────────────────────────────────────────────────────────────┘
```

### Verification Pseudocode

```
function verifyRequest(request):
    consumer = extractConsumer(request)
    network = extractNetwork(request)

    key = consumer + ":" + providerAddr + ":" + network
    cached = cache.get(key)

    if cached == null:
        // Cache miss - check chain
        agreement = contract.getActiveAgreement(consumer, providerAddr, network)
        if agreement == null:
            return REQUIRE_402
        cached = cacheAgreement(agreement)

    // Check validity
    if cached.agreement.status != Active:
        return REQUIRE_402

    if cached.agreement.endTime < now():
        return EXPIRED

    // For usage-based, check balance
    if cached.planType != Unlimited:
        cost = getMethodCost(cached, request.method)
        remaining = cached.agreement.totalUnits
                  - cached.agreement.usedUnits
                  - cached.localUsage

        if remaining < cost:
            return EXHAUSTED

    return VALID
```

---

## Key Design Decisions

### 1. On-Chain Agreements (Not NFTs)

We chose simple smart contracts over NFTs because:
- Lower implementation complexity
- Agreements are typically non-transferable
- No secondary market needed for subscriptions
- Simpler provider integration

### 2. x402 v2 Sessions as Primary Auth

x402 v2 introduces session-based payments:
- Pay once, get session token
- Reuse session for subsequent requests
- Built-in subscription support

This replaces SIWE for paying users, simplifying the auth flow.

### 3. Optimistic Deduction

For usage-based models (Request Count, Credit-Based):
- Check balance with single atomic read
- Serve request immediately
- Deduct usage asynchronously after response
- Rate limit when balance approaches zero

This ensures zero latency added to the request path.

### 4. All Models Require Prepayment

Unlike traditional SaaS with "free tiers," all DIN subscription models require upfront payment:
- Prevents abuse of unlimited tiers
- Ensures providers are compensated
- Simplifies provider verification (agreement exists = paid)

---

## Provider Experience

Providers define their pricing plans:

| Plan | Monthly Fee | Requests | Credits | Method Pricing |
|------|-------------|----------|---------|----------------|
| Basic | $100 | 500,000 | - | 1 credit/request |
| Pro | $500 | Unlimited | - | - |
| Enterprise | $0 (usage) | - | Deposit | Custom |

Providers register plans in the marketplace contract and set:
- Pricing model (Unlimited, Request Count, Credit-Based)
- Price per unit (monthly fee, per request, per credit)
- Method-specific costs (for Credit-Based)
- Supported networks

---

## Consumer Experience

### SDK Integration

```typescript
const din = new DinClient({
  privateKey: process.env.PRIVATE_KEY,
});

// Browse available plans
const plans = await din.marketplace.getPlans('ethereum-mainnet');

// Subscribe to a plan
const agreement = await din.marketplace.subscribe({
  provider: '0xProviderAddress',
  plan: 'unlimited-monthly',
  duration: 30, // days
});

// Make requests (zero latency - agreement pre-authorizes)
const response = await din.request('ethereum-mainnet', {
  body: { jsonrpc: '2.0', method: 'eth_blockNumber', params: [], id: 1 },
});
```

---

## Success Metrics

1. **Latency reduction** - Zero payment overhead for subscribed users
2. **Provider adoption** - Number of providers offering subscription plans
3. **Consumer adoption** - Percentage of requests via subscription vs micropayments
4. **Revenue predictability** - Providers can forecast monthly revenue

---

## Open Questions

1. **Refund policy** - What happens if a provider goes offline during subscription?
2. **Plan changes** - Can consumers upgrade/downgrade mid-subscription?
3. **Multi-provider bundles** - Future support for bundled plans across providers?

---

## Next Steps

1. Design marketplace smart contract
2. Implement x402 v2 session support in SDK
3. Build provider plan management UI
4. Integration testing with testnet providers
5. Documentation and examples

---

## Related Documents

- [Technical Documentation](/din-payments/) - Full implementation details
- [RFC: DIN TypeScript SDK](/RFC-din-ts-sdk.md) - SDK architecture
- [x402 Payment Integration](/din-router-sdk/04-x402-payments.md) - Current payment flow
