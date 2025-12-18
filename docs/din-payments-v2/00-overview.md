# DIN Protocol Payment System Overview

## Introduction

The DIN Protocol Payment System is a protocol-mediated payment infrastructure that simplifies how developers access blockchain infrastructure. Instead of managing relationships with individual RPC providers, consumers deposit funds once and the protocol handles everything: routing requests to the best providers, tracking usage, and settling payments automatically.

## Core Value Proposition

**For Consumers (Developers):**
- Deposit once, access any provider
- No negotiations or API keys per provider
- Automatic failover between providers
- Competitive pricing through provider competition
- Predictable costs with spending limits

**For Providers (Node Operators):**
- Publish pricing, serve requests, get paid automatically
- No billing infrastructure needed
- Compete on quality and price to win traffic
- Automatic payment settlement

## Problem Statement

Building a decentralized RPC network requires solving several payment challenges:

| Challenge | Description |
|-----------|-------------|
| **Consumer complexity** | Consumers shouldn't need to discover, evaluate, and manage relationships with individual providers |
| **Automatic failover** | If a provider goes down, routing should automatically switch without consumer intervention |
| **Unified liquidity** | Consumer funds should work across any provider in the network |
| **Provider discovery** | Consumers shouldn't need to understand which providers serve which services |
| **Competitive pricing** | Providers should compete on price and quality, benefiting consumers |

### The Coordination Problem

The network supports multiple entry points and providers:
- **Many SDKs** - Direct client libraries used by developers
- **Many Routers** - Gateway instances run by different parties
- **Many Providers** - RPC node operators serving requests

The protocol must coordinate usage tracking and settlement across all these participants, ensuring accurate attestation and fair payment distribution.

## Proposed Solution

A protocol-mediated system where:

1. **Providers publish rate cards** to the protocol (pricing for their services)
2. **Consumers deposit into the protocol** and set spending preferences
3. **Protocol routes requests** based on health, price, and consumer constraints
4. **Protocol settles payments** to providers based on verified usage

```
                    DIN Protocol-Mediated Payment Model

        Consumer ─────── deposits into ─────── DIN Protocol
                                                    │
                                                    │ routes requests
                                                    │ settles payments
                                                    ▼
                                                Providers
                         (protocol handles routing & payments)
```

## System Architecture

### High-Level Overview

```
+------------------------------------------------------------------+
|                       DIN Protocol Layer                          |
|                                                                   |
|  +----------------+  +----------------+  +----------------+       |
|  | Consumer       |  | Provider Rate  |  | Settlement     |       |
|  | Accounts       |  | Registry       |  | Engine         |       |
|  |                |  |                |  |                |       |
|  | - Deposits     |  | - Rate cards   |  | - Verify usage |       |
|  | - Locked       |  | - CU costs     |  | - Batch settle |       |
|  | - Preferences  |  | - Method prices|  | - Disputes     |       |
|  +-------+--------+  +-------+--------+  +-------+--------+       |
|          |                   |                   |                |
|          +-------------------+-------------------+                |
|                              |                                    |
|                  +-----------v-----------+                        |
|                  |    Routing Engine     |                        |
|                  |                       |                        |
|                  |  - Health scores      |                        |
|                  |  - Price filtering    |                        |
|                  |  - Load balancing     |                        |
|                  +-----------------------+                        |
|                                                                   |
+------------------------------------------------------------------+
         ^                                           |
         | deposits                                  | routes requests
         | session auth                              | settles payments
         |                                           v
+---------------------+                   +---------------------+
|     Consumers       |                   |     Providers       |
|                     |                   |                     |
|  +-----+  +-----+   |                   |  +---+ +---+ +---+  |
|  | SDK |  | SDK |   |                   |  | P | | P | | P |  |
|  +-----+  +-----+   |                   |  +---+ +---+ +---+  |
|                     |                   |                     |
|  +-----+  +-----+   |                   |  Publish rate cards |
|  | RTR |  | RTR |   |                   |  Serve requests     |
|  +-----+  +-----+   |                   |  Report usage       |
|                     |                   |                     |
+---------------------+                   +---------------------+
```

### Network Topology

The system supports many consumers (SDKs, Routers) and many providers:

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

## Key Components

### 1. DINProtocol Smart Contract

The unified smart contract on Base chain that manages:
- Consumer deposits and withdrawals
- Provider rate cards
- Session management (fund locking)
- Settlement and reconciliation
- Dispute resolution

### 2. Consumer SDK

TypeScript library for developers that handles:
- Wallet connection and deposits
- Session management
- Request routing
- Usage tracking
- Checkpoint submission

### 3. Provider Sidecar

Go service that providers run alongside their nodes:
- Session proof verification
- Usage tracking and reporting
- Health monitoring
- Settlement confirmation

### 4. Settlement Coordinator

Off-chain service that orchestrates:
- Checkpoint triggers
- Usage claim collection
- Bilateral reconciliation
- Batch settlement submission

## Payment Flow Summary

```
PAYMENT FLOW
============

1. SETUP (one-time)
   Consumer deposits USDC into DINProtocol contract
   Provider publishes rate card to DINProtocol contract

2. SESSION START
   Consumer calls startSession() on-chain
   Funds locked from deposit for session duration

3. REQUEST FLOW
   Consumer sends request with signed session proof
   Provider verifies proof against on-chain state (cached)
   Provider serves request
   Both parties track usage locally

4. CHECKPOINT (every 30 min or 60% spend)
   Both parties submit signed usage claims
   Settlement Coordinator reconciles claims
   Providers paid, session continues

5. SESSION END
   Final settlement
   Unused funds returned to consumer balance
```

## Supported Services

| Service Type | Blockchain | Request Type | Examples |
|--------------|------------|--------------|----------|
| `evm` | Ethereum-compatible | JSON-RPC | ethereum-mainnet, base-mainnet, polygon-mainnet |
| `solana` | Solana | JSON-RPC | solana-mainnet |
| `starknet` | Starknet | JSON-RPC | starknet-mainnet |
| `bitcoin` | Bitcoin | JSON-RPC | bitcoin-mainnet |
| `tron-full-node` | Tron | JSON-RPC | tron-mainnet |
| `beacon-chain` | Ethereum Consensus | REST | ethereum-beacon |
| `bitcoin-esplora` | Bitcoin (explorer) | REST | bitcoin-esplora |

## Document Index

This documentation is organized into the following sections:

| Document | Description |
|----------|-------------|
| [01 - Deposits and Sessions](./01-deposits-and-sessions.md) | Consumer deposits, session lifecycle, fund locking |
| [02 - Pricing and Routing](./02-pricing-and-routing.md) | Rate cards, pricing models, routing algorithm |
| [03 - Usage and Reconciliation](./03-usage-and-reconciliation.md) | Usage tracking, bilateral reconciliation |
| [04 - Settlement](./04-settlement.md) | Settlement coordinator, batch settlement, provider earnings |
| [05 - Dispute Resolution](./05-dispute-resolution.md) | Escalation process, penalties, anti-abuse |
| [06 - Smart Contract](./06-smart-contract.md) | DINProtocol contract architecture |
| [07 - SDK Integration](./07-sdk-integration.md) | Consumer SDK, provider sidecar |
| [08 - Appendix](./08-appendix.md) | Alternative payment paths, protocol fees, metrics |

## Related Documents

- [Product Proposal](./PRODUCT-PROPOSAL-din-payments-v2.md) - High-level product overview
- [Technical RFC](./RFC-din-payments-v2.md) - Full technical specification
