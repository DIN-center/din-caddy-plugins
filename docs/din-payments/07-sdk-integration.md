# SDK Integration

## Overview

This document covers the Consumer SDK and Provider SDK/Sidecar implementations for integrating with the DIN Protocol.

## Consumer SDK

### Installation

```bash
npm install @din-center/sdk
```

### Basic Usage

```typescript
import { DinClient } from '@din-center/sdk';

// Initialize
const din = new DinClient({
  privateKey: process.env.PRIVATE_KEY,
  persistencePath: '~/.din/session-log.json',  // JSON file for crash recovery
});

// Deposit funds (one-time setup)
await din.deposit({ amount: 100 }); // $100 USDC

// Start session
const session = await din.startSession({
  paymentMode: 'cu',           // 'cu' or 'request'
  maxCUsPerRequest: 100,       // Max CUs per request (CU mode only)
  maxSpend: 10,                // Max USDC for session
  routingStrategy: 'balanced', // 'cost', 'health', or 'balanced'
  minHealthThreshold: 0.5,     // Minimum provider health
  duration: 3600,              // 1 hour
});

// Make requests
const result = await din.request('ethereum-mainnet', {
  method: 'eth_call',
  params: [{ to: '0x...', data: '0x...' }, 'latest'],
});

// End session
await din.endSession();
```

### Configuration Options

```typescript
interface DinClientConfig {
  // Required
  privateKey: string;              // Wallet private key for signing

  // Optional
  persistencePath?: string;        // Session log file path (default: ~/.din/session-log.json)
  rpcUrl?: string;                 // Base RPC URL (default: public Base RPC)
  contractAddress?: string;        // DINProtocol contract (default: mainnet address)
}
```

### Session Configuration

```typescript
interface SessionConfig {
  // Payment preferences
  paymentMode: 'cu' | 'request';   // How to price requests
  maxCUsPerRequest?: number;       // Max CUs per request (CU mode)
  maxSpend: number;                // Max USDC for session

  // Routing preferences
  routingStrategy?: 'cost' | 'health' | 'balanced';  // Default: 'balanced'
  minHealthThreshold?: number;     // Min provider health (0-1, default: 0.5)

  // Session bounds
  duration?: number;               // Duration in seconds (default: 3600)
}
```

### Payment Modes

#### CU-Based Payment

```typescript
const session = await din.startSession({
  paymentMode: 'cu',
  maxCUsPerRequest: 100,   // Filter providers charging > 100 CUs
  maxSpend: 10,
  duration: 3600,
});

// Usage tracked in CUs
const usage = await din.getSessionUsage();
// {
//   paymentMode: "cu",
//   usedCUs: 5000,
//   usedSpend: 0.40,        // 5000 CUs × $0.00008/CU
//   remainingCUs: 95000,
//   remainingSpend: 9.60,
// }
```

#### Per-Request Payment

```typescript
const session = await din.startSession({
  paymentMode: 'request',
  maxSpend: 10,
  duration: 3600,
});

// Usage tracked by direct cost
const usage = await din.getSessionUsage();
// {
//   paymentMode: "request",
//   usedSpend: 0.001,        // Provider's direct price
//   remainingSpend: 9.999,
// }
```

### Routing Strategies

| Strategy | Description | Best For |
|----------|-------------|----------|
| `cost` | Always cheapest provider | Batch jobs, price-sensitive apps |
| `health` | Weighted random by health | Mission-critical, production |
| `balanced` | Weighted by value score | Most users (default) |

```typescript
// Cost-optimized for batch processing
const session = await din.startSession({
  paymentMode: 'cu',
  routingStrategy: 'cost',
  maxSpend: 100,
});

// Health-optimized for production
const session = await din.startSession({
  paymentMode: 'cu',
  routingStrategy: 'health',
  minHealthThreshold: 0.8,  // High quality floor
  maxSpend: 50,
});
```

### Account Management

```typescript
// Check balance
const balance = await din.getBalance();
// {
//   total: 100.00,
//   locked: 10.00,      // In active sessions
//   available: 90.00,   // Can withdraw or use for new sessions
//   totalSpent: 50.00,  // Lifetime spend
// }

// Deposit more funds
await din.deposit({ amount: 50 });

// Withdraw available funds
await din.withdraw({ amount: 25 });
```

### Session Management

```typescript
// Get current session usage
const usage = await din.getSessionUsage();
// {
//   sessionId: "abc123",
//   paymentMode: "cu",
//   usedCUs: 5000,
//   usedSpend: 0.40,
//   remainingCUs: 95000,
//   remainingSpend: 9.60,
//   byProvider: {
//     "0xProviderA": { cus: 3000, cost: 0.24 },
//     "0xProviderB": { cus: 2000, cost: 0.16 }
//   }
// }

// End session early
await din.endSession();

// Resume from crashed session
const din = new DinClient({
  privateKey: process.env.PRIVATE_KEY,
  persistencePath: '~/.din/session-log.json',
});
// SDK automatically loads and resumes active session from file
```

### Error Handling

```typescript
try {
  const result = await din.request('ethereum-mainnet', {
    method: 'eth_call',
    params: [...],
  });
} catch (error) {
  if (error.code === 'NO_PROVIDERS') {
    // No providers available for service + payment mode
    console.log(error.suggestion); // "Switch to CU payment mode"
  } else if (error.code === 'INSUFFICIENT_FUNDS') {
    // Session spending limit exceeded
    await din.endSession();
    await din.startSession({ maxSpend: 50 }); // Start new session
  } else if (error.code === 'SESSION_EXPIRED') {
    // Session timed out
    await din.startSession({ duration: 3600 });
  }
}
```

---

## Provider SDK

### Installation

```bash
npm install @din-center/provider-sdk
```

### Basic Usage

```typescript
import { DinProvider } from '@din-center/provider-sdk';

// Initialize
const provider = new DinProvider({
  privateKey: process.env.PROVIDER_KEY,
  rpcEndpoint: 'http://localhost:8545',  // Your node
});

// Register rate card
await provider.registerRateCard({
  services: {
    'ethereum-mainnet': {
      serviceType: 'evm',
      supportsCUPricing: true,
      supportsRequestPricing: true,
      defaultCUCost: 5,
      methodCUs: {
        'eth_call': 10,
        'eth_getLogs': 50,
      },
      defaultPricePerRequest: 0.001,
      methodPrices: {
        'eth_call': 0.0008,
        'eth_getLogs': 0.004,
      },
    },
  },
});
```

### Rate Card Configuration

```typescript
interface RateCardConfig {
  services: {
    [serviceName: string]: ServiceConfig;
  };
}

interface ServiceConfig {
  serviceType: string;              // 'evm', 'solana', 'beacon-chain', etc.

  // Payment mode flags (at least one must be true)
  supportsCUPricing: boolean;       // Accept CU-based payments
  supportsRequestPricing: boolean;  // Accept per-request payments

  // CU-based pricing
  defaultCUCost?: number;           // Default CUs for unlisted methods
  methodCUs?: Record<string, number>;  // CU cost per method

  // Per-request pricing
  defaultPricePerRequest?: number;  // Default price (USDC)
  methodPrices?: Record<string, number>;  // Price per method (USDC)
}
```

### Payment Mode Examples

#### CU-Only Provider

```typescript
// Efficient infrastructure, compete on CU cost
await provider.registerRateCard({
  services: {
    'ethereum-mainnet': {
      serviceType: 'evm',
      supportsCUPricing: true,
      supportsRequestPricing: false,  // CU only
      defaultCUCost: 1,
      methodCUs: {
        'eth_call': 5,
        'eth_getLogs': 25,
        'debug_traceCall': 250,
      },
    },
  },
});
```

#### Per-Request Only Provider

```typescript
// Simple flat pricing for REST APIs
await provider.registerRateCard({
  services: {
    'bitcoin-esplora': {
      serviceType: 'bitcoin-esplora',
      supportsCUPricing: false,
      supportsRequestPricing: true,  // Per-request only
      defaultPricePerRequest: 0.0001,
      methodPrices: {
        '/address/{address}': 0.0001,
        '/address/{address}/txs': 0.0002,
        '/tx/{txid}': 0.0001,
      },
    },
  },
});
```

#### Both Modes Provider

```typescript
// Maximum consumer pool
await provider.registerRateCard({
  services: {
    'ethereum-mainnet': {
      serviceType: 'evm',
      supportsCUPricing: true,
      supportsRequestPricing: true,
      defaultCUCost: 5,
      methodCUs: { 'eth_call': 10, 'eth_getLogs': 50 },
      defaultPricePerRequest: 0.001,
      methodPrices: { 'eth_call': 0.0008, 'eth_getLogs': 0.004 },
    },
  },
});
```

### Provider Stats

```typescript
// Get earnings stats (USDC paid directly to wallet)
const stats = await provider.getStats({ period: '7d' });
// {
//   pending: 45.50,         // Current period, not yet settled
//   totalEarned: 1530.30,   // Lifetime earnings
//   requestsServed: 2_500_000,
//   cusServed: 5_200_000,
//   uniqueConsumers: 342,
//   byPaymentMode: {
//     cu: { revenue: 180.00, requests: 2_000_000 },
//     request: { revenue: 54.80, requests: 500_000 }
//   }
// }
```

---

## Provider Sidecar

The Provider Sidecar is a Go service that runs alongside provider nodes to handle session verification and usage tracking.

### Architecture

```
PROVIDER SIDECAR
================

Consumer Request
      |
      v
+------------------+
|  Provider Node   |
|  (RPC endpoint)  |
+--------+---------+
         |
         v
+------------------+
|  DIN Sidecar     |
|                  |
|  - Session cache |
|  - Usage tracking|
|  - Proof verify  |
+--------+---------+
         |
         v
  Response to Consumer
```

### Session Verification Flow

```
SESSION VERIFICATION
====================

1. Request arrives with sessionId

2. Check cache:
   - Cache hit? Serve request immediately
   - Cache miss? Continue...

3. If cache miss, check if proof included:
   - No proof? Return 401 + X-DIN-PROOF header
   - Has proof? Continue...

4. Verify proof:
   - Verify signature (recovers consumer address)
   - Query contract: isSessionActive(sessionId)?
   - Valid? Cache session, serve request
   - Invalid? Return 401 Unauthorized

5. Subsequent requests:
   - Cache hit (no crypto, ~0.01ms)
   - Until checkpoint expires cache
```

### Sidecar Configuration

```yaml
# sidecar.yaml
din:
  contract_address: "0x..."
  rpc_url: "https://mainnet.base.org"

provider:
  private_key: "${PROVIDER_KEY}"

cache:
  session_ttl: 1800  # 30 minutes

usage:
  checkpoint_interval: 1800  # 30 minutes
  spend_threshold: 0.6       # 60% spend triggers checkpoint
```

### Usage Tracking

The sidecar tracks usage per session:

```go
type SessionUsage struct {
    SessionID  string
    Consumer   string
    Methods    map[string]MethodUsage
    TotalCUs   uint64
    TotalCost  uint64  // In USDC base units
    LastUpdate time.Time
}

type MethodUsage struct {
    Requests uint64
    CUs      uint64
    Cost     uint64
}
```

### Checkpoint Submission

At checkpoint intervals, the sidecar reports usage:

```go
// Checkpoint triggered by time or spend threshold
func (s *Sidecar) SubmitCheckpoint(sessionID string) error {
    usage := s.getUsage(sessionID)

    // Sign usage claim
    claim := UsageClaim{
        SessionID: sessionID,
        Provider:  s.address,
        Amount:    usage.TotalCost,
        Timestamp: time.Now().Unix(),
    }
    signature := s.sign(claim)

    // Submit to coordinator
    return s.coordinator.SubmitClaim(claim, signature)
}
```

---

## Integration Examples

### Web Application

```typescript
// Frontend integration
import { DinClient } from '@din-center/sdk';

const din = new DinClient({
  privateKey: userWallet.privateKey,
});

// One-time deposit (show in UI)
if ((await din.getBalance()).available < 10) {
  await din.deposit({ amount: 50 });
}

// Start session for user's browsing session
const session = await din.startSession({
  paymentMode: 'cu',
  maxSpend: 5,
  duration: 3600,
});

// Make requests as user interacts
async function fetchTokenBalance(address: string) {
  return din.request('ethereum-mainnet', {
    method: 'eth_call',
    params: [{ to: tokenContract, data: balanceOfData(address) }, 'latest'],
  });
}
```

### Backend Service

```typescript
// Backend with long-running session
const din = new DinClient({
  privateKey: process.env.SERVICE_KEY,
  persistencePath: '/var/lib/din/session-log.json',
});

// Ensure deposit
const balance = await din.getBalance();
if (balance.available < 100) {
  await din.deposit({ amount: 500 });
}

// Long session for service
const session = await din.startSession({
  paymentMode: 'cu',
  maxSpend: 100,
  routingStrategy: 'health',     // Prioritize reliability
  minHealthThreshold: 0.8,
  duration: 86400,               // 24 hours
});

// Handle requests
app.get('/api/balance/:address', async (req, res) => {
  const balance = await din.request('ethereum-mainnet', {
    method: 'eth_getBalance',
    params: [req.params.address, 'latest'],
  });
  res.json({ balance });
});
```

### Serverless Function

```typescript
// Lambda/Cloud Function
import { DinClient } from '@din-center/sdk';

// Shared wallet across function invocations
const din = new DinClient({
  privateKey: process.env.DIN_KEY,
  // Note: Persistence may not work in serverless
  // Use external state store if needed
});

export async function handler(event: any) {
  // Short session per invocation
  const session = await din.startSession({
    paymentMode: 'cu',
    maxSpend: 0.10,  // Small budget per invocation
    duration: 60,    // 1 minute
  });

  try {
    const result = await din.request('ethereum-mainnet', {
      method: event.method,
      params: event.params,
    });
    return { statusCode: 200, body: JSON.stringify(result) };
  } finally {
    await din.endSession();
  }
}
```

---

## Best Practices

### For Consumers

1. **Reuse sessions** - Don't create new sessions for every request
2. **Set appropriate limits** - `maxSpend` should cover expected usage
3. **Handle errors gracefully** - Implement retry logic with exponential backoff
4. **Monitor usage** - Check `getSessionUsage()` periodically
5. **Choose the right strategy** - Use `cost` for batch, `health` for production

### For Providers

1. **Support both modes** - Maximize consumer pool
2. **Set competitive CU costs** - Compete on efficiency
3. **Monitor health scores** - High health = more traffic
4. **Respond to escalations** - No-response = automatic penalty
5. **Keep accurate logs** - Essential for dispute resolution

