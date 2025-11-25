# Provider Selection

## Overview

When a request comes in, the SDK must select one provider from potentially many available for that network. The selection algorithm prioritizes quality (via watcher scores) while supporting session affinity for consistent routing.

## Selection Flow

```
din.request('ethereum-mainnet', options)
         │
         ▼
┌─────────────────────┐
│ 1. Get network from │
│    cache            │
└─────────┬───────────┘
         │
         ▼
┌─────────────────────┐
│ 2. Filter active    │
│    providers with   │
│    score > 0        │
└─────────┬───────────┘
         │
         ▼
┌─────────────────────┐     YES    ┌─────────────────────┐
│ 3. Session ID       │───────────▶│ Hash-based pick     │
│    provided?        │            │ (deterministic)     │
└─────────┬───────────┘            └─────────────────────┘
         │ NO
         ▼
┌─────────────────────┐
│ 4. Weighted random  │
│    selection        │
└─────────────────────┘
```

## Algorithm Steps

### Step 1: Get Candidates

```typescript
function selectProvider(
  providers: Provider[],
  sessionId?: string
): Provider {
  // Filter to only Active providers with positive scores
  let candidates = providers.filter(p =>
    p.status === 'Active' && (p.score ?? 0) > 0
  );

  // Fallback: if all providers are down, use all anyway
  if (candidates.length === 0) {
    candidates = providers;
  }

  // Continue to step 2 or 3...
}
```

### Step 2: Session Affinity (Optional)

If a `sessionId` is provided, use deterministic hash-based selection:

```typescript
if (sessionId && candidates.length > 1) {
  const hash = hashString(sessionId);
  const index = hash % candidates.length;
  return candidates[index];
}
```

**Use cases for session affinity:**
- User wallet address → consistent provider per user
- Transaction ID → same provider for related requests
- Any string → deterministic routing

### Step 3: Weighted Random Selection

Without session affinity, select randomly weighted by score:

```typescript
function weightedRandomSelect(providers: Provider[]): Provider {
  // Sum all scores
  const totalWeight = providers.reduce((sum, p) => sum + (p.score ?? 0), 0);

  // Pick random point in [0, totalWeight)
  let random = Math.random() * totalWeight;

  // Walk through until we pass the random point
  for (const provider of providers) {
    random -= (provider.score ?? 0);
    if (random <= 0) return provider;
  }

  // Fallback (shouldn't reach)
  return providers[providers.length - 1];
}
```

## Weighted Selection Example

Given 3 providers with scores:

| Provider | Score | Weight % | Selection Range |
|----------|-------|----------|-----------------|
| A        | 0.8   | 44%      | 0.0 - 0.8       |
| B        | 0.6   | 33%      | 0.8 - 1.4       |
| C        | 0.4   | 22%      | 1.4 - 1.8       |

Total weight: 1.8

**Selection process:**
1. Generate random number: e.g., `1.2`
2. Start with `random = 1.2`
3. Check Provider A: `1.2 - 0.8 = 0.4` (still positive, continue)
4. Check Provider B: `0.4 - 0.6 = -0.2` (negative, select B!)

**Result:** Provider B selected (~33% of the time on average)

## Score Computation

Scores come from the Watcher API and are computed using these weights:

```typescript
function computeProviderScore(
  blockConsistency: number,    // Is provider on correct block?
  stateConsistency: number,    // Does state match other providers?
  latency: number              // Normalized response time (0-1, higher = faster)
): number {
  // Weights from Go router implementation
  const score =
    (blockConsistency * 0.5) +  // 50% weight
    (stateConsistency * 0.3) +  // 30% weight
    (latency * 0.2);            // 20% weight

  return Math.max(0, Math.min(1, score)); // Clamp to [0, 1]
}
```

### Score Components

| Component | Weight | Description |
|-----------|--------|-------------|
| Block Consistency | 50% | Is the provider reporting the correct latest block? |
| State Consistency | 30% | Does the provider's state match consensus? |
| Latency | 20% | How fast does the provider respond? |

## Staleness Decay

If scores become stale (watcher sync fails), they decay toward a midpoint:

```typescript
function applyStaleDecay(
  score: number,
  elapsedMs: number,
  gracePeriodMs: number = 3600000,  // 60 min
  decayTauMs: number = 3600000,     // 60 min half-life
  midpoint: number = 0.5
): number {
  // No decay during grace period
  if (elapsedMs < gracePeriodMs) return score;

  // Exponential decay toward midpoint
  const t = (elapsedMs - gracePeriodMs) / 60000; // minutes
  const tc = decayTauMs / 60000;
  return midpoint + (score - midpoint) * Math.exp(-t / tc);
}
```

**Example decay:**
- Score: 0.9
- After 60 min grace + 60 min decay: ~0.7
- After 60 min grace + 120 min decay: ~0.6
- Eventually converges to 0.5 (midpoint)

## Session Affinity Details

### Hash Function

Use a simple but effective string hash:

```typescript
function hashString(str: string): number {
  let hash = 5381;
  for (let i = 0; i < str.length; i++) {
    hash = ((hash << 5) + hash) + str.charCodeAt(i);
  }
  return Math.abs(hash);
}
```

### Usage Patterns

**Global session (per-client):**
```typescript
const din = new DinClient({
  privateKey: process.env.PRIVATE_KEY!,
  sessionId: 'user-wallet-address',  // All requests use same provider
});
```

**Per-request session:**
```typescript
const response = await din.request('ethereum-mainnet', {
  body: { ... },
  sessionId: 'tx-123',  // Override for this request only
});
```

### Consistency Guarantees

Session affinity provides **best-effort** consistency:

- Same sessionId → same provider (when available)
- If provider becomes unavailable, selection falls through to weighted random
- Provider list changes can shift the hash mapping

## Edge Cases

### No Active Providers

```typescript
// If all filtered out, fall back to all providers
if (candidates.length === 0) {
  candidates = providers;
}
```

### All Scores Zero

If all providers have zero scores:
- `totalWeight = 0`
- `Math.random() * 0 = 0`
- First provider is selected

### Single Provider

With only one provider:
- Session affinity: always returns that provider
- Weighted random: always returns that provider

## Implementation Reference

The TypeScript implementation matches the Go router:

| Go File | TypeScript Equivalent |
|---------|----------------------|
| `modules/din_scorebased_selector.go` | `src/selector/weighted-selector.ts` |
| `lib/watcherscore/combiners.go` | `src/watcher/score-manager.ts` |
