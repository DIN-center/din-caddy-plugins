# Implementation Guide

## Prerequisites

Before implementing the router SDK:

1. **@din-center/registry SDK** must exist (or be mocked for testing)
2. **Watcher API** endpoints must be documented
3. **x402-axios** package must be available

## Implementation Order

### Phase 1: Foundation

#### 1. Core Types (`src/types.ts`)

Define all interfaces first:

```typescript
// DinConfig, RequestOptions, DinResponse
// Network, Provider, NetworkService
// Score, WatcherResponse
```

**Dependencies:** None
**Estimated complexity:** Low

#### 2. Registry Sync (`src/registry/sync.ts`)

Timer-based sync manager using din-registry-ts:

```typescript
// RegistrySyncManager class
// - start(), stop()
// - syncRegistry(), syncScores()
// - getNetwork(), getNetworks()
```

**Dependencies:** @din-center/registry
**Estimated complexity:** Medium

### Phase 2: Data Layer

#### 3. Watcher Client (`src/watcher/client.ts`)

HTTP client for Watcher API:

```typescript
// WatcherClient class
// - getScores(networkName)
// - getProviderScore(providerId)
```

**Dependencies:** axios
**Estimated complexity:** Low

#### 4. Score Manager (`src/watcher/score-manager.ts`)

Score computation and staleness decay:

```typescript
// computeProviderScore(blockConsistency, stateConsistency, latency)
// applyStaleDecay(score, elapsedMs, ...)
```

**Dependencies:** None
**Estimated complexity:** Low

### Phase 3: Selection Logic

#### 5. Provider Selector (`src/selector/`)

Weighted random selection and session affinity:

```typescript
// weighted-selector.ts
// - selectProvider(providers, sessionId?)
// - weightedRandomSelect(providers)

// session-affinity.ts
// - hashString(str)
// - selectBySession(providers, sessionId)
```

**Dependencies:** None
**Estimated complexity:** Low

### Phase 4: Payment

#### 6. Payment Client (`src/payment/`)

x402 integration with axios:

```typescript
// X402PaymentClient class
// - constructor(privateKey, network)
// - makeRequest(url, options)
```

**Dependencies:** x402-axios, axios, viem
**Estimated complexity:** Medium

### Phase 5: Integration

#### 7. Main Client (`src/client.ts`)

Orchestrate all components:

```typescript
// DinClient class
// - constructor(config)
// - request(network, options)
// - getNetworks(), getProviders(network)
// - start(), stop(), refresh()
```

**Dependencies:** All above modules
**Estimated complexity:** Medium

### Phase 6: Quality

#### 8. Tests

Unit tests for each module:

```
tests/
├── types.test.ts
├── registry-sync.test.ts
├── watcher-client.test.ts
├── score-manager.test.ts
├── weighted-selector.test.ts
├── session-affinity.test.ts
├── payment-client.test.ts
└── client.test.ts
```

**Testing approach:**
- Mock @din-center/registry for registry tests
- Mock axios for HTTP tests
- Mock x402-axios for payment tests

#### 9. Examples

Working code examples:

```
examples/
├── basic-jsonrpc.ts
├── rest-api.ts
├── session-affinity.ts
└── lifecycle-management.ts
```

## File Structure

```
din-router-sdk/
├── package.json
├── tsconfig.json
├── src/
│   ├── index.ts
│   ├── client.ts
│   ├── types.ts
│   │
│   ├── registry/
│   │   ├── index.ts
│   │   ├── sync.ts
│   │   └── types.ts
│   │
│   ├── watcher/
│   │   ├── index.ts
│   │   ├── client.ts
│   │   ├── score-manager.ts
│   │   └── types.ts
│   │
│   ├── selector/
│   │   ├── index.ts
│   │   ├── weighted-selector.ts
│   │   └── session-affinity.ts
│   │
│   ├── payment/
│   │   ├── index.ts
│   │   ├── x402-client.ts
│   │   └── types.ts
│   │
│   └── utils/
│       ├── hash.ts
│       └── retry.ts
│
├── tests/
└── examples/
```

## Package.json

```json
{
  "name": "@din-center/router",
  "version": "0.1.0",
  "main": "dist/index.js",
  "module": "dist/index.mjs",
  "types": "dist/index.d.ts",
  "scripts": {
    "build": "tsup src/index.ts --format cjs,esm --dts",
    "test": "vitest",
    "lint": "eslint src/",
    "typecheck": "tsc --noEmit"
  },
  "dependencies": {
    "@din-center/registry": "^0.1.0",
    "x402-axios": "^0.1.0",
    "axios": "^1.6.0",
    "viem": "^2.0.0"
  },
  "devDependencies": {
    "typescript": "^5.3.0",
    "tsup": "^8.0.0",
    "vitest": "^1.0.0",
    "@types/node": "^20.0.0",
    "eslint": "^8.0.0"
  }
}
```

## Critical Reference Files

From the Go codebase (for implementation reference):

| Go File | Purpose | TS Equivalent |
|---------|---------|---------------|
| `upstream/.../din/types.go` | Registry data types | `src/registry/types.ts` |
| `upstream/.../din/client.go` | Registry client | `@din-center/registry` |
| `modules/din_middleware_helpers.go` | Registry sync | `src/registry/sync.ts` |
| `upstream/.../watcher/types.go` | Watcher types | `src/watcher/types.ts` |
| `lib/watcherscore/watcherscore_manager.go` | Score computation | `src/watcher/score-manager.ts` |
| `lib/watcherscore/combiners.go` | Score weights | `src/watcher/score-manager.ts` |
| `modules/din_scorebased_selector.go` | Provider selection | `src/selector/` |

## Testing Strategy

### Unit Tests

Each module should have isolated unit tests:

```typescript
// tests/weighted-selector.test.ts
describe('weightedRandomSelect', () => {
  it('should select providers proportional to score', () => {
    // Mock Math.random for deterministic tests
  });

  it('should handle single provider', () => { });
  it('should handle zero scores', () => { });
});
```

### Integration Tests

Test the full flow with mocks:

```typescript
// tests/client.test.ts
describe('DinClient', () => {
  it('should auto-start on first request', () => { });
  it('should route to highest score provider', () => { });
  it('should handle payment flow', () => { });
});
```

### Mock Strategy

```typescript
// tests/mocks/registry.ts
export const mockRegistryClient = {
  getNetworks: vi.fn().mockResolvedValue([
    { name: 'ethereum-mainnet', status: 'Active', ... }
  ]),
  getProviders: vi.fn().mockResolvedValue([
    { name: 'Provider A', score: 0.9, ... }
  ]),
};
```

## Build Configuration

### tsconfig.json

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "module": "ESNext",
    "moduleResolution": "bundler",
    "strict": true,
    "declaration": true,
    "outDir": "dist",
    "rootDir": "src"
  },
  "include": ["src/**/*"],
  "exclude": ["node_modules", "dist", "tests"]
}
```

### tsup.config.ts

```typescript
import { defineConfig } from 'tsup';

export default defineConfig({
  entry: ['src/index.ts'],
  format: ['cjs', 'esm'],
  dts: true,
  splitting: false,
  sourcemap: true,
  clean: true,
});
```

## Public API Exports

```typescript
// src/index.ts
export { DinClient } from './client';
export type {
  DinConfig,
  RequestOptions,
  DinResponse,
  Network,
  Provider,
  NetworkService,
} from './types';
```

## Key Implementation Notes

1. **Auto-start behavior** - Client should lazily initialize on first `request()` call
2. **Error propagation** - Don't swallow errors; let users handle them
3. **Type safety** - Export all types for TypeScript users
4. **Tree-shaking** - Use named exports, avoid side effects
5. **Browser compatibility** - Avoid Node.js-specific APIs where possible
