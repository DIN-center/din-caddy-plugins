# AI Smart Routing

DIN's AI smart routing module (`http.handlers.din_ai`) routes OpenAI-compatible chat completion requests across multiple AI model providers with health-aware failover, TTFT-based load balancing, and streaming support.

## Overview

Clients send standard `POST /v1/chat/completions` requests and DIN handles provider selection, failover, protocol translation, and observability transparently. The module is a Caddy middleware that reads the request body, selects a provider, optionally transforms the request/response format (for non-OpenAI providers like Anthropic), and proxies the response back to the client.

### File structure

```
lib/ai/              — Pure types, interfaces, adapters (zero Caddy dependency)
  types.go           — OpenAI and Anthropic request/response types
  interface.go       — IStreamingHTTPClient interface
  adapter.go         — ProviderAdapter interface
  adapter_openai.go  — OpenAI passthrough adapter
  adapter_anthropic.go — Anthropic Messages API translation adapter
  hashing.go         — Session hash and TTFT-weighted selection

modules/ai/          — Caddy middleware module
  middleware.go      — Main module: ServeHTTP, Caddyfile parsing, Validate
  streaming.go       — SSE streaming proxy with first-chunk validation
  healthcheck.go     — Periodic health check system
  provider.go        — AIProvider struct with health state machine
  tier.go            — Tier-based provider selection
  metrics.go         — Prometheus metrics registration
  consts.go          — Constants and defaults
```

## Configuration

### Caddyfile

```caddyfile
din_ai {
    healthcheck_interval 30     # seconds between health checks (default: 30)
    healthcheck_threshold 3     # consecutive failures before unhealthy (default: 3)
    request_attempt_count 3     # max provider attempts per request (default: 3)

    tiers {
        fast {
            providers {
                groq-llama https://api.groq.com/openai/v1/chat/completions {
                    model llama-3.1-8b-instant
                    headers {
                        Authorization "Bearer {env.GROQ_API_KEY}"
                    }
                }
                deepseek-chat https://api.deepseek.com/v1/chat/completions {
                    model deepseek-chat
                    headers {
                        Authorization "Bearer {env.DEEPSEEK_API_KEY}"
                    }
                }
            }
        }
        balanced {
            providers {
                openai-gpt4o https://api.openai.com/v1/chat/completions {
                    model gpt-4o
                    headers {
                        Authorization "Bearer {env.OPENAI_API_KEY}"
                    }
                }
                anthropic-sonnet https://api.anthropic.com/v1/messages {
                    model claude-sonnet-4-20250514
                    adapter anthropic
                    headers {
                        x-api-key {env.ANTHROPIC_API_KEY}
                        anthropic-version 2023-06-01
                    }
                }
            }
        }
        premium {
            providers {
                openai-o3 https://api.openai.com/v1/chat/completions {
                    model o3
                    headers {
                        Authorization "Bearer {env.OPENAI_API_KEY}"
                    }
                    health_check {
                        max_completion_tokens 1
                    }
                }
            }
        }
    }
}
```

### Provider options

| Option | Required | Description |
|--------|----------|-------------|
| `model` | Yes | Model identifier sent to the backend (e.g., `gpt-4o`, `claude-sonnet-4-20250514`) |
| `adapter` | No | `openai` (default) or `anthropic`. Controls request/response translation |
| `headers` | No | Static headers sent with every request (supports `{env.VAR}` expansion) |
| `health_check` | No | Per-provider health check overrides (e.g., `max_completion_tokens 1` for reasoning models) |

### Environment variable expansion

Headers support `{env.VAR_NAME}` patterns that are expanded at Caddyfile parse time (not request time). Changing API keys requires `caddy reload`. Unset variables produce a stderr warning and expand to empty string.

### Validation

The module validates configuration at two points:
- **Caddyfile parsing** — Rejects negative intervals, duplicate tier/provider names, unknown adapter types, invalid URLs (must be http/https with a host), missing model/URL fields
- **`Validate()` method** — Runs after `Provision()` for JSON API configs. Checks all intervals > 0, at least one tier with providers, valid adapter types

## Request/Response Format

### Request

```
POST /v1/chat/completions
Content-Type: application/json
X-DIN-Tier: balanced
X-DIN-Session-Id: conv_abc123

{
  "model": "ignored",
  "messages": [{"role": "user", "content": "Hello"}],
  "stream": true,
  "temperature": 0.7,
  "top_p": 0.9,
  "stop": ["END"]
}
```

### Request headers

| Header | Required | Default | Description |
|--------|----------|---------|-------------|
| `Content-Type` | Yes | — | Must be `application/json` |
| `X-DIN-Tier` | No | `balanced` | Quality tier to route to (`fast`, `balanced`, or `premium`) |
| `X-DIN-Session-Id` | No | — | Session identifier for multi-turn conversations. When present, the same ID always routes to the same provider via deterministic hash. When absent, TTFT-weighted selection is used |

The `model` field in the request body is overwritten with the selected provider's configured model. Clients don't need to know which model they're talking to.

Request body limit is **10MB**. Bodies exceeding this return `413 Request Entity Too Large`.

### Response headers

| Header | Description |
|--------|-------------|
| `X-DIN-Provider` | Name of the provider that served the request |
| `X-DIN-Model` | Model ID used |
| `X-DIN-Tier` | Tier that was selected |
| `X-DIN-Request-Id` | Unique request ID (32 hex chars) |
| `X-DIN-Session-Pinned` | `true` if session stickiness was used |

### Response format

Standard OpenAI chat completion format for both streaming and non-streaming responses.

## Routing

### Tier selection

Requests are routed to a tier based on the `X-DIN-Tier` header (default: `balanced`). Each tier contains one or more providers. The tier defines the quality/cost tradeoff.

### Provider selection within a tier

1. **Session stickiness** — If `X-DIN-Session-Id` is present, the session ID is hashed (FNV-32a) to deterministically select a provider. This is stateless — all proxy instances independently hash to the same provider with zero shared state.

2. **TTFT-weighted selection** — Without a session ID, providers are selected randomly weighted by inverse time-to-first-token. Faster providers receive proportionally more traffic.

3. **Failover** — If a provider fails, the next untried provider is selected. The middleware retries up to `request_attempt_count` times (default: 3, capped at available provider count).

### Health-aware filtering

Only providers in `Healthy` or `Warning` state receive traffic. `Unhealthy` providers are excluded from the available pool. If all providers in a tier are unhealthy, the request returns `503 Service Unavailable`.

## Health Check System

### How it works

A single goroutine runs on a ticker (default: every 30 seconds). On each tick, it spawns concurrent goroutines to health-check all providers across all tiers. Each check sends a minimal chat completion request (`"ping"` with `max_tokens: 1`).

### State machine

```
Healthy ──[threshold failures]──► Unhealthy
   ▲                                  │
   │                                  │
   └──[threshold successes]───────────┘

Healthy ──[429 response]──► Warning ──[success]──► Healthy
```

- **Healthy → Unhealthy**: After `healthcheck_threshold` (default: 3) consecutive failures
- **Unhealthy → Healthy**: After `healthcheck_threshold` consecutive successes
- **Healthy → Warning**: On 429 (rate limited) response
- **Warning → Healthy**: On next successful check

### Request-level health feedback

Request failures in the retry loop also update provider health via `MarkPingWarning()`:
- Streaming errors, non-streaming errors, and non-200 status codes trigger a warning
- This is less aggressive than health check failures — warnings degrade state but don't immediately mark a provider unhealthy
- Transform errors (our code's fault) don't affect provider health

### Shutdown behavior

Health checks use a cancellable context derived from the module's quit channel. When Caddy shuts down or reloads, in-flight health check HTTP requests are cancelled immediately rather than blocking until the 120s client timeout.

### Per-provider overrides

Reasoning models (e.g., OpenAI `o3`) require `max_completion_tokens` instead of `max_tokens`. Use the `health_check` block to override:

```caddyfile
health_check {
    max_completion_tokens 1
}
```

The override is applied on top of the base `max_tokens: 1`. When `max_completion_tokens` is present, `max_tokens` is automatically removed (OpenAI rejects requests with both).

## Anthropic Adapter

The Anthropic adapter translates between OpenAI chat completion format and Anthropic's Messages API.

### What's supported

| Feature | Status |
|---------|--------|
| Text messages (user, assistant) | Supported |
| System messages | Supported (multiple concatenated with newline) |
| `max_tokens` | Supported (defaults to 4096 if not set) |
| `temperature` | Supported |
| `top_p` | Supported |
| `stop` sequences | Supported (string or array converted to `stop_sequences`) |
| Streaming (SSE) | Supported (full event sequence translation) |
| Non-streaming | Supported |

### What's not supported (MVP)

| Feature | Behavior |
|---------|----------|
| Tool use / function calling | Response-only tool_use blocks: error if no text present, text returned with tool calls dropped if mixed |
| Extended thinking | Thinking blocks dropped |
| Image / PDF input | Fails at unmarshal (content is string-only) |
| `top_k` parameter | Not forwarded |

### Streaming translation

Anthropic uses a stateful multi-event SSE protocol. The adapter tracks event sequences to assemble OpenAI-format chunks:

```
Anthropic                          → OpenAI
message_start                      → {"choices":[{"delta":{"role":"assistant"}}]}
content_block_delta (text_delta)   → {"choices":[{"delta":{"content":"..."}}]}
message_delta (stop_reason)        → {"choices":[{"finish_reason":"stop"}]}
message_stop                       → [DONE]
```

Malformed `message_start` events return an error instead of silently producing empty IDs.

## Metrics

All metrics use the `din_ai_` prefix.

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `din_ai_requests_total` | Counter | tier, provider, model, status | Total requests by outcome |
| `din_ai_tokens_input_total` | Counter | tier, provider, model | Input tokens consumed |
| `din_ai_tokens_output_total` | Counter | tier, provider, model | Output tokens generated |
| `din_ai_health_checks_total` | Counter | provider, status_code, health_status | Health check results |

TTFT measurements for routing decisions are tracked in-memory on provider structs (rolling window of 20 measurements), not exported to Prometheus.

## Provider Compatibility

| Provider | Adapter | Notes |
|----------|---------|-------|
| OpenAI | `openai` (passthrough) | Native OpenAI format |
| Anthropic | `anthropic` (translation) | Requires adapter + `anthropic-version` header |
| Mistral | `openai` | Native OpenAI-compatible |
| DeepSeek | `openai` | Native OpenAI-compatible |
| Grok/xAI | `openai` | Native OpenAI-compatible |
| Groq | `openai` | Native OpenAI-compatible |
| Moonshot | `openai` | Native OpenAI-compatible |
| Google Gemini | `openai` | Via `/v1beta/openai/` endpoint |

Most providers natively expose OpenAI-compatible APIs. Only Anthropic requires a custom translation adapter.

## Architecture Decisions

### Direct proxy vs Caddy reverse_proxy

The AI module handles proxying directly rather than using Caddy's `reverse_proxy` with custom upstreams/selectors. This is because AI routing requires request/response body transformation (for Anthropic), which Caddy's reverse_proxy doesn't support.

### Streaming failover with first-chunk validation

Providers can return HTTP 200 but send an error as the first SSE event (rate limiting, overloaded). The middleware validates the first SSE chunk before committing to the client. If it's an error, the middleware retries with the next provider transparently.

### Session stickiness via deterministic hashing

Session IDs are hashed (FNV-32a) with sorted provider names to deterministically select a provider. This is stateless — all 9+ proxy instances independently hash to the same provider with zero shared state, no Redis, no coordination. Same pattern used in the blockchain proxy's `HeaderHashSelection`.

### Health check system comparison with RPC

The AI health check system follows the same async-ticker pattern as the existing RPC health check system:

| Aspect | RPC system | AI system |
|--------|-----------|-----------|
| Provider iteration | Sequential (slow provider blocks cycle) | Concurrent goroutine per provider |
| Shutdown | Channel close only (in-flight HTTP blocks) | Context cancellation aborts in-flight requests |
| State machine | `consecutiveUnhealthyChecks` counter | Separate `failures`/`successes` counters |
| Recovery | Resets on any non-unhealthy status | Requires threshold successes from unhealthy |

Both use: ticker-based scheduling, quit channel signaling, `sync.Once` cleanup, threshold-based state transitions.

---

## Known Limitations

- **120s flat timeout** — Same HTTP client timeout for streaming, non-streaming, and health checks. Long streams are killed at 120s. Needs separate clients with appropriate timeouts.
- **New provider thundering herd** — Providers with zero TTFT measurements get very high selection weight (`1e9`), causing nearly all traffic to route to them until measurements accumulate.
- **No authentication** — MVP has no caller auth on the AI endpoint.
- **No rate limiting** — No per-user or per-key rate limits at the gateway level.
- **String-only content** — `ChatMessage.Content` is `string`, not supporting OpenAI's array-of-content-parts format for multimodal input.

## Open Issues (Tech Debt)

These are tracked in the [code review document](https://github.com/DIN-center/din-caddy-plugins/issues/190) and should be addressed in follow-up PRs:

### Orange flags (tech debt)
- O1: `ensureMetricsRegistered` coupling in tests
- O2: Integration test boilerplate (extract helper)
- O3: Double JSON parse on every request
- O4: Missing `context.Context` propagation to non-streaming upstream calls
- O5: `Tier` struct missing JSON tags (only Caddyfile works, not JSON API)
- O6: `ChatMessage.Content` string-only (no multimodal)
- O7: New providers get near-100% traffic (zero TTFT weight)
- O8: `Cleanup()` panics on nil logger if called before `Provision()`
- O9: `writeErrorResponse` ignores `json.Marshal` error
- O10: `w.Write()` return values ignored
- O11: Health check goroutines unbounded (no concurrency limit)
- O12: `streamToClient` double-closes `respBody`
- O13: Empty provider/model labels in 502 failure metrics
- O14: No excess-argument checking in Caddyfile directives

### Test coverage gaps
- T1: Caddyfile parsing tests (partially addressed — duplicate/negative/adapter tests added)
- T2: Streaming failover unit test with mocks
- T3: `Provision()` tests (Validate tests added)
- T4: Weak metric assertions (no counter increment verification)
- T5: Health check recovery test through `checkProvider`
- T8: Health threshold boundary off-by-one verification

## Future Work

### Near-term (next PRs)
- Separate HTTP clients with appropriate timeouts for streaming vs non-streaming vs health checks
- Default TTFT for new providers (prevent thundering herd)
- Authentication/authorization on the AI endpoint
- `Tier` struct JSON tags for Caddy JSON API support
- Multimodal content support (`content` as string or array)
- Concurrency limit for health check goroutines

### Medium-term
- Rate limiting per provider (token bucket or sliding window)
- Cost tracking and budgets per tier
- Model passthrough mode (user specifies exact model)
- Tool calling / function calling translation for Anthropic
- Extended thinking support for Anthropic
- Circuit breaker pattern (replace threshold-based health)
- Configurable timeouts per provider
- Retry-After header respect from 429 responses
- Request/response logging (opt-in debugging)

### Long-term (see RFP roadmap)
- Dynamic cost optimization with automated price fetching
- Spend controls and budget enforcement per API key
- Quality benchmarking and adaptive tier assignments
- Prompt-aware routing (classify task type, route to best model)
- Cascade routing (try cheap model, retry with premium if low confidence)
- BYOK (bring your own keys)
- Data residency controls
- SDK libraries (Python, TypeScript, Go)
- LangChain / LlamaIndex integration
