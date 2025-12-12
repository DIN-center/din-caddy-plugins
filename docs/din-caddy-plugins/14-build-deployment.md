# Build & Deployment

This document covers building, running, and deploying DIN Caddy Plugins.

## Prerequisites

- Go 1.23+
- Docker (optional, for containerized deployment)
- Make

## Build System

### Makefile Commands

```bash
# Build
make build          # Build with xcaddy
make docker-build   # Build Docker image

# Run
make run            # Run with Caddyfile.private
make run-dev        # Run with development config
make run-prod       # Run with production config

# Test
make test           # Run all tests
make test-coverage  # Run with coverage report
make test-race      # Run with race detector
make benchmark      # Run benchmarks

# Quality
make lint           # Run golangci-lint
make format         # Format code
make vet            # Run go vet
make check          # Run all checks
make secure         # Run govulncheck

# Utilities
make validate-config  # Validate Caddyfile
make generate-mocks   # Generate mock files
make secrets          # Generate Caddyfile from env

# Clean
make clean          # Remove build artifacts
```

---

## Building

### Local Build with xcaddy

xcaddy compiles Caddy with custom plugins:

```bash
make build
```

This runs:
```bash
xcaddy build --with github.com/DIN-center/din-caddy-plugins=. --output build/din-caddy
```

Output: `build/din-caddy`

### Manual xcaddy Build

```bash
xcaddy build \
    --with github.com/DIN-center/din-caddy-plugins=. \
    --with github.com/caddy-dns/cloudflare \
    --output build/din-caddy
```

### Docker Build

```bash
make docker-build
# or
docker build -t din-caddy .
```

---

## Dockerfile

**File**: `Dockerfile`

```dockerfile
# Build stage
FROM caddy:2.8.4-builder AS builder

WORKDIR /build
COPY . .

RUN xcaddy build \
    --with github.com/DIN-center/din-caddy-plugins=. \
    --output /build/din-caddy

# Runtime stage
FROM caddy:2.8.4

COPY --from=builder /build/din-caddy /usr/bin/caddy
COPY Caddyfile /etc/caddy/Caddyfile

EXPOSE 8080 2019

CMD ["caddy", "run", "--config", "/etc/caddy/Caddyfile"]
```

---

## Running Locally

### Development Mode

```bash
make run
# or
./build/din-caddy run --config Caddyfile.private
```

### With Environment Variables

```bash
export INFURA_KEY=your-key
export ALCHEMY_KEY=your-key
make run
```

### With Docker

```bash
docker run -p 8080:8080 -p 2019:2019 \
    -e INFURA_KEY=your-key \
    -e ALCHEMY_KEY=your-key \
    din-caddy
```

---

## Development Environment

### Start Full Stack

```bash
make dev
# or
docker compose up
```

This starts:
- DIN Caddy (port 8080)
- Prometheus (port 9090)
- Grafana (port 3000)
- Loki (port 3100)
- OTEL Collector (port 4317, 4318)

### Stop Stack

```bash
docker compose down
```

### View Logs

```bash
docker compose logs -f din-caddy
```

---

## Configuration Management

### Environment Variables

Create `.env.local` from template:

```bash
cp .env.example .env.local
# Edit .env.local with your values
```

### Generate Caddyfile from Environment

```bash
make secrets
# or
go run scripts/generate-caddyfile.go
```

This injects environment variables into `Caddyfile` template.

### Validate Configuration

```bash
make validate-config
# or
./build/din-caddy validate --config Caddyfile
```

---

## Deployment Options

### 1. Binary Deployment

```bash
# Build
make build

# Deploy binary
scp build/din-caddy server:/usr/local/bin/
scp Caddyfile server:/etc/caddy/

# Run on server
ssh server "din-caddy run --config /etc/caddy/Caddyfile"
```

### 2. Docker Deployment

```bash
# Build and push
docker build -t registry.example.com/din-caddy:latest .
docker push registry.example.com/din-caddy:latest

# Run on server
docker run -d \
    --name din-caddy \
    -p 8080:8080 \
    -e INFURA_KEY=$INFURA_KEY \
    registry.example.com/din-caddy:latest
```

### 3. Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: din-caddy
spec:
  replicas: 3
  selector:
    matchLabels:
      app: din-caddy
  template:
    metadata:
      labels:
        app: din-caddy
    spec:
      containers:
        - name: din-caddy
          image: registry.example.com/din-caddy:latest
          ports:
            - containerPort: 8080
            - containerPort: 2019
          env:
            - name: INFURA_KEY
              valueFrom:
                secretKeyRef:
                  name: din-secrets
                  key: infura-key
          resources:
            requests:
              memory: "256Mi"
              cpu: "250m"
            limits:
              memory: "512Mi"
              cpu: "500m"
          livenessProbe:
            httpGet:
              path: /health
              port: 2019
            initialDelaySeconds: 10
            periodSeconds: 5
          readinessProbe:
            httpGet:
              path: /health
              port: 2019
            initialDelaySeconds: 5
            periodSeconds: 3
---
apiVersion: v1
kind: Service
metadata:
  name: din-caddy
spec:
  selector:
    app: din-caddy
  ports:
    - name: http
      port: 8080
      targetPort: 8080
    - name: admin
      port: 2019
      targetPort: 2019
```

---

## CI/CD

### GitHub Actions Workflows

#### Unit Tests

**File**: `.github/workflows/unit_tests.yml`

```yaml
name: Unit Tests
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.23'
      - run: make test
      - run: make test-race
```

#### E2E Tests

**File**: `.github/workflows/e2e_integration_test.yml`

```yaml
name: E2E Tests
on: [push, pull_request]

jobs:
  e2e:
    runs-on: ubuntu-latest
    services:
      ethereum:
        image: trufflesuite/ganache:latest
        ports:
          - 8545:8545
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.23'
      - run: make build
      - run: make e2e-test
```

#### Generate Caddyfile

**File**: `.github/workflows/generate_caddyfile.yml`

```yaml
name: Generate Caddyfile
on:
  workflow_dispatch:

jobs:
  generate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - run: go run scripts/generate-caddyfile.go
        env:
          INFURA_KEY: ${{ secrets.INFURA_KEY }}
          ALCHEMY_KEY: ${{ secrets.ALCHEMY_KEY }}
```

---

## Health Checks

### Caddy Admin API

```bash
# Check health
curl http://localhost:2019/health

# Get config
curl http://localhost:2019/config/

# Reload config
curl -X POST http://localhost:2019/load \
    -H "Content-Type: application/json" \
    -d @caddy.json
```

### Metrics Endpoint

```bash
curl http://localhost:2019/metrics
```

---

## Logging Configuration

### Development

```caddyfile
{
    log {
        level DEBUG
        format console
    }
}
```

### Production

```caddyfile
{
    log {
        level INFO
        format json
        output stdout
    }
}
```

### Environment-based

```bash
LOG_LEVEL=debug ./build/din-caddy run
```

---

## Performance Tuning

### Resource Limits

```yaml
# Docker Compose
services:
  din-caddy:
    deploy:
      resources:
        limits:
          memory: 1G
          cpus: '2'
        reservations:
          memory: 256M
          cpus: '0.5'
```

### Connection Limits

```caddyfile
{
    servers {
        max_header_size 16KB
    }
}

reverse_proxy {
    transport http {
        dial_timeout 5s
        response_header_timeout 30s
        keepalive 30s
        max_conns_per_host 100
    }
}
```

---

## Troubleshooting

### Build Issues

```bash
# Clean and rebuild
make clean
make build

# Update dependencies
go mod tidy
```

### Runtime Issues

```bash
# Check config syntax
make validate-config

# View detailed logs
LOG_LEVEL=debug ./build/din-caddy run

# Check Caddy admin API
curl http://localhost:2019/config/
```

### Docker Issues

```bash
# Check container logs
docker logs din-caddy

# Shell into container
docker exec -it din-caddy sh

# Check process
docker exec din-caddy ps aux
```

---

## Related Documentation

- [Project Structure](./01-project-structure.md) - Directory layout
- [Caddyfile Configuration](./11-caddyfile-configuration.md) - Config reference
- [Metrics & Monitoring](./10-metrics-monitoring.md) - Observability
