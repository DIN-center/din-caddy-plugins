# Metrics & Monitoring

DIN Caddy Plugins provides comprehensive observability through Prometheus metrics, structured logging, and OpenTelemetry tracing.

## Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                      DIN Caddy Plugins                          │
│  ┌───────────────┐  ┌───────────────┐  ┌───────────────────┐   │
│  │ Prometheus    │  │ Zap Logger    │  │ OTEL Tracer       │   │
│  │ (Metrics)     │  │ (Logs)        │  │ (Traces)          │   │
│  └───────┬───────┘  └───────┬───────┘  └─────────┬─────────┘   │
└──────────┼──────────────────┼────────────────────┼─────────────┘
           │                  │                    │
           ▼                  ▼                    ▼
┌───────────────────┐ ┌───────────────┐ ┌─────────────────────┐
│    Prometheus     │ │     Loki      │ │   OTEL Collector    │
└───────────────────┘ └───────────────┘ └─────────────────────┘
           │                  │                    │
           └──────────────────┴────────────────────┘
                              │
                              ▼
                    ┌───────────────────┐
                    │      Grafana      │
                    └───────────────────┘
```

---

## Prometheus Metrics

**Package**: `lib/prometheus/`

### Metric Categories

#### Request Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `din_requests_total` | Counter | network, provider, method, status | Total requests |
| `din_request_duration_seconds` | Histogram | network, provider, method | Request latency |
| `din_request_size_bytes` | Histogram | network, provider | Request body size |
| `din_response_size_bytes` | Histogram | network, provider | Response body size |

#### Health Check Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `din_health_check_total` | Counter | network, provider, status | Health check count |
| `din_health_check_duration_seconds` | Histogram | network, provider | Check duration |
| `din_provider_health_status` | Gauge | network, provider | Current status (0=Healthy, 1=Warning, 2=Unhealthy) |
| `din_provider_block_number` | Gauge | network, provider | Latest block number |
| `din_provider_block_lag` | Gauge | network, provider | Blocks behind consensus |

#### Provider Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `din_provider_score` | Gauge | network, provider | Current watcher score |
| `din_provider_failures_total` | Counter | network, provider, error_type | Failure count |
| `din_provider_selection_total` | Counter | network, provider | Selection count |

#### Registry Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `din_registry_sync_total` | Counter | status | Sync attempts |
| `din_registry_sync_duration_seconds` | Histogram | - | Sync duration |
| `din_registry_networks_total` | Gauge | - | Networks from registry |

#### Authentication Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `din_auth_requests_total` | Counter | provider, status | Auth attempts |
| `din_auth_token_renewals_total` | Counter | provider | Token renewals |

### Metric Implementation

```go
// lib/prometheus/prometheus.go

type PrometheusClient struct {
    requestsTotal    *prometheus.CounterVec
    requestDuration  *prometheus.HistogramVec
    healthStatus     *prometheus.GaugeVec
    providerScore    *prometheus.GaugeVec
    // ...
}

func (c *PrometheusClient) RecordRequest(network, provider, method string, status int, duration time.Duration) {
    c.requestsTotal.WithLabelValues(network, provider, method, strconv.Itoa(status)).Inc()
    c.requestDuration.WithLabelValues(network, provider, method).Observe(duration.Seconds())
}
```

### Metric Registration

Metrics are registered in `module.go`:

```go
func init() {
    prometheus.MustRegister(
        requestsTotal,
        requestDuration,
        healthStatus,
        providerScore,
        // ...
    )
}
```

### Prometheus Endpoint

Metrics are exposed at `/metrics`:

```
# HELP din_requests_total Total number of requests
# TYPE din_requests_total counter
din_requests_total{network="ethereum-mainnet",provider="infura",method="eth_blockNumber",status="200"} 1234

# HELP din_request_duration_seconds Request duration in seconds
# TYPE din_request_duration_seconds histogram
din_request_duration_seconds_bucket{network="ethereum-mainnet",provider="infura",method="eth_blockNumber",le="0.1"} 1000
```

---

## Request Sampling

**File**: `lib/prometheus/sampler.go`

To reduce metric cardinality, requests can be sampled:

```go
type Sampler struct {
    SampleRate float64  // 0.0 to 1.0
    Methods    []string // Methods to sample
}

func (s *Sampler) ShouldSample(method string) bool {
    if !contains(s.Methods, method) {
        return true  // Always record non-sampled methods
    }
    return rand.Float64() < s.SampleRate
}
```

### Configuration

```caddyfile
din {
    metrics {
        sample_rate 0.1  # Sample 10% of high-volume methods
        sampled_methods {
            eth_call
            eth_getLogs
        }
    }
}
```

---

## Structured Logging

**Package**: `lib/logger/`

### Logger Setup

```go
// lib/logger/client.go

func NewLogger(level string, env string) *zap.Logger {
    config := zap.NewProductionConfig()

    if env == "development" {
        config = zap.NewDevelopmentConfig()
    }

    config.Level = zap.NewAtomicLevelAt(parseLevel(level))
    config.OutputPaths = []string{"stdout"}

    logger, _ := config.Build()
    return logger
}
```

### Log Levels

| Level | Usage |
|-------|-------|
| DEBUG | Detailed debugging information |
| INFO | Normal operational messages |
| WARN | Warning conditions |
| ERROR | Error conditions |

### Log Format

Production (JSON):
```json
{"level":"info","ts":1704067200.123,"caller":"middleware/din_middleware.go:123","msg":"Request processed","network":"ethereum-mainnet","provider":"infura","method":"eth_blockNumber","duration":0.045}
```

Development (Console):
```
2024-01-01T12:00:00.123Z INFO  middleware/din_middleware.go:123  Request processed  {"network": "ethereum-mainnet", "provider": "infura", "method": "eth_blockNumber", "duration": 0.045}
```

### Common Log Fields

```go
logger.Info("Request processed",
    zap.String("network", network),
    zap.String("provider", provider),
    zap.String("method", method),
    zap.Duration("duration", duration),
    zap.Int("status", statusCode),
)
```

---

## OpenTelemetry Tracing

### OTEL Configuration

**File**: `services/otel-collector/config.yaml`

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  batch:
    timeout: 1s
    send_batch_size: 1024

exporters:
  otlp:
    endpoint: "tempo:4317"
    tls:
      insecure: true

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlp]
```

### Environment Variables

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317
OTEL_SERVICE_NAME=din-caddy
OTEL_TRACES_SAMPLER=parentbased_traceidratio
OTEL_TRACES_SAMPLER_ARG=0.1
```

---

## Local Development Stack

**File**: `compose.yml`

```yaml
services:
  din-caddy:
    build: .
    ports:
      - "8080:8080"
      - "2019:2019"  # Admin API

  prometheus:
    image: prom/prometheus:v2.45.0
    volumes:
      - ./services/prometheus:/etc/prometheus
    ports:
      - "9090:9090"

  loki:
    image: grafana/loki:2.9.0
    volumes:
      - ./services/loki:/etc/loki
    ports:
      - "3100:3100"

  grafana:
    image: grafana/grafana:10.0.0
    volumes:
      - ./services/grafana/datasources:/etc/grafana/provisioning/datasources
      - ./services/grafana/dashboards:/etc/grafana/provisioning/dashboards
    ports:
      - "3000:3000"

  otel-collector:
    image: otel/opentelemetry-collector:0.82.0
    volumes:
      - ./services/otel-collector:/etc/otel
    ports:
      - "4317:4317"
      - "4318:4318"
```

### Starting the Stack

```bash
make dev  # or docker compose up
```

### Access Points

| Service | URL |
|---------|-----|
| DIN Router | http://localhost:8080 |
| Caddy Admin | http://localhost:2019 |
| Prometheus | http://localhost:9090 |
| Grafana | http://localhost:3000 |
| Loki | http://localhost:3100 |

---

## Prometheus Configuration

**File**: `services/prometheus/prometheus.yml`

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'din-caddy'
    static_configs:
      - targets: ['din-caddy:2019']
    metrics_path: /metrics

  - job_name: 'caddy'
    static_configs:
      - targets: ['din-caddy:2019']
    metrics_path: /metrics
```

---

## Grafana Dashboards

**Location**: `services/grafana/dashboards/`

### Pre-built Dashboards

1. **DIN Overview** - High-level system health
2. **Request Metrics** - Request volume, latency, errors
3. **Provider Health** - Health check status, block numbers
4. **Network Details** - Per-network breakdown

### Key Panels

**Request Rate by Network**:
```promql
sum(rate(din_requests_total[5m])) by (network)
```

**P99 Latency by Provider**:
```promql
histogram_quantile(0.99, sum(rate(din_request_duration_seconds_bucket[5m])) by (le, provider))
```

**Unhealthy Providers**:
```promql
din_provider_health_status == 2
```

**Provider Block Lag**:
```promql
din_provider_block_lag > 5
```

---

## Alerting

### Example Alert Rules

```yaml
groups:
  - name: din-alerts
    rules:
      - alert: ProviderUnhealthy
        expr: din_provider_health_status == 2
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Provider {{ $labels.provider }} is unhealthy"

      - alert: HighErrorRate
        expr: rate(din_requests_total{status=~"5.."}[5m]) > 0.1
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "High error rate on {{ $labels.network }}"

      - alert: ProviderLagging
        expr: din_provider_block_lag > 10
        for: 3m
        labels:
          severity: warning
        annotations:
          summary: "Provider {{ $labels.provider }} lagging by {{ $value }} blocks"
```

---

## Debugging Tips

### Check Provider Health

```bash
curl -s localhost:2019/metrics | grep din_provider_health_status
```

### View Request Latency

```bash
curl -s localhost:2019/metrics | grep din_request_duration_seconds
```

### Check Registry Sync Status

```bash
curl -s localhost:2019/metrics | grep din_registry_sync
```

### View Logs

```bash
# All logs
docker compose logs din-caddy

# Follow logs
docker compose logs -f din-caddy

# Filter by level
docker compose logs din-caddy 2>&1 | grep ERROR
```

---

## Related Documentation

- [Health Checks](./06-health-checks.md) - Health metrics details
- [Dynamic Load Balancing](./07-dynamic-load-balancing.md) - Score metrics
- [Build & Deployment](./14-build-deployment.md) - Running the stack
