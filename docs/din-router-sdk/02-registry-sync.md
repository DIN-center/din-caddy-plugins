# Registry Sync

## Overview

The Registry Sync Manager maintains an up-to-date cache of network and provider data using a **two-phase approach**:

### Phase 1: Bootstrap (Free)
- Uses DIN-provided public Linea RPC for initial registry sync
- No payment required - free bootstrap to lower barrier to entry
- Gets initial list of networks and providers, including:
  - **Linea RPC providers** (for ongoing registry syncs)
  - **Watcher providers** (for ongoing score syncs)
  - All other RPC providers

### Phase 2: Ongoing Sync (All x402 USDC on Linea)
- **Registry sync**: Linea providers from the network (x402 USDC payment)
- **Score sync**: Watcher providers from the network (x402 USDC payment)
- Sustainable model - all providers get paid for ongoing access

**Key insight:** The Watcher is registered in the DIN Registry (network: `watchers`), accessed via x402 (USDC on Linea) just like RPC endpoints.

## Sync Strategy

### Timer-Based Polling

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              DinClient                                   │
│                                                                          │
│  ┌──────────────┐    ┌──────────────┐    ┌───────────────────────────┐  │
│  │ Registry     │    │ Score        │    │ Network Cache (Map)       │  │
│  │ Timer (60s)  │    │ Timer (30s)  │    │ - Linea providers         │  │
│  └──────┬───────┘    └──────┬───────┘    │ - Watcher providers       │  │
│         │                   │            │ - All RPC providers       │  │
│         │                   │            └───────────────────────────┘  │
│         ▼                   ▼                         ▲                 │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                        Data Sources                               │  │
│  │  BOOTSTRAP:  Free DIN RPC ───────────────────────────────────────►│  │
│  │  ONGOING:    Linea Provider (x402 USDC) ─────────────────────────►│  │
│  │              Watchers Provider (x402 USDC) ──────────────────────►│  │
│  └──────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────┘
```

### Configuration

| Parameter | Default | Description |
|-----------|---------|-------------|
| `registrySyncIntervalMs` | 60000 (60s) | How often to fetch networks/providers |
| `scoreSyncIntervalMs` | 30000 (30s) | How often to fetch quality scores |

## Implementation

### RegistrySyncManager Class

```typescript
import { createPublicClient, http } from 'viem';
import { linea } from 'viem/chains';

export class RegistrySyncManager {
  private registryCache: Map<string, Network> = new Map();
  private registryTimer: NodeJS.Timeout | null = null;
  private scoreTimer: NodeJS.Timeout | null = null;
  private bootstrapClient: ReturnType<typeof createPublicClient>;
  private x402Client: X402PaymentClient;
  private isInitialized: boolean = false;

  constructor(
    private watcherClient: WatcherClient,
    private x402PaymentClient: X402PaymentClient,
    private config: {
      registrySyncIntervalMs: number;
      scoreSyncIntervalMs: number;
      bootstrapRpcUrl?: string;  // Free bootstrap RPC (init only)
    }
  ) {
    // Bootstrap client - free DIN-provided RPC for initial sync only
    this.bootstrapClient = createPublicClient({
      chain: linea,
      transport: http(config.bootstrapRpcUrl ?? 'https://linea.din.dev/rpc'),
    });
    this.x402Client = x402PaymentClient;
  }

  async start(): Promise<void> {
    // Initial sync (blocking)
    await this.syncRegistry();
    await this.syncScores();

    // Start background timers
    this.registryTimer = setInterval(
      () => this.syncRegistry(),
      this.config.registrySyncIntervalMs
    );

    this.scoreTimer = setInterval(
      () => this.syncScores(),
      this.config.scoreSyncIntervalMs
    );
  }

  stop(): void {
    if (this.registryTimer) clearInterval(this.registryTimer);
    if (this.scoreTimer) clearInterval(this.scoreTimer);
  }

  // ... sync methods
}
```

### Registry Sync Flow

```typescript
private async syncRegistry(): Promise<void> {
  let registryData: Network[];

  if (!this.isInitialized) {
    // PHASE 1: Bootstrap - use free DIN-provided RPC
    registryData = await this.bootstrapSync();
    this.isInitialized = true;
  } else {
    // PHASE 2: Ongoing - use x402 payment to Linea providers
    registryData = await this.x402Sync();
  }

  // Process and cache the data
  this.processRegistryData(registryData);
}

// Phase 1: Free bootstrap sync
private async bootstrapSync(): Promise<Network[]> {
  // Use free DIN-provided Linea RPC
  return await this.bootstrapClient.readContract({
    address: DIN_REGISTRY_ADDRESS,
    abi: dinRegistryAbi,
    functionName: 'getNetworks',
  });
}

// Phase 2: Paid ongoing sync via x402
private async x402Sync(): Promise<Network[]> {
  // Select a Linea provider from the cache (discovered during bootstrap)
  const lineaNetwork = this.registryCache.get('linea-mainnet');
  const lineaProvider = this.selectProvider(lineaNetwork.providers);

  // Make x402 payment request to Linea provider
  const response = await this.x402Client.makeRequest(lineaProvider.serviceUrl, {
    method: 'POST',
    body: JSON.stringify({
      jsonrpc: '2.0',
      method: 'eth_call',
      params: [{ to: DIN_REGISTRY_ADDRESS, data: encodeGetNetworks() }],
      id: 1,
    }),
  });

  return decodeNetworks(response.data.result);
}
```

### Score Sync Flow

```typescript
private async syncScores(): Promise<void> {
  // 1. Select a watcher provider from the registry (discovered during bootstrap)
  const watcherNetwork = this.registryCache.get('watchers');
  const watcherProvider = this.selectProvider(watcherNetwork.providers);

  // 2. For each cached network, fetch scores via x402 payment (USDC on Linea)
  for (const [networkName, network] of this.registryCache) {
    // Make x402 USDC payment to watcher provider
    const response = await this.x402Client.makeRequest(
      `${watcherProvider.serviceUrl}/scores/${networkName}`
    );

    // 3. Update provider scores
    for (const [providerId, score] of response.data) {
      const provider = network.providers.get(providerId);
      if (provider) {
        provider.score = score.value;
      }
    }
  }
}
```

## Data Sources

### DIN Registry (Linea Smart Contract)

The SDK reads from the DIN Registry smart contract on Linea using a **two-phase approach**:

#### Phase 1: Bootstrap (Free)
```typescript
// DIN-provided free Linea RPC - for initial sync only
const bootstrapClient = createPublicClient({
  chain: linea,
  transport: http('https://linea.din.dev/rpc'),
});

// Initial registry read (free)
const networks = await bootstrapClient.readContract({
  address: DIN_REGISTRY_ADDRESS,
  abi: dinRegistryAbi,
  functionName: 'getNetworks',
});
// Now we have Linea providers in cache!
```

#### Phase 2: Ongoing (x402 Payment)
```typescript
// After bootstrap, use Linea providers from the network
const lineaProvider = selectProvider(cache.get('linea-mainnet').providers);

// Make x402 payment to Linea provider for registry reads
const response = await x402Client.makeRequest(lineaProvider.serviceUrl, {
  method: 'POST',
  body: { jsonrpc: '2.0', method: 'eth_call', params: [...] },
});
```

Contract interactions are handled internally:
- Contract ABIs bundled in SDK
- Multi-contract traversal (Registry → Network → Provider → Service)
- Data parsing and type conversion

### Watcher (Registry Service via x402 USDC)

The Watcher service is registered in the DIN Registry (network: `watchers`), accessed via x402 payments (USDC on Linea) just like RPC endpoints:

```typescript
interface Score {
  value: number;           // 0.0 to 1.0
  blockConsistency: number;
  stateConsistency: number;
  latency: number;
  updatedAt: Date;
}
```

```typescript
// Watcher is a service in the registry - accessed via x402 USDC
const watcherNetwork = cache.get('watchers');
const watcherProvider = selectProvider(watcherNetwork.providers);

// Make x402 USDC payment to watcher provider
const response = await x402Client.makeRequest(
  `${watcherProvider.serviceUrl}/scores/${networkName}`
);
```

**Why x402 for Watcher?**
- Watcher providers run infrastructure and deserve compensation
- Same payment model as RPC providers - consistent architecture
- Multiple watcher providers can compete on quality/price

## Cache Structure

```typescript
// In-memory cache
private registryCache: Map<string, Network> = new Map();

// Network structure (after sync)
{
  name: "ethereum-mainnet",
  address: "0x...",
  status: "Active",
  config: { ... },
  providers: Map {
    "0xProviderAddr1" => {
      name: "Provider A",
      address: "0xProviderAddr1",
      paymentAddress: "0xPaymentAddr1",
      status: "Active",
      services: [...],
      score: 0.85  // Updated by score sync
    },
    "0xProviderAddr2" => { ... }
  }
}
```

## Lifecycle

### Startup Sequence

```
1. new DinClient(config)     → Constructor, no sync yet
2. din.start()               → Or auto on first request
   │
   │  PHASE 1: BOOTSTRAP (FREE)
   ├── await bootstrapSync()   → Use free DIN Linea RPC
   │   └── Cache now includes: Linea providers, Watcher providers, all RPC providers
   │
   │  PHASE 2: ONGOING (ALL x402 USDC on Linea)
   ├── startRegistryTimer()    → Background timer (Linea providers + x402 USDC)
   └── startScoreTimer()       → Background timer (Watcher providers + x402 USDC)
3. din.request(...)          → Uses cached data, x402 USDC payment to RPC provider
```

### Shutdown Sequence

```
1. din.stop()
   ├── clearInterval(registryTimer)
   └── clearInterval(scoreTimer)
```

## Error Handling

### Sync Failures

- Individual sync failures should not crash the client
- Log errors and continue using cached data
- Retry on next timer tick

```typescript
private async syncRegistry(): Promise<void> {
  try {
    const networks = await this.registryClient.getNetworks();
    // ... update cache
  } catch (error) {
    console.error('Registry sync failed:', error);
    // Continue using existing cache
  }
}
```

### Stale Data

If sync consistently fails, cached data becomes stale. The score computation includes staleness decay:

```typescript
function applyStaleDecay(
  score: number,
  elapsedMs: number,
  gracePeriodMs: number = 3600000,  // 60 min grace
  decayTauMs: number = 3600000,
  midpoint: number = 0.5
): number {
  if (elapsedMs < gracePeriodMs) return score;

  const t = (elapsedMs - gracePeriodMs) / 60000;
  const tc = decayTauMs / 60000;
  return midpoint + (score - midpoint) * Math.exp(-t / tc);
}
```

## Why Timer-Based?

The Go router uses epoch-based sync (checking block numbers). For the TypeScript SDK, we chose timer-based polling because:

1. **Simpler implementation** - No block number tracking needed
2. **Predictable behavior** - Fixed intervals are easier to reason about
3. **Sufficient for SDK** - SDK doesn't need block-level precision
4. **Configurable** - Users can adjust intervals based on needs
