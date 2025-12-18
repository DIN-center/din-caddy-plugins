# Deposits and Sessions

## Overview

The DIN Protocol uses a deposit-based model where consumers pre-fund their accounts, then create sessions to use those funds for RPC requests. This document covers:
- Consumer deposits and account management
- Session lifecycle and fund locking
- Session authorization (decentralized)
- Checkpoints for long-running sessions
- Multi-instance session sharing

## Consumer Deposits

### Deposit Model

Consumers deposit USDC into the DINProtocol contract. On Base chain, this costs ~$0.001-0.01 per transaction.

```
Consumer                     Protocol Contract
   |                              |
   |  1. approve(protocol, $500)  |
   |     (one-time or per-deposit)|
   |----------------------------->|
   |                              |
   |  2. deposit($500)            |
   |----------------------------->|
   |                              |
   |  [Money held in contract]    |
   |  [Can withdraw anytime]      |
   |  [Use for many sessions]     |
```

### Why Deposit Instead of Pay-Per-Request?

| Approach | Pros | Cons |
|----------|------|------|
| Pay per request | No upfront commitment | Payment overhead on every request, slower |
| Deposit model | Fast requests (no payment per call), unified balance | Requires upfront deposit |

### Why Not EIP-2612 Permit?

We considered a permit-based approach where consumers sign off-chain and funds stay in their wallet. However:

1. **Security risk** - Consumer can move funds after signing permit but before settlement
2. **Marginal benefit** - On Base, saving one transaction saves ~$0.002-0.02
3. **Added complexity** - Permit handling, EIP-712 domains, deadline management
4. **Deposit is better UX** - Deposit once, start many sessions without re-approving

The deposit model is simpler, more secure, and costs nearly nothing on Base.

### Consumer Account Structure

```solidity
struct ConsumerAccount {
    uint256 balance;           // USDC deposited (not locked)
    uint256 locked;            // USDC locked in active sessions
    uint256 totalSpent;        // Historical spend (for analytics)
    bool active;               // Account active
}
```

**Consumer's view:**

```
Your DIN Balance
----------------

Total Deposited:    $100.00
Locked (sessions):  $ 50.00
Available:          $ 50.00

Active Sessions:
- Session #abc123: $50 locked
  Expires: Dec 18, 3:00 PM
```

### Deposit vs Start Session

```
DEPOSIT ($0.001-0.01 gas)
-------------------------
- Transfers USDC from wallet -> contract
- Adds funds to your account balance
- One-time or whenever you need more funds
- Funds remain available (not locked)

START SESSION ($0.001-0.01 gas)
-------------------------------
- No USDC transfer - just internal accounting
- Locks portion of deposited funds for this session
- Every time you start a new usage session
- Locked funds can't be withdrawn until session ends

Example Flow:
-------------
1. Consumer deposits $100 USDC (one transaction)
   -> Balance: deposited=$100, locked=$0, available=$100

2. Consumer starts session, locks $50
   -> Balance: deposited=$100, locked=$50, available=$50

3. Consumer makes requests, spends $35 over time
   -> (tracked off-chain, settled at checkpoints)

4. Session ends, settled for $35, unlocks remaining $15
   -> Balance: deposited=$65, locked=$0, available=$65

5. Consumer starts new session, locks $30
   -> Balance: deposited=$65, locked=$30, available=$35

Benefits:
- Deposit once, start many sessions (no repeated deposits)
- Small sessions don't require large upfront deposits
- Unused funds immediately available for new sessions
- Security: locked funds guaranteed for provider payment
```

---

## Session Lifecycle

### Overview

Sessions allow consumers to make many requests without per-request payments. The system uses **on-chain fund locking** for security and **consumer-signed session proofs** verified by providers.

```
SESSION LIFECYCLE
=================

1. DEPOSIT (one-time)
   Consumer deposits USDC into the protocol contract.
   Funds are held but not locked - available for use.

   Transaction: On-chain (consumer pays ~$0.001-0.01)

2. START SESSION
   Consumer locks a portion of deposited funds on-chain.
   Consumer signs a session proof with their wallet.

   Transaction: On-chain (consumer pays ~$0.001-0.01)

3. USE SESSION
   Consumer sends requests with signed session proof.
   Provider verifies on-chain (once), then caches.

   Transaction: Off-chain (no gas costs)

4. CHECKPOINT (every 30 min or 60% spend)
   Protocol settles usage for the period.
   Both consumer and provider report usage.
   Providers get paid, session continues.

   Transaction: On-chain (protocol pays, batched)

5. FINAL SETTLEMENT
   Protocol settles remaining usage.
   Unused funds returned to consumer's available balance.

   Transaction: On-chain (protocol pays, batched)
```

### Gas Costs (Base Chain)

| Action | Who Pays | Estimated Cost | What It Does |
|--------|----------|----------------|--------------|
| Deposit USDC | Consumer | ~$0.001-0.01 | Transfer USDC from wallet INTO contract |
| Start session | Consumer | ~$0.001-0.01 | Reserve deposited funds for session |
| Make RPC requests | - | $0 (off-chain) | Consumer-signed proof, provider verifies |
| Checkpoint settlement | Protocol | ~$0.001/session | Settle usage, keep session alive |
| Final settlement | Protocol | ~$0.001/session | Close session, unlock unused |
| Withdraw USDC | Consumer | ~$0.001-0.01 | Transfer USDC from contract to wallet |

### Full Session Flow

```
Consumer                   Smart Contract             Provider Sidecar
    |                          |                          |
    |  1. startSession($50)    |                          |
    |     (on-chain tx)        |                          |
    |------------------------->|                          |
    |                          |                          |
    |  2. SessionStarted event |                          |
    |     (sessionId, expiry)  |                          |
    |<-------------------------|                          |
    |                          |                          |
    |  3. Sign session proof   |                          |
    |     with wallet          |                          |
    |     (off-chain, no gas)  |                          |
    |                          |                          |
    |  4. First RPC Request + Signed Proof                |
    |---------------------------------------------------->|
    |                          |                          |
    |                          |     Verify signature     |
    |                          |     Query chain:         |
    |                          |<---- session valid? -----|
    |                          |----- yes, active ------->|
    |                          |     Cache session        |
    |                          |                          |
    |  5. Response                                        |
    |<----------------------------------------------------|
    |                          |                          |
    |  6. Subsequent requests (cache hit - no chain query)|
    |---------------------------------------------------->|
    |<----------------------------------------------------|
    |                          |                          |
    |  7. Checkpoint (periodic)                           |
    |                          |<---- Report usage -------|
    |----- Report usage ------>|                          |
    |                          |  Reconcile + settle      |
    |                          |                          |
```

---

## On-Chain Fund Locking

When a consumer starts a session, funds are **locked in the smart contract**. This prevents the consumer from withdrawing funds committed to a session.

```solidity
contract DINProtocol {
    mapping(address => uint256) public deposits;
    mapping(address => uint256) public locked;
    mapping(bytes32 => Session) public sessions;
    mapping(address => bytes32) public activeSession;  // One active session per consumer

    struct Session {
        address consumer;
        uint256 maxSpend;
        uint256 lockedAmount;
        uint64 validUntil;
        uint256 snapshotBlock;     // Block at which rate card prices were snapshotted
        bool active;
    }

    function deposit(uint256 amount) external {
        USDC.transferFrom(msg.sender, address(this), amount);
        deposits[msg.sender] += amount;
    }

    function startSession(
        uint256 maxSpend,
        uint64 duration,
        string calldata paymentMode,
        string calldata routingStrategy
    ) external returns (bytes32 sessionId) {
        // Enforce one active session per consumer
        require(activeSession[msg.sender] == bytes32(0), "Session already active");

        // Check available balance
        uint256 available = deposits[msg.sender] - locked[msg.sender];
        require(available >= maxSpend, "Insufficient unlocked balance");

        // Lock funds
        locked[msg.sender] += maxSpend;

        // Create session ID (unique per consumer + timestamp + amount)
        sessionId = keccak256(abi.encodePacked(msg.sender, block.timestamp, maxSpend));

        // Track active session
        activeSession[msg.sender] = sessionId;

        sessions[sessionId] = Session({
            consumer: msg.sender,
            maxSpend: maxSpend,
            lockedAmount: maxSpend,
            validUntil: uint64(block.timestamp) + duration,
            snapshotBlock: block.number,
            active: true
        });

        emit SessionStarted(sessionId, msg.sender, maxSpend, duration);
    }

    function endSession(bytes32 sessionId) external {
        Session storage session = sessions[sessionId];
        require(session.consumer == msg.sender, "Not session owner");
        require(session.active, "Session not active");

        session.active = false;

        // Clear active session tracker (allows consumer to start new session)
        activeSession[msg.sender] = bytes32(0);

        emit SessionEnded(sessionId, msg.sender);
    }

    function withdraw(uint256 amount) external {
        uint256 available = deposits[msg.sender] - locked[msg.sender];
        require(available >= amount, "Insufficient unlocked balance");

        deposits[msg.sender] -= amount;
        USDC.transfer(msg.sender, amount);
    }
}
```

---

## Session Authorization (Decentralized)

### Why Decentralized?

Unlike centralized systems that require a Protocol API to issue tokens, DIN uses a **fully decentralized** approach: consumers sign their own session proofs, and providers verify them by checking on-chain state directly.

| Centralized (Protocol API) | Decentralized (Consumer Self-Sign) |
|---------------------------|-----------------------------------|
| Single point of failure | No central dependency |
| Protocol must be online | Works with just the chain |
| Trust in Protocol service | Trust in smart contract only |
| Protocol can censor | Censorship resistant |

### How It Works

```
DECENTRALIZED SESSION PROOF FLOW
================================

1. CONSUMER CREATES SESSION ON-CHAIN
   Consumer calls startSession()
   -> Contract locks funds
   -> Emits SessionStarted event
   -> Returns sessionId

2. CONSUMER SIGNS SESSION PROOF (off-chain, no gas)
   Consumer's wallet signs: {
     sessionId: "0x123...",
     timestamp: 1702857600
   }
   This signed message IS the "token" - no Protocol API needed.

3. PROVIDER VERIFIES ON-CHAIN (first request only)
   Sidecar receives request + signed proof:
   a) Verify signature -> confirms consumer identity
   b) Query smart contract -> confirms session is active
   c) Cache result -> skip chain query for subsequent requests

4. SUBSEQUENT REQUESTS (cache hit)
   Consumer sends sessionId with requests
   Sidecar checks cache -> processes immediately
```

### Session Proof Structure

```typescript
interface SessionProof {
  // Session reference (matches on-chain)
  sessionId: string;          // e.g. "0x7a3b...f9c2"
  consumer: string;           // e.g. "0x1234...5678"

  // Timestamp for replay protection
  timestamp: number;          // Unix timestamp in seconds

  // Consumer's signature (EIP-191 personal_sign)
  signature: string;          // Signed by consumer's wallet
}

// Message format for signing (EIP-191)
const message = `DIN Session Proof
Session: ${sessionId}
Timestamp: ${timestamp}`;
```

### Session Proof Security Considerations

**Threat Model:**

Session proofs are bearer tokens - anyone with a valid proof can make requests against the session until it expires. If a session proof is leaked (e.g., logged in plaintext, intercepted), an attacker could:
- Make requests using the consumer's locked funds
- Exhaust the session's spending limit

**Mitigations:**

1. **Short Proof Validity Windows:** The `timestamp` in the proof is validated by providers. Proofs older than 5 minutes are rejected, requiring the SDK to generate fresh proofs periodically.

2. **TLS Required:** All communication between SDK and providers must use HTTPS to prevent interception.

3. **Session Spending Limits:** Even if a proof is leaked, damage is bounded by the session's `maxSpend`. Checkpoints every 30 minutes further limit exposure.

4. **Provider Caching:** Providers cache session verification, not the proof itself. The proof is only transmitted on first request or cache miss.

5. **Monitoring:** The SDK can detect unusual spending patterns (rapid requests from unknown IPs) and alert the consumer.

**Best Practice:** Treat session proofs like API keys - don't log them, transmit only over TLS, and use the minimum necessary `maxSpend` for each session.

### Session Duration vs Proof Validity

These are two separate concepts that work together:

| Concept | Duration | What It Controls |
|---------|----------|------------------|
| **Session** | Up to 7 days | How long funds are locked; how long you can make requests |
| **Proof** | 5 minutes | How long a single signed authentication token is accepted |

**How they work together:**

```
SESSION LIFETIME (UP TO 7 DAYS)
===============================

Day 1, 10:00 AM - Consumer starts session (locks funds on-chain)
    |
    |  SDK signs proof: "session ABC, timestamp 10:00"
    |  Proof valid for 5 minutes
    |
Day 1, 10:05 AM - Proof expires
    |  SDK automatically signs NEW proof: "session ABC, timestamp 10:05"
    |  (Consumer doesn't notice - SDK handles this)
    |
    |  ... SDK keeps rotating proofs every ~5 minutes ...
    |
Day 7, 10:00 AM - Session expires on-chain
    |  Even fresh proofs are rejected (session no longer active)
```

**The SDK handles proof refresh automatically:**

```typescript
// SDK internal logic (transparent to consumer)
class DinClient {
  private currentProof: SessionProof;
  private proofValidityMs = 5 * 60 * 1000; // 5 minutes

  async request(service: string, params: any) {
    // Refresh proof if expiring soon (within 30 seconds)
    if (this.proofExpiresSoon()) {
      this.currentProof = this.signFreshProof();
    }

    return this.sendRequest(service, params, this.currentProof);
  }

  private proofExpiresSoon(): boolean {
    const age = Date.now() - this.currentProof.timestamp * 1000;
    return age > (this.proofValidityMs - 30000);
  }
}
```

**Why this design?**

- **Long sessions (7 days):** Convenient for consumers - start once, use for days without interruption
- **Short proofs (5 minutes):** Secure - if a proof leaks, attacker has very limited time to exploit it
- **Automatic refresh:** Best of both worlds - security without user friction

### Request Format

```typescript
// Default request format - sessionId only (provider has session cached)
{
  sessionId: "0x7a3b...f9c2",
  request: {
    method: "eth_blockNumber",
    params: [],
    id: 1
  }
}

// If provider returns 401 + X-DIN-PROOF, retry with full proof
{
  proof: {
    sessionId: "0x7a3b...f9c2",
    consumer: "0x1234...5678",
    timestamp: 1702857600,
    signature: "0x3a4b5c...d8e9f0"
  },
  request: {
    method: "eth_blockNumber",
    params: [],
    id: 1
  }
}
```

### Provider-Driven Proof Flow

```
SDK                              Provider
 |                                  |
 |  Request (sessionId only)        |
 |--------------------------------->|
 |                                  |  Cache miss
 |  401 + X-DIN-PROOF               |
 |<---------------------------------|
 |                                  |
 |  Retry with full proof           |
 |--------------------------------->|
 |                                  |  Verify + cache
 |  200 OK                          |
 |<---------------------------------|
 |                                  |
 |  Next request (sessionId only)   |
 |--------------------------------->|
 |                                  |  Cache hit
 |  200 OK                          |
 |<---------------------------------|
```

### Performance Characteristics

| Request Type | Latency | What Happens |
|--------------|---------|--------------|
| Cache hit | ~0.01ms | Cache lookup only, no crypto |
| Cache miss (first request) | ~1ms + retry | 401 response, SDK retries with proof |
| Retry with proof | ~200-500ms | Signature verify + on-chain lookup |
| After cache expires | ~200-500ms | Re-verify on-chain at checkpoint boundary |

---

## Checkpoints for Long Sessions

Checkpoints allow sessions to run indefinitely without locking large amounts upfront.

```
WITHOUT CHECKPOINTS:
- Lock $500 for 1-week session (ties up funds)

WITH CHECKPOINTS:
- Lock $50, checkpoint every 30 min or when 60% spent


Timeline:
-----------------------------------------------------------------

Session       Checkpoint     Checkpoint     Checkpoint        Session
Start         #1             #2             #3          ...   End
|             |              |              |                  |
v             v              v              v                  v
Lock $50      Settle $40     Settle $45     Settle $30        Final
              (bilateral     (bilateral     (bilateral        settle
              reconciliation) reconciliation) reconciliation)


After each checkpoint:
- Actual usage settled on-chain (batched)
- Unused funds remain locked for next period
- If balance runs low, consumer tops up or session ends
```

### Checkpoint Triggers

| Trigger | Threshold | Use Case |
|---------|-----------|----------|
| Time-based | Every 30 minutes | Predictable settlement, cache refresh |
| Spend-based | When 60% of locked amount used | High-volume apps, multi-provider protection |

Whichever trigger hits first initiates a checkpoint.

---

## Shared Sessions Across SDK Instances

One session can be used across multiple SDK instances **if they share the same wallet private key**:

```
ONE WALLET, MULTIPLE SDK INSTANCES
==================================

                    Consumer Wallet: 0xABC
                              |
                              |  One session, same proof
                              |  $50 locked on-chain
                              |
            +-----------------+-----------------+
            |                 |                 |
            v                 v                 v
    +-------------+   +-------------+   +-------------+
    | SDK Instance|   | SDK Instance|   | SDK Instance|
    | (Laptop)    |   | (Server)    |   | (Lambda)    |
    | same key    |   | same key    |   | same key    |
    +------+------+   +------+------+   +------+------+
           |                 |                 |
           |  Same proof     |  Same proof     |  Same proof
           |                 |                 |
           +-----------------+-----------------+
                             |
                             v
                    Provider Sidecars

    All usage aggregated under one session.
    Total spend across all instances capped at $50 locked amount.
```

### Different Keys = Different Sessions

If you need security isolation (different keys for different environments):

```
Laptop Key: 0xAAA              Server Key: 0xBBB
      |                               |
      |  Session 1                    |  Session 2
      |  $25 locked                   |  $50 locked
      |                               |
      v                               v
+-------------+              +-------------+
| SDK Instance|              | SDK Instance|
| (Laptop)    |              | (Server)    |
+-------------+              +-------------+

Each key has its own session with its own locked funds.
More secure (keys isolated) but requires more locked capital.
```

**Recommendation:** For high-security environments, use separate sessions per deployment. For development or trusted environments, share the wallet key across instances.

---

## Session Configuration Options

When starting a session, consumers can configure:

```typescript
const session = await din.startSession({
  // Payment preferences
  paymentMode: 'cu',            // 'cu' or 'request'
  maxCUsPerRequest: 100,        // Max CUs per request (CU mode)
  maxSpend: 50,                 // Max USDC spend for session

  // Routing preferences
  routingStrategy: 'balanced',  // 'cost', 'health', or 'balanced'
  minHealthThreshold: 0.5,      // Minimum provider health (0-1)

  // Session bounds
  duration: 3600,               // Duration in seconds (1 hour)
});
```

| Option | Description | Default |
|--------|-------------|---------|
| `paymentMode` | How to price requests ('cu' or 'request') | 'cu' |
| `maxCUsPerRequest` | Filter providers by max CUs per request | No limit |
| `maxSpend` | Maximum USDC to lock for session | Required |
| `routingStrategy` | How to select providers | 'balanced' |
| `minHealthThreshold` | Minimum provider health score | 0.5 |
| `duration` | Session duration in seconds | 3600 |

See [02 - Pricing and Routing](./02-pricing-and-routing.md) for details on pricing modes and routing strategies.

---

## Future Considerations

### Multiple Sessions Per Consumer

Currently, each consumer can have only one active session at a time. This simplifies session ID collision prevention and fund accounting. However, future versions may support multiple concurrent sessions for use cases like:

- **Different routing preferences:** Cost-optimized session for batch jobs, health-optimized for production
- **Isolated budgets:** Separate spending limits for different applications using the same wallet
- **Multi-environment:** Dev, staging, and prod sessions with different configurations

**Potential Implementation:**

```solidity
// Instead of single active session:
// mapping(address => bytes32) public activeSession;

// Support multiple sessions per consumer:
mapping(address => bytes32[]) public activeSessions;
uint256 public constant MAX_CONCURRENT_SESSIONS = 5;

function startSession(...) external returns (bytes32 sessionId) {
    require(activeSessions[msg.sender].length < MAX_CONCURRENT_SESSIONS, "Max sessions reached");
    // ... rest of implementation
}
```

This enhancement is planned for a future protocol version based on user demand.
