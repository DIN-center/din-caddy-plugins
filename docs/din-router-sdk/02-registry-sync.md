# Registry Sync

## Overview

The Registry Sync Manager maintains an up-to-date cache of network and provider data. It uses timer-based polling to periodically refresh data from two sources:

1. **DIN Registry** (via `@din-center/registry`) - Network and provider information
2. **Watcher API** - Provider quality scores

## Sync Strategy

### Timer-Based Polling

```
┌─────────────────────────────────────────────────────────────┐
│                      DinClient                               │
│                                                              │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐   │
│  │ Registry     │    │ Score        │    │ Network      │   │
│  │ Timer (60s)  │    │ Timer (30s)  │    │ Cache (Map)  │   │
│  └──────┬───────┘    └──────┬───────┘    └──────────────┘   │
│         │                   │                    ▲           │
│         ▼                   ▼                    │           │
│  ┌──────────────┐    ┌──────────────┐           │           │
│  │ Registry SDK │    │ Watcher API  │───────────┘           │
│  │ getNetworks()│    │ getScores()  │  (updates scores)     │
│  └──────────────┘    └──────────────┘                       │
└─────────────────────────────────────────────────────────────┘
```

### Configuration

| Parameter | Default | Description |
|-----------|---------|-------------|
| `registrySyncIntervalMs` | 60000 (60s) | How often to fetch networks/providers |
| `scoreSyncIntervalMs` | 30000 (30s) | How often to fetch quality scores |

## Implementation

### RegistrySyncManager Class

```typescript
import { DinRegistryClient } from '@din-center/registry';

export class RegistrySyncManager {
  private registryCache: Map<string, Network> = new Map();
  private registryTimer: NodeJS.Timeout | null = null;
  private scoreTimer: NodeJS.Timeout | null = null;

  constructor(
    private registryClient: DinRegistryClient,
    private watcherClient: WatcherClient,
    private config: {
      registrySyncIntervalMs: number;
      scoreSyncIntervalMs: number;
    }
  ) {}

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
  // 1. Fetch all networks from registry SDK
  const networks = await this.registryClient.getNetworks();

  // 2. Process each network
  for (const network of networks) {
    if (network.status === 'Active') {
      // 3. Fetch providers for active networks
      const providers = await this.registryClient.getProviders(network.name);

      // 4. Attach providers to network
      network.providers = new Map(providers.map(p => [p.address, p]));

      // 5. Update cache
      this.registryCache.set(network.name, network);
    } else {
      // 6. Remove inactive networks
      this.registryCache.delete(network.name);
    }
  }
}
```

### Score Sync Flow

```typescript
private async syncScores(): Promise<void> {
  // For each cached network
  for (const [networkName, network] of this.registryCache) {
    // 1. Fetch scores from watcher
    const scores = await this.watcherClient.getScores(networkName);

    // 2. Update provider scores
    for (const [providerId, score] of scores) {
      const provider = network.providers.get(providerId);
      if (provider) {
        provider.score = score.value;
      }
    }
  }
}
```

## Data Sources

### @din-center/registry SDK

The router SDK expects the registry SDK to provide:

```typescript
interface DinRegistryClient {
  getNetworks(): Promise<Network[]>;
  getProviders(networkName: string): Promise<Provider[]>;
  getNetwork(name: string): Promise<Network | null>;
  getProvider(address: string): Promise<Provider | null>;
}
```

The registry SDK handles all smart contract complexity:
- Contract ABIs
- Multi-contract traversal (Registry → Network → Provider → Service)
- Data parsing and type conversion

### Watcher API

The Watcher API provides quality metrics:

```typescript
interface WatcherClient {
  getScores(networkName: string): Promise<Map<string, Score>>;
}

interface Score {
  value: number;           // 0.0 to 1.0
  blockConsistency: number;
  stateConsistency: number;
  latency: number;
  updatedAt: Date;
}
```

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
   ├── await syncRegistry()  → Blocking initial sync
   ├── await syncScores()    → Blocking initial sync
   ├── startRegistryTimer()  → Background timer
   └── startScoreTimer()     → Background timer
3. din.request(...)          → Uses cached data
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
