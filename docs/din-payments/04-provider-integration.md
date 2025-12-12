# Provider Integration Guide

## Overview

This guide covers how RPC providers integrate with the DIN Payments system to offer subscription plans and verify consumer agreements.

## Integration Components

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Provider Stack                                     │
│                                                                              │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                      Plan Manager                                    │   │
│   │   - Register plans in marketplace contract                           │   │
│   │   - Configure method costs (for credit-based)                        │   │
│   │   - Monitor plan performance                                         │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                    Agreement Verifier                                │   │
│   │   - Cache active agreements locally                                  │   │
│   │   - Verify on each request (from cache)                              │   │
│   │   - Sync with chain periodically                                     │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                     Usage Tracker                                    │   │
│   │   - Buffer usage locally (optimistic)                                │   │
│   │   - Batch sync to chain periodically                                 │   │
│   │   - Handle low-balance warnings                                      │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                     x402 Handler                                     │   │
│   │   - Handle consumers without agreements                              │   │
│   │   - Return 402 with payment requirements                             │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Step 1: Register Plans

### Plan Configuration

Define your pricing plans:

```go
type PlanConfig struct {
    NetworkName    string
    PlanType       PlanType
    Name           string
    Description    string
    Price          *big.Int       // USDC (6 decimals)
    Units          uint64         // Requests or credits per unit
    ValidityDays   uint64
    MethodCredits  map[string]uint64  // For credit-based only
    DefaultCredits uint64             // For credit-based only
}

// Example plans
var ethereumPlans = []PlanConfig{
    {
        NetworkName:  "ethereum-mainnet",
        PlanType:     Unlimited,
        Name:         "Enterprise Unlimited",
        Description:  "Unlimited requests for 30 days",
        Price:        big.NewInt(500_000_000),  // $500 USDC
        ValidityDays: 30,
    },
    {
        NetworkName:  "ethereum-mainnet",
        PlanType:     RequestCount,
        Name:         "Pay-per-Million",
        Description:  "1M requests",
        Price:        big.NewInt(50_000_000),   // $50 USDC
        Units:        1_000_000,
        ValidityDays: 90,
    },
    {
        NetworkName:   "ethereum-mainnet",
        PlanType:      CreditBased,
        Name:          "Credit-Based",
        Description:   "Method-specific pricing",
        Price:         big.NewInt(100),         // $0.0001 per credit
        Units:         1,                       // 1 credit per unit
        ValidityDays:  90,
        DefaultCredits: 1,
        MethodCredits: map[string]uint64{
            "eth_call":           2,
            "eth_getLogs":        5,
            "debug_traceCall":    50,
            "debug_traceTransaction": 100,
        },
    },
}
```

### Contract Interaction

```go
type PlanManager struct {
    contract    *DINMarketplace
    providerKey *ecdsa.PrivateKey
}

func (m *PlanManager) RegisterPlan(ctx context.Context, config PlanConfig) (uint64, error) {
    // Register base plan
    tx, err := m.contract.RegisterPlan(
        &bind.TransactOpts{
            Context: ctx,
            Signer:  m.signer,
        },
        config.NetworkName,
        uint8(config.PlanType),
        config.Price,
        config.Units,
        config.ValidityDays,
    )
    if err != nil {
        return 0, err
    }

    receipt, err := bind.WaitMined(ctx, m.client, tx)
    if err != nil {
        return 0, err
    }

    // Extract planId from event
    planId := m.extractPlanId(receipt)

    // If credit-based, set method costs
    if config.PlanType == CreditBased && len(config.MethodCredits) > 0 {
        methods := make([]string, 0, len(config.MethodCredits))
        credits := make([]*big.Int, 0, len(config.MethodCredits))

        for method, cost := range config.MethodCredits {
            methods = append(methods, method)
            credits = append(credits, big.NewInt(int64(cost)))
        }

        _, err = m.contract.SetMethodCosts(
            &bind.TransactOpts{Context: ctx, Signer: m.signer},
            planId,
            methods,
            credits,
        )
        if err != nil {
            return planId, fmt.Errorf("failed to set method costs: %w", err)
        }
    }

    return planId, nil
}
```

## Step 2: Agreement Verification

### Local Cache

```go
type AgreementCache struct {
    mu          sync.RWMutex
    agreements  map[string]*CachedAgreement  // key -> agreement
    contract    *DINMarketplace
    syncTicker  *time.Ticker
}

type CachedAgreement struct {
    Agreement    Agreement
    LocalUsage   uint64     // Usage buffered locally
    LastSync     time.Time
    PlanType     PlanType
    MethodCosts  map[string]uint64
}

// Key format: consumer:provider:network
func makeKey(consumer, provider, network string) string {
    return fmt.Sprintf("%s:%s:%s", consumer, provider, network)
}

func (c *AgreementCache) Get(consumer, network string) (*CachedAgreement, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()

    key := makeKey(consumer, c.providerAddr, network)
    agreement, exists := c.agreements[key]
    return agreement, exists
}
```

### Sync with Chain

```go
func (c *AgreementCache) StartSync(interval time.Duration) {
    c.syncTicker = time.NewTicker(interval)

    go func() {
        for range c.syncTicker.C {
            c.syncFromChain()
        }
    }()
}

func (c *AgreementCache) syncFromChain() {
    // Get all provider agreements
    agreementIds, err := c.contract.GetProviderAgreements(nil, c.providerAddr)
    if err != nil {
        log.Printf("Failed to fetch agreements: %v", err)
        return
    }

    c.mu.Lock()
    defer c.mu.Unlock()

    for _, id := range agreementIds {
        agreement, err := c.contract.Agreements(nil, id)
        if err != nil {
            continue
        }

        // Skip inactive agreements
        if agreement.Status != Active {
            continue
        }

        plan, _ := c.contract.Plans(nil, agreement.PlanId)
        key := makeKey(
            agreement.Consumer.Hex(),
            c.providerAddr,
            plan.NetworkName,
        )

        // Update cache, preserving local usage buffer
        existing, exists := c.agreements[key]
        localUsage := uint64(0)
        if exists {
            localUsage = existing.LocalUsage
        }

        c.agreements[key] = &CachedAgreement{
            Agreement:   agreement,
            LocalUsage:  localUsage,
            LastSync:    time.Now(),
            PlanType:    PlanType(plan.PlanType),
            MethodCosts: c.loadMethodCosts(agreement.PlanId),
        }
    }
}
```

### Verification Middleware

```go
type AgreementVerifier struct {
    cache       *AgreementCache
    network     string
    x402Handler *X402Handler  // Fallback for non-subscribed
}

func (v *AgreementVerifier) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Extract consumer address from x402 session or signature
        consumer, err := v.extractConsumer(r)
        if err != nil {
            v.x402Handler.Handle402(w, r)
            return
        }

        // Check for active agreement
        agreement, exists := v.cache.Get(consumer, v.network)
        if !exists {
            // No subscription - fall back to x402 micropayments
            v.x402Handler.Handle402(w, r)
            return
        }

        // Verify agreement is valid
        if !v.isValid(agreement) {
            v.x402Handler.Handle402(w, r)
            return
        }

        // For usage-based, check remaining balance
        if agreement.PlanType != Unlimited {
            method := v.extractMethod(r)
            cost := v.getMethodCost(agreement, method)

            remaining := agreement.Agreement.TotalUnits -
                        agreement.Agreement.UsedUnits -
                        agreement.LocalUsage

            if remaining < cost {
                v.handleInsufficientBalance(w, remaining)
                return
            }
        }

        // Store agreement in context for post-request usage tracking
        ctx := context.WithValue(r.Context(), "agreement", agreement)

        // Serve the request
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

func (v *AgreementVerifier) isValid(agreement *CachedAgreement) bool {
    return agreement.Agreement.Status == Active &&
           agreement.Agreement.EndTime.Int64() > time.Now().Unix()
}
```

## Step 3: Usage Tracking

### Optimistic Deduction

```go
type UsageTracker struct {
    cache           *AgreementCache
    flushTicker     *time.Ticker
    flushBatchSize  int
}

// Called after each request (non-blocking)
func (t *UsageTracker) RecordUsage(ctx context.Context, method string) {
    agreement, ok := ctx.Value("agreement").(*CachedAgreement)
    if !ok || agreement == nil {
        return
    }

    // Skip for unlimited plans
    if agreement.PlanType == Unlimited {
        return
    }

    // Calculate cost
    cost := t.getMethodCost(agreement, method)

    // Atomic increment of local buffer
    atomic.AddUint64(&agreement.LocalUsage, cost)
}

func (t *UsageTracker) getMethodCost(agreement *CachedAgreement, method string) uint64 {
    if agreement.PlanType == RequestCount {
        return 1  // Always 1 for request count
    }

    // Credit-based: look up method cost
    if cost, ok := agreement.MethodCosts[method]; ok {
        return cost
    }
    return 1  // Default cost
}
```

### Batch Sync to Chain

```go
func (t *UsageTracker) StartFlush(interval time.Duration) {
    t.flushTicker = time.NewTicker(interval)

    go func() {
        for range t.flushTicker.C {
            t.flushUsage()
        }
    }()
}

func (t *UsageTracker) flushUsage() {
    t.cache.mu.Lock()
    defer t.cache.mu.Unlock()

    var agreementIds []*big.Int
    var usages []*big.Int

    for _, agreement := range t.cache.agreements {
        localUsage := atomic.SwapUint64(&agreement.LocalUsage, 0)
        if localUsage == 0 {
            continue
        }

        agreementIds = append(agreementIds, agreement.Agreement.Id)
        usages = append(usages, big.NewInt(int64(localUsage)))

        // Batch in chunks
        if len(agreementIds) >= t.flushBatchSize {
            t.sendBatch(agreementIds, usages)
            agreementIds = nil
            usages = nil
        }
    }

    // Send remaining
    if len(agreementIds) > 0 {
        t.sendBatch(agreementIds, usages)
    }
}

func (t *UsageTracker) sendBatch(ids, usages []*big.Int) {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    _, err := t.contract.BatchRecordUsage(
        &bind.TransactOpts{Context: ctx, Signer: t.signer},
        ids,
        usages,
    )
    if err != nil {
        log.Printf("Failed to batch record usage: %v", err)
        // TODO: Retry logic or persist locally
    }
}
```

## Step 4: x402 Fallback

For consumers without agreements, fall back to x402 micropayments:

```go
type X402Handler struct {
    paymentAddr    string
    pricePerMethod map[string]*big.Int
    defaultPrice   *big.Int
}

func (h *X402Handler) Handle402(w http.ResponseWriter, r *http.Request) {
    method := extractMethod(r)
    price := h.getPrice(method)

    requirements := X402Requirements{
        Scheme:            "exact",
        Network:           "linea",
        MaxAmountRequired: price.String(),
        Resource:          r.URL.Path,
        PayTo:             h.paymentAddr,
        MaxTimeoutSeconds: 300,
    }

    w.Header().Set("X-PAYMENT-REQUIRED", requirements.Encode())
    w.WriteHeader(http.StatusPaymentRequired)
}

func (h *X402Handler) getPrice(method string) *big.Int {
    if price, ok := h.pricePerMethod[method]; ok {
        return price
    }
    return h.defaultPrice
}
```

## Caddy Integration

### Caddyfile Configuration

```caddyfile
:8080 {
    # DIN Agreement verification
    route /* {
        din_agreement_verify {
            network ethereum-mainnet
            provider_address 0x1234...
            marketplace_contract 0xABCD...
            cache_sync_interval 30s
            usage_flush_interval 60s
        }

        # Fallback to x402 if no agreement
        din_x402 {
            payment_address 0x5678...
            default_price 1000000000000000
            method_prices {
                eth_call 1000000000000000
                eth_getLogs 5000000000000000
                debug_traceCall 50000000000000000
            }
        }

        reverse_proxy localhost:8545
    }
}
```

### Caddy Module

```go
func init() {
    caddy.RegisterModule(AgreementVerifier{})
}

func (AgreementVerifier) CaddyModule() caddy.ModuleInfo {
    return caddy.ModuleInfo{
        ID:  "http.handlers.din_agreement_verify",
        New: func() caddy.Module { return new(AgreementVerifier) },
    }
}

type AgreementVerifier struct {
    Network             string `json:"network,omitempty"`
    ProviderAddress     string `json:"provider_address,omitempty"`
    MarketplaceContract string `json:"marketplace_contract,omitempty"`
    CacheSyncInterval   string `json:"cache_sync_interval,omitempty"`
    UsageFlushInterval  string `json:"usage_flush_interval,omitempty"`

    cache   *AgreementCache
    tracker *UsageTracker
}
```

## Monitoring

### Metrics to Track

```go
var (
    agreementsActive = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "din_agreements_active",
            Help: "Number of active agreements",
        },
        []string{"plan_type", "network"},
    )

    requestsServed = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "din_requests_served_total",
            Help: "Total requests served",
        },
        []string{"agreement_type", "network"},  // "subscription" or "micropayment"
    )

    usageBufferSize = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "din_usage_buffer_size",
            Help: "Pending usage to sync to chain",
        },
        []string{"network"},
    )
)
```

## Error Handling

### Insufficient Balance

```go
func (v *AgreementVerifier) handleInsufficientBalance(w http.ResponseWriter, remaining uint64) {
    w.Header().Set("X-DIN-Balance-Remaining", fmt.Sprintf("%d", remaining))
    w.Header().Set("X-DIN-Error", "insufficient_balance")
    w.WriteHeader(http.StatusPaymentRequired)

    json.NewEncoder(w).Encode(map[string]interface{}{
        "error": "Insufficient balance",
        "remaining": remaining,
        "action": "Please purchase more credits or upgrade your plan",
    })
}
```

### Agreement Expired

```go
func (v *AgreementVerifier) handleExpired(w http.ResponseWriter, agreement *CachedAgreement) {
    w.Header().Set("X-DIN-Error", "agreement_expired")
    w.WriteHeader(http.StatusPaymentRequired)

    json.NewEncoder(w).Encode(map[string]interface{}{
        "error": "Agreement expired",
        "expired_at": agreement.Agreement.EndTime,
        "action": "Please renew your subscription",
    })
}
```
