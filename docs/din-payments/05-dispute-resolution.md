# Dispute Resolution

## Overview

This document covers how disputes are handled, from auto-averaging for minor discrepancies to escalation for significant abuse.

## Auto-Average Settlement (Default)

The vast majority of claim mismatches are resolved automatically by averaging - no manual intervention needed:

```
AUTO-AVERAGE SETTLEMENT
=======================

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
AUTO-AVERAGE FLOW
=================

Consumer                 Contract                  Provider
    |                       |                          |
    |  Claim: $12.00        |         Claim: $15.00    |
    |---------------------->|<-------------------------|
    |                       |                          |
    |                       |  Calculate average:      |
    |                       |  ($12 + $15) / 2 = $13.50|
    |                       |                          |
    |                       |  Settle at $13.50        |
    |                       |------------------------->|
    |                       |                          |
    |  Debit $13.50         |                          |
    |<----------------------|                          |
```

---

## Discrepancy Thresholds

| Scenario | Threshold | Action |
|----------|-----------|--------|
| Exact match | 0% difference | Settle immediately |
| Minor discrepancy | < 5% difference | Average and settle |
| Major discrepancy | >= 5% difference | Hold 48 hours for escalation |

### Major Discrepancy Hold

When a major discrepancy is detected, settlement is **held for 48 hours** before defaulting to average:

```
MAJOR DISCREPANCY FLOW
======================

Hour 0: Discrepancy detected (>= 5%)
    |
    |  Settlement HELD (not processed)
    |  48-hour window for escalation
    |
    +--- Escalation filed during hold?
    |       |
    |       v
    |     Skip to Escalation Process
    |
    +--- No escalation within 48 hours?
            |
            v
          Settle by average (auto-averaged)
          Consumer still has 7 days to escalate after settlement
```

This gives both parties time to review logs and escalate if they believe there's significant abuse.

---

## Escalation Process

If a consumer suspects a provider is consistently inflating claims, they can escalate. The timeline differs based on discrepancy size.

### Time Windows

| Phase | Duration | Action Required |
|-------|----------|-----------------|
| Major discrepancy hold | 48 hours after detection | Settlement held; either party can escalate |
| Post-settlement window | 7 days after settlement | Consumer can still escalate after auto-average |
| Provider response | 7 days after escalation | Provider must submit proof |
| DIN team review | 14 days after response | Team makes decision |
| Penalty applied (if guilty) | Immediate | AVS reward deducted + reputation penalty |

### Escalation Flow

```
ESCALATION PROCESS
==================

Day 0: Consumer files escalation
    |  - Provides session IDs
    |  - Provides local usage logs (JSON file)
    |  - Pays escalation fee ($25, refunded if valid)
    |  - Must complete identity verification
    |
    |  Provider notified, has 7 days to respond
    |
Day 7: Provider response deadline
    |
    +--- Provider submits proof (server logs)
    |       |
    |       v
    |    DIN team reviews (14 days)
    |       |
    |       +--- Provider at fault
    |       |       - 25% AVS reward deducted (1st strike)
    |       |       - Consumer refunded from deduction
    |       |       - Strike added to provider record
    |       |       - Health score reduced
    |       |
    |       +--- Consumer at fault
    |               - Consumer loses escalation fee
    |               - Warning added to consumer record
    |
    +--- Provider fails to respond
            |
            v
         Provider penalized via AVS rewards
            |
            - 25% AVS reward deducted (1st offense)
            - Strike added to record
            - 3rd no-response → Removed + slashed
```

---

## Penalties

Penalties are deducted from the provider's AVS staking rewards - no separate fee payment required.

### Provider Penalties

| Offense | 1st Strike | 2nd Strike | 3rd Strike |
|---------|------------|------------|------------|
| Guilty or no-response | 25% reward reduction | 50% reward reduction | 100% reward reduction + removed |

### Consumer Penalties

| Offense | 1st Offense | 2nd Offense | 3rd Offense |
|---------|-------------|-------------|-------------|
| False claim | Lose escalation fee | 2x fee penalty | Banned from escalations |

### AVS Reward Deduction Examples

```
AVS REWARD DEDUCTIONS
=====================

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

### Reputation Impact

```
REPUTATION IMPACT ON PROVIDER
=============================

Before dispute:                  After guilty verdict:
+--------------------+           +--------------------+
|  Health Score: 95  |           |  Health Score: 75  |
|  AVS Reward: 100%  |           |  AVS Reward: 75%   |
|  Traffic Share: 25%|           |  Traffic Share: 15%|
+--------------------+           +--------------------+

Result: Provider earns less until they rebuild reputation
```

Additional effects:
- **Health score reduced** - Watcher marks provider as less reliable
- **Less traffic routed** - Routing algorithms deprioritize low-reputation providers

---

## Escalation Fee

The escalation fee prevents spam escalations while protecting legitimate claims:

| Outcome | Fee Handling |
|---------|--------------|
| Consumer's claim validated | Fee refunded in full ($25 returned) |
| Provider found at fault | Fee refunded + consumer compensated from AVS deduction |
| Consumer's claim invalid | Fee forfeited ($25 kept by protocol) |
| Consumer's claim malicious | Fee forfeited + consumer banned |

### Fee Flow

```
ESCALATION FEE FLOW
===================

Consumer                  Protocol Treasury              Provider
    |                            |                          |
    |  Pay $25 escrow            |                          |
    |-------------------------->|                          |
    |                            |                          |
    |                    Fee held in escrow                 |
    |                            |                          |
    |  ... investigation ...     |                          |
    |                            |                          |
Outcome A: Consumer valid        |                          |
    |                            |                          |
    |  Refund $25                |  Deduct from AVS rewards |
    |<---------------------------|------------------------->|
    |                            |                          |
    |  + Compensation from       |                          |
    |    provider's reward       |                          |
    |                            |                          |
Outcome B: Consumer invalid      |                          |
    |                            |                          |
    |  Fee forfeited             |                          |
    |  (stays in treasury)       |                          |
```

**Why $25?**
- High enough to deter frivolous escalations
- Low enough to not discourage legitimate claims
- Covers administrative cost of manual review

---

## Identity Verification (Anti-Abuse)

To prevent consumers from creating new wallets to spam escalations, identity verification is required.

### Requirements

| Verification Type | Description | When Required |
|-------------------|-------------|---------------|
| Wallet linking | Consumer must have minimum $100 lifetime spend | Always (automated) |
| Email verification | Consumer provides email, receives confirmation | First escalation |
| KYC (optional) | Full identity verification | After 2 escalations |

### Anti-Abuse Mechanism

```
ANTI-ABUSE MECHANISM
====================

Attack: Create new wallet → Spam escalation → Create another wallet

Defense:
+-------------------------------------------------------------+
|  1. Minimum spend requirement ($100 lifetime)               |
|     New wallets can't escalate until they've spent $100     |
|     Cost to spam: $100 + $25 fee per fake account          |
|                                                             |
|  2. Email verification (linked to consumer identity)        |
|     Same email can't be used for multiple escalations       |
|     across different wallets within 90 days                 |
|                                                             |
|  3. Progressive KYC                                         |
|     After 2 escalations: KYC required                       |
|     Creates legal accountability for false claims           |
+-------------------------------------------------------------+
```

### Verified Consumer Definition

A consumer is considered "verified" when they meet ALL of the following:

| Requirement | How It's Checked |
|-------------|------------------|
| $100+ lifetime spend | Automated - protocol tracks total settled usage per wallet |
| Email verified | Consumer submits email, clicks confirmation link |
| No active ban | No 2+ false claims on record for this identity |

### Rate Limits

| Consumer Status | Escalation Limit |
|-----------------|------------------|
| Unverified (< $100 spend OR no email) | Cannot escalate |
| Verified (meets all requirements) | 3 escalations per 90 days |
| Verified + 1 prior false claim | 1 escalation per 180 days |
| Verified + 2+ false claims | Banned from escalations |

### Escalation Tracking

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

---

## Evidence Requirements

### Consumer Evidence

When filing an escalation, consumers must provide:

| Evidence | Description |
|----------|-------------|
| Session IDs | The specific sessions in question |
| Usage logs | Local `~/.din/session-log.json` file |
| Time range | When the alleged over-reporting occurred |
| Pattern description | Why they believe this is intentional abuse |

### Provider Evidence

When responding to an escalation, providers must provide:

| Evidence | Description |
|----------|-------------|
| Server logs | Raw request logs showing requests served |
| Method breakdown | CUs charged per method |
| Timestamps | When each request was processed |
| Technical explanation | Why their numbers may differ |

---

## Resolution Outcomes

### Provider Found at Fault

```
PROVIDER GUILTY
===============

1. Consumer receives:
   - Escalation fee refunded ($25)
   - Compensation from provider's AVS deduction
   - Difference between claimed and actual usage

2. Provider receives:
   - Strike on record
   - AVS reward reduction (25%/50%/100%)
   - Health score reduction
   - Reduced traffic routing

3. If 3rd strike:
   - Provider removed from protocol
   - Full AVS reward forfeited
```

### Consumer Found at Fault

```
CONSUMER INVALID CLAIM
======================

1. Consumer:
   - Loses escalation fee ($25)
   - Warning added to record
   - If 2nd false claim: Banned from escalations

2. Provider:
   - No penalty
   - Original settlement stands
```

### No Response from Provider

```
PROVIDER NO-RESPONSE
====================

Treated as admission of guilt:
- Provider penalized via AVS rewards
- Consumer compensated
- Strike added to provider record
```

---

## Best Practices

### For Consumers

1. **Keep local logs** - Don't delete `~/.din/session-log.json`
2. **Monitor spending** - Track your usage against provider claims
3. **Escalate promptly** - File within 7 days of settlement
4. **Provide evidence** - Include all relevant logs and session IDs

### For Providers

1. **Maintain accurate logs** - Keep detailed server logs
2. **Respond to escalations** - No-response is treated as guilt
3. **Be consistent** - Accurate reporting builds reputation
4. **Compete on quality** - Don't inflate claims; it damages reputation

### Escalation is Rare

**Auto-averaging handles 99% of cases.** Escalation is only for repeated, significant abuse where there's a clear pattern of malicious behavior.

---

## Edge Case Resolution

### Provider Offline During Settlement

If a provider is unresponsive during settlement:

```
PROVIDER OFFLINE SCENARIO
=========================

Checkpoint triggered (30 min or 60% spend)
    |
    |  Consumer submits usage claim
    |  Provider fails to respond within 15 minutes
    |
    v
Fallback: Settle based on consumer's claim
    |
    |  Provider can dispute later (within 7 days)
    |  if they have evidence of higher usage
    |
    v
Session continues with consumer's claimed amount
```

**Rationale:** Consumers shouldn't be blocked from settling due to provider issues. The session log is authoritative for the consumer's view.

### Network Congestion Handling

When network congestion prevents timely settlement:

```
NETWORK CONGESTION OPTIONS
==========================

Option 1: Extend checkpoint window
- If settlement tx is pending > 10 minutes
- Extend session validity by 30 minutes
- Retry settlement with higher gas priority

Option 2: Optimistic continuation
- Continue session with local tracking
- Batch multiple checkpoint settlements
- Settle when congestion clears

Recommended: Option 1 for safety, Option 2 for UX
```

| Scenario | Action |
|----------|--------|
| Settlement tx pending | Extend session, retry with priority gas |
| Settlement tx failed | Retry with increased gas, notify consumer |
| Prolonged congestion (>1 hour) | Pause new sessions, complete existing |

### Ambiguous Evidence Cases

When both parties provide seemingly valid evidence:

```
AMBIGUOUS EVIDENCE RESOLUTION
=============================

Both parties submit logs showing different amounts
    |
    v
DIN team reviews:
1. Timestamp alignment between consumer and provider logs
2. Request-response correlation
3. Network evidence (if available)
4. Pattern analysis (is this consistent with normal behavior?)
    |
    +--- Clear fault identified --> Standard penalty
    |
    +--- Genuinely ambiguous --> Split the difference
            |
            - Neither party penalized
            - Settlement at midpoint
            - Flag for monitoring (future sessions tracked closely)
```

**Split-the-difference principle:** When fault cannot be determined, neither party is penalized but the issue is logged for pattern detection.

---

## Future Enhancements

The following functionality is planned for future releases to strengthen dispute resolution:

| Feature | Description | Status |
|---------|-------------|--------|
| **Session Pause** | Temporarily pause a session during investigation | Planned |
| **Provider Suspension** | Temporarily suspend a provider pending review | Planned |
| **Consumer Blacklisting** | Block repeat bad actors across all wallets | Planned |
| **Automated Pattern Detection** | ML-based detection of suspicious claim patterns | Future |

These features will be implemented as the protocol matures and edge cases are better understood through production usage.

