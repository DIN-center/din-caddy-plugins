# Project Structure

This document provides a complete map of the din-caddy-plugins repository, explaining the purpose of each directory and key files.

## Root Directory

```
din-caddy-plugins/
├── module.go              # Module registration entry point
├── go.mod                 # Go module definition (Go 1.23.0)
├── go.sum                 # Dependency checksums
├── Dockerfile             # Multi-stage Docker build
├── Makefile               # Build, test, and development commands
├── compose.yml            # Docker Compose for local dev stack
├── Caddyfile              # Production configuration template
├── Caddyfile.private      # Local development configuration (gitignored)
├── README.md              # Project README
├── AGENTS.md              # AI agent configuration
├── .env.example           # Environment variable template
├── .golangci.yaml         # Linter configuration
├── .gitignore             # Git ignore rules
└── .tool-versions         # asdf version manager config
```

### Key Root Files

| File | Purpose |
|------|---------|
| `module.go` | Registers all Caddy modules, Prometheus metrics, and Caddyfile directives. This is the entry point that Caddy uses to load the plugins. |
| `Makefile` | Contains all build, test, lint, and development commands. Run `make help` for options. |
| `Caddyfile` | Production configuration template with placeholders for environment variables. |
| `compose.yml` | Local development environment with Grafana, Prometheus, Loki, and OTEL Collector. |

---

## `/modules` - Core Caddy Modules

The main middleware implementations that integrate with Caddy's reverse proxy.

```
modules/
├── din_middleware.go           # Main HTTP middleware handler (1013 lines)
├── din_upstreams.go            # Upstream source provider
├── din_select.go               # Provider selection policy
├── din_scorebased_selector.go  # Score-based weighted selection
├── network.go                  # Network configuration struct (869 lines)
├── provider.go                 # Provider configuration struct
├── consts.go                   # Constants and handler types
├── din_provider_filter.go      # Provider filtering logic
├── din_middleware_helpers.go   # Helper functions for middleware
├── response_writer_wrapper.go  # Custom response writer for metrics
├── caddy_unmarshaller.go       # Caddyfile parsing logic
├── helpers.go                  # General utility functions
├── *_test.go                   # Test files (13 total)
└── testdata/                   # Test fixtures
```

### Key Module Files

| File | Responsibility |
|------|----------------|
| `din_middleware.go` | Main entry point. Handles request routing, network resolution, provider filtering, and orchestrates health checks and registry sync. |
| `din_upstreams.go` | Implements `reverseproxy.UpstreamSource`. Provides list of eligible providers from request context. |
| `din_select.go` | Implements `reverseproxy.Selector`. Selects one provider using session affinity or score-based fallback. |
| `din_scorebased_selector.go` | Weighted random selection based on watcher scores. |
| `network.go` | Defines the `Network` struct with all configuration options, health check state, and block history. |
| `provider.go` | Defines the `Provider` struct with endpoint configuration, auth, and health status. |

---

## `/lib` - Core Libraries

Shared libraries used by the modules.

```
lib/
├── auth/                   # Authentication framework
│   ├── interface.go        # IAuthClient interface
│   ├── interface_mock.go   # Mock for testing
│   ├── siwe/               # Sign-In with Ethereum
│   │   ├── client.go       # SIWE client implementation
│   │   ├── server.go       # SIWE server middleware
│   │   └── *_test.go
│   └── oidc/               # OAuth2/OIDC
│       ├── client.go       # OIDC client implementation
│       └── client_test.go
│
├── http/                   # HTTP utilities
│   ├── http.go             # HTTP client wrapper
│   ├── interface.go        # IHTTPClient interface
│   ├── types.go            # JSON-RPC request/response types
│   └── *_test.go
│
├── logger/                 # Logging utilities
│   └── client.go           # Structured logging (zap wrapper)
│
├── network/                # Network handler framework
│   ├── handlers.go         # NetworkHandler interface (92 methods)
│   ├── registry.go         # Handler type registry
│   ├── evm_handler.go      # EVM JSON-RPC handler
│   ├── beacon_handler.go   # Beacon Chain REST handler
│   ├── bitcoin_handler.go  # Bitcoin JSON-RPC handler
│   ├── bitcoin_esplora_handler.go  # Bitcoin Esplora REST
│   ├── starknet_handler.go # StarkNet handler
│   ├── solana_handler.go   # Solana handler
│   ├── tron_handler.go     # Tron Full Node handler
│   ├── json_rpc_*.go       # JSON-RPC utilities
│   ├── request_path_config.go  # Path configuration
│   ├── *_test.go           # Handler tests
│   └── testdata/           # Test fixtures (JSON responses)
│
├── prometheus/             # Metrics and monitoring
│   ├── prometheus.go       # Prometheus client
│   ├── sampler.go          # Request sampling
│   ├── interface.go        # IPrometheusClient interface
│   └── *_test.go
│
├── utils/                  # General utilities
│   └── Various helpers
│
├── web3/                   # Web3 utilities
│   └── Web3-related helpers
│
└── watcherscore/           # Dynamic load balancing
    ├── interface.go        # IWatcherScoreManager interface
    ├── types.go            # ProviderMetric, Score types
    ├── watcherscore_manager.go  # Score computation engine
    ├── transformers.go     # Score transformations
    ├── combiners.go        # Metric combination strategies
    ├── math.go             # Mathematical utilities
    ├── watcher_metrics.go  # Metrics aggregation
    ├── watcher_helpers.go  # Helper functions
    ├── consts.go           # Constants and formulas
    └── *_test.go           # Tests (6 files)
```

### Library Packages

| Package | Purpose |
|---------|---------|
| `auth` | Authentication abstraction with SIWE and OIDC implementations |
| `http` | HTTP client interface and JSON-RPC type definitions |
| `logger` | Structured logging wrapper around zap |
| `network` | Network handler interface and implementations for different blockchains |
| `prometheus` | Metrics collection and request sampling |
| `watcherscore` | Dynamic load balancing score computation |

---

## `/cmd` - Command-Line Tools

```
cmd/
└── siwe-token/
    ├── main.go      # SIWE token generation CLI
    └── README.md    # Usage documentation
```

The `siwe-token` CLI generates SIWE authentication tokens for testing and development.

---

## `/services` - Monitoring Stack Configurations

```
services/
├── prometheus/
│   └── prometheus.yml     # Prometheus scrape configuration
├── grafana/
│   ├── datasources/       # Grafana data source configs
│   └── dashboards/        # Pre-built dashboards
├── loki/
│   └── loki-config.yml    # Loki log aggregation config
└── otel-collector/
    └── config.yaml        # OpenTelemetry collector config
```

These configurations are used by `compose.yml` for the local development monitoring stack.

---

## `/scripts` - Utility Scripts

```
scripts/
└── generate-caddyfile.go  # Generates Caddyfile from environment variables
```

Used in CI/CD to inject secrets into Caddyfile templates.

---

## `/docs` - Documentation

```
docs/
├── README.md                       # Documentation index
├── authentication.md               # Auth protocol details
├── health_checks.md                # Health check system
├── registry_sync.md                # Registry synchronization
├── network_specific_routing.md     # Method-based routing
├── initial_design.md               # Design philosophy
├── multi_network_type_support.md   # Network handler architecture
├── monitoring_setup.md             # Monitoring configuration
├── chain-id-migration.md           # Chain ID standards
├── adding_new_networks_form.md     # Adding network handlers
├── bitcoin-esplora-guide.md        # Bitcoin Esplora setup
├── SECRET_MANAGEMENT.md            # Secret management
│
├── din-router-sdk/                 # SDK documentation
│   ├── 01-architecture.md
│   ├── 02-registry-sync.md
│   ├── 03-provider-selection.md
│   ├── 04-x402-payments.md
│   ├── 05-api-reference.md
│   ├── 06-usage-examples.md
│   └── 07-implementation-guide.md
│
├── providers/                      # Provider documentation
│   ├── README.md
│   ├── edge.md
│   └── sidecar.md
│
├── gateway/                        # Gateway configuration
├── dynamic-load-balancing/         # Dynamic LB docs
│
├── din-caddy-plugins/              # This documentation set
│   └── *.md
│
└── RFC-din-ts-sdk.md               # TypeScript SDK RFC
```

---

## `/upstream` - Vendored Dependencies

```
upstream/
└── github.com/DIN-center/din-sc/apps/din-go/
    └── ...                         # DIN smart contract Go bindings
```

The DIN smart contract bindings are vendored here to avoid external dependency issues.

---

## `/.github` - CI/CD Workflows

```
.github/
└── workflows/
    ├── unit_tests.yml              # Run unit tests on push
    ├── e2e_integration_test.yml    # End-to-end integration tests
    └── generate_caddyfile.yml      # Generate config from secrets
```

---

## `/build` - Build Artifacts

```
build/
└── din-caddy                       # Compiled binary (gitignored)
```

Generated by `make build`. Contains the custom Caddy binary with DIN plugins.

---

## File Naming Conventions

| Pattern | Meaning |
|---------|---------|
| `*_test.go` | Test files |
| `*_mock.go` or `interface_mock.go` | Generated mocks for testing |
| `*_handler.go` | Network handler implementations |
| `*_helper*.go` | Helper/utility functions |
| `types.go` | Type definitions |
| `interface.go` | Interface definitions |
| `consts.go` | Constants and enums |

---

## Navigation Tips

1. **Start with `module.go`** - See what modules are registered
2. **Follow `din_middleware.go`** - Understand request flow
3. **Check `lib/network/handlers.go`** - See handler interface
4. **Read specific handlers** - e.g., `evm_handler.go` for EVM logic
5. **Review `Caddyfile`** - Understand configuration structure

## Related Documentation

- [Overview](./00-overview.md) - Project summary
- [Core Modules](./02-core-modules.md) - Detailed module documentation
- [Build & Deployment](./14-build-deployment.md) - Building the project
