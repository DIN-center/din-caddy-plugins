# Dynamic Load Balancing

Dynamic load balancing uses real-time provider quality metrics from the DIN Watcher service to intelligently distribute traffic. Providers with better performance receive proportionally more requests.

## Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        DIN Watcher API                          │
│                   (External Monitoring Service)                  │
└─────────────────────────────────────────────────────────────────┘
                              │
                  Periodic Score Fetch
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Watcher Score Manager                          │
│  ┌───────────────┐  ┌───────────────┐  ┌───────────────────┐   │
│  │ Metric        │  │ Metric        │  │ Score             │   │
│  │ Generator     │──▶│ Combiner      │──▶│ Transformer       │   │
│  └───────────────┘  └───────────────┘  └───────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                              │
                       Score Storage
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                  DinScoreBasedSelector                           │
│              (Weighted Random Selection)                         │
└─────────────────────────────────────────────────────────────────┘
```

## Architecture

**Package**: `lib/watcherscore/`

### Key Components

| Component | File | Purpose |
|-----------|------|---------|
| `IWatcherScoreManager` | `interface.go` | Score management interface |
| `WatcherScoreManager` | `watcherscore_manager.go` | Score computation engine |
| `ProviderMetric` | `types.go` | Individual metric (0-1 scale) |
| `Score` | `types.go` | Aggregated provider score |
| `ProviderMetricGenerator` | `watcher_metrics.go` | Generates metrics from API |
| `ProviderMetricCombiner` | `combiners.go` | Combines metrics into score |
| `ScoreTransformer` | `transformers.go` | Applies transformations |

---

## IWatcherScoreManager Interface

```go
type IWatcherScoreManager interface {
    // Start begins score synchronization
    Start() error

    // Stop halts synchronization
    Stop()

    // GetScore returns the current score for a provider
    GetScore(providerName string) (float64, error)

    // GetAllScores returns scores for all providers
    GetAllScores() map[string]float64

    // UpdateScores manually triggers a score update
    UpdateScores() error
}
```

---

## Score Computation Pipeline

### 1. Metric Generation

Metrics are fetched from the DIN Watcher API:

```go
type ProviderMetric struct {
    Name      string    // Metric name
    Value     float64   // 0.0 to 1.0
    Timestamp time.Time // When collected
}
```

**Metrics Collected**:

| Metric | Description | Range |
|--------|-------------|-------|
| `block_consistency` | How well provider tracks chain head | 0-1 |
| `state_consistency` | Accuracy of state responses | 0-1 |
| `latency` | Response time (inverted) | 0-1 |

### 2. Metric Combination

Metrics are combined using a weighted formula:

```go
type ScoreFormula struct {
    Weights map[string]float64 // Metric weights
}

// Default weights
var DefaultWeights = map[string]float64{
    "block_consistency": 0.4,
    "state_consistency": 0.4,
    "latency":           0.2,
}
```

**Combination Logic**:

```go
func (c *WeightedCombiner) Combine(metrics []ProviderMetric) float64 {
    var score float64
    var totalWeight float64

    for _, m := range metrics {
        weight := c.weights[m.Name]
        score += m.Value * weight
        totalWeight += weight
    }

    return score / totalWeight
}
```

### 3. Score Transformation

Raw scores are transformed to handle edge cases:

```go
type ScoreTransformer interface {
    Transform(score float64, metadata ScoreMetadata) float64
}
```

**Transformations Applied**:

| Transformer | Purpose |
|-------------|---------|
| `GracePeriodTransformer` | Maintains score during brief outages |
| `ConvergenceTransformer` | Smooths score during recovery |
| `FloorTransformer` | Ensures minimum score |
| `CeilingTransformer` | Caps maximum score |

---

## Configuration

### DynamicLoadBalancingConfig

```go
type DynamicLoadBalancingConfig struct {
    // Enable dynamic load balancing
    Enabled bool `json:"enabled,omitempty"`

    // Watcher API endpoint
    WatcherURL string `json:"watcher_url,omitempty"`

    // Sync interval in seconds
    SyncInterval int `json:"sync_interval,omitempty"`

    // Score formula weights
    Weights map[string]float64 `json:"weights,omitempty"`

    // Grace period for stale scores (seconds)
    GracePeriod int `json:"grace_period,omitempty"`

    // Convergence period (seconds)
    ConvergencePeriod int `json:"convergence_period,omitempty"`
}
```

### Default Values

| Parameter | Default | Description |
|-----------|---------|-------------|
| `enabled` | false | Must be explicitly enabled |
| `sync_interval` | 30 | Fetch scores every 30 seconds |
| `grace_period` | 60 | Keep stale scores for 60 seconds |
| `convergence_period` | 120 | Smooth recovery over 2 minutes |

### Caddyfile Configuration

```caddyfile
din {
    dynamic_load_balancing {
        enabled true
        watcher_url https://watcher.din.network/api/v1
        sync_interval 30
        grace_period 60
        convergence_period 120

        weights {
            block_consistency 0.4
            state_consistency 0.4
            latency 0.2
        }
    }
}
```

---

## Score-Based Selection

**File**: `modules/din_scorebased_selector.go`

### Selection Algorithm

```go
func (s *DinScoreBasedSelector) Select(pool UpstreamPool, r *http.Request, rw http.ResponseWriter) *Upstream {
    // 1. Get scores for all upstreams
    scores := getScoresFromContext(r)

    // 2. Build cumulative probability distribution
    var cumulative float64
    distribution := make([]float64, len(pool))
    for i, upstream := range pool {
        score := scores[upstream.Dial]
        cumulative += score
        distribution[i] = cumulative
    }

    // 3. Generate random number
    random := rand.Float64() * cumulative

    // 4. Select based on distribution
    for i, threshold := range distribution {
        if random <= threshold {
            return pool[i]
        }
    }

    return pool[len(pool)-1]
}
```

### Selection Probability

Given scores:
- Provider A: 0.8
- Provider B: 0.6
- Provider C: 0.4

Total: 1.8

Probabilities:
- A: 0.8/1.8 = **44.4%**
- B: 0.6/1.8 = **33.3%**
- C: 0.4/1.8 = **22.2%**

### Fallback Behavior

If no scores available:
```go
if len(scores) == 0 {
    // Random selection among all upstreams
    return pool[rand.Intn(len(pool))]
}
```

---

## Score Transformers

### Grace Period Transformer

Maintains scores during brief metric outages:

```go
type GracePeriodTransformer struct {
    Period    time.Duration
    LastScore float64
    LastTime  time.Time
}

func (t *GracePeriodTransformer) Transform(score float64, meta ScoreMetadata) float64 {
    if meta.IsStale && time.Since(t.LastTime) < t.Period {
        return t.LastScore  // Use cached score
    }
    t.LastScore = score
    t.LastTime = time.Now()
    return score
}
```

### Convergence Transformer

Smooths score changes during recovery:

```go
type ConvergenceTransformer struct {
    Period       time.Duration
    RecoveryRate float64
}

func (t *ConvergenceTransformer) Transform(score float64, meta ScoreMetadata) float64 {
    if meta.IsRecovering {
        // Gradually increase score
        progress := meta.RecoveryDuration / t.Period
        return meta.PreviousScore + (score - meta.PreviousScore) * progress
    }
    return score
}
```

---

## Watcher API Integration

### API Endpoint

```
GET /api/v1/networks/{network}/providers/scores
```

### Response Format

```json
{
    "network": "ethereum-mainnet",
    "timestamp": "2024-01-01T12:00:00Z",
    "providers": [
        {
            "name": "provider-a",
            "score": 0.85,
            "metrics": {
                "block_consistency": 0.9,
                "state_consistency": 0.8,
                "latency": 0.75
            }
        }
    ]
}
```

### Sync Cycle

```go
func (m *WatcherScoreManager) syncLoop() {
    ticker := time.NewTicker(m.syncInterval)
    for {
        select {
        case <-ticker.C:
            m.fetchAndUpdateScores()
        case <-m.stopCh:
            return
        }
    }
}
```

---

## Score Storage

Scores are stored in memory per network:

```go
type scoreStore struct {
    mu     sync.RWMutex
    scores map[string]*Score
}

type Score struct {
    Value     float64
    Timestamp time.Time
    Metrics   map[string]float64
}
```

### Thread Safety

All score operations are protected:

```go
func (s *scoreStore) GetScore(provider string) float64 {
    s.mu.RLock()
    defer s.mu.RUnlock()
    if score, ok := s.scores[provider]; ok {
        return score.Value
    }
    return 1.0  // Default score
}
```

---

## Integration with DinSelect

DinScoreBasedSelector is used as the fallback in DinSelect:

```go
type DinSelect struct {
    SessionHeader string
    Fallback      reverseproxy.Selector  // DinScoreBasedSelector
}

func (s *DinSelect) Select(pool UpstreamPool, r *http.Request, rw http.ResponseWriter) *Upstream {
    // Check for session affinity
    if sessionID := r.Header.Get(s.SessionHeader); sessionID != "" {
        return s.selectByHash(pool, sessionID)
    }

    // Fallback to score-based
    return s.Fallback.Select(pool, r, rw)
}
```

---

## Metrics Emitted

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `din_provider_score` | Gauge | network, provider | Current score |
| `din_watcher_sync_duration_seconds` | Histogram | network | Sync latency |
| `din_watcher_sync_errors_total` | Counter | network, error | Sync failures |

---

## Debugging

### Check Current Scores

Scores are logged at DEBUG level:

```
DEBUG: Provider scores updated
  provider=infura score=0.85
  provider=alchemy score=0.78
  provider=quicknode score=0.72
```

### Force Score Update

Scores can be manually refreshed via the Watcher API sync.

---

## Related Documentation

- [Core Modules](./02-core-modules.md) - DinScoreBasedSelector
- [Health Checks](./06-health-checks.md) - Health-based filtering
- [Metrics & Monitoring](./10-metrics-monitoring.md) - Score metrics
- [Caddyfile Configuration](./11-caddyfile-configuration.md) - Configuration
