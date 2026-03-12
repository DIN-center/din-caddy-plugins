# Code Review Guidelines

This document defines standards for reviewing code in this repository. The project is a Caddy-based reverse proxy plugin in Go that routes blockchain/RPC traffic across multiple upstream providers with scoring, health checks, and authentication.

---

## Core Principles

Reviews must evaluate four pillars in order of priority:

1. **Correctness** — does it do what it says, under all conditions?
2. **Maintainability** — can the next engineer understand, extend, and safely change it?
3. **Safety** — can it be exploited, leak data, or corrupt state?
4. **Performance** — will it hold up under production load?


A fifth implicit concern runs through all of them: **testability**. Code that cannot be cleanly tested is a liability.

---

## Performance

### Request path

The hot path (provider selection → upstream dispatch → response forwarding) must remain allocation-free where possible.

- Avoid heap allocations in `ServeHTTP`. Use `sync.Pool` for buffers, avoid `fmt.Sprintf` in favor of `strings.Builder` or pre-formatted byte slices.
- Do not hold locks across network I/O. Lock only to read/mutate shared state, release immediately.
- Keep selector logic (score-based, round-robin, filter) O(n) or better over the provider list. Document complexity if it is not obvious.
- `response_writer_wrapper.go` buffers the full response body for JSON-RPC parsing. Flag any change that increases buffering scope or delays the first write to the client.

### Concurrency

- Shared mutable state (`Network`, `Provider`, upstream lists, block-lag caches) must be protected. Prefer `sync/atomic` for counters and single-value flags; use `sync.RWMutex` for structs updated infrequently but read on every request.
- Goroutines spawned for background tasks (health checks, watcher score sync) must have a clear lifecycle: started in `Provision`, stopped in `Cleanup`, no goroutine leak on reload.
- Never start an unbounded number of goroutines proportional to request count.

### Caching

- Block-lag and health state are cached to avoid per-request RPC calls. Confirm cache invalidation paths are correct and that stale data cannot permanently disable a healthy provider.

---

## Security

### Input validation

- All external input — request bodies, query strings, path parameters, JWT claims, SIWE messages — must be validated before use.
- JSON-RPC method names and parameters received from clients must be treated as untrusted. Do not reflect them into upstream URLs, log lines, or error messages without sanitisation.
- Reject oversized request bodies at the middleware boundary. Do not read an unbounded body into memory.

### Secret handling

- Provider credentials, API keys, and registry endpoint URLs must never appear in log output. Use `zap`'s `zap.Stringer` or redaction helpers, not `%v` on structs containing secrets.
- Do not log raw request or response bodies at `INFO` or above. Debug logging of bodies must be gated behind an explicit config flag.

### Dependency hygiene

- New external dependencies require justification. Prefer standard library where reasonable. Check the module's maintenance status and known CVEs before approving.
- Cryptographic operations must use `crypto/` standard library primitives or well-audited third-party packages (e.g., `golang-jwt/jwt`). No hand-rolled crypto.

### Error handling

- Errors returned to clients must not expose internal provider URLs, upstream addresses, or stack traces.
- Distinguish client errors (4xx) from upstream errors (5xx) from internal errors. Mapping must be consistent across all handler paths.

---

## Test Coverage

### What must be tested

Every PR touching the following must include tests:

| Area | Minimum coverage expectation |
|---|---|
| Provider selection (`din_select.go`, `din_scorebased_selector.go`) | All selection strategies, including tie-breaking and empty-pool edge cases |
| Health and block-lag evaluation (`network_block_lag.go`, `network_evaluate_health_test.go`) | Healthy, degraded, and fully-down scenarios |
| Middleware request flow (`din_middleware.go`) | Happy path, auth failure, upstream error, retry, and timeout |
| Network-specific handlers (`lib/network/*_handler.go`) | Valid request, malformed request, upstream error per protocol (EVM, Solana, etc.) |
| Auth (`lib/auth/`) | Valid token, expired token, invalid signature, missing header |
| JSON-RPC parsing (`lib/network/json_rpc_parser.go`) | Single call, batch, malformed JSON, unknown method |

### Test quality

- Table-driven tests are preferred for functions with multiple input/output cases.
- Mock interfaces (`interface_mock.go`, `mockgen`/`testify/mock`) must be used for external dependencies (upstream HTTP, registry, watcher). Tests must not make real network calls.
- Subtests must have descriptive names: `t.Run("returns 503 when all providers unhealthy", ...)`.
- Do not assert on log output as a proxy for behavior. Assert on return values, response codes, and state changes.
- Flaky tests (time-dependent, random seed-dependent or relying on goroutine scheduling) are treated as bugs and must be fixed or deleted.

### Benchmarks

Performance-critical paths (selection, scoring, body buffering) must have `Benchmark*` functions. A PR that changes these paths must include benchmark results in the description showing no regression.

---

## Code Structure

- New protocol handlers in `lib/network/` must implement the `Handler` interface. Do not add protocol-specific logic to the middleware core.
- Caddy module provisioning (`Provision`) must be idempotent and safe to call on reload. Resources acquired in `Provision` must be released in `Cleanup`.
- Configuration structs must be JSON-serialisable with explicit field tags. Omit empty optional fields with `omitempty`. Do not use `interface{}` in config types.
- Keep files focused: a file that grows beyond ~400 lines is a signal to split by responsibility.

---

## Maintainability

Good code in this codebase outlasts any single contributor. Reviewers should push back on changes that make the system harder to reason about, even when they are technically correct.

### Abstractions over procedural accumulation

The greatest maintainability risk in a middleware codebase is logic that grows by accretion: flag after flag, special case after special case, all inline. When reviewing, ask whether new behaviour belongs to an existing concept or signals a new one that deserves a name and a home.

- If a function is doing two distinct things, it should be two functions. If two functions are doing the same thing with minor variation, they should share a common abstraction.
- Before adding a parameter or branch to an existing function, ask whether the variation is better expressed as a new implementation of an interface. In this codebase, `Handler`, `Selector`, and `Filter` are the natural extension points — reach for them first.
- Named domain types communicate intent. A `ProviderScore` is not a `float64`. A `NetworkName` is not a `string`. Wrapping primitives catches argument-order bugs and makes call sites self-documenting.

### DRY — but at the right level

Duplication is not always bad; the wrong abstraction is worse.

- **Duplicated logic** (the same conditional, the same transformation, the same error-mapping) should be extracted once it appears in a third place, or sooner if it is complex or security-relevant.
- **Duplicated structure** (two files that look similar) may reflect two genuinely separate concerns. Do not collapse them into a parameterised mega-function just to reduce line count.
- Shared helpers belong in `lib/`. If a helper is relevant only to one module, keep it local. Do not create `utils.go` catch-alls.

### SOLID — applied pragmatically

Rigorous SOLID compliance is not required, but the principles point at real failure modes worth checking:

- **Single responsibility**: a struct or function should have one reason to change. `DinMiddleware` is the legitimate entry point of this plugin and owns the Caddy lifecycle — everything else should be narrower. If a new type is taking on multiple unrelated responsibilities, split it.
- **Open/closed**: adding a new blockchain protocol should not require editing the middleware core. New behaviour should be expressed by implementing an existing interface, not by adding another `if network == "newchain"` branch.
- **Liskov substitution**: mock implementations of interfaces must behave consistently with the real ones. A mock that silently swallows errors instead of returning them produces tests that pass for the wrong reasons.
- **Interface segregation**: define interfaces at the call site, scoped to what the caller actually uses. A function that only calls `SelectProvider` should not depend on a type that also owns `Provision`, `Cleanup`, and `ServeHTTP`.
- **Dependency inversion**: the middleware core depends on abstractions, not on concrete implementations. Constructors should accept interfaces wherever the dependency might need to be swapped or mocked.

### Naming and comments

- Names should reflect domain concepts, not implementation details. `scoreBasedSelector` over `weightedList`; `providerFilter` over `listReducer`.
- Avoid abbreviations that are not universally understood in this domain (`rpc`, `jwt`, `evm` are fine; `mgr`, `proc`, `h` as a variable name for a complex handler are not).
- Comments explain *why*, not *what*. A comment restating the code is noise. A comment explaining a non-obvious invariant, a protocol quirk, or a deliberate trade-off is valuable.

---

## Review Checklist

Before approving a PR, confirm:

- [ ] No new heap allocations on the hot path without justification
- [ ] All goroutines have a bounded lifetime and are cleaned up on shutdown
- [ ] No secrets, provider URLs, or raw bodies in log output
- [ ] Auth middleware cannot be bypassed by omitting headers
- [ ] External input is validated before use
- [ ] New error paths return appropriate HTTP status codes without leaking internals
- [ ] Tests cover the happy path, at least two failure modes, and edge cases
- [ ] No real network calls in unit tests
- [ ] Benchmarks included for performance-sensitive changes
- [ ] No new dependency added without a comment explaining why it is preferred over stdlib
- [ ] `Provision`/`Cleanup` symmetry maintained for new Caddy modules
- [ ] New behaviour extends an interface rather than branching inside existing logic
- [ ] Duplicated logic extracted; no unjustified copy-paste across handlers or packages
- [ ] Names reflect domain concepts; no unexplained abbreviations or generic identifiers
