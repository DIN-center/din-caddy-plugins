# Spike Summary: Architectural Refactoring

This document explains a series of refactoring commits that improved the din-caddy-plugins codebase. Each section covers an architectural tension that existed, how it was resolved, and includes before/after examples. Written for junior developers learning good Go patterns.

## Overview

Seven commits transformed a monolithic `modules/` package into a cleaner architecture:

```
67ba6e1 refactor(auth): implement auth factory pattern
6173169 refactor(modules): split din_middleware.go by responsibility
cc445b9 refactor(network): split network.go by responsibility
ef1e15b refactor(network): define handler capability interfaces
8e2ab33 feat(tests): add HTTP integration tests for request routing and failover
da230de refactor(network): extract network package to internal/network
93af7bf fix(tests): resolve race conditions in TestCleanup and TestCleanupConcurrency
```

---

## 1. Auth Factory Pattern (`67ba6e1`)

### The Tension

Authentication client creation was scattered throughout the codebase. The `provider.go` file knew how to construct SIWE clients, OIDC clients, and handle their configuration. This violated the Single Responsibility Principle: the Provider struct shouldn't know authentication implementation details.

### Before

```go
// modules/provider.go
func (p *provider) setupAuth(authConfig AuthConfig) error {
    switch authConfig.Type {
    case "siwe":
        // 20 lines of SIWE-specific setup
        client, err := siwe.NewClient(authConfig.URL, ...)
        p.authClient = client
    case "oidc":
        // 20 lines of OIDC-specific setup
        client, err := oidc.NewClient(authConfig.ClientID, ...)
        p.authClient = client
    }
}
```

Every time you add a new auth type, you'd modify `provider.go`. The Provider knows too much about authentication internals.

### After

```go
// lib/auth/factory.go
type Factory interface {
    CreateClient(config map[string]interface{}) (IAuthClient, error)
}

// lib/auth/siwe/factory.go
type SIWEFactory struct{}

func (f *SIWEFactory) CreateClient(config map[string]interface{}) (auth.IAuthClient, error) {
    // SIWE-specific construction logic lives here
}

// lib/auth/oidc/factory.go
type OIDCFactory struct{}

func (f *OIDCFactory) CreateClient(config map[string]interface{}) (auth.IAuthClient, error) {
    // OIDC-specific construction logic lives here
}
```

```go
// modules/caddy_unmarshaller.go
func (p *provider) setupAuth(authConfig AuthConfig) error {
    factory := auth.GetFactory(authConfig.Type)
    client, err := factory.CreateClient(authConfig.Config)
    p.authClient = client
}
```

### Why This Matters

**Open/Closed Principle**: Adding a new auth type (say, API keys) means creating a new factory file; you never touch Provider or the unmarshaller. The system is open for extension, closed for modification.

**Testability**: You can mock the factory interface in tests rather than mocking the entire auth subsystem.

---

## 2. Split din_middleware.go by Responsibility (`6173169`)

### The Tension

`din_middleware.go` was 900+ lines handling HTTP requests, lifecycle management, and background goroutines. Finding anything required scrolling through unrelated code. The file had too many reasons to change.

### Before

```
modules/
  din_middleware.go  (887 lines: HTTP handling + lifecycle + background tasks)
```

### After

```
modules/
  din_middleware.go   (core struct definition and Caddy interface)
  din_http.go         (ServeHTTP and request handling - 411 lines)
  din_lifecycle.go    (Provision, Validate, Cleanup - 406 lines)
  din_background.go   (health checks, registry sync - 104 lines)
```

### The Pattern: Vertical Slicing

Each file now has a single axis of change:
- **din_http.go**: Changes when request handling logic changes
- **din_lifecycle.go**: Changes when startup/shutdown behavior changes
- **din_background.go**: Changes when background task behavior changes

This isn't about line count; it's about cohesion. Code that changes together should live together.

---

## 3. Split network.go by Responsibility (`cc445b9`)

### The Tension

Same problem as din_middleware.go. `network.go` was 716 lines mixing health check logic with block lag calculations. These are conceptually distinct: health checks determine if a provider is up, block lag determines if it's keeping pace with the chain.

### Before

```
modules/
  network.go  (716 lines: health checks + block lag + provider management)
```

### After

```
modules/
  network.go           (core Network struct and basic methods)
  network_health.go    (health check logic - 412 lines)
  network_block_lag.go (block lag calculations - 322 lines)
```

### Why Separate Health and Block Lag?

They have different:
- **Change triggers**: Health check logic changes when you add new failure modes; block lag changes when you tune chain-specific timing
- **Testing needs**: Health checks need mock HTTP responses; block lag needs mock timestamps
- **Mental models**: One is "is this provider responding?" the other is "is this provider keeping up?"

---

## 4. Handler Capability Interfaces (`ef1e15b`)

### The Tension

The Network struct needed to call handler methods, but not all handlers support all operations. EVM handlers support `eth_chainId`; Beacon handlers don't. The code was full of type assertions and handler-type checks:

### Before

```go
// Scattered throughout network_health.go
if n.HandlerType == EVMHandler {
    chainId, err := n.handler.GetChainID(...)  // Might panic if handler doesn't have this!
}

if n.HandlerType == EVMHandler || n.HandlerType == BeaconHandler {
    // Archive check logic
}
```

This is fragile. Add a new handler type and you need to audit every conditional.

### After

```go
// lib/network/capabilities.go
type ChainIdentifier interface {
    GetChainID(url string, headers map[string]string, ...) (string, error)
    ValidateChainID(chainID string) error
}

type ArchiveChecker interface {
    SupportsArchiveMode() bool
    PerformArchiveCheck(url string, ...) error
}

type BlockFetcher interface {
    GetLatestBlockNumber(url string, ...) (*LatestBlockResult, error)
    FormatBlockHeight(height int64) string
}
```

```go
// network_health.go
if chainIdentifier, ok := n.Handler.(networklib.ChainIdentifier); ok {
    chainId, err := chainIdentifier.GetChainID(...)
    // Safe: we only call this if the handler declares the capability
}
```

### The Pattern: Interface Segregation

Instead of one fat `NetworkHandler` interface with 15 methods (where handlers implement stubs for unsupported operations), we have small focused interfaces. A handler declares its capabilities by which interfaces it implements.

This is compile-time documentation: looking at a handler's type declaration tells you exactly what it can do.

### Case Study: Bitcoin Esplora (REST vs JSON-RPC)

This pattern directly solved the Bitcoin Esplora problem. Bitcoin has two different API styles:

| | Bitcoin Core | Bitcoin Esplora |
|---|---|---|
| **Protocol** | JSON-RPC | REST |
| **Health check** | `POST {"method":"getblockcount"}` | `GET /blocks/tip/height` |
| **Empty body** | Error (invalid JSON-RPC) | OK (normal for GET) |
| **Chain ID** | Can query via RPC | No concept of chain ID |

**Before capability interfaces**, the middleware had hardcoded assumptions:

```go
// din_http.go - BROKEN for REST handlers
if len(bodyBytes) == 0 {
    return fmt.Errorf("request body is empty")  // Rejects valid REST GET requests!
}

// Health checks assumed JSON-RPC
payload := fmt.Sprintf(`{"jsonrpc":"2.0","method":"%s","id":1}`, method)
```

**After capability interfaces**, the handler declares its protocol:

```go
// lib/network/bitcoin_esplora_handler.go
func (h *BitcoinEsploraHandler) GetRequestType() RequestType {
    return RequestTypeREST  // Not RequestTypeRPC
}

func (h *BitcoinEsploraHandler) GetHealthCheckHTTPMethod() string {
    return "GET"  // Not POST
}

func (h *BitcoinEsploraHandler) CreateHealthCheckPayload(method string) ([]byte, error) {
    return nil, nil  // REST GET has no body
}
```

The middleware uses `GetRequestType()` to branch behavior:

```go
// din_http.go - FIXED
if len(bodyBytes) == 0 && networkObj.Handler.GetRequestType() == networklib.RequestTypeRPC {
    // Only reject empty body for JSON-RPC handlers
    return fmt.Errorf("request body is empty")
}
// REST handlers with empty body pass through fine
```

This is the power of capability interfaces: the handler declares what it supports, and the middleware adapts. Adding a new protocol (say, GraphQL) means implementing the interfaces; you don't modify the middleware's conditional logic.

---

## 5. HTTP Integration Tests (`8e2ab33`)

### The Tension

The codebase had unit tests but no integration tests verifying the full request flow. You could have all unit tests pass while the actual HTTP routing was broken.

### What Was Added

```go
// modules/integration_test.go

func TestRequestRouting(t *testing.T) {
    // Sets up a real DinMiddleware with mock providers
    // Makes actual HTTP requests through the full stack
    // Verifies correct provider selection, failover, etc.
}

func TestProviderFailover(t *testing.T) {
    // Verifies that when primary provider fails,
    // requests route to backup providers
}
```

### The Pattern: Test Pyramid

```
        /\
       /  \     E2E tests (few, slow, high confidence)
      /----\
     /      \   Integration tests (medium) <-- this commit
    /--------\
   /          \ Unit tests (many, fast, focused)
  --------------
```

Integration tests fill the gap between "each function works" and "the system works." They catch issues like:
- Middleware ordering problems
- Context propagation bugs
- HTTP header handling

---

## 6. Extract Network Package (`da230de`)

### The Tension

Everything lived in `modules/`, which depends on Caddy. But the Network and Provider types are domain logic that shouldn't need Caddy to test. The dependency was backwards: domain logic depended on infrastructure.

### Before

```
modules/
  network.go           (depends on Caddy via same package)
  provider.go          (depends on Caddy via same package)
  din_middleware.go    (Caddy module)
```

Testing Network required importing all of `modules/`, which pulled in Caddy.

### After

```
internal/
  network/
    network.go         (pure domain logic)
    provider.go        (pure domain logic)
    interfaces.go      (INetwork, IProvider)

modules/
  network.go           (type alias: type network = internalnetwork.Network)
  provider.go          (type alias: type provider = internalnetwork.Provider)
  din_middleware.go    (Caddy module, imports internal/network)
```

### The Type Alias Bridge

```go
// modules/network.go
import internalnetwork "github.com/DIN-center/din-caddy-plugins/internal/network"

type network = internalnetwork.Network
```

This lets existing code continue using `*network` while the real implementation lives in `internal/network`. It's a migration strategy: you can incrementally move code without a big-bang rewrite.

**Is the alias still needed?** Yes. There are 100+ references to `*network` and `*provider` throughout `modules/`. The alias keeps the code readable; without it you'd have `*internalnetwork.Network` everywhere, which is verbose and obscures intent.

### The Pattern: Dependency Inversion

```
Before:  [Domain Logic] ----depends on----> [Caddy Infrastructure]

After:   [Caddy Infrastructure] ----depends on----> [Domain Logic]
```

High-level policy (Caddy integration) depends on low-level detail (network logic), not vice versa. This is the "D" in SOLID.

---

## 7. Fix Race Conditions (`93af7bf`)

### The Tension

Two tests had data races that `go test -race` caught:

```
WARNING: DATA RACE
Read at 0x00c000616178 by goroutine 315
Previous write at 0x00c000616178 by goroutine 314
```

### Before: The Broken Pattern

```go
func TestCleanup(t *testing.T) {
    closedCount := 0  // Shared variable, no protection

    for _, network := range d.Networks {
        go func(ch chan struct{}) {
            <-ch
            closedCount++  // WRITE from multiple goroutines
        }(network.Quit)
    }

    d.Cleanup()
    time.Sleep(50 * time.Millisecond)  // Hope they finish...

    assert.Equal(t, expected, closedCount)  // READ while writes may happen
}
```

Two problems:
1. **Data race**: `closedCount++` isn't atomic; concurrent increments can lose updates
2. **Timing**: `time.Sleep` is hope, not synchronization

### After: The Correct Pattern

```go
func TestCleanup(t *testing.T) {
    var closedCount atomic.Int32
    var wg sync.WaitGroup

    for _, network := range d.Networks {
        wg.Add(1)
        go func(ch chan struct{}) {
            defer wg.Done()
            <-ch
            closedCount.Add(1)  // Atomic increment
        }(network.Quit)
    }

    d.Cleanup()
    wg.Wait()  // Blocks until all goroutines complete

    assert.Equal(t, expected, int(closedCount.Load()))
}
```

### The Pattern: Synchronization Primitives

| Need | Use |
|------|-----|
| Wait for goroutines to finish | `sync.WaitGroup` |
| Shared counter | `atomic.Int32` |
| Shared boolean | `atomic.Bool` |
| Complex shared state | `sync.Mutex` or channels |

**Rule of thumb**: If you're using `time.Sleep` to wait for goroutines, you're doing it wrong.

---

## Summary

| Commit | Tension | Resolution |
|--------|---------|------------|
| Auth factory | Provider knew auth internals | Factory pattern; auth types self-register |
| Split din_middleware | 900-line god file | Vertical slicing by responsibility |
| Split network | Mixed health + block lag | Separate files for separate concerns |
| Capability interfaces | Type assertions everywhere | Interface segregation; handlers declare capabilities |
| Integration tests | No end-to-end coverage | Test pyramid; verify full request flow |
| Extract network pkg | Domain logic coupled to Caddy | Dependency inversion; domain in internal/ |
| Fix races | Data races in tests | WaitGroup + atomics |

Each change makes the codebase easier to test, easier to extend, and easier to understand. That's the goal of refactoring: not changing what code does, but improving how it's organized.
