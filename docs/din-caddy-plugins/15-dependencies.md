# Dependencies

This document describes the key dependencies used in DIN Caddy Plugins, their purposes, and version information.

## Go Version

```
go 1.23.0
toolchain go1.24.2
```

---

## Core Dependencies

### Caddy Server

```
github.com/caddyserver/caddy/v2 v2.8.4
```

**Purpose**: HTTP server and reverse proxy framework

**Key packages used**:
- `caddy` - Module registration and lifecycle
- `caddyhttp` - HTTP middleware handling
- `caddyhttp/reverseproxy` - Reverse proxy and upstream management
- `caddyconfig/caddyfile` - Caddyfile parsing

### Ethereum Go

```
github.com/ethereum/go-ethereum v1.15.11
```

**Purpose**: Ethereum utilities and types

**Key packages used**:
- `crypto` - ECDSA key handling for SIWE
- `common` - Ethereum address types
- `hexutil` - Hex encoding/decoding

### SIWE (Sign-In with Ethereum)

```
github.com/spruceid/siwe-go v0.2.1
```

**Purpose**: SIWE message parsing and verification

**Key features**:
- SIWE message creation
- Signature verification
- Nonce management

### JWT

```
github.com/golang-jwt/jwt/v5 v5.2.1
```

**Purpose**: JWT token handling

**Key features**:
- Token creation and signing
- Token parsing and validation
- Claims management

---

## Observability

### Prometheus

```
github.com/prometheus/client_golang v1.19.1
```

**Purpose**: Metrics collection and exposition

**Key packages**:
- `prometheus` - Metric types (Counter, Gauge, Histogram)
- `promhttp` - HTTP handler for /metrics

### Zap Logger

```
go.uber.org/zap v1.27.0
```

**Purpose**: Structured logging

**Features**:
- High-performance logging
- JSON and console formats
- Log levels

### OpenTelemetry

```
go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.49.0
go.opentelemetry.io/otel v1.24.0
```

**Purpose**: Distributed tracing

**Features**:
- HTTP instrumentation
- Trace context propagation
- OTLP export

---

## HTTP & Networking

### Standard Library Extensions

```
golang.org/x/net v0.43.0
```

**Purpose**: Extended networking utilities

**Key packages**:
- `http2` - HTTP/2 support
- `context` - Request context handling

### Backoff

```
github.com/cenkalti/backoff/v5 v5.0.3
```

**Purpose**: Retry logic with exponential backoff

**Features**:
- Exponential backoff
- Maximum retry limits
- Context-aware retries

---

## Cloud Services

### AWS SDK

```
github.com/aws/aws-sdk-go-v2 v1.33.0
github.com/aws/aws-sdk-go-v2/config v1.29.0
github.com/aws/aws-sdk-go-v2/service/dynamodb v1.40.0
```

**Purpose**: AWS service integration (if needed for storage/config)

### Google Cloud

```
google.golang.org/api v0.183.0
```

**Purpose**: Google Cloud API client

---

## Testing

### Testify

```
github.com/stretchr/testify v1.10.0
```

**Purpose**: Testing assertions and utilities

**Key packages**:
- `assert` - Assertions
- `require` - Fatal assertions
- `mock` - Mocking support

### Go Mock

```
go.uber.org/mock v0.5.2
```

**Purpose**: Mock generation

**Usage**:
```bash
mockgen -source=interface.go -destination=interface_mock.go -package=pkg
```

---

## Vendored Dependencies

### DIN Smart Contract Bindings

**Location**: `upstream/github.com/DIN-center/din-sc/apps/din-go/`

**Purpose**: Go bindings for DIN Registry smart contract

Vendored to ensure:
- Consistent versions
- Offline builds
- No external dependency issues

---

## Build Tools

### xcaddy

```bash
go install github.com/caddyserver/xcaddy/cmd/xcaddy@latest
```

**Purpose**: Build Caddy with custom plugins

### golangci-lint

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

**Purpose**: Linting and static analysis

### govulncheck

```bash
go install golang.org/x/vuln/cmd/govulncheck@latest
```

**Purpose**: Security vulnerability scanning

### goimports

```bash
go install golang.org/x/tools/cmd/goimports@latest
```

**Purpose**: Import organization and formatting

---

## Version Management

### go.mod

```
module github.com/DIN-center/din-caddy-plugins

go 1.23.0

require (
    github.com/caddyserver/caddy/v2 v2.8.4
    github.com/ethereum/go-ethereum v1.15.11
    github.com/spruceid/siwe-go v0.2.1
    github.com/golang-jwt/jwt/v5 v5.2.1
    github.com/prometheus/client_golang v1.19.1
    go.uber.org/zap v1.27.0
    // ... more dependencies
)
```

### Updating Dependencies

```bash
# Update all dependencies
go get -u ./...

# Update specific dependency
go get -u github.com/caddyserver/caddy/v2@latest

# Tidy module
go mod tidy
```

### Checking for Updates

```bash
go list -m -u all
```

---

## Security Scanning

### Vulnerability Check

```bash
make secure
# or
govulncheck ./...
```

### Dependency Audit

```bash
go mod verify
```

---

## Dependency Graph

```
din-caddy-plugins
├── caddy/v2
│   ├── caddyhttp
│   └── reverseproxy
├── go-ethereum
│   └── crypto
├── siwe-go
│   └── ethereum signatures
├── jwt/v5
│   └── token handling
├── prometheus
│   └── metrics
├── zap
│   └── logging
└── otel
    └── tracing
```

---

## Compatibility Notes

### Caddy Version

Must use Caddy v2.8.4 or compatible version. The module system changed in v2.7.

### Go Version

Requires Go 1.23+ for:
- Generic type parameters
- Improved error handling
- Performance improvements

### Ethereum Go

v1.15.x is required for:
- Updated crypto packages
- EIP-4844 support
- Performance improvements

---

## License Information

| Dependency | License |
|------------|---------|
| Caddy | Apache 2.0 |
| go-ethereum | LGPL-3.0 |
| siwe-go | MIT |
| jwt/v5 | MIT |
| prometheus | Apache 2.0 |
| zap | MIT |
| testify | MIT |

---

## Related Documentation

- [Build & Deployment](./14-build-deployment.md) - Building with dependencies
- [Project Structure](./01-project-structure.md) - Where dependencies are used
