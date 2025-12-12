# Pricing Models

## Overview

DIN supports three pricing models that providers can offer to consumers. Each model serves different use cases and has different implementation requirements.

## Model 1: Unlimited (Fixed Monthly)

### Description

Consumer pays a fixed monthly fee for unlimited requests during the subscription period. No tracking of individual requests required.

### Use Cases

- Enterprise applications with heavy, consistent usage
- Consumers who need predictable monthly costs
- High-volume batch processing

### Contract Data Structure

```solidity
struct UnlimitedPlan {
    uint256 planId;
    address provider;
    string networkName;          // e.g., "ethereum-mainnet"
    uint256 monthlyPrice;        // in USDC (6 decimals)
    uint256 minDurationDays;     // minimum commitment
    bool active;
}

struct UnlimitedAgreement {
    uint256 agreementId;
    uint256 planId;
    address consumer;
    address provider;
    uint256 startTime;
    uint256 endTime;
    uint256 paidAmount;
    AgreementStatus status;
}
```

### Provider Implementation

```go
type UnlimitedVerifier struct {
    agreementCache map[string]*Agreement  // consumer address -> agreement
    cacheTTL       time.Duration
}

func (v *UnlimitedVerifier) Verify(consumerAddr string) (bool, error) {
    agreement, exists := v.agreementCache[consumerAddr]
    if !exists {
        // Fetch from chain
        agreement = v.fetchFromChain(consumerAddr)
        v.agreementCache[consumerAddr] = agreement
    }

    // Simple check: is agreement active and not expired?
    return agreement.Status == Active &&
           agreement.EndTime > time.Now().Unix(), nil
}
```

### Pricing Examples

| Network | Monthly Price | Min Duration |
|---------|---------------|--------------|
| Ethereum Mainnet | $500 | 30 days |
| Polygon | $200 | 30 days |
| Arbitrum | $300 | 30 days |

---

## Model 2: Request Count

### Description

Consumer purchases a fixed number of requests (e.g., 1 million). Each request decrements the counter by 1, regardless of method type.

### Use Cases

- Light or variable usage patterns
- Testing and development
- Cost-conscious consumers
- Simple, predictable per-request pricing

### Contract Data Structure

```solidity
struct RequestCountPlan {
    uint256 planId;
    address provider;
    string networkName;
    uint256 pricePerMillion;     // USDC per 1M requests
    uint256 minPurchase;         // minimum requests to buy
    uint256 validityDays;        // how long credits last
    bool active;
}

struct RequestCountAgreement {
    uint256 agreementId;
    uint256 planId;
    address consumer;
    address provider;
    uint256 totalRequests;       // purchased amount
    uint256 usedRequests;        // consumed amount
    uint256 expiresAt;
    AgreementStatus status;
}
```

### Provider Implementation

```go
type RequestCountVerifier struct {
    agreementCache map[string]*RequestCountAgreement
    usageBuffer    map[string]uint64  // buffered usage to batch update
    flushInterval  time.Duration
}

func (v *RequestCountVerifier) Verify(consumerAddr string) (bool, error) {
    agreement := v.getAgreement(consumerAddr)
    if agreement == nil {
        return false, nil
    }

    // Check: has remaining requests and not expired
    remaining := agreement.TotalRequests - agreement.UsedRequests
    buffered := v.usageBuffer[consumerAddr]

    return remaining > buffered &&
           agreement.ExpiresAt > time.Now().Unix(), nil
}

func (v *RequestCountVerifier) RecordUsage(consumerAddr string) {
    // Optimistic: increment local buffer (async)
    v.usageBuffer[consumerAddr]++
}

func (v *RequestCountVerifier) FlushUsage() {
    // Periodically sync to chain
    for addr, count := range v.usageBuffer {
        v.updateOnChain(addr, count)
        delete(v.usageBuffer, addr)
    }
}
```

### Optimistic Deduction Flow

```
Request arrives
      │
      ▼
┌─────────────────────────────────────────────┐
│  Check: remainingRequests > bufferedUsage?  │
│  (Single atomic read from cache)            │
└─────────────────┬───────────────────────────┘
                  │
        ┌─────────┴─────────┐
        │ Yes               │ No
        ▼                   ▼
   Serve request       Return 402
        │
        ▼
   Increment buffer (async, non-blocking)
        │
        ▼
   Periodic flush to chain (batch update)
```

### Pricing Examples

| Network | Price per 1M | Min Purchase | Validity |
|---------|--------------|--------------|----------|
| Ethereum Mainnet | $50 | 100,000 | 90 days |
| Polygon | $20 | 100,000 | 90 days |
| Arbitrum | $30 | 100,000 | 90 days |

---

## Model 3: Credit-Based (Method Pricing)

### Description

Consumer deposits credits. Each method has a different credit cost, allowing fine-grained pricing based on computational complexity.

### Use Cases

- APIs with varying method costs (simple reads vs complex traces)
- Enterprise customers with specific method needs
- Providers who want to price expensive methods differently

### Contract Data Structure

```solidity
struct CreditPlan {
    uint256 planId;
    address provider;
    string networkName;
    uint256 creditPrice;         // USDC per credit
    uint256 minDeposit;          // minimum credits to purchase
    uint256 validityDays;
    mapping(string => uint256) methodCredits;  // method -> credits
    uint256 defaultCredits;      // for unlisted methods
    bool active;
}

struct CreditAgreement {
    uint256 agreementId;
    uint256 planId;
    address consumer;
    address provider;
    uint256 totalCredits;
    uint256 usedCredits;
    uint256 expiresAt;
    AgreementStatus status;
}
```

### Method Credit Examples

| Method | Credits | Rationale |
|--------|---------|-----------|
| `eth_blockNumber` | 1 | Simple, cached |
| `eth_getBalance` | 1 | Simple state read |
| `eth_call` | 2 | State execution |
| `eth_getLogs` | 5 | Index scan |
| `eth_getTransactionReceipt` | 2 | Single lookup |
| `debug_traceCall` | 50 | Heavy computation |
| `debug_traceTransaction` | 100 | Very heavy |

### Provider Implementation

```go
type CreditVerifier struct {
    agreementCache map[string]*CreditAgreement
    methodCredits  map[string]uint64  // method -> credits
    defaultCredits uint64
    usageBuffer    map[string]uint64  // consumer -> pending credits
}

func (v *CreditVerifier) Verify(consumerAddr string, method string) (bool, error) {
    agreement := v.getAgreement(consumerAddr)
    if agreement == nil {
        return false, nil
    }

    // Get method cost
    cost := v.getMethodCost(method)

    // Check: has enough credits
    remaining := agreement.TotalCredits - agreement.UsedCredits
    buffered := v.usageBuffer[consumerAddr]

    return remaining > buffered + cost &&
           agreement.ExpiresAt > time.Now().Unix(), nil
}

func (v *CreditVerifier) getMethodCost(method string) uint64 {
    if cost, exists := v.methodCredits[method]; exists {
        return cost
    }
    return v.defaultCredits
}

func (v *CreditVerifier) RecordUsage(consumerAddr string, method string) {
    cost := v.getMethodCost(method)
    v.usageBuffer[consumerAddr] += cost
}
```

### Pricing Examples

| Network | Credit Price | Default Cost | Min Deposit |
|---------|--------------|--------------|-------------|
| Ethereum Mainnet | $0.0001 | 1 credit | 10,000 |
| Polygon | $0.00005 | 1 credit | 10,000 |
| Arbitrum | $0.00008 | 1 credit | 10,000 |

---

## Model Comparison

| Aspect | Unlimited | Request Count | Credit-Based |
|--------|-----------|---------------|--------------|
| Pricing | Fixed monthly | Per-request | Per-credit |
| Tracking | None | Simple counter | Method-aware |
| Provider complexity | Low | Medium | High |
| Consumer predictability | High | Medium | Low |
| Best for | Heavy users | Light users | Mixed usage |

## Provider Configuration

Providers define their plans in JSON format:

```json
{
  "provider": "0xProviderAddress",
  "network": "ethereum-mainnet",
  "plans": [
    {
      "type": "unlimited",
      "name": "Enterprise Unlimited",
      "monthlyPrice": 500000000,
      "minDurationDays": 30
    },
    {
      "type": "request_count",
      "name": "Pay-per-Request",
      "pricePerMillion": 50000000,
      "minPurchase": 100000,
      "validityDays": 90
    },
    {
      "type": "credit",
      "name": "Credit-Based",
      "creditPrice": 100,
      "minDeposit": 10000,
      "validityDays": 90,
      "methodCredits": {
        "eth_call": 2,
        "eth_getLogs": 5,
        "debug_traceCall": 50
      },
      "defaultCredits": 1
    }
  ]
}
```

## Rate Limiting

For usage-based models, providers should implement rate limiting when balance is low:

```go
func (v *Verifier) CheckRateLimit(consumerAddr string) RateLimitStatus {
    remaining := v.getRemainingBalance(consumerAddr)

    switch {
    case remaining <= 0:
        return RateLimitBlocked  // Return 402
    case remaining < v.lowBalanceThreshold:
        return RateLimitWarning  // Add warning header
    default:
        return RateLimitOK
    }
}
```

Warning header example:
```http
X-DIN-Balance-Warning: remaining=1000,threshold=5000
```
