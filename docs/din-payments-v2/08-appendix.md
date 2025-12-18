# Appendix

## Overview

This document covers additional topics including alternative payment paths, protocol fees, success metrics, and open questions for the DIN Protocol.

---

## x402 Alternative Payment Path

For users without a deposit, x402 micropayments provide an alternative entry point.

### Payment Path Selection

```
PAYMENT PATH SELECTION
======================

New request arrives
       |
       |  Has active session with locked funds?
       |
       +-- YES --> Use Session-Based Payment (this RFC)
       |           - Lower overhead per request
       |           - Faster (no payment per request)
       |           - Better for regular users
       |
       +-- NO ---> Use x402 Micropayments
                   - No deposit required
                   - Pay per request
                   - Higher per-request cost
                   - Good for first-time/low-volume users
```

### When to Use x402

| Use Case | Recommendation |
|----------|----------------|
| First-time user | x402 (try before depositing) |
| One-off request | x402 (not worth depositing) |
| Low volume (< 100 requests/month) | x402 (simpler) |
| Regular usage | Session-based (cheaper) |
| High volume | Session-based (much cheaper) |

### Onboarding Flow

```
x402 ONBOARDING FLOW
====================

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

### Fee Flow Example

```
PROTOCOL FEE FLOW (Example: 5%)
===============================

Consumer spends $100 during session
            |
            v
Settlement breakdown:
+--------------------------------+
|  Gross spend:        $100.00   |
|  Protocol fee (5%):  -$5.00    |
|  Net to providers:   $95.00    |
+--------------------------------+
            |
            v
Provider distribution:
+--------------------------------+
|  Provider A (60%):   $57.00    |
|  Provider B (40%):   $38.00    |
+--------------------------------+
```

### Potential Fee Usage

| Use | Description |
|-----|-------------|
| Settlement Coordinator gas | Covers transaction costs for batched settlements |
| Infrastructure | Coordinator service hosting, monitoring |
| Protocol development | Ongoing improvements, audits |
| Ecosystem grants | Developer incentives, integrations |

### Fee Scenarios

| Fee Rate | Consumer Spend | Protocol Revenue | Provider Revenue |
|----------|----------------|------------------|------------------|
| 2% | $1,000 | $20 | $980 |
| 5% | $1,000 | $50 | $950 |
| 10% | $1,000 | $100 | $900 |

### Contract Implementation (If Implemented)

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

### Key Performance Indicators

| Metric | Target | Description |
|--------|--------|-------------|
| Consumer UX | < 30 seconds | Time to first request |
| Routing efficiency | > 90% | Requests served by optimal provider |
| Settlement accuracy | < 1% | Dispute rate |
| Provider earnings | < 24 hours | Average time to payment |
| Service health | > 99% | Automatic failover success rate |

### Operational Metrics

| Metric | Description |
|--------|-------------|
| Active sessions | Number of concurrent active sessions |
| Total locked value | USDC locked in active sessions |
| Checkpoint success rate | % of checkpoints completing without error |
| Provider response time | Time for providers to sign claims |
| Gas costs per settlement | Average gas cost per settlement batch |

---

## Open Questions

### Governance

- Who operates the protocol initially?
- What is the path to decentralization?
- How are protocol parameters updated?

### Staking

- Should providers stake for quality guarantees?
- What are slashing conditions?
- How does staking interact with AVS rewards?

### Cross-Chain

- Settlement on Base, providers on multiple chains?
- How to handle multi-chain routing?
- Cross-chain session management?

---

## Implementation Roadmap

### Phase 1: Foundation

1. Design and deploy DINProtocol contract on Base testnet (Sepolia)
2. Implement session-based authorization in SDK
3. Build aggregated usage tracking with JSON file persistence
4. Create provider rate card registration system

### Phase 2: Settlement

5. Implement Settlement Coordinator service
6. Build checkpoint triggering system
7. Implement bilateral reconciliation logic
8. Add batch settlement to contract

### Phase 3: Production

9. Integration testing with multiple providers
10. Security audit
11. Mainnet deployment
12. Provider onboarding

---

## Supported Services Reference

### Service Types

| Service Type | Request Type | Examples |
|--------------|--------------|----------|
| `evm` | JSON-RPC | ethereum-mainnet, base-mainnet, polygon-mainnet |
| `solana` | JSON-RPC | solana-mainnet |
| `starknet` | JSON-RPC | starknet-mainnet |
| `bitcoin` | JSON-RPC | bitcoin-mainnet |
| `tron-full-node` | JSON-RPC | tron-mainnet |
| `beacon-chain` | REST | ethereum-beacon |
| `bitcoin-esplora` | REST | bitcoin-esplora |

### Adding New Services

New service types can be added by:
1. Defining the service type identifier
2. Specifying the request format (JSON-RPC or REST)
3. Adding method definitions for CU costs
4. Updating routing logic if needed

---

## Glossary

| Term | Definition |
|------|------------|
| **CU (Compute Unit)** | Standardized unit of compute cost. Protocol sets universal CU price. |
| **Session** | Time-bounded authorization to use locked funds for requests |
| **Checkpoint** | Periodic settlement point (every 30 min or 60% spend) |
| **Rate Card** | Provider's published pricing for their services |
| **Settlement Coordinator** | Off-chain service that orchestrates checkpoints and settlements |
| **Bilateral Reconciliation** | Two-party verification where both consumer and provider attest to usage |
| **Health Score** | Provider reliability metric (0-1) used in routing decisions |
| **Value Score** | Combined metric of health and cost efficiency for balanced routing |
| **x402** | Pay-per-request micropayment protocol for users without deposits |
| **AVS Rewards** | Actively Validated Services rewards for providers (used for penalties) |

---

## Related Documents

- [x402 Payment Protocol](https://github.com/coinbase/x402) - Underlying payment protocol
- [DIN Router SDK](/docs/din-router-sdk/) - Router architecture
- [DIN Protocol](/docs/din-protocol/) - Unified registry and payment system
- [Product Proposal](./PRODUCT-PROPOSAL-din-payments-v2.md) - High-level product overview
- [Technical RFC](./RFC-din-payments-v2.md) - Full technical specification

---

## Gas Cost Reference (Base Chain)

### Consumer Operations

| Operation | Estimated Gas | Estimated Cost |
|-----------|---------------|----------------|
| Deposit USDC | ~65,000 | ~$0.001-0.01 |
| Start session | ~80,000 | ~$0.001-0.01 |
| End session | ~50,000 | ~$0.001-0.01 |
| Withdraw USDC | ~60,000 | ~$0.001-0.01 |

### Protocol Operations

| Operation | Estimated Gas | Estimated Cost |
|-----------|---------------|----------------|
| Single settlement | ~50,000 | ~$0.001 |
| Batch settlement (10) | ~200,000 | ~$0.004 |
| Batch settlement (50) | ~700,000 | ~$0.014 |

### Provider Operations

| Operation | Estimated Gas | Estimated Cost |
|-----------|---------------|----------------|
| Register rate card | ~150,000 | ~$0.003 |
| Update rate card | ~80,000 | ~$0.002 |

---

## Error Codes Reference

| Code | Description | Resolution |
|------|-------------|------------|
| `NO_PROVIDERS` | No providers support the requested service + payment mode | Try different payment mode or service |
| `NO_PROVIDERS_FOR_MODE` | No providers support requested payment mode | Switch payment mode |
| `COST_LIMIT_EXCEEDED` | All providers exceed cost limit | Increase `maxCUsPerRequest` or `maxPricePerMethod` |
| `INSUFFICIENT_FUNDS` | Session spending limit exceeded | End session and start new one with higher limit |
| `SESSION_EXPIRED` | Session duration exceeded | Start new session |
| `SESSION_NOT_FOUND` | Session doesn't exist on-chain | Re-create session |
| `INVALID_SIGNATURE` | Consumer signature verification failed | Check wallet key |
| `PROVIDER_UNHEALTHY` | All providers below health threshold | Lower `minHealthThreshold` or wait |

