# DIN Payments: Executive Summary

**Status:** Proposal | **Date:** December 2025 | **Audience:** Product Team

---

## Overview

DIN Payments introduces a **protocol-mediated payment system** that simplifies how developers access blockchain infrastructure. Instead of managing relationships with individual RPC providers, consumers deposit funds once and the protocol handles everything: routing requests to the best providers, tracking usage, and settling payments automatically.

**The core value proposition:**
- **For Consumers (Developers):** Deposit once, access any provider. No negotiations, no API keys per provider, automatic failover.
- **For Providers (Node Operators):** Publish pricing, serve requests, get paid automatically. No billing infrastructure needed.

---

## The Problem

### For Developers (Consumers)
1. Must discover, evaluate, and onboard with each RPC provider individually
2. Need to manage API keys, billing accounts, and rate limits per provider
3. No automatic failover—if a provider goes down, their app breaks
4. Comparing pricing across providers is confusing and time-consuming
5. Locked into provider relationships; switching is painful

### For Node Operators (Providers)
1. Must build billing, authentication, and usage tracking infrastructure
2. Difficult to compete against established players for visibility
3. Getting paid requires chasing invoices or complex payment integrations
4. No standardized way to publish pricing and capabilities

### The Coordination Challenge

The network has many SDKs, routers, and providers that must work together. The protocol coordinates usage tracking and settlement across all participants, ensuring accurate attestation and fair payment—without any single party being able to cheat.

---

## Core Concepts

### 1. Deposits & Sessions

**Deposits:** Consumers deposit USDC into the protocol contract—like loading a prepaid card. Funds can be used across any provider or withdrawn at any time.

**Sessions:** A usage period where funds are locked and the consumer can make requests without per-request payments.

| Phase | What Happens |
|-------|--------------|
| **Start** | Consumer locks funds (e.g., $50) from deposit. On-chain transaction (~$0.001 on Base). |
| **Use** | Consumer makes RPC requests with signed session proof. Provider verifies once, then caches. |
| **Checkpoint** | Every 30 min or at 60% spend—usage reconciled, providers paid, session continues. |
| **End** | Final settlement. Unused funds returned to consumer's available balance. |

**Key properties:**
- Sessions last up to **7 days**
- Prices are **snapshotted at session start** (mid-session rate changes don't affect active sessions)
- **Consumer protection:** If coordinator is unresponsive, consumers can force-unlock funds 24 hours after session expiry

**Checkpoint triggers:**

| Trigger | Threshold | Initiated By |
|---------|-----------|--------------|
| Time-based | Every 30 minutes | Coordinator |
| Spend-based | At 60% of locked amount | Consumer SDK |

Without checkpoints, a consumer could lock $50, use $45 of services, then disappear. Checkpoints ensure providers get paid incrementally.

---

### 2. Provider Rate Cards

Providers publish pricing to the protocol. Two models supported:

| Model | How It Works | Example | Best For |
|-------|--------------|---------|----------|
| **CU-based** | Protocol sets $/CU; provider sets CUs per method | eth_call = 10 CUs × $0.00008 = $0.0008 | Complexity-scaled pricing |
| **Per-request** | Provider sets direct price per method | eth_call = $0.0001 | Simple flat billing |

Providers can support one or both models. More flexibility = larger potential consumer pool.

**Multi-service providers:** A single provider can offer multiple blockchain services (ethereum-mainnet, solana-mainnet, bitcoin-mainnet)—each with its own rate card.

**Price change notifications:** When providers update rate cards, existing sessions continue at original prices. New sessions use updated prices.

---

### 3. Smart Routing

The protocol automatically selects the best provider for each request based on consumer preferences.

**Routing flow:**
1. Filter by payment mode (CU or per-request)
2. Filter by service + method support
3. Filter by cost limit (maxCUs or maxPrice)
4. Filter by health threshold
5. Apply routing strategy

**Routing strategies:**

| Strategy | How It Works | Best For |
|----------|--------------|----------|
| **cost** | Always pick cheapest provider | Budget-conscious, batch jobs |
| **health** | Weight by reliability score | Mission-critical applications |
| **balanced** | Weight by value: `health × (cheapestCost / providerCost)` | Most users (default) |

The balanced strategy creates a competitive marketplace where providers must offer both quality AND competitive pricing to win traffic.

---

### 4. Bilateral Reconciliation

Both parties independently track and report usage; the protocol reconciles differences.

**Why both parties?**

| Trust Model | Problem |
|-------------|---------|
| Trust consumer only | Consumer under-reports, providers don't get paid fairly |
| Trust provider only | Provider over-reports, consumers get overcharged |
| Trust protocol only | Single point of failure, centralization risk |
| **Bilateral** | Neither party can cheat; disputes are resolvable |

**Reconciliation rules:**

| Scenario | Threshold | Action |
|----------|-----------|--------|
| Exact match | 0% difference | Settle immediately |
| Minor discrepancy | < 5% difference | Average and settle |
| Major discrepancy | ≥ 5% difference | Hold 48 hours for escalation |

**Method attestation:** Providers cryptographically sign every response with headers (`X-DIN-Method`, `X-DIN-CUs`, `X-DIN-Sig`). This locks in what the provider claims to have served at request time—not settlement time.

Without attestation, a provider could serve a cheap method (1 CU) but claim an expensive one (50 CUs) at settlement. Auto-averaging would give them 25.5 CUs. With attestation, the consumer holds cryptographic proof of what was actually served—the provider cannot lie.

**Trust guarantees:**
- **Consumer protected from:** Provider over-reporting (consumer claim caps charge), paying more than session max
- **Provider protected from:** Consumer under-reporting (can dispute with logs), non-payment (funds locked in contract)

---

### 5. Dispute Resolution

**Default: Auto-averaging** handles most discrepancies. Neither party gains significantly by lying:
- Consumer lies low → saves at most half the difference
- Provider lies high → gains at most half the difference
- Incentive to cheat is small relative to reputation damage

**Cumulative pattern detection:** The protocol tracks discrepancy patterns over a 30-day window. If a provider consistently over-reports (e.g., +3% average across 50 settlements), they're flagged and their health score is reduced.

**Escalation process:** For suspected malicious behavior, consumers can escalate with a $25 fee (refunded if valid). Provider has 7 days to respond with evidence. DIN team reviews within 14 days.

**Penalty escalation:**

| Offense | 1st Strike | 2nd Strike | 3rd Strike |
|---------|------------|------------|------------|
| Provider guilty | 25% reward reduction | 50% reward reduction | Removed |
| Consumer false claim | Lose $25 fee | Lose $50 fee | Banned from escalations |

---

### 6. Settlement Coordinator

An off-chain service that orchestrates checkpoints and batches settlements on-chain.

**Why off-chain?**
- Lower gas costs (only final settlements on-chain)
- Faster reconciliation (not limited by block times)
- Easy to iterate and improve
- Privacy (only settlements public, not individual claims)

**Key property:** The coordinator has **no custody of funds**. It only orchestrates signed claims. The smart contract remains the source of truth.

**If coordinator goes down:**
- Existing sessions continue working (providers verify on-chain state directly)
- Settlement is delayed but not lost
- Funds remain secure in contract
- After session expiry + 24 hours, consumers can force-unlock directly

**Phase 1 (known limitation):** DIN team operates the coordinator. This is an intentional trade-off for faster iteration. Future phases will explore decentralized coordination.

---

### 7. Abuse Prevention

| Mechanism | What It Prevents | How It Works |
|-----------|------------------|--------------|
| **Abandonment tracking** | Session creation spam | Sessions with < 5% usage flagged; high abandonment = rate limits |
| **Session rate limits** | Probing/DoS attacks | 0-10% abandonment: unlimited; >50%: 1 session/hour |
| **Method attestation** | Provider billing fraud | Cryptographic proof of what was served |
| **Cumulative detection** | Systematic over-reporting | 30-day pattern tracking; health score reduction |

---

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Provider over-reporting | Consumers overcharged | Bilateral reconciliation; method attestation; cumulative detection |
| Consumer under-reporting | Providers underpaid | Provider dispute with logs; escalation process |
| Settlement delays | Provider cash flow issues | 30-minute checkpoints; predictable settlement |
| Coordinator downtime | Settlement delayed | Sessions continue; force-unlock after 24h; funds safe |
| Smart contract bugs | Potential fund loss | Audits, gradual rollout, bug bounties |
| Session creation spam | Provider resource waste | Abandonment tracking; rate limits |
| Coordinator centralization | Single point of trust | No custody; force-unlock; decentralization roadmap |

---

## Success Metrics

| Area | Metric | Target |
|------|--------|--------|
| **Consumer** | Time to first request | < 30 seconds |
| **Consumer** | Automatic failover success | > 99% |
| **Consumer** | Cost savings vs direct | > 20% |
| **Provider** | Time to payment | < 24 hours |
| **Provider** | Traffic increase | > 50% |
| **Protocol** | Routing efficiency | > 90% optimal |
| **Protocol** | Dispute rate | < 1% |
| **Protocol** | Settlement accuracy | > 99.9% |

---

## Open Questions

1. **Launch pricing:** CU-only, per-request only, or both from day one?
2. **Checkpoint frequency:** Is 30 minutes the right balance?
3. **Dispute thresholds:** Is 5% the right threshold for flagging vs. auto-averaging?
4. **Multi-session support:** Should consumers have multiple concurrent sessions (e.g., dev vs. prod)?
5. **Coordinator decentralization:** When to prioritize moving from DIN-operated to decentralized?
6. **Session duration:** Is 7 days the right maximum?

---

## Related Documents

- [Full Product Proposal](./PRODUCT-PROPOSAL-din-payments.md) — Complete specification with diagrams and examples
- [Technical RFC](./RFC-din-payments.md) — Implementation details and contract interfaces
