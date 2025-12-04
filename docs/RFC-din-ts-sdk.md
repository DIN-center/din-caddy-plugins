# RFC: DIN TypeScript SDK

**Status:** Draft
**Authors:** DIN Team
**Date:** December 2024

---

## Summary

A TypeScript SDK (`@din-center/router`) that provides developers a simple interface to access DIN network providers with automatic registry sync, quality-based provider selection, and x402 micropayments.

---

## Motivation

Currently, integrating with the DIN network requires:
- Direct smart contract interactions for registry data
- Manual watcher API calls for provider quality scores
- Custom x402 payment handling

This SDK consolidates these into a single package with a minimal API surface.

---

## Design Principles

1. **Permissionless bootstrap** - Free DIN Linea RPC for initial registry sync
2. **x402 payments on Linea** - All ongoing payments use USDC on Linea blockchain
3. **Private key for payments** - Wallet needed for x402 payments to providers
4. **Network-agnostic** - Works with any blockchain (EVM, Solana, Bitcoin, etc.)

---

## Proposed API

```typescript
import { DinClient } from '@din-center/router';

const din = new DinClient({
  privateKey: process.env.PRIVATE_KEY,  // For x402 payments to providers
});

const response = await din.request('ethereum-mainnet', {
  body: { jsonrpc: '2.0', method: 'eth_blockNumber', params: [], id: 1 },
});

console.log(response.data.result);      // Block number
console.log(response.provider.name);    // Selected provider
console.log(response.paymentInfo);      // { amount, txHash }
```

---

## Architecture

```
User Application
       │
       ▼
┌──────────────────────────────────────────────────────────────┐
│                    @din-center/router                        │
│                                                              │
│  ┌────────────────┐  ┌────────────────┐  ┌────────────────┐  │
│  │ Registry Sync  │  │ Watcher Client │  │ x402 Payment   │  │
│  │ (two-phase)    │  │ (x402)         │  │ (USDC on Linea)│  │
│  └───────┬────────┘  └───────┬────────┘  └───────┬────────┘  │
│          │                   │                   │           │
│          ▼                   ▼                   ▼           │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │              Provider Selector                          │ │
│  │         (weighted random + session affinity)            │ │
│  └─────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────┘
       │                   │                   │
       ▼                   ▼                   ▼
┌──────────────────────────────────────────────────────────────┐
│                     Data Sources                              │
├──────────────────────────────────────────────────────────────┤
│  BOOTSTRAP (FREE):          │  ONGOING (x402 USDC on Linea): │
│  ┌────────────────────┐     │  ┌────────────────────────┐    │
│  │ DIN Public Linea   │     │  │ Linea Providers        │    │
│  │ RPC (one-time)     │     │  │ → Registry sync        │    │
│  └────────────────────┘     │  └────────────────────────┘    │
│         │                   │  ┌────────────────────────┐    │
│         ▼                   │  │ Watcher providers     │    │
│  Gets: Linea providers,     │  │ → Score sync           │    │
│        Watcher providers,  │  └────────────────────────┘    │
│        All RPC providers    │  ┌────────────────────────┐    │
│                             │  │ RPC Providers          │    │
│                             │  │ → User requests        │    │
│                             │  └────────────────────────┘    │
└──────────────────────────────────────────────────────────────┘
```

---

## Two-Phase Access Model

### Phase 1: Bootstrap (Free)

On initialization, the SDK uses a **DIN-provided public Linea RPC** to fetch the initial registry snapshot:

- **Source**: DIN Registry smart contract on Linea mainnet
- **Access**: Free bootstrap RPC (`https://linea.din.dev/rpc`)
- **Purpose**: Get initial list of networks, providers, including:
  - Linea RPC providers (for ongoing registry syncs)
  - **Watcher providers** (for score data)
  - All other RPC providers

```typescript
// On init, SDK uses free bootstrap RPC to get registry data
const din = new DinClient({ privateKey: '...' });
await din.start();  // Uses free DIN Linea RPC for initial sync
// Cache now includes: Linea providers, Watcher providers, RPC providers
```

### Phase 2: Ongoing Operations (All x402 USDC on Linea)

After initialization, **all data access uses x402 payments (USDC on Linea)** to providers discovered during bootstrap:

| Operation | Provider Source | Payment |
|-----------|-----------------|---------|
| Registry sync | Linea providers | x402 (USDC) |
| Score fetch | **Watcher providers** | x402 (USDC) |
| RPC requests | Network providers | x402 (USDC) |

```typescript
// After init, everything uses x402 payments (USDC on Linea)
// Registry syncs → Linea providers (x402)
// Score syncs → Watcher providers (x402)
// User requests → RPC providers (x402)
```

### Watcher as a Registry Service

The Watcher is registered in the DIN Registry as a network, just like RPC endpoints:

- **Network name**: `watchers`
- **Providers**: Multiple watcher instances registered as providers
- **Access**: REST API calls via x402 payment (USDC on Linea)
- **Data**: Provider quality scores (block consistency, state consistency, latency)

```typescript
// SDK treats watcher like any other network
const watcherNetwork = cache.get('watchers');
const watcherProvider = selectProvider(watcherNetwork.providers);
const scores = await x402Client.makeRequest(watcherProvider.serviceUrl + '/scores/ethereum-mainnet');
```

---

## Core Components

### 1. Registry Sync Manager

Maintains cached network/provider data from DIN smart contracts on Linea using a **two-phase approach**:

| Config | Default | Description |
|--------|---------|-------------|
| `registrySyncIntervalMs` | 60000 (60s) | Network/provider refresh interval |
| `bootstrapRpcUrl` | `https://linea.din.dev/rpc` | Free bootstrap RPC (init only) |

**Data Flow:**
```
INIT:     Bootstrap RPC (free) ──► Read Registry ──► Cache (includes Linea providers)
                                                            │
ONGOING:  Linea Provider (x402) ◄───────────────────────────┘
               │
               └── Timer (60s) ──► Read Registry ──► Update Cache
```

### 2. Watcher Client

Fetches provider quality metrics from **Watcher providers registered in the DIN Registry** via x402 payments (USDC on Linea).

| Config | Default | Description |
|--------|---------|-------------|
| `scoreSyncIntervalMs` | 30000 (30s) | Score refresh interval |
| `watcherNetworkName` | `watchers` | Network name for watcher in registry |

**Access Pattern:**
```typescript
// Watcher is a service in the registry, accessed like any RPC
const watcherNetwork = cache.get('watchers');
const watcherProvider = selectProvider(watcherNetwork.providers);
const scores = await x402Client.request(watcherProvider.serviceUrl + '/scores/' + networkName);
```

**Score Formula:**
```typescript
score = (blockConsistency * 0.5) + (stateConsistency * 0.3) + (latency * 0.2)
```

### 3. Provider Selector

**Selection Algorithm:**
1. Filter active providers with `score > 0`
2. If `sessionId` provided → deterministic hash-based selection
3. Otherwise → weighted random selection by score

### 4. x402 Payment Client

Handles micropayments (USDC on Linea) for all provider requests:

```
Request ──► 402 Payment Required ──► Sign USDC payment ──► Retry with X-PAYMENT ──► 200 OK
```

**Payment Details:**
- **Token**: USDC
- **Chain**: Linea mainnet
- **Signing**: viem wallet

---

## Type Definitions

```typescript
interface DinConfig {
  // Required for all x402 payments (USDC on Linea)
  privateKey: string;

  // Optional overrides (defaults provided)
  bootstrapRpcUrl?: string;          // Default: 'https://linea.din.dev/rpc' (free, init only)
  watcherNetworkName?: string;       // Default: 'watchers' (network name in registry)
  registrySyncIntervalMs?: number;   // Default: 60000
  scoreSyncIntervalMs?: number;      // Default: 30000
  sessionId?: string;                // Optional sticky routing
}

interface DinResponse<T> {
  data: T;
  status: number;
  headers: Record<string, string>;
  provider: { name: string; url: string };
  paymentInfo?: { amount: string; txHash?: string };
}
```

---

## Dependencies

```json
{
  "viem": "^2.0.0",
  "x402-axios": "^0.1.0",
  "axios": "^1.6.0"
}
```

---

## File Structure

```
din-router-sdk/
├── src/
│   ├── index.ts              # Public exports
│   ├── client.ts             # DinClient
│   ├── types.ts
│   ├── registry/
│   │   ├── sync.ts           # RegistrySyncManager
│   │   └── linea-client.ts   # Linea RPC interactions
│   ├── watcher/
│   │   ├── client.ts         # WatcherClient (public API)
│   │   └── score-manager.ts  # Score computation
│   ├── selector/
│   │   ├── weighted-selector.ts
│   │   └── session-affinity.ts
│   └── payment/
│       └── x402-client.ts    # X402PaymentClient
└── tests/
```

---

## Future Considerations

1. **Watcher x402** - May add x402 payment option for watcher API access
2. **Caching** - Persistent cache (localStorage/file) for faster startup
3. **Fallback providers** - Automatic failover on provider errors

---

## Next Steps

1. Implement Linea registry client (contract reads via DIN RPC)
2. Implement public watcher client (no API key)
3. Implement x402 payment client
4. Integration testing with testnet
