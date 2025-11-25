# Architecture Overview

## System Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              User Application                                │
│                                                                              │
│   din.request('ethereum-mainnet', { body: { jsonrpc: '2.0', ... } })        │
└─────────────────────────────────┬───────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           @din-center/router                                 │
│                                                                              │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐              │
│  │  DinClient      │  │  Registry Sync  │  │  X402 Payment   │              │
│  │  (Orchestrator) │  │  Manager        │  │  Client         │              │
│  └────────┬────────┘  └────────┬────────┘  └────────┬────────┘              │
│           │                    │                    │                        │
│           │           ┌────────┴────────┐          │                        │
│           │           │                 │          │                        │
│           ▼           ▼                 ▼          ▼                        │
│  ┌─────────────────┐  ┌─────────────┐  ┌─────────────────┐                  │
│  │  Provider       │  │  Network    │  │  Watcher        │                  │
│  │  Selector       │  │  Cache      │  │  Client         │                  │
│  └─────────────────┘  └─────────────┘  └─────────────────┘                  │
└─────────────────────────────────────────────────────────────────────────────┘
          │                    │                    │
          │                    ▼                    ▼
          │   ┌────────────────────────────────────────────────┐
          │   │           @din-center/registry                  │
          │   │   (Handles smart contract interactions)         │
          │   └────────────────────────────────────────────────┘
          │                    │
          │                    ▼
          │   ┌────────────────────────────────────────────────┐
          │   │           DIN Smart Contracts                   │
          │   │   DinRegistryHandler → Network → Provider       │
          │   └────────────────────────────────────────────────┘
          │
          ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         Selected RPC Provider                                │
│                    (e.g., Infura, Alchemy, QuickNode)                       │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Component Overview

### DinClient (Main Entry Point)

The orchestrator that coordinates all SDK functionality:

- Manages client lifecycle (start/stop)
- Routes requests to the correct network
- Coordinates provider selection
- Handles request execution with payments

### Registry Sync Manager

Maintains an up-to-date cache of network and provider data:

- Polls registry every 60s (configurable)
- Polls watcher for scores every 30s (configurable)
- Caches data in memory for fast access
- Handles network/provider status changes

### Provider Selector

Chooses the best provider for each request:

- Filters by active status and positive scores
- Supports session affinity (sticky routing)
- Uses weighted random selection based on scores

### X402 Payment Client

Handles all payment logic:

- Wraps axios with x402 interceptor
- Signs payments using viem
- Extracts payment info from responses

### Watcher Client

Fetches quality metrics for providers:

- Block consistency scores
- State consistency scores
- Latency metrics

## Data Flow

### Request Flow

```
1. User calls din.request('ethereum-mainnet', options)
                    │
                    ▼
2. DinClient ensures client is started (auto-start)
                    │
                    ▼
3. Get network from cache
                    │
                    ▼
4. Get providers for network (with scores)
                    │
                    ▼
5. Select best provider (weighted random or session affinity)
                    │
                    ▼
6. Build request URL: provider.serviceUrl + options.path
                    │
                    ▼
7. Send request via X402 Payment Client
                    │
                    ├─── Provider returns 402 ───┐
                    │                            ▼
                    │                   Sign payment (viem)
                    │                            │
                    │                   Retry with X-PAYMENT header
                    │                            │
                    ◄────────────────────────────┘
                    │
                    ▼
8. Return DinResponse to user
```

### Sync Flow

```
┌─────────────────────────────────────────────────────────────┐
│                     Background Sync                          │
│                                                              │
│   Registry Timer (60s)          Score Timer (30s)           │
│         │                              │                     │
│         ▼                              ▼                     │
│   ┌─────────────┐               ┌─────────────┐             │
│   │ Fetch all   │               │ Fetch scores│             │
│   │ networks    │               │ per network │             │
│   └──────┬──────┘               └──────┬──────┘             │
│          │                             │                     │
│          ▼                             ▼                     │
│   ┌─────────────┐               ┌─────────────┐             │
│   │ Fetch       │               │ Update      │             │
│   │ providers   │               │ provider    │             │
│   │ per network │               │ scores      │             │
│   └──────┬──────┘               └─────────────┘             │
│          │                                                   │
│          ▼                                                   │
│   ┌─────────────┐                                           │
│   │ Update      │                                           │
│   │ cache       │                                           │
│   └─────────────┘                                           │
└─────────────────────────────────────────────────────────────┘
```

## Package Dependencies

```
@din-center/router
├── @din-center/registry    # Smart contract interactions
├── x402-axios              # Payment interceptor
├── axios                   # HTTP client
└── viem                    # Wallet/signing
```

## File Structure

```
din-router-sdk/
├── src/
│   ├── index.ts                    # Public exports
│   ├── client.ts                   # DinClient class
│   ├── types.ts                    # Core type definitions
│   │
│   ├── registry/
│   │   ├── index.ts
│   │   ├── sync.ts                 # RegistrySyncManager
│   │   └── types.ts                # Network, Provider types
│   │
│   ├── watcher/
│   │   ├── index.ts
│   │   ├── client.ts               # WatcherClient
│   │   ├── score-manager.ts        # Score computation
│   │   └── types.ts                # Score types
│   │
│   ├── selector/
│   │   ├── index.ts
│   │   ├── weighted-selector.ts    # Weighted random
│   │   └── session-affinity.ts     # Hash-based routing
│   │
│   ├── payment/
│   │   ├── index.ts
│   │   ├── x402-client.ts          # X402PaymentClient
│   │   └── types.ts                # Payment types
│   │
│   └── utils/
│       ├── hash.ts                 # Hashing utilities
│       └── retry.ts                # Retry logic
│
├── tests/
└── examples/
```

## Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Registry access | Separate SDK | Isolates contract complexity |
| HTTP client | Axios | x402-axios compatibility |
| Signing | Viem | Modern, well-maintained |
| Caching | In-memory Map | Simple, fast |
| Sync strategy | Timer-based polling | Simpler than epoch-based |
