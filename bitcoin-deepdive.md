# Bitcoin Esplora: A Case Study in Architectural Friction

This document analyzes the git history around Bitcoin Esplora support to understand why it caused significant churn, and whether that friction was inherent to the API's complexity or a symptom of missing abstractions.

## Timeline of Esplora Churn

```
Jul 30  772ae21  "added basic bitcoin esplora controls" (first impl, 447 lines)
Jul 31  900d9b0  "refactored for rpc abstraction"
Aug 1   7890c00  "addressed latest PR comments"
Aug 5   54108d0  "finalize bitcoin esplora"
Aug 5   b74b5bc  "finalize bitcoin esplora" (duplicate commit!)
Aug 5   ce1900e  "Universal Blockchain API Gateway Support (#122)" - MASSIVE rewrite
Aug 19  75894a8  "validation cloud esplora support and refresh logic updated"
Aug 20  a9296cd  "Feat/support bitcoin esplora (#126)" - ANOTHER major PR
Sep 17  2b13dab  "Allow post requests in esplora" - BUGFIX: POST was blocked!
Nov 12  eeb5c6c  "Don't include path for metrics method in esplora" - BUGFIX
Nov 17  4d620c8  "added bitcoin esplora method handlers for reduced cardinality"
Dec 12  4811e2f  "Refactor query param handling into ConfigureRequestPath"
```

Key observations:
- 6 commits just to get initial implementation working (Jul 30 - Aug 5)
- Two "finalize" commits on the same day
- A "Universal Blockchain API Gateway" PR that was supposed to abstract handler differences
- Then ANOTHER Esplora-specific PR after the "Universal" one
- Bugfixes trickling in for months afterward

## The Actual Problem

**It wasn't Esplora's complexity. It was missing abstractions.**

### 1. No RequestType Abstraction

The middleware had hardcoded assumptions about request bodies:

```go
// din_http.go - assumed all handlers use JSON-RPC
if len(bodyBytes) == 0 {
    return fmt.Errorf("request body is empty")
}
```

This is correct for JSON-RPC (empty body = invalid request), but REST GET requests legitimately have empty bodies. Instead of abstracting "what does an empty body mean for this handler?", exceptions were bolted on.

### 2. Wrong Assumption About REST

The initial Esplora handler explicitly blocked POST requests:

```go
// From commit a9296cd - bitcoin_esplora_handler.go
func (h *BitcoinEsploraHandler) ValidateRequest(req *http.Request) error {
    if req.Method == "POST" {
        return &HTTPError{
            StatusCode: http.StatusMethodNotAllowed,
            Message:    "POST method not allowed for Bitcoin Esplora API",
        }
    }
    // ...
}
```

But Esplora's `/tx` endpoint uses POST for broadcasting transactions. This assumption had to be reverted a month later in commit 2b13dab:

```go
// The fix - just remove the validation entirely
func (h *BitcoinEsploraHandler) ValidateRequest(req *http.Request) error {
    // For now we let everything through
    return nil
}
```

### 3. Path Prefix Confusion

The health check endpoint changed between implementations:

- First version: `/api/blocks/tip/height` (with `/api/` prefix)
- Later version: `/blocks/tip/height` (without prefix)

Different Esplora providers (Blockstream, Mempool.space, self-hosted) use different URL structures. Without a clean path configuration interface, this was handled ad-hoc.

### 4. The "Universal" Abstraction That Wasn't

The "Universal Blockchain API Gateway Support (#122)" PR was supposed to create a clean abstraction for all handler types. But it was immediately followed by ANOTHER Esplora-specific PR (#126), suggesting the abstraction didn't actually handle REST vs RPC properly.

## What Good Architecture Would Have Prevented

With capability interfaces from the start:

```go
type Handler interface {
    GetRequestType() RequestType           // REST vs RPC - explicit from day 1
    GetHealthCheckHTTPMethod() string      // "GET" vs "POST" - handler decides
    CreateHealthCheckPayload() ([]byte, error)  // nil for REST GET, JSON for RPC
}

type RequestType int
const (
    RequestTypeRPC RequestType = iota
    RequestTypeREST
    RequestTypeGraphQL
)
```

The middleware would branch on capabilities, not handler types:

```go
// din_http.go - capability-based branching
if len(bodyBytes) == 0 && handler.GetRequestType() == RequestTypeRPC {
    // Only reject empty body for JSON-RPC handlers
    return fmt.Errorf("request body is empty")
}
// REST handlers with empty body pass through fine
```

This is exactly what the capability interfaces in `lib/network/capabilities.go` now provide:

```go
// Handler declares its protocol
func (h *BitcoinEsploraHandler) GetRequestType() RequestType {
    return RequestTypeREST
}

func (h *BitcoinEsploraHandler) GetHealthCheckHTTPMethod() string {
    return "GET"  // Not POST like JSON-RPC handlers
}

func (h *BitcoinEsploraHandler) CreateHealthCheckPayload(method string) ([]byte, error) {
    return nil, nil  // REST GET has no body
}
```

## Conclusion

The friction came from **retrofitting REST onto a JSON-RPC-assumed architecture**. Each fix was a patch on top of patches rather than fixing the underlying abstraction.

| What Happened | What Should Have Happened |
|---------------|---------------------------|
| Empty body = error (hardcoded) | Handler declares what empty body means |
| POST blocked for "REST" handler | Handler declares allowed methods |
| Path prefixes handled ad-hoc | Handler configures its own paths |
| Handler-type conditionals everywhere | Capability interfaces with type assertions |

Bitcoin Esplora introduced REST support, which is a legitimate new capability. But REST vs RPC is a well-understood distinction that should have been abstracted from day 1. The churn wasn't because Esplora is complex; it was because the architecture forced special-casing instead of composition.

**The lesson**: When adding a new "type" of thing (REST handler alongside RPC handlers), the right response is to abstract the difference, not to add conditionals for the new type. Interface segregation (small, focused interfaces that handlers opt into) prevents the need to modify existing code when new handler types arrive.
