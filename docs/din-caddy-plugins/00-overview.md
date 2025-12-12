# DIN Caddy Plugins - Overview

## What is DIN Caddy Plugins?

DIN Caddy Plugins is a production-grade blockchain RPC reverse proxy built as a set of middleware modules for [Caddy 2](https://caddyserver.com/). It provides intelligent request routing to multiple blockchain providers with sophisticated health monitoring, dynamic load balancing, and automatic failover capabilities.

The system is designed to serve as the routing layer for the **Decentralized Infrastructure Network (DIN)**, enabling reliable access to blockchain data across multiple networks and providers.

## Core Capabilities

### Multi-Chain Support
Routes requests to 7+ blockchain network types:
- **EVM Networks**: Ethereum, Optimism, Arbitrum, Polygon, Base, Linea, etc.
- **Bitcoin**: JSON-RPC and Esplora REST API
- **Beacon Chain**: Ethereum Consensus Layer (REST API)
- **Solana**: JSON-RPC
- **StarkNet**: JSON-RPC
- **Tron**: Full Node JSON-RPC

### Intelligent Provider Selection
- **Health-Based Routing**: Only routes to healthy providers
- **Session Affinity**: Sticky sessions via `Din-Session-Id` header
- **Score-Based Selection**: Weighted random selection using real-time provider metrics
- **Priority Tiers**: Support for primary, secondary, and fallback providers
- **Method Filtering**: Route specific RPC methods to capable providers

### Continuous Health Monitoring
- Per-provider block number tracking
- Block lag and stall detection
- Chain ID validation
- Archive node capability verification
- Configurable thresholds and intervals

### Dynamic Load Balancing
- Integration with DIN Watcher service
- Real-time provider scoring based on:
  - Block consistency
  - State consistency
  - Response latency
- Automatic traffic rebalancing

### DIN Registry Integration
- On-chain network and provider discovery
- Automatic configuration synchronization
- Epoch-based update cycles

### Authentication
- **SIWE (Sign-In with Ethereum)**: Cryptographic authentication
- **OIDC**: OAuth2/OpenID Connect support
- Automatic session management and token renewal

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         Client Request                          │
└─────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────┐
│                        Caddy HTTP Server                        │
└─────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────┐
│                        DIN Middleware                           │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │ Network      │  │ Provider     │  │ Request Processing   │  │
│  │ Resolution   │  │ Filtering    │  │ (Method Extraction)  │  │
│  └──────────────┘  └──────────────┘  └──────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────┐
│                        DIN Upstreams                            │
│         (Provides eligible providers from context)              │
└─────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────┐
│                         DIN Select                              │
│  ┌──────────────────────┐  ┌───────────────────────────────┐   │
│  │ Header Hash Selector │  │ Score-Based Selector          │   │
│  │ (Session Affinity)   │  │ (Weighted Random by Score)    │   │
│  └──────────────────────┘  └───────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Caddy Reverse Proxy                          │
│              (Request forwarding to upstream)                   │
└─────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Provider Endpoint                           │
│              (Blockchain node / RPC service)                    │
└─────────────────────────────────────────────────────────────────┘
```

## Background Services

The system runs several background goroutines:

| Service | Purpose | Interval |
|---------|---------|----------|
| Health Checks | Monitor provider block numbers and status | 5s (configurable) |
| Registry Sync | Sync configuration from DIN Registry | Every epoch (~2000 blocks) |
| Watcher Score Sync | Fetch provider scores from Watcher API | Configurable |

## Key Design Principles

1. **Modular Architecture**: Each component (middleware, upstreams, selector) is a separate Caddy module
2. **Handler Abstraction**: Network-specific logic is encapsulated in handler implementations
3. **Graceful Degradation**: System continues operating even when some providers fail
4. **Configuration Flexibility**: Supports Caddyfile config, environment variables, and on-chain registry
5. **Observability First**: Built-in Prometheus metrics, structured logging, and OpenTelemetry support

## Technology Stack

- **Language**: Go 1.23+
- **Framework**: Caddy 2.8.4
- **Build Tool**: xcaddy
- **Metrics**: Prometheus
- **Logging**: Zap (structured logging)
- **Tracing**: OpenTelemetry
- **Authentication**: SIWE, JWT, OIDC

## Quick Reference

| Aspect | Details |
|--------|---------|
| Main Package | `modules/` |
| Library Code | `lib/` |
| Entry Point | `module.go` |
| Config Format | Caddyfile |
| Build Command | `make build` |
| Test Command | `make test` |
| Run Command | `make run` |

## Related Documentation

- [Project Structure](./01-project-structure.md) - Directory layout and file purposes
- [Core Modules](./02-core-modules.md) - Main Caddy module implementations
- [Request Flow](./09-request-flow.md) - Complete request lifecycle
- [Caddyfile Configuration](./11-caddyfile-configuration.md) - Configuration reference
