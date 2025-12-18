# Settlement

## Overview

This document covers how the protocol settles usage claims, pays providers, and handles the end-to-end settlement flow.

## Settlement Coordinator

The Settlement Coordinator is an off-chain service (hosted by DIN initially) that orchestrates checkpoints and settlement. This is simpler than on-chain averaging while maintaining bilateral verification.

### Coordinator Responsibilities

```
SETTLEMENT COORDINATOR
======================

1. TRIGGER CHECKPOINTS
   - Emit checkpoint events every 30 minutes (time-based)
   - OR when any session reports 60% spend (spend-based)
   - Whichever comes first

2. COLLECT CLAIMS
   - Accept usage reports from all SDK instances
   - Accept usage reports from all providers
   - Aggregate multiple SDK instance reports per session

3. RECONCILE
   - Match consumer <-> provider claims
   - Apply averaging for minor mismatches (auto-settle)
   - Hold major mismatches (>=5%) for 48 hours
   - Flag for escalation if either party disputes

4. SUBMIT ON-CHAIN
   - Batch multiple settlements into one transaction
   - Coordinator pays gas (funded by protocol fee)
   - Can batch daily to reduce transfers if desired
```

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

---

## Checkpoint Flow

### Checkpoint Triggers

Checkpoints are triggered by **whichever comes first**:

| Trigger | Threshold | Rationale |
|---------|-----------|-----------|
| Time-based | Every 30 minutes | Predictable settlement for all sessions |
| Spend-based | When 60% of locked amount used | Protect providers from session exhaustion |

### Why 30 Minutes?

- Multiple providers may serve the same session
- Each provider's session cache expires
- 30-minute checkpoints ensure providers can re-verify on-chain state regularly
- Balances settlement frequency vs gas costs

### Checkpoint Settlement Flow

```
CHECKPOINT SETTLEMENT FLOW
==========================

Consumer SDK                Protocol Contract              Provider Sidecar
     |                            |                              |
     |                            |                              |
     |  1. Sign usage claim       |       1. Sign usage claim    |
     |     (I used $12 at         |          (I served $12 to    |
     |      Provider X)           |           Consumer Y)        |
     |                            |                              |
     +------------+---------------+---------------+--------------+
                  |                               |
                  |     2. Submit both signed     |
                  |        claims to coordinator  |
                  |                               |
                  +---------------+---------------+
                                  |
                                  v
                  +-------------------------------+
                  |   SETTLEMENT COORDINATOR      |
                  |                               |
                  |   3. Compare claims           |
                  |   4. Apply reconciliation     |
                  |   5. Get signatures on final  |
                  +---------------+---------------+
                                  |
                                  v
                  +-------------------------------+
                  |   CONTRACT (on-chain)         |
                  |                               |
                  |   - Verify consumer sig       |
                  |   - Verify provider sig       |
                  |   - Transfer USDC to provider |
                  +-------------------------------+
```

---

## On-Chain Settlement

### Batch Settlement

Settlement happens at checkpoints or session end. Both consumer and provider submit signed usage claims, and the contract reconciles them.

```solidity
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

// Settlement with bilateral signatures - pays provider directly
function settleBatch(SignedClaim[] calldata claims) external {
    for (uint i = 0; i < claims.length; i++) {
        SignedClaim calldata sc = claims[i];
        Session storage session = sessions[sc.claim.sessionId];

        require(session.active, "Session not active");

        // Verify consumer signature
        // Verify provider signature
        // Ensure amounts match
        // Ensure funds available

        // Transfer from consumer's locked funds to provider
        session.spentAmount += sc.claim.amount;
        usdc.transfer(sc.claim.provider, sc.claim.amount);

        emit UsageSettled(sc.claim.sessionId, sc.claim.provider, sc.claim.amount);
    }
}
```

### Who Submits Settlements?

- Either consumer or provider can submit the transaction
- Both signatures are required, so neither can cheat
- Settlement Coordinator typically batches and submits
- Coordinator pays gas (funded by protocol fee)

---

## Provider Earnings

Providers receive USDC directly to their wallet at settlement - no withdrawal step needed.

```
DIRECT PAYMENT FLOW
===================

Settlement                              Provider Wallet
    |                                        |
    |  settleBatch() executes                |
    |                                        |
    |  usdc.transfer(provider, amount)       |
    |--------------------------------------->|
    |                                        |
    |                                        |  USDC received
    |                                        |  immediately
```

### Settlement Frequency Options

The Settlement Coordinator can batch transfers to reduce transaction count:

| Mode | Transfer Frequency | Use Case |
|------|-------------------|----------|
| **Immediate** | Every 30-min checkpoint | Real-time earnings, higher gas cost |
| **Daily batch** | Once per day | Reduced gas, aggregated settlements |

**Daily batching** sums all of a provider's earnings across the day and transfers once:

```
DAILY BATCHING EXAMPLE
======================

Provider A earnings:
- Checkpoint 1: $5.00
- Checkpoint 2: $8.00
- Checkpoint 3: $3.00
- ... (48 checkpoints/day)

Daily batch: Single $250 transfer instead of 48 transfers
```

**Trade-off:** Providers wait up to 24 hours for earnings vs. receiving every 30 minutes. Configurable per provider or protocol-wide.

### Provider Stats

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

## Session End Settlement

### Final Settlement Flow

When a session ends (expires or consumer closes):

```
SESSION END SETTLEMENT
======================

1. Session reaches expiry OR consumer calls endSession()

2. Final checkpoint triggered:
   - Consumer submits final usage claim
   - Providers submit their final claims

3. Reconciliation:
   - Match all consumer-provider claim pairs
   - Apply reconciliation rules
   - Get final signatures

4. On-chain settlement:
   - Transfer owed amounts to providers
   - Return unused locked funds to consumer's available balance

5. Session marked inactive:
   - Consumer can start new sessions
   - Unused funds immediately available
```

### Unused Funds

Funds that weren't spent during the session are returned to the consumer's available balance (not locked):

```
SESSION END ACCOUNTING
======================

Session started: $50 locked
Session spent:   $35 paid to providers

Final state:
- Provider earnings: $35 (transferred)
- Consumer available balance: +$15 (unlocked)
- Consumer locked balance: $0 (session ended)
```

---

## Settlement Gas Costs

### Cost Analysis (Base Chain)

| Action | Gas | Est. Cost | Who Pays |
|--------|-----|-----------|----------|
| Single settlement | ~50k | ~$0.001 | Protocol |
| Batch settlement (10 claims) | ~200k | ~$0.004 | Protocol |
| Batch settlement (50 claims) | ~700k | ~$0.014 | Protocol |

### Gas Optimization

1. **Batching:** Combine multiple settlements into one transaction
2. **Daily aggregation:** Sum provider earnings and transfer once daily
3. **Skip zero-value claims:** Don't submit empty checkpoints

---

## Failure Handling

### Settlement Coordinator Offline

```
COORDINATOR OFFLINE
===================

1. Checkpoint time arrives

2. Coordinator unreachable

3. Sessions continue operating:
   - Providers verify sessions on-chain directly
   - Usage tracking continues locally
   - Funds remain locked and secure

4. When coordinator returns:
   - Process backlog of checkpoints
   - Submit batched settlements
   - Catch up on missed periods

Note: No funds at risk during coordinator outage.
```

### Signature Collection Timeout

If one party doesn't sign within the timeout window:

```
SIGNATURE TIMEOUT
=================

Case 1: Consumer doesn't sign
- Provider can escalate with their logs
- Protocol reviews and may settle based on evidence

Case 2: Provider doesn't sign
- Treated as provider confirming consumer's claim
- Settle at consumer's claimed amount

Case 3: Neither signs
- Session times out
- Unused funds returned to consumer
- Provider forfeits unclaimed earnings
```

### Transaction Failure

```
TRANSACTION FAILURE
===================

1. settleBatch() transaction fails

2. Coordinator retries with:
   - Higher gas price
   - Smaller batch size
   - Individual settlements as fallback

3. If persistent failure:
   - Alert DIN team
   - Manual intervention
   - Funds remain secure in contract
```

---

## Settlement Security

### Bilateral Signature Requirement

Every settlement requires signatures from both parties:

| Attack | Why It Fails |
|--------|--------------|
| Consumer tries to underpay | Provider won't sign |
| Provider tries to overcharge | Consumer won't sign |
| Coordinator manipulates amounts | Both parties verify before signing |
| Replay attack | Timestamp and session ID prevent reuse |

### Settlement Amount Caps

```
SETTLEMENT CAPS
===============

1. Per-checkpoint cap: spentAmount <= lockedAmount - previousSpent

2. Session total cap: sum(allSettlements) <= maxSpend

3. Individual claim cap: claimAmount <= remaining locked balance

These are enforced both off-chain (coordinator) and on-chain (contract).
```

### Audit Trail

All settlements are recorded on-chain:

```typescript
// Events emitted for every settlement
event UsageSettled(
    bytes32 indexed sessionId,
    address indexed provider,
    uint256 amount
);

event SessionEnded(
    bytes32 indexed sessionId,
    address indexed consumer,
    uint256 totalSpent,
    uint256 refunded
);
```

