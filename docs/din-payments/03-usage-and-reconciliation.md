# Usage and Reconciliation

## Overview

This document covers how usage is tracked across the network and how the protocol reconciles claims from consumers and providers.

## Usage Tracking

### Why Aggregated Tracking?

To avoid massive logs, usage is aggregated by key dimensions rather than storing every individual request. This dramatically reduces storage requirements while maintaining accuracy for settlement.

```
AGGREGATED USAGE TRACKING
=========================

Instead of storing every request:

    { requestId: 1, provider: A, method: eth_call, CUs: 2, cost: 0.00016 }
    { requestId: 2, provider: A, method: eth_call, CUs: 2, cost: 0.00016 }
    { requestId: 3, provider: A, method: eth_call, CUs: 2, cost: 0.00016 }
    ... millions of rows ...


We aggregate by (provider, method):

    {
      key: "0xProviderA:eth_call",
      provider: "0xProviderA",
      method: "eth_call",
      totalCUs: 6000,           // Aggregated
      requestCount: 3000,       // Number of requests
      totalCost: 0.48           // Sum of individual costs
    }
```

### Aggregation Dimensions

Usage is aggregated by these dimensions:

| Dimension | Purpose |
|-----------|---------|
| `sessionId` | Isolate usage per session |
| `provider` | Track usage per provider for settlement |
| `method` | Enable method-level cost analysis |
| `pricingMode` | Separate CU vs per-request calculations |

### Usage Update Flow

```
REQUEST → USAGE UPDATE
======================

Request arrives
      |
      v
1. Route to provider, get response

2. Calculate usage:
   provider = "0xProviderA"
   method = "eth_call"
   CUs = 10 (from provider's rate card)
   cost = 10 * $0.00008 = $0.0008 (protocol CU price)

3. Aggregate key = "0xProviderA:eth_call"

4. UPSERT into usage_aggregates:
   - If key exists: Add to totals
   - If key doesn't exist: Create new entry

5. Update session totals:
   session.usedCUs += 10
   session.usedSpend += 0.0008
```

---

## Local Storage

### Session Log File

The SDK stores session state in a local JSON file (`~/.din/session-log.json`). This enables:
- **Session recovery** - Resume after SDK restart
- **Multi-instance sharing** - Same wallet key across instances
- **Checkpoint preparation** - Quick access to usage data

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

**Note:** Amounts are in USDC base units (6 decimals). 240000 = $0.24.

### Provider Logs

Providers also track usage locally for reconciliation:

```json
// Provider's usage log for session
{
  "sessionId": "0x7a3b...f9c2",
  "consumer": "0x1234...5678",
  "served": {
    "eth_call": { "cus": 3000, "requests": 1000 },
    "eth_blockNumber": { "cus": 500, "requests": 500 }
  },
  "lastUpdated": 1702857650
}
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
BILATERAL RECONCILIATION FLOW
=============================

CONSUMER SIDE                                  PROVIDER SIDE
-------------                                  -------------

SDK/Router tracks usage           <--------->  Provider logs requests
locally during session                         served for session

      |                                             |
      |  Session ends or checkpoint                 |
      v                                             v

Consumer submits:                            Provider submits:
"I used 3000 CUs from                        "I served 3000 CUs to
 Provider A"                                  Consumer X"

      |                                             |
      +-----------------------+---------------------+
                              |
                              v
                 +------------------------+
                 |   PROTOCOL COMPARES    |
                 +-----------+------------+
                             |
            +----------------+----------------+
            |                |                |
            v                v                v
         MATCH         MINOR DIFF       MAJOR DIFF
       (3000 = 3000)  (3000 vs 3100)  (3000 vs 4000)
            |                |                |
            v                v                v
         Settle         Average &      Hold 48 hours
       immediately       settle        for escalation
                       (3050 CUs)            |
                                             v
                                       No escalation?
                                       Settle by average
```

### Reconciliation Rules

| Scenario | Threshold | Action |
|----------|-----------|--------|
| Exact match | 0% difference | Settle immediately |
| Minor discrepancy | < 5% difference | Average and settle |
| Major discrepancy | >= 5% difference | Hold for 48 hours; if no escalation, settle by average |

### Example Reconciliation

```
RECONCILIATION EXAMPLE
======================

Consumer claims for Provider A:
  eth_call: 3000 CUs ($0.24)
  eth_getLogs: 2000 CUs ($0.16)
  Total: 5000 CUs ($0.40)

Provider A claims:
  eth_call: 3100 CUs ($0.248)
  eth_getLogs: 2000 CUs ($0.16)
  Total: 5100 CUs ($0.408)

Difference: 100 CUs / 5000 CUs = 2% (minor discrepancy)

Resolution: Average
  eth_call: (3000 + 3100) / 2 = 3050 CUs ($0.244)
  eth_getLogs: 2000 CUs ($0.16)
  Final: 5050 CUs ($0.404)
```

---

## Trust Model

```
TRUST GUARANTEES
================

Consumer protected from:
------------------------
- Provider over-reporting (consumer's claim caps the charge)
- Paying more than session max (locked amount enforced)
- Malicious providers (minimum health threshold)

Provider protected from:
------------------------
- Consumer under-reporting (provider can dispute with logs)
- Non-payment (consumer pre-deposited funds are locked)
- Session exhaustion (checkpoints ensure timely payment)

Protocol guarantees:
--------------------
- Settlement only occurs when both parties agree (or dispute resolved)
- Neither party can unilaterally extract funds
- Transparent reconciliation process
```

### Why Not Single-Party Attestation?

| Model | Problem |
|-------|---------|
| Trust consumer only | Consumer under-reports, providers don't get paid fairly |
| Trust provider only | Provider over-reports, consumers get overcharged |
| Trust protocol/router only | Single point of failure, centralization risk |
| **Bilateral reconciliation** | Neither party can cheat, disputes are resolvable |

---

## Usage Claim Structure

### Consumer Usage Claim

Submitted at checkpoint or session end:

```typescript
interface UsageClaim {
  sessionId: string;
  consumer: string;
  checkpointNumber: number;
  timestamp: number;

  aggregates: ProviderAggregate[];

  signature: string;  // Consumer's signature
}

interface ProviderAggregate {
  provider: string;
  methodUsage: MethodUsage[];
  totalCUs: number;
  totalCost: number;
}

interface MethodUsage {
  method: string;
  cus: number;
  requests: number;
  cost: number;
}
```

### Provider Usage Claim

Submitted in response to consumer claim:

```typescript
interface ProviderClaim {
  sessionId: string;
  provider: string;
  checkpointNumber: number;
  timestamp: number;

  // Provider's view of usage
  claimedCUs: number;
  claimedCost: number;

  // Match consumer claim?
  status: 'confirmed' | 'disputed';

  // If disputed
  actualCUs?: number;
  actualCost?: number;
  evidence?: string;

  signature: string;  // Provider's signature
}
```

---

## Multi-Instance Usage Aggregation

When a session is shared across multiple SDK instances:

```
MULTI-INSTANCE AGGREGATION
==========================

                    Consumer Wallet: 0xABC
                           Session: $50 locked
                              |
            +-----------------+-----------------+
            |                 |                 |
            v                 v                 v
    +-------------+   +-------------+   +-------------+
    | SDK Instance|   | SDK Instance|   | SDK Instance|
    | (Laptop)    |   | (Server)    |   | (Lambda)    |
    | $5 used     |   | $20 used    |   | $8 used     |
    +------+------+   +------+------+   +------+------+
           |                 |                 |
           |  Report usage   |  Report usage   |  Report usage
           |                 |                 |
           +-----------------+-----------------+
                             |
                             v
                  +---------------------+
                  | Settlement          |
                  | Coordinator         |
                  |                     |
                  | Aggregate by        |
                  | session + provider: |
                  | Total: $33 used     |
                  +---------------------+
```

The Settlement Coordinator:
1. Receives usage reports from all SDK instances
2. Aggregates by session and provider
3. Compares aggregated consumer claims with provider claims
4. Reconciles and settles

---

## Usage Limits and Alerts

### Session Spending Limits

The SDK enforces spending limits locally:

```typescript
// Before each request
if (session.spentAmount + estimatedCost > session.maxSpend) {
  throw new Error('Session spending limit would be exceeded');
}

// After each request
session.spentAmount += actualCost;
```

### Checkpoint Triggers

| Trigger | Threshold | Action |
|---------|-----------|--------|
| Time-based | Every 30 minutes | Initiate checkpoint |
| Spend-based | 60% of locked amount used | Initiate checkpoint |

Whichever trigger hits first initiates the checkpoint.

### Why These Thresholds?

**30 minutes (time-based):**
- Multiple providers may serve the same session
- Each provider's session cache expires
- Regular checkpoints keep caches fresh
- Predictable settlement timing

**60% spend (spend-based):**
- Protects providers from session exhaustion
- Ensures providers receive payment before consumer funds run low
- Prevents race conditions when multiple providers serve one session

---

## Handling Edge Cases

### SDK Crash Recovery

If the SDK crashes mid-session:

```
SDK CRASH RECOVERY
==================

1. SDK crashes during active session

2. On restart, SDK checks ~/.din/session-log.json

3. If session exists and not expired:
   - Load session state
   - Verify on-chain (session still active?)
   - Resume tracking from last saved state

4. If session expired while offline:
   - Load usage data
   - Submit final claim to coordinator
   - Start new session if needed
```

### Provider Downtime

If a provider goes down during a session:

```
PROVIDER DOWNTIME
=================

1. Request fails to Provider A

2. Protocol routes to Provider B (automatic failover)

3. Usage tracked separately:
   - Provider A: usage before downtime
   - Provider B: usage after failover

4. At checkpoint:
   - Both providers receive claims for their portion
   - Each provider confirms their own usage
```

### Network Partition

If coordinator is unreachable:

```
COORDINATOR UNREACHABLE
=======================

1. Checkpoint trigger fires

2. SDK cannot reach coordinator

3. SDK continues operation:
   - Local usage tracking continues
   - Session remains valid (on-chain state unchanged)
   - Provider caches may expire (will re-verify)

4. When coordinator returns:
   - Submit accumulated usage
   - Catch up on missed checkpoints

Note: Funds remain secure in contract during coordinator outage.
```

