# AI Smart Routing: Red and Orange Fix Plan

Plan for addressing RED (must fix before merge) and ORANGE (fix soon after merge) items from the feat/ai-smart-routing code review.

**Last Updated:** 2026-02-23
**Review Document:** `/Users/natefikru/go/src/din/din-markdown/reviews/ai-smart-routing-code-review.md`
**PR:** https://github.com/DIN-center/din-caddy-plugins/pull/193

---

## Status Summary

All RED items are resolved. All ORANGE items are resolved except two low-impact items deferred to future work.

| ID | Item | Status | Commit |
|----|------|--------|--------|
| R1 | Streaming transform errors drop content silently | Resolved | `0d992af` (streaming error handling) |
| R2 | Cleanup can panic on nil logger | Resolved | `0d992af` (nil logger guard) |
| O1 | Double-close of respBody | Resolved | `0d992af` (sync.Once guard) |
| O2 | 502 metrics with empty provider/model | **Deferred** | Low impact, tracked as O13 in review doc |
| O3 | Zero TTFT thundering herd | Resolved | Sane default TTFT baseline per docs |
| O4 | Health check goroutines unbounded | Resolved | `0d992af` (atomic skip-if-running guard) |
| O5 | Path suffix matching too permissive | Resolved | Exact match (`requestPath != "/v1/chat/completions"`) |
| O6 | writeErrorResponse ignores marshal error | **Deferred** | Unlikely to fail for static struct, tracked as O9 in review doc |

---

## Resolved Items

### R1. Streaming transform errors drop content silently

**File:** `modules/ai/streaming.go`

**Resolution:** Streaming error handling hardened in commit `0d992af`. Transform errors now propagate correctly and trigger failover rather than silently dropping content.

---

### R2. Cleanup can panic on nil logger

**File:** `modules/ai/middleware.go`

**Resolution:** Nil logger guard added in commit `0d992af`:

```go
m.cleanupOnce.Do(func() {
    if m.quit != nil {
        close(m.quit)
    }
    if m.logger != nil {
        m.logger.Info("DIN AI middleware cleaned up")
    }
})
```

Verified by `TestCleanup_StopsHealthChecks`.

---

### O1. Double-close of respBody

**File:** `modules/ai/streaming.go`

**Resolution:** `sync.Once` guard added in commit `0d992af`:

```go
var closeOnce sync.Once
closeBody := func() { closeOnce.Do(func() { respBody.Close() }) }
defer closeBody()
```

---

### O3. Zero TTFT thundering herd

**File:** `lib/ai/hashing.go`

**Resolution:** Sane default TTFT baseline applied so unmeasured providers get reasonable traffic share instead of ~100%.

---

### O4. Health check goroutines unbounded

**File:** `modules/ai/healthcheck.go`

**Resolution:** Atomic skip-if-running guard added in commit `0d992af`. If a previous health check round is still in flight, the current tick is skipped rather than stacking goroutines.

---

### O5. Path suffix matching too permissive

**File:** `modules/ai/middleware.go`

**Resolution:** Changed from `strings.HasSuffix` to exact path match:

```go
if r.Method != http.MethodPost || requestPath != "/v1/chat/completions" {
```

---

## Deferred Items

### O2. 502 metrics with empty provider/model

**File:** `modules/ai/middleware.go`

**Status:** Deferred — low impact. Empty label values in the all-providers-exhausted path don't affect routing behavior. Tracked as O13 in the review document for a future metrics overhaul.

---

### O6. writeErrorResponse ignores marshal error

**File:** `modules/ai/middleware.go`

**Status:** Deferred — `json.Marshal` of a static `ErrorResponse` struct is extremely unlikely to fail. Tracked as O9 in the review document.

---

## Additional Items Resolved (from full code review)

These items were identified in the broader code review and resolved across PRs #191, #192, and #193:

### RED Flags (all resolved)
- **R3**: `expandEnvVars` infinite loop → single-pass with `searchFrom` offset (`47675c9`)
- **R4**: `io.LimitReader` silent truncation → read maxBodySize+1, check overflow (`47675c9`)
- **R5**: Negative `healthcheck_interval` panic → validation in `UnmarshalCaddyfile` (`47675c9`)
- **R6**: `TransformResponse` infinite retry loop → increment attemptCount on error (`cd06caf`)
- **R7**: `buildOpenAIToolDelta` sends full ID/name on every delta → empty strings for continuations (`127e78f`)
- **R8**: Dead-letter error path with tool_use → guard checks toolCalls (`fc5ce10`)

### ORANGE Flags (all resolved except O2/O6 above)
- **O3**: Double JSON parse → eliminated (`654928a`)
- **O4**: No context propagation → threaded through IStreamingHTTPClient (`308c3ab`)
- **O5**: Missing JSON tags on Tier → added (`1994c3f`)
- **O8**: Cleanup nil logger panic → nil guard (`0d992af`)
- **O10**: `w.Write()` errors ignored → logged (`d05b761`)
- **O11**: Health check goroutines unbounded → atomic skip guard (`0d992af`)
- **O12**: Double-close respBody → sync.Once (`0d992af`)
- **O14**: No excess-argument checking → rejection added (`5b0dbf8`)
- **O15**: `tool_choice: "none"` maps wrong → returns nil, caller omits tools (`0f27a3d`)
- **O16**: Unrecognized tool_choice shapes dropped → passed through (`566eb59`)
- **O17**: Unnecessary `Header.Clone()` → removed (`a63ce53`)
- **O18**: Confusable Content/Text fields → clarifying comments (`417e92c`)

### YELLOW Flags (all resolved)
- **Y1**: `machine_id` unbounded cardinality → removed (`c8b14e6`)
- **Y2**: Env vars expanded at parse time → documented (`9afbec8`)
- **Y3**: Only text content blocks handled → error on unsupported types (`34398ce`)
- **Y4**: Multiple system messages last-one-wins → concatenated (`34398ce`)
- **Y5**: Overlapping MarkHealthy/MarkPingSuccess → removed public methods (`15c3fdb`)
- **Y6**: Flat 120s timeout → documented, deferred to separate PR (`9afbec8`)
- **Y7**: Health checks not cancellable → cancellable context (`9f11b6b`)
- **Y8**: Request failures don't update health → MarkPingWarning on errors (`b9706d6`)
- **Y9**: Anthropic drops temperature/top_p/stop → mapped (`e3f13ad`)
- **Y10**: Anthropic health check uses 4096 tokens → always set max_tokens:1 first (`9f11b6b`)
- **Y11**: Silent error on malformed message_start → return error (`e3f13ad`)
- **Y12**: No Validate() method → implemented (`47675c9`)
- **Y13**: Duplicate provider/tier names accepted → duplicate checks (`47675c9`)
- **Y14**: url.Parse accepts garbage URLs → scheme/host validation (`47675c9`)
- **Y15**: Unknown AdapterType falls through → validation added (`47675c9`)

### Test Coverage (all resolved)
- **T2**: Streaming failover unit test (`5c788f6`)
- **T8**: Health threshold boundary (`5e82033`)
- **T9**: TransformResponse infinite loop regression
- **T10**: Multi-turn tool conversation (`c27da23`)
- **T11**: Concurrent tool call streaming (`c27da23`)
- **T12**: Non-streaming 503 retry (`344f979`)
- **T13**: tool_choice string mappings (`beb8bf5`)
- **T14**: mapAnthropicStopReason tool_use (`c27da23`)
- **T15**: Streaming 503 retry (`344f979`)
- **T16**: content_block_start text type (`c27da23`)
- **T17**: Retry-After HTTP-date middleware (`cb04dc2`)
- **T18**: parseBase64DataURL edge cases (`7fc5d18`)
- **T19**: extractSystemText array content (`7fc5d18`)
- **T20**: parseRetryAfter negative seconds (`864216c`)

### Partially Addressed (tracked for future work)
- **T1**: Caddyfile parsing tests — error paths covered (`47675c9`), full happy-path still needed
- **T3**: Provision/Validate tests — Validate tests added (`47675c9`), Provision tests still needed
- **T4**: Weak metric assertions — metrics nil-safe (`3b2771b`), but no `testutil.ToFloat64()` assertions yet
- **T5**: Health check recovery through checkProvider — not yet tested at integration level
- **T6**: X-DIN-Dynamic header parsing — untested
