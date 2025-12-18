# DIN Payments: Product Proposal

**Status:** Proposal
**Date:** December 2025
**Audience:** Product Team

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Problem Statement](#problem-statement)
   - [Current Pain Points](#current-pain-points)
   - [The Coordination Problem](#the-coordination-problem)
3. [Core Concepts](#core-concepts)
   - [1. Deposit Model](#1-deposit-model)
   - [2. Sessions](#2-sessions)
   - [3. Provider Rate Cards](#3-provider-rate-cards)
   - [4. Smart Routing](#4-smart-routing)
   - [5. Bilateral Reconciliation](#5-bilateral-reconciliation)
   - [6. Dispute Resolution](#6-dispute-resolution)
   - [7. Settlement Coordinator](#7-settlement-coordinator)
   - [8. Usage Tracking](#8-usage-tracking)
   - [9. Session Authorization (Decentralized)](#9-session-authorization-decentralized)
4. [Supported Services](#supported-services)
5. [Success Metrics](#success-metrics)
6. [Risks and Mitigations](#risks-and-mitigations)
7. [Open Questions for Discussion](#open-questions-for-discussion)
8. [Glossary](#glossary)
9. [Related Documents](#related-documents)

---

## Executive Summary

DIN Payments introduces a **protocol-mediated payment system** that simplifies how developers access blockchain infrastructure. Instead of managing relationships with individual RPC providers, consumers deposit funds once and the protocol handles everything: routing requests to the best providers, tracking usage, and settling payments automatically.

**The core value proposition:**
- **For Consumers (Developers):** Deposit once, access any provider. No negotiations, no API keys per provider, automatic failover.
- **For Providers (Node Operators):** Publish pricing, serve requests, get paid automatically. No billing infrastructure needed.

---

## Problem Statement

### Current Pain Points

**For Developers (Consumers):**
1. Must discover, evaluate, and onboard with each RPC provider individually
2. Need to manage API keys, billing accounts, and rate limits per provider
3. No automatic failover - if a provider goes down, their app breaks
4. Comparing pricing across providers is confusing and time-consuming
5. Locked into provider relationships; switching is painful

**For Node Operators (Providers):**
1. Must build billing, authentication, and usage tracking infrastructure
2. Difficult to compete against established players for visibility
3. Getting paid requires chasing invoices or complex payment integrations
4. No standardized way to publish pricing and capabilities

### The Coordination Problem

The network has many participants that need to work together:

```
    SDK #1 ----\                                  /---- Provider A
    SDK #2 -----+--\                          /--+---- Provider B
    SDK #N ----/   |                          |   \---- Provider C
                   |                          |
                   v                          v
           +----------------------------------------------+
           |               DIN PROTOCOL                   |
           |                                              |
           |  - Receives requests from any SDK/Router     |
           |  - Routes to best available provider         |
           |  - Tracks usage per consumer per provider    |
           |  - Settles payments periodically             |
           +----------------------------------------------+
                   ^                          ^
                   |                          |
    Router #1 -----+--/                  \---+---- Provider D
    Router #2 ----/                          \---- Provider N
```

The protocol must coordinate usage tracking and settlement across all these participants, ensuring accurate attestation and fair payment distribution—without any single party being able to cheat.

---

## Core Concepts

### 1. Deposit Model

Consumers deposit USDC into the protocol contract. This is like loading a prepaid card—funds are held by the contract and can be used across any provider or withdrawn at any time.

**Why deposits instead of pay-per-request?**

| Approach | Pros | Cons |
|----------|------|------|
| Pay per request | No upfront commitment | Payment overhead on every request, slower |
| Deposit model | Fast requests (no payment per call), unified balance | Requires upfront deposit |

On Base chain, deposit transactions cost ~$0.001-0.01—negligible. The deposit model enables:
- **Speed:** No payment negotiation per request
- **Flexibility:** Use funds across any provider
- **Security:** Locked funds guarantee provider payment

**Account structure:**
```
Consumer Account
├── balance: $100      (total deposited, not locked)
├── locked: $50        (reserved for active sessions)
├── available: $50     (can withdraw or use for new sessions)
└── totalSpent: $230   (lifetime spend, for analytics)
```

---

### 2. Sessions

A session is a usage period where funds are locked and the consumer can make requests without per-request payments.

**Session lifecycle:**

```
SESSION LIFECYCLE
=================

1. START SESSION
   Consumer locks $50 from their deposit.
   This is an on-chain transaction (~$0.001-0.01 on Base).
   Funds move from "available" to "locked" in their account.
   Sessions can last up to 7 days.

2. USE SESSION
   Consumer makes RPC requests with a signed session proof.
   Provider verifies proof against on-chain state (once),
   then caches for subsequent requests. No gas costs.

3. CHECKPOINT (every 30 minutes or at 60% spend)
   Protocol settles usage for the period. Both consumer
   and provider report usage for bilateral reconciliation.
   Providers get paid, session continues.

4. END SESSION
   Final settlement occurs. Unused locked funds are
   returned to consumer's available balance.
```

**Session duration:**

Sessions can last up to **7 days**—long enough for continuous applications without requiring frequent restarts. Consumers start a session once and use it for days without interruption. Checkpoints ensure providers get paid throughout the session, not just at the end.

**Consumer protection:**

If something goes wrong (e.g., coordinator is unresponsive), consumers aren't stuck with locked funds forever. After a session expires plus a 24-hour grace period, consumers can reclaim any remaining locked funds directly from the contract—no coordinator involvement needed.

**Price Snapshotting:**

Provider rate card prices are snapshotted at session start. This means price changes during the session don't affect in-flight sessions - consumers pay what they expected when they started.

**Why checkpoints?**

Without checkpoints, a consumer could lock $50, use $45 worth of services, then disappear without settling. Checkpoints ensure:
- Providers get paid incrementally (every 30 minutes)
- Consumer can't accumulate unbounded debt
- Long-running sessions are supported (checkpoint, continue, checkpoint, continue...)

**Checkpoint triggers:**

| Trigger | Threshold | Initiated By | Why |
|---------|-----------|--------------|-----|
| Time-based | Every 30 minutes | Coordinator | Predictable settlement cadence |
| Spend-based | At 60% of locked amount | Consumer SDK | Protects providers from session exhaustion |

Whichever comes first triggers the checkpoint.

**Checkpoint ownership:**

Time-based checkpoints are triggered by the Settlement Coordinator on a fixed schedule. Spend-based checkpoints are triggered by the Consumer SDK. This is because only the consumer knows their total spend across all providers—individual providers only see their own portion.

**Session abuse prevention:**

To prevent abuse through rapid session creation (e.g., probing provider networks without paying), the protocol tracks abandonment rates. A session is considered "abandoned" if less than 5% of locked funds are used. Consumers with high abandonment rates face session creation rate limits. Legitimate users are unaffected; abusers are throttled automatically.

---

### 3. Provider Rate Cards

Providers publish their pricing to the protocol. Rate cards define what services a provider offers and how much they charge.

**Two pricing models:**

```
PRICING MODELS
==============

CU-BASED (Compute Units)
------------------------
- Protocol sets a universal price per CU (e.g., $0.00008)
- Providers set CU costs per method:
    eth_call: 10 CUs
    eth_getLogs: 50 CUs
    debug_traceCall: 500 CUs
- Cost = method CUs x protocol price per CU
- Good for: Predictable costs scaled by complexity

PER-REQUEST
-----------
- Providers set direct prices per method:
    eth_call: $0.0001
    eth_getLogs: $0.0005
    debug_traceCall: $0.01
- Cost = method price (direct)
- Good for: Simple, predictable billing
```

**Consumer chooses their payment mode when starting a session.** The protocol only routes to providers that support that mode.

**Example rate card:**
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
        "debug_traceCall": 500
      },
      "methodPrices": {
        "eth_call": 0.0001,
        "eth_getLogs": 0.0005,
        "debug_traceCall": 0.01
      }
    }
  }
}
```

Providers can support one or both pricing models. More flexibility = more potential consumers routed to you.

**Multi-service providers:**

A single provider can offer multiple blockchain services. For example, one provider might support ethereum-mainnet, solana-mainnet, and bitcoin-mainnet—each with its own rate card. Consumers benefit from simplified access; providers benefit from serving more traffic through a single integration.

**Price change notifications:**

When providers update their rate cards, existing sessions continue at the original prices (price snapshotting protects consumers). New sessions use the updated prices. The protocol notifies consumers of price changes so they can make informed decisions about when to start new sessions.

---

### 4. Smart Routing

The protocol automatically selects the best provider for each request based on consumer preferences.

**Routing algorithm:**

```
ROUTING DECISION FLOW
=====================

Step 1: Filter by payment mode
------------------------------
Consumer chose "cu" payment mode.
  Provider A: supportsCUPricing=true  --> Include
  Provider B: supportsCUPricing=true  --> Include
  Provider C: supportsCUPricing=false --> Exclude

Step 2: Filter by service + method support
------------------------------------------
Request is for ethereum-mainnet, method eth_call.
  Provider A: supports both --> Include
  Provider B: supports both --> Include

Step 3: Filter by cost limit
----------------------------
(CU mode) Consumer set maxCUsPerRequest: 100
  Provider A: eth_call costs 10 CUs --> Under max, include
  Provider B: eth_call costs 150 CUs --> Over max, exclude

(Per-request mode) Consumer set maxPricePerMethod: $0.001
  Provider A: eth_call costs $0.0005 --> Under max, include
  Provider B: eth_call costs $0.002 --> Over max, exclude

Note: In CU mode, the protocol sets a universal CU price (e.g., $0.00008/CU).
Providers cannot change this price - they can only set how many CUs each method costs.

Step 4: Filter by health threshold
----------------------------------
Consumer set minHealth: 0.5
  Provider A: health=0.98 --> Above threshold, include

Step 5: Apply routing strategy
------------------------------
Select from remaining providers based on strategy.
```

**Routing strategies:**

| Strategy | How It Works | Best For |
|----------|--------------|----------|
| **cost** | Always pick the cheapest provider | Budget-conscious, non-critical workloads |
| **health** | Weight selection by health score (higher health = more likely) | Mission-critical applications |
| **balanced** (default) | Weight by value score: `health × (cheapestCost / providerCost)` | Most users—rewards both quality and price |

**Balanced strategy example:**

```
Provider A: health=0.98, cost=$0.0008
Provider B: health=0.85, cost=$0.0004

Cheapest cost: $0.0004

Value scores:
  Provider A: 0.98 × (0.0004 / 0.0008) = 0.49
  Provider B: 0.85 × (0.0004 / 0.0004) = 0.85

Result: Provider B selected ~63% of the time
        (better value despite lower health)
```

The balanced strategy creates a competitive marketplace where providers must offer both quality AND competitive pricing to win traffic.

---

### 5. Bilateral Reconciliation

This is how we ensure neither consumers nor providers can cheat the system.

**The problem with single-party attestation:**

| Trust Model | Problem |
|-------------|---------|
| Trust consumer only | Consumer under-reports usage, providers don't get paid fairly |
| Trust provider only | Provider over-reports usage, consumers get overcharged |
| Trust protocol only | Single point of failure, centralization risk |

**Solution: Both parties report, protocol reconciles.**

```
BILATERAL RECONCILIATION
========================

At checkpoint or session end:

  Consumer SDK reports:           Provider reports:
  "I used 3000 CUs from           "I served 3000 CUs to
   Provider A"                     Consumer X"
          |                              |
          +-------------+----------------+
                        |
                        v
               PROTOCOL COMPARES
                        |
          +-------------+-------------+
          |             |             |
          v             v             v
       MATCH       MINOR DIFF    MAJOR DIFF
    (3000=3000)  (3000 vs 3100) (3000 vs 5000)
          |             |             |
          v             v             v
       Settle      Average &     Hold 48 hours
     immediately     settle       for escalation
                   (3050 CUs)          |
                                       v
                                 No escalation?
                                 Settle by average
```

**Reconciliation thresholds:**

| Scenario | Threshold | Action |
|----------|-----------|--------|
| Exact match | 0% difference | Settle immediately |
| Minor discrepancy | < 5% difference | Average and settle automatically |
| Major discrepancy | ≥ 5% difference | Hold for 48 hours; if no escalation, settle by average |

**Trust guarantees:**

```
Consumer protected from:
  - Provider over-reporting (consumer's claim caps the charge)
  - Paying more than session max (locked amount enforced)

Provider protected from:
  - Consumer under-reporting (provider can dispute with logs)
  - Non-payment (funds are locked in contract before session starts)

Protocol guarantees:
  - Settlement only when both parties agree (or dispute resolved)
  - Neither party can unilaterally extract funds
  - Transparent, auditable reconciliation
```

**Method attestation (cryptographic proof):**

To strengthen bilateral reconciliation, providers cryptographically sign every response with attestation headers (`X-DIN-Method`, `X-DIN-CUs`, `X-DIN-Sig`). This locks in what the provider claims to have served at request time—not settlement time.

Without attestation, a provider could serve a cheap method (1 CU) but claim an expensive method (50 CUs) at settlement. Auto-averaging would give them 25.5 CUs—a net win for lying.

With attestation, the consumer holds the provider's own signature proving what was actually served. If the provider's settlement claim contradicts their signatures, they're caught lying and face penalties. This makes dishonest behavior provably detectable.

Attestation adds minimal latency (~0.2ms per request) and no storage overhead—signatures are verified in memory and discarded. Only disputed requests are stored as evidence.

---

### 6. Dispute Resolution

When consumer and provider claims differ significantly, the protocol resolves disputes fairly.

**Default: Auto-Average Settlement**

Most discrepancies are minor and result from timing differences, dropped requests, or counting edge cases. These are resolved automatically:

```
Consumer claims: $12.00
Provider claims: $15.00
───────────────────────
Settlement: $13.50 (average)
```

**Why averaging works:**

Neither party gains significantly by lying:
- If consumer lies low: saves at most half the difference
- If provider lies high: gains at most half the difference
- The incentive to cheat is small relative to reputation damage
- Fully automated—no human intervention needed

**Cumulative pattern detection:**

To prevent providers from systematically over-reporting just below the 5% threshold, the protocol tracks discrepancy patterns over time. If a provider consistently reports higher than consumers (e.g., average +3% across 50 settlements), they're flagged for review and their health score is reduced. This protects consumers from "death by a thousand cuts" gaming.

**Escalation for suspected malicious behavior:**

If a consumer believes a provider is systematically over-reporting, they can escalate. The timeline differs based on discrepancy size:

```
ESCALATION TIMELINE
===================

For MAJOR discrepancies (≥5%):
------------------------------

Hour 0: Discrepancy detected
    |
    |  Settlement HELD (not processed)
    |  48-hour window for escalation
    |
    +-- Escalation filed during hold?
    |     |
    |     v
    |   Skip to "Escalation Process" below
    |
    +-- No escalation within 48 hours?
          |
          v
        Settle by average (auto-averaged)
        Consumer still has 7 days to escalate after settlement


For MINOR discrepancies (<5%) or post-settlement:
-------------------------------------------------

Day 0: Settlement occurs (auto-averaged)
    |
    |  Consumer has 7 days to escalate
    v


ESCALATION PROCESS
------------------

Day 0: Consumer files escalation
    |  - Provides session IDs and local usage logs
    |  - Pays $25 escalation fee (refunded if valid)
    |  - Must meet identity requirements (see below)
    |
    |  Provider has 7 days to respond
    v
Day 7: Provider response deadline
    |
    +-- Provider submits server logs as evidence
    |       |
    |       v
    |   DIN team reviews (14 days)
    |       |
    |       +-- Provider at fault:
    |       |     - AVS rewards deducted (25% first offense)
    |       |     - Consumer refunded + compensated
    |       |     - Strike on provider record
    |       |
    |       +-- Consumer at fault:
    |             - Consumer loses $25 fee
    |             - Warning on consumer record
    |
    +-- Provider fails to respond
          - Provider penalized (treated as admission)
          - 3rd no-response = removed from protocol
```

**Penalty escalation:**

| Offense | 1st Strike | 2nd Strike | 3rd Strike |
|---------|------------|------------|------------|
| Provider guilty/no-response | 25% reward reduction | 50% reward reduction | Removed from protocol |
| Consumer false claim | Lose $25 fee | Lose $50 fee | Banned from escalations |

**Anti-abuse measures:**

To prevent consumers from creating new wallets to spam escalations:

| Requirement | Description |
|-------------|-------------|
| Minimum spend | $100 lifetime spend before escalating |
| Email verification | Required on first escalation |
| Progressive KYC | Full identity verification after 2 escalations |

This creates real cost and accountability for bad actors while keeping the system accessible for legitimate disputes.

---

### 7. Settlement Coordinator

The Settlement Coordinator is an off-chain service that orchestrates checkpoints and batches settlements on-chain.

> **Note:** A protocol fee may be introduced in the future to fund Settlement Coordinator operations and protocol development. If implemented, it would be a small percentage (e.g., 2-5%) deducted from settlements. See [08 - Appendix](./08-appendix.md) for details.

**Why off-chain coordination?**

| Aspect | Fully On-Chain | Off-Chain Coordinator |
|--------|----------------|----------------------|
| Gas costs | High (every claim on-chain) | Low (only final settlements) |
| Complexity | Complex on-chain averaging | Simple off-chain aggregation |
| Flexibility | Hard to update | Easy to iterate |
| Speed | Limited by block times | Instant reconciliation |
| Privacy | All claims public | Only settlements public |

**Key insight:** The coordinator has no custody of funds. It only orchestrates the collection of signed claims. The smart contract remains the source of truth for fund locking and settlement.

**What happens if the coordinator goes down?**
- Existing sessions continue working (providers verify on-chain state directly)
- Settlement is delayed but not lost (claims can be submitted when coordinator returns)
- Funds remain secure in the contract
- After session expiry + 24 hours, consumers can reclaim locked funds directly

**Phase 1 approach (known limitation):**

In Phase 1, the DIN team operates the coordinator. This is an intentional trade-off for faster iteration and simpler deployment. The coordinator has no custody of funds—it only orchestrates. If compromised, the worst case is delayed or incorrect settlements, not fund loss.

**Path to decentralization:**

Future phases will explore decentralized coordination options including multi-coordinator consensus and keeper networks. The goal is to eliminate the single coordinator as a point of trust while maintaining the efficiency of off-chain reconciliation.

**Coordinator responsibilities:**

```
SETTLEMENT COORDINATOR
======================

1. TRIGGER CHECKPOINTS
   - Every 30 minutes (time-based)
   - Or when any session hits 60% spend (spend-based)

2. COLLECT CLAIMS
   - Accept usage reports from all SDK instances
   - Accept usage reports from all providers
   - Aggregate multiple SDK instance reports per session
     (same consumer can use multiple devices)

3. RECONCILE
   - Match consumer <-> provider claims
   - Apply averaging for minor mismatches
   - Flag major mismatches (>5%) for review

4. SUBMIT ON-CHAIN
   - Batch multiple settlements into one transaction
   - Coordinator pays gas (amortized across settlements)
   - USDC transferred directly to provider wallets
```

**Multi-SDK aggregation example:**

A consumer might use the same session from multiple devices or services:

```
SDK Instance 1 (web app): $7.00 to Provider A, $3.00 to Provider B
SDK Instance 2 (backend): $5.00 to Provider A, $2.00 to Provider B
────────────────────────────────────────────────────────────────────
Total consumer claim:     $12.00 to Provider A, $5.00 to Provider B
```

The coordinator aggregates these before comparing with provider claims.

---

### 8. Usage Tracking

Usage is tracked locally by both consumers (SDK) and providers, then submitted at checkpoints for reconciliation.

**Aggregated tracking (not per-request):**

Storing every individual request would create massive logs. Instead, usage is aggregated by key dimensions:

```
Instead of:
  { requestId: 1, provider: A, method: eth_call, CUs: 2 }
  { requestId: 2, provider: A, method: eth_call, CUs: 2 }
  { requestId: 3, provider: A, method: eth_call, CUs: 2 }
  ... millions of rows ...

We track:
  { provider: A, method: eth_call, totalCUs: 6000, totalCost: $0.48, requestCount: 3000 }
```

**Local persistence:**

The SDK stores session state in a local JSON file (`~/.din/session-log.json`). This enables:
- Crash recovery (resume session after restart)
- Multi-instance coordination (multiple SDK instances can contribute to same session)
- Offline capability (sync when back online)

---

### 9. Session Authorization (Decentralized)

There is no central token server. Consumers sign their own session proofs, and providers verify against on-chain state.

**Flow:**

```
SESSION AUTHORIZATION
=====================

Consumer                                              Provider
    |                                                     |
    |  1. Start session (on-chain)                        |
    |     Lock $50, get sessionId                         |
    |                                                     |
    |  2. Sign session proof (off-chain, wallet)          |
    |     "I am 0xConsumer, session abc123"               |
    |                                                     |
    |  3. First request + signed proof                    |
    |---------------------------------------------------->|
    |                                                     |
    |                            4. Verify signature      |
    |                               Query chain:          |
    |                               "Is session           |
    |                                abc123 valid?"       |
    |                               Cache result.         |
    |                                                     |
    |  5. Response                                        |
    |<----------------------------------------------------|
    |                                                     |
    |  6. Subsequent requests (cached, no chain query)    |
    |---------------------------------------------------->|
    |<----------------------------------------------------|
    |     ... fast, no verification overhead ...          |
```

**Why this is decentralized:**
- No protocol API issues tokens (consumer signs their own proof)
- Provider verifies directly against blockchain state
- No single point of trust or failure in authorization

**Shared sessions across instances:**

The same wallet can use a session from multiple SDK instances (web app, backend service, mobile app). Each instance:
1. Signs with the same wallet
2. Tracks usage locally
3. Reports to coordinator at checkpoint

The coordinator aggregates all instances' reports for that session.

---

## Supported Services

### Current Service Types

| Service Type | Blockchain | Request Type | Examples |
|--------------|------------|--------------|----------|
| `evm` | Ethereum-compatible | JSON-RPC | ethereum-mainnet, base-mainnet, polygon-mainnet |
| `solana` | Solana | JSON-RPC | solana-mainnet |
| `starknet` | Starknet | JSON-RPC | starknet-mainnet |
| `bitcoin` | Bitcoin | JSON-RPC | bitcoin-mainnet |
| `tron-full-node` | Tron | JSON-RPC | tron-mainnet |
| `beacon-chain` | Ethereum Consensus | REST | ethereum-beacon |
| `bitcoin-esplora` | Bitcoin (explorer) | REST | bitcoin-esplora |

New service types can be added as the ecosystem grows—layer 2s, indexing services, archive nodes, etc.

---

## Success Metrics

### Consumer Metrics

| Metric | Target | What It Measures |
|--------|--------|------------------|
| Time to first request | < 30 seconds | Onboarding friction |
| Automatic failover success | > 99% | Reliability value delivered |
| Cost savings vs direct | > 20% | Economic value of competition |

### Provider Metrics

| Metric | Target | What It Measures |
|--------|--------|------------------|
| Time to payment | < 24 hours | Settlement efficiency |
| Traffic increase | > 50% | Value of protocol routing |
| Billing overhead saved | > 10 hours/month | Operational efficiency |

### Protocol Metrics

| Metric | Target | What It Measures |
|--------|--------|------------------|
| Routing efficiency | > 90% optimal | Algorithm quality |
| Dispute rate | < 1% | System trust/accuracy |
| Settlement accuracy | > 99.9% | Reconciliation reliability |

---

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Provider over-reporting | Consumers overcharged | Bilateral reconciliation; method attestation; consumer claim caps charge; cumulative pattern detection |
| Consumer under-reporting | Providers underpaid | Provider can dispute with server logs; escalation process |
| Settlement delays | Provider cash flow issues | 30-minute checkpoint cycles; predictable settlement |
| Coordinator downtime | Settlement delayed | Sessions continue working; funds remain secure in contract; forceUnlock after 24h |
| Smart contract bugs | Potential fund loss | Audits, gradual rollout, bug bounties |
| Low provider adoption | Limited routing options | Competitive economics, easy integration, no billing overhead |
| Provider exits network | Active sessions disrupted | 7-day exit cooldown; sessions complete before provider fully exits |
| Coordinator centralization (Phase 1) | Single point of trust | No fund custody; consumers can forceUnlock; decentralization roadmap |
| Session creation spam | Provider resource waste | Abandonment rate tracking; session creation rate limits based on usage patterns |

---

## Open Questions for Discussion

1. **Launch pricing model:** Should we launch with CU-only, per-request only, or both from day one?

2. **Checkpoint frequency:** Is 30 minutes the right balance between settlement speed and overhead?

3. **Dispute thresholds:** Is 5% the right threshold for flagging discrepancies vs. auto-averaging?

4. **Provider onboarding:** What documentation and tooling do providers need to integrate smoothly?

5. **Consumer UX:** How do we make deposits feel familiar to non-crypto-native developers?

6. **Multi-session support:** Currently each consumer can have one active session at a time. Should we support multiple concurrent sessions (e.g., separate budgets for dev vs. prod)?

7. **Coordinator decentralization timeline:** When should we prioritize moving from DIN-operated coordinator to a decentralized solution?

8. **Session duration limits:** Is 7 days the right maximum session duration, or should it be longer/shorter?

---

## Glossary

| Term | Definition |
|------|------------|
| **Consumer** | Developer or application using RPC services |
| **Provider** | Node operator serving RPC requests |
| **Session** | A usage period with locked funds; enables requests without per-call payments (max 7 days) |
| **Deposit** | USDC transferred to the protocol contract; can be used across any provider |
| **Locked** | Funds reserved for an active session; cannot be withdrawn until session ends |
| **Rate Card** | Provider's published pricing structure for their services |
| **CU (Compute Unit)** | Standardized measure of computational complexity for pricing |
| **Checkpoint** | Periodic settlement during a session (every 30 min or 60% spend) |
| **Bilateral Reconciliation** | Both consumer and provider report usage; protocol averages discrepancies |
| **Settlement Coordinator** | Off-chain service that orchestrates checkpoints and batches on-chain settlements |
| **Escalation** | Formal dispute process when a party suspects malicious behavior |
| **USDC** | USD-pegged stablecoin used for all payments in the protocol |
| **Price Snapshotting** | Prices are locked at session start; mid-session rate card changes don't affect active sessions |
| **ForceUnlock** | Consumer ability to reclaim locked funds 24 hours after session expiry if settlement hasn't occurred |
| **Provider Sidecar** | Service that runs alongside provider nodes to handle session verification and usage tracking |
| **TTL (Time To Live)** | How long cached data remains valid before expiring (30 minutes for session cache) |
| **Method Attestation** | Cryptographic signature on each response proving what method/CUs were served; prevents providers from lying at settlement |
| **Abandonment Rate** | Percentage of sessions created with less than 5% usage; used for rate limiting session creation abuse |

---

## Related Documents

- [Technical RFC](./RFC-din-payments.md) - Full implementation specification
- [DIN Protocol](/docs/din-protocol/) - Unified registry and payment system
