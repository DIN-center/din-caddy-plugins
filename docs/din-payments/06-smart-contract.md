# Smart Contract Architecture

## Overview

This document covers the DINProtocol smart contract deployed on Base chain. The protocol uses a **unified contract** that combines the registry and payment system into a single contract.

## Contract Design

### Why a Unified Contract?

| Approach | Pros | Cons |
|----------|------|------|
| Separate contracts (Registry + Settlement) | Modular | Complex interactions, multiple deployments |
| **Unified contract** | Simple, single source of truth | Larger contract size |

We chose a unified contract for simplicity - consumers deposit, providers register, and settlement all happen in one place.

### DINProtocol Contract

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/utils/cryptography/ECDSA.sol";

contract DINProtocol {
    using ECDSA for bytes32;

    // ============ State Variables ============

    IERC20 public immutable usdc;

    // Protocol-level pricing (set by governance)
    uint256 public pricePerCU;  // Universal CU price in USDC (6 decimals)

    // Consumer accounts
    mapping(address => ConsumerAccount) public consumers;

    // Provider rate cards
    mapping(address => ProviderRateCard) public providerRates;

    // Session tracking
    mapping(bytes32 => Session) public sessions;
    mapping(bytes32 => bool) public sessionSettled;
    mapping(address => bytes32) public activeSession;  // One active session per consumer

    // ============ Structs ============

    struct ConsumerAccount {
        uint256 balance;           // USDC deposited (not locked)
        uint256 locked;            // USDC locked in active sessions
        uint256 totalSpent;        // Historical spend (for analytics)
        bool active;               // Account active
    }

    struct Session {
        address consumer;
        uint256 maxSpend;
        uint256 lockedAmount;
        uint256 spentAmount;
        uint64 validUntil;
        uint256 snapshotBlock;     // Block at which rate card prices were snapshotted
        bool active;
    }

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

    // ============ Consumer Functions ============

    function deposit(uint256 amount) external {
        usdc.transferFrom(msg.sender, address(this), amount);
        consumers[msg.sender].balance += amount;
        consumers[msg.sender].active = true;

        emit ConsumerDeposited(msg.sender, amount);
    }

    function withdraw(uint256 amount) external {
        ConsumerAccount storage account = consumers[msg.sender];
        uint256 available = account.balance - account.locked;
        require(available >= amount, "Insufficient unlocked balance");

        account.balance -= amount;
        usdc.transfer(msg.sender, amount);

        emit ConsumerWithdrawn(msg.sender, amount);
    }

    function startSession(
        uint256 maxSpend,
        uint64 duration,
        string calldata paymentMode,
        string calldata routingStrategy
    ) external returns (bytes32 sessionId) {
        // Enforce one active session per consumer
        require(activeSession[msg.sender] == bytes32(0), "Session already active");

        ConsumerAccount storage account = consumers[msg.sender];

        // Check available balance
        uint256 available = account.balance - account.locked;
        require(available >= maxSpend, "Insufficient unlocked balance");

        // Lock funds
        account.locked += maxSpend;

        // Create session ID (unique per consumer + timestamp + amount)
        sessionId = keccak256(abi.encodePacked(
            msg.sender,
            block.timestamp,
            maxSpend
        ));

        // Track active session
        activeSession[msg.sender] = sessionId;

        sessions[sessionId] = Session({
            consumer: msg.sender,
            maxSpend: maxSpend,
            lockedAmount: maxSpend,
            spentAmount: 0,
            validUntil: uint64(block.timestamp) + duration,
            snapshotBlock: block.number,  // Snapshot rate card prices at session start
            active: true
        });

        emit SessionStarted(sessionId, msg.sender, maxSpend, duration);
    }

    // ============ Provider Functions ============

    function registerRateCard(RateCardParams calldata params) external {
        // Store provider rate card
        // ... implementation details
        emit ProviderRateCardUpdated(msg.sender);
    }

    function updateRateCard(RateCardParams calldata params) external {
        // Update existing rate card
        // ... implementation details
        emit ProviderRateCardUpdated(msg.sender);
    }

    // Note: No withdrawEarnings - providers receive USDC directly at settlement

    // ============ Settlement Functions ============

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
            address recoveredConsumer = claimHash
                .toEthSignedMessageHash()
                .recover(sc.consumerSig);
            require(
                recoveredConsumer == session.consumer,
                "Invalid consumer signature"
            );

            // Verify provider signature
            address recoveredProvider = claimHash
                .toEthSignedMessageHash()
                .recover(sc.providerSig);
            require(
                recoveredProvider == sc.claim.provider,
                "Invalid provider signature"
            );

            // Transfer from consumer's locked funds directly to provider wallet
            uint256 amount = sc.claim.amount;
            require(
                session.lockedAmount - session.spentAmount >= amount,
                "Insufficient locked funds"
            );

            session.spentAmount += amount;

            // Update consumer account
            consumers[session.consumer].locked -= amount;
            consumers[session.consumer].balance -= amount;
            consumers[session.consumer].totalSpent += amount;

            // Direct transfer to provider - no withdrawal step needed
            usdc.transfer(sc.claim.provider, amount);

            emit UsageSettled(sc.claim.sessionId, sc.claim.provider, amount);
        }
    }

    function settleDispute(
        bytes32 sessionId,
        DisputeResolution calldata resolution
    ) external {
        // Only protocol operator can settle disputes
        // ... implementation details
        emit DisputeResolved(sessionId, resolution.settledAmount);
    }

    function endSession(bytes32 sessionId) external {
        Session storage session = sessions[sessionId];
        require(session.consumer == msg.sender, "Not session owner");
        require(session.active, "Session not active");

        // Final settlement would happen here via coordinator
        // ...

        // Mark session inactive
        session.active = false;

        // Clear active session tracker (allows consumer to start new session)
        activeSession[msg.sender] = bytes32(0);

        emit SessionEnded(sessionId, msg.sender, session.spentAmount, session.lockedAmount - session.spentAmount);
    }

    // ============ View Functions ============

    function getConsumerBalance(address consumer)
        external
        view
        returns (uint256 total, uint256 locked, uint256 available)
    {
        ConsumerAccount storage account = consumers[consumer];
        total = account.balance;
        locked = account.locked;
        available = total - locked;
    }

    function getSession(bytes32 sessionId)
        external
        view
        returns (Session memory)
    {
        return sessions[sessionId];
    }

    function isSessionActive(bytes32 sessionId)
        external
        view
        returns (bool)
    {
        Session storage session = sessions[sessionId];
        return session.active && block.timestamp < session.validUntil;
    }

    // ============ Events ============

    event ConsumerDeposited(address indexed consumer, uint256 amount);
    event ConsumerWithdrawn(address indexed consumer, uint256 amount);
    event SessionStarted(
        bytes32 indexed sessionId,
        address indexed consumer,
        uint256 maxSpend,
        uint64 duration
    );
    event ProviderRateCardUpdated(address indexed provider);
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
    event DisputeRaised(bytes32 indexed sessionId, address indexed raiser);
    event DisputeResolved(bytes32 indexed sessionId, uint256 settledAmount);
}
```

---

## Provider Rate Card Storage

### On-Chain Structure

Rate cards are stored in the contract with method-specific pricing:

```solidity
// Method-specific pricing (stored as mappings)
mapping(uint256 => mapping(uint16 => uint256)) public methodCUCosts;
mapping(uint256 => mapping(uint16 => uint256)) public methodPrices;

// Protocol-level pricing (set by governance)
uint256 public pricePerCU;  // Universal CU price in USDC (6 decimals)
```

### Provider Registration

```solidity
struct RateCardParams {
    string[] services;              // Services this provider supports
    bool supportsCUPricing;         // Accept CU-based payments
    bool supportsRequestPricing;    // Accept per-request payments
    uint256 defaultCUCost;          // Default CUs for unlisted methods
    uint256 defaultPricePerRequest; // Default price for unlisted methods
    MethodCUCost[] methodCUs;       // CU costs per method
    MethodPrice[] methodPrices;     // Direct prices per method
}

struct MethodCUCost {
    string service;
    string method;
    uint256 cuCost;
}

struct MethodPrice {
    string service;
    string method;
    uint256 price;  // USDC with 6 decimals
}
```

---

## Session Management

### Session Lifecycle

```
SESSION ON-CHAIN LIFECYCLE
==========================

1. Consumer calls startSession()
   - Locks funds from deposited balance
   - Creates session record
   - Emits SessionStarted event

2. Session is active
   - isSessionActive() returns true
   - Providers can query session state
   - Usage tracked off-chain

3. Checkpoints (every 30 min or 60% spend)
   - settleBatch() called with signed claims
   - Providers receive USDC directly
   - Session.spentAmount increases
   - Session remains active

4. Session ends (expiry or manual close)
   - Final settlement
   - Unused funds unlocked
   - Session marked inactive
   - Emits SessionEnded event
```

### Session Validation

Providers verify sessions by querying the contract:

```solidity
// Provider sidecar queries
function isSessionActive(bytes32 sessionId) external view returns (bool);
function getSession(bytes32 sessionId) external view returns (Session memory);
```

Providers cache this result and only re-verify at checkpoint boundaries.

---

## Settlement Mechanics

### Bilateral Signature Verification

Every settlement requires signatures from both parties:

```solidity
// Claim hash computation
bytes32 claimHash = keccak256(abi.encode(
    sessionId,
    provider,
    amount,
    timestamp
));

// Recover signer addresses
address recoveredConsumer = claimHash
    .toEthSignedMessageHash()
    .recover(consumerSig);

address recoveredProvider = claimHash
    .toEthSignedMessageHash()
    .recover(providerSig);

// Verify both signatures
require(recoveredConsumer == session.consumer);
require(recoveredProvider == claim.provider);
```

### Direct Provider Payment

Providers receive USDC directly at settlement - no withdrawal step:

```solidity
// Transfer from consumer's locked funds to provider wallet
usdc.transfer(claim.provider, amount);
```

This simplifies the flow and means providers don't need to interact with the contract to receive earnings.

---

## Access Control

### Roles

| Role | Capabilities |
|------|--------------|
| Consumer | Deposit, withdraw, start/end sessions |
| Provider | Register/update rate cards |
| Protocol Operator | Submit batch settlements, resolve disputes, update protocol params |
| Governance | Update pricePerCU, protocol fees |

### Protocol Operator

The Protocol Operator (Settlement Coordinator) is authorized to:
- Submit batch settlements
- Resolve disputes
- Update protocol parameters

```solidity
modifier onlyOperator() {
    require(msg.sender == operator, "Not authorized");
    _;
}

function settleBatch(SignedClaim[] calldata claims) external onlyOperator {
    // ...
}
```

---

## Security Considerations

### Fund Safety

| Risk | Mitigation |
|------|------------|
| Unauthorized withdrawal | Signature verification on all settlements |
| Double spending | Session.spentAmount tracking, on-chain state |
| Session manipulation | Session state stored on-chain, provider verification |
| Operator compromise | Operator cannot extract funds without valid signatures |

### Signature Security

- EIP-191 personal_sign for all signatures
- Timestamp included to prevent replay attacks
- Session ID prevents cross-session attacks

### Upgrade Path

For future upgrades:
- Contract uses OpenZeppelin's UUPS or Transparent proxy pattern
- State preserved across upgrades
- Emergency pause functionality included

---

## Gas Optimization

### Batch Operations

Settlements are batched to reduce gas costs:

| Operation | Gas (single) | Gas (batch of 10) | Savings |
|-----------|--------------|-------------------|---------|
| Settlement | ~50,000 | ~200,000 | 4x more efficient |

### Storage Optimization

- Pack structs to minimize storage slots
- Use uint64 for timestamps (not uint256)
- Store capabilities as bitmask (not string array)

---

## Events for Indexing

All major state changes emit events for off-chain indexing:

```solidity
// Consumer events
event ConsumerDeposited(address indexed consumer, uint256 amount);
event ConsumerWithdrawn(address indexed consumer, uint256 amount);

// Session events
event SessionStarted(
    bytes32 indexed sessionId,
    address indexed consumer,
    uint256 maxSpend,
    uint64 duration
);
event SessionEnded(
    bytes32 indexed sessionId,
    address indexed consumer,
    uint256 totalSpent,
    uint256 refunded
);

// Provider events
event ProviderRateCardUpdated(address indexed provider);

// Settlement events
event UsageSettled(
    bytes32 indexed sessionId,
    address indexed provider,
    uint256 amount
);

// Dispute events
event DisputeRaised(bytes32 indexed sessionId, address indexed raiser);
event DisputeResolved(bytes32 indexed sessionId, uint256 settledAmount);
```

### Indexer Queries

Common queries the indexer supports:
- Consumer balance history
- Provider earnings by period
- Session usage breakdown
- Dispute history

---

## Deployment

### Chain Selection

The contract is deployed on **Base** (Ethereum L2) for:
- Low gas costs (~$0.001-0.01 per transaction)
- Ethereum security
- Fast finality
- USDC availability

### Constructor Parameters

```solidity
constructor(
    address _usdc,           // USDC token address on Base
    uint256 _pricePerCU,     // Initial CU price (e.g., 80 = $0.00008)
    address _operator        // Initial protocol operator
)
```

### Initialization

```solidity
// Deploy with initial parameters
DINProtocol protocol = new DINProtocol(
    0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913, // USDC on Base
    80,                                          // $0.00008 per CU
    0xOperatorAddress                            // Settlement coordinator
);
```

