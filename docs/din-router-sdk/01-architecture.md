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
│  │  (Orchestrator) │  │  (permissionless)│  │  Client         │              │
│  └────────┬────────┘  └────────┬────────┘  └────────┬────────┘              │
│           │                    │                    │                        │
│           │           ┌────────┴────────┐          │                        │
│           │           │                 │          │                        │
│           ▼           ▼                 ▼          ▼                        │
│  ┌─────────────────┐  ┌─────────────┐  ┌─────────────────┐                  │
│  │  Provider       │  │  Network    │  │  Watcher        │                  │
│  │  Selector       │  │  Cache      │  │  (permissionless)│                  │
│  └─────────────────┘  └─────────────┘  └─────────────────┘                  │
└─────────────────────────────────────────────────────────────────────────────┘
          │                    │                    │
          │                    ▼                    ▼
          │   ┌────────────────────────────────────────────────┐
          │   │        DIN Linea RPC (permissionless)          │
          │   │   (DIN-provided bootstrap RPC for registry)    │
          │   └────────────────────────────────────────────────┘
          │                    │
          │                    ▼
          │   ┌────────────────────────────────────────────────┐
          │   │     DIN Registry Smart Contract (Linea)        │
          │   │   DinRegistryHandler → Network → Provider       │
          │   └────────────────────────────────────────────────┘
          │
          ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         Selected RPC Provider                                │
│               (e.g., Infura, Alchemy, QuickNode) - x402 payment             │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Two-Phase Access Model

The SDK uses a two-phase approach:

| Phase | Data Source | Access Method | Payment |
|-------|-------------|---------------|---------|
| **Bootstrap** | Registry (Linea) | DIN-provided free RPC | FREE |
| **Ongoing** | Registry (Linea) | Linea providers from registry | x402 (USDC) |
| **Ongoing** | Watcher scores | Watcher providers from registry | x402 (USDC) |
| **Ongoing** | RPC requests | RPC providers from registry | x402 (USDC) |

**Key insight:** The Watcher is registered in the DIN Registry (network name: `watchers`), just like RPC endpoints. After bootstrap, all data access uses x402 payments (USDC on Linea).

This creates a sustainable model:
- **Low barrier to entry** - Free bootstrap, no upfront costs
- **Sustainable** - All providers (Linea, Watcher, RPC) get paid for ongoing access
- **Decentralized** - After bootstrap, you're using the network

## Component Overview

### DinClient (Main Entry Point)

The orchestrator that coordinates all SDK functionality:

- Manages client lifecycle (start/stop)
- Routes requests to the correct network
- Coordinates provider selection
- Handles request execution with payments

### Registry Sync Manager

Maintains an up-to-date cache of network and provider data using a **two-phase approach**:

**Phase 1 (Bootstrap):**
- Uses DIN-provided free Linea RPC for initial sync
- Gets list of networks, providers, and **Linea RPC endpoints**

**Phase 2 (Ongoing):**
- Uses Linea providers from the network with x402 payments
- Polls registry every 60s (configurable)
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

Fetches quality metrics from **Watcher providers registered in the DIN Registry** via x402 payments (USDC on Linea):

- Block consistency scores
- State consistency scores
- Latency metrics

The Watcher is registered in the registry (network: `watchers`), accessed like any RPC endpoint.

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
├── viem                    # Wallet/signing + Linea contract reads
├── x402-axios              # Payment interceptor
└── axios                   # HTTP client
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
│   │   ├── linea-client.ts         # Linea RPC interactions (permissionless)
│   │   └── types.ts                # Network, Provider types
│   │
│   ├── watcher/
│   │   ├── index.ts
│   │   ├── client.ts               # WatcherClient (permissionless, no API key)
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
| Registry access | DIN-provided Linea RPC | Permissionless, no API key needed |
| Watcher access | Public HTTP API | Permissionless, no API key needed |
| HTTP client | Axios | x402-axios compatibility |
| Signing | Viem | Modern, well-maintained, also handles contract reads |
| Caching | In-memory Map | Simple, fast |
| Sync strategy | Timer-based polling | Simpler than epoch-based |
