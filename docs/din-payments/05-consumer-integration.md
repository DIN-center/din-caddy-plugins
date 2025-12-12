# Consumer Integration Guide

## Overview

This guide covers how consumers (SDK developers, router operators) integrate with the DIN Payments system to browse plans, create agreements, and make requests.

## SDK Integration

### Basic Usage

```typescript
import { DinClient } from '@din-center/router';

// Initialize client with wallet for payments
const din = new DinClient({
  privateKey: process.env.PRIVATE_KEY,  // For x402 payments
});

// Browse plans
const plans = await din.marketplace.getPlans('ethereum-mainnet');

// Subscribe to a plan
const agreement = await din.marketplace.subscribe({
  planId: plans[0].id,
  quantity: 1,  // 1 month for unlimited, 1M requests for request-count
});

// Make requests (zero latency if subscribed)
const response = await din.request('ethereum-mainnet', {
  body: { jsonrpc: '2.0', method: 'eth_blockNumber', params: [], id: 1 },
});

console.log('Result:', response.data.result);
console.log('Provider:', response.provider.name);
```

## Marketplace Client

### Client Structure

```typescript
export class MarketplaceClient {
  private contract: DINMarketplace;
  private signer: Signer;
  private cache: PlanCache;

  constructor(config: MarketplaceConfig) {
    this.contract = new Contract(
      config.contractAddress,
      DINMarketplaceABI,
      config.provider
    );
    this.signer = new Wallet(config.privateKey, config.provider);
    this.cache = new PlanCache();
  }

  // ... methods below
}
```

### Browse Plans

```typescript
interface Plan {
  id: bigint;
  provider: string;
  providerName: string;
  networkName: string;
  planType: 'unlimited' | 'request_count' | 'credit_based';
  price: bigint;           // USDC (6 decimals)
  units: bigint;           // Requests or credits per purchase
  validityDays: number;
  active: boolean;
  methodCosts?: Record<string, number>;  // For credit-based
}

async getPlans(
  networkName: string,
  options?: {
    provider?: string;
    planType?: PlanType;
    includeInactive?: boolean;
  }
): Promise<Plan[]> {
  // Get all plan IDs for network
  const planIds = await this.contract.getNetworkPlans(networkName);

  const plans: Plan[] = [];

  for (const planId of planIds) {
    const plan = await this.contract.plans(planId);

    // Apply filters
    if (!options?.includeInactive && !plan.active) continue;
    if (options?.provider && plan.provider !== options.provider) continue;
    if (options?.planType && plan.planType !== options.planType) continue;

    // Get provider name from registry
    const providerName = await this.getProviderName(plan.provider);

    // Get method costs for credit-based plans
    let methodCosts: Record<string, number> | undefined;
    if (plan.planType === PlanType.CreditBased) {
      methodCosts = await this.getMethodCosts(planId);
    }

    plans.push({
      id: planId,
      provider: plan.provider,
      providerName,
      networkName: plan.networkName,
      planType: this.mapPlanType(plan.planType),
      price: plan.price,
      units: plan.units,
      validityDays: Number(plan.validityDays),
      active: plan.active,
      methodCosts,
    });
  }

  return plans;
}

// Helper to get method costs for credit-based plans
private async getMethodCosts(planId: bigint): Promise<Record<string, number>> {
  // Common methods to check
  const methods = [
    'eth_call',
    'eth_getBalance',
    'eth_getBlockByNumber',
    'eth_getTransactionByHash',
    'eth_getTransactionReceipt',
    'eth_getLogs',
    'eth_sendRawTransaction',
    'debug_traceCall',
    'debug_traceTransaction',
  ];

  const costs: Record<string, number> = {};

  for (const method of methods) {
    const cost = await this.contract.methodCosts(planId, method);
    if (cost > 0) {
      costs[method] = Number(cost);
    }
  }

  // Get default cost
  const defaultCost = await this.contract.defaultMethodCost(planId);
  costs['_default'] = Number(defaultCost);

  return costs;
}
```

### Create Agreement (Subscribe)

```typescript
interface SubscribeOptions {
  planId: bigint;
  quantity: number;  // Months for unlimited, units for usage-based
}

interface Agreement {
  id: bigint;
  planId: bigint;
  consumer: string;
  provider: string;
  startTime: Date;
  endTime: Date;
  paidAmount: bigint;
  totalUnits: bigint;
  usedUnits: bigint;
  status: 'active' | 'expired' | 'cancelled' | 'exhausted';
}

async subscribe(options: SubscribeOptions): Promise<Agreement> {
  // Get plan details
  const plan = await this.contract.plans(options.planId);
  if (!plan.active) {
    throw new Error('Plan is not active');
  }

  // Calculate payment amount
  const paymentAmount = plan.price * BigInt(options.quantity);

  // Check USDC allowance
  const allowance = await this.usdc.allowance(
    await this.signer.getAddress(),
    this.contract.address
  );

  if (allowance < paymentAmount) {
    // Approve USDC spending
    const approveTx = await this.usdc.connect(this.signer).approve(
      this.contract.address,
      paymentAmount
    );
    await approveTx.wait();
  }

  // Create agreement
  const tx = await this.contract.connect(this.signer).createAgreement(
    options.planId,
    options.quantity
  );

  const receipt = await tx.wait();

  // Extract agreement ID from event
  const event = receipt.events?.find(e => e.event === 'AgreementCreated');
  const agreementId = event?.args?.agreementId;

  // Fetch and return agreement
  return this.getAgreement(agreementId);
}
```

### Check Agreement Status

```typescript
async getAgreement(agreementId: bigint): Promise<Agreement> {
  const agreement = await this.contract.agreements(agreementId);

  return {
    id: agreementId,
    planId: agreement.planId,
    consumer: agreement.consumer,
    provider: agreement.provider,
    startTime: new Date(Number(agreement.startTime) * 1000),
    endTime: new Date(Number(agreement.endTime) * 1000),
    paidAmount: agreement.paidAmount,
    totalUnits: agreement.totalUnits,
    usedUnits: agreement.usedUnits,
    status: this.mapStatus(agreement.status),
  };
}

async getMyAgreements(options?: {
  networkName?: string;
  status?: 'active' | 'all';
}): Promise<Agreement[]> {
  const consumerAddr = await this.signer.getAddress();
  const agreementIds = await this.contract.getConsumerAgreements(consumerAddr);

  const agreements: Agreement[] = [];

  for (const id of agreementIds) {
    const agreement = await this.getAgreement(id);

    // Apply filters
    if (options?.status === 'active' && agreement.status !== 'active') continue;
    if (options?.networkName) {
      const plan = await this.contract.plans(agreement.planId);
      if (plan.networkName !== options.networkName) continue;
    }

    agreements.push(agreement);
  }

  return agreements;
}
```

### Monitor Usage

```typescript
interface UsageInfo {
  agreement: Agreement;
  remaining: bigint;
  percentUsed: number;
  estimatedDaysRemaining?: number;
}

async getUsage(agreementId: bigint): Promise<UsageInfo> {
  const agreement = await this.getAgreement(agreementId);
  const plan = await this.contract.plans(agreement.planId);

  if (plan.planType === PlanType.Unlimited) {
    return {
      agreement,
      remaining: BigInt(Number.MAX_SAFE_INTEGER),
      percentUsed: 0,
    };
  }

  const remaining = agreement.totalUnits - agreement.usedUnits;
  const percentUsed = Number(agreement.usedUnits * 100n / agreement.totalUnits);

  // Estimate days remaining based on usage rate
  const daysSinceStart = Math.max(1,
    (Date.now() - agreement.startTime.getTime()) / (1000 * 60 * 60 * 24)
  );
  const usagePerDay = Number(agreement.usedUnits) / daysSinceStart;
  const estimatedDaysRemaining = usagePerDay > 0
    ? Math.floor(Number(remaining) / usagePerDay)
    : undefined;

  return {
    agreement,
    remaining,
    percentUsed,
    estimatedDaysRemaining,
  };
}
```

## Request Flow

### With Active Agreement

```typescript
async request(
  networkName: string,
  options: RequestOptions
): Promise<DinResponse> {
  // Check for active agreement
  const agreement = await this.findActiveAgreement(networkName);

  if (agreement) {
    // Use agreement - no 402 negotiation needed
    const provider = this.selectProvider(networkName, agreement.provider);

    const response = await this.x402Client.makeRequest(provider.serviceUrl, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-DIN-Agreement': agreement.id.toString(),
      },
      body: JSON.stringify(options.body),
    });

    return {
      data: response.data,
      status: response.status,
      headers: response.headers,
      provider: { name: provider.name, url: provider.serviceUrl },
      agreement: {
        id: agreement.id,
        remaining: await this.getRemainingUnits(agreement.id),
      },
    };
  }

  // No agreement - use x402 micropayments
  return this.requestWithMicropayment(networkName, options);
}

private async findActiveAgreement(networkName: string): Promise<Agreement | null> {
  const agreements = await this.getMyAgreements({
    networkName,
    status: 'active',
  });

  return agreements[0] ?? null;
}
```

### Response Types

```typescript
interface DinResponse<T = any> {
  data: T;
  status: number;
  headers: Record<string, string>;
  provider: {
    name: string;
    url: string;
  };
  // Present if using micropayments
  paymentInfo?: {
    amount: string;
    txHash?: string;
  };
  // Present if using agreement
  agreement?: {
    id: bigint;
    remaining: bigint;
  };
}
```

## Low Balance Handling

### Monitor and Alert

```typescript
class UsageMonitor {
  private thresholds = {
    warning: 20,   // 20% remaining
    critical: 5,   // 5% remaining
  };

  async checkUsage(agreementId: bigint): Promise<UsageAlert | null> {
    const usage = await this.marketplace.getUsage(agreementId);
    const percentRemaining = 100 - usage.percentUsed;

    if (percentRemaining <= this.thresholds.critical) {
      return {
        level: 'critical',
        message: `Critical: Only ${percentRemaining}% remaining`,
        remaining: usage.remaining,
        action: 'Purchase more credits immediately',
      };
    }

    if (percentRemaining <= this.thresholds.warning) {
      return {
        level: 'warning',
        message: `Warning: ${percentRemaining}% remaining`,
        remaining: usage.remaining,
        estimatedDaysRemaining: usage.estimatedDaysRemaining,
        action: 'Consider purchasing more credits',
      };
    }

    return null;
  }
}
```

### Auto-Renewal

```typescript
class AutoRenewer {
  private renewalThreshold = 10;  // Renew when 10% remaining

  async checkAndRenew(agreementId: bigint): Promise<Agreement | null> {
    const usage = await this.marketplace.getUsage(agreementId);
    const percentRemaining = 100 - usage.percentUsed;

    if (percentRemaining > this.renewalThreshold) {
      return null;  // No renewal needed
    }

    // Get current plan
    const agreement = usage.agreement;
    const plan = await this.marketplace.getPlan(agreement.planId);

    // Check wallet balance
    const balance = await this.usdc.balanceOf(this.walletAddress);
    if (balance < plan.price) {
      console.warn('Insufficient USDC for renewal');
      return null;
    }

    // Create new agreement
    return this.marketplace.subscribe({
      planId: agreement.planId,
      quantity: 1,
    });
  }
}
```

## Router Integration

For DIN Router (Go/Caddy), similar patterns apply:

### Configuration

```go
type MarketplaceConfig struct {
    ContractAddress string        `json:"contract_address"`
    PrivateKey      string        `json:"private_key"`
    CheckInterval   time.Duration `json:"check_interval"`
    AutoRenew       bool          `json:"auto_renew"`
    RenewalThreshold float64      `json:"renewal_threshold"`  // 0.1 = 10%
}
```

### Agreement Manager

```go
type AgreementManager struct {
    config       MarketplaceConfig
    contract     *DINMarketplace
    agreements   map[string]*Agreement  // network -> active agreement
    mu           sync.RWMutex
}

func (m *AgreementManager) GetActiveAgreement(network string) (*Agreement, bool) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    agreement, exists := m.agreements[network]
    if !exists || !agreement.IsActive() {
        return nil, false
    }
    return agreement, true
}

func (m *AgreementManager) StartMonitoring(ctx context.Context) {
    ticker := time.NewTicker(m.config.CheckInterval)

    go func() {
        for {
            select {
            case <-ticker.C:
                m.checkAndRenew()
            case <-ctx.Done():
                ticker.Stop()
                return
            }
        }
    }()
}
```

## Error Handling

### Common Errors

```typescript
class AgreementError extends Error {
  constructor(
    message: string,
    public code: string,
    public agreementId?: bigint
  ) {
    super(message);
  }
}

// Error codes
const ErrorCodes = {
  PLAN_NOT_ACTIVE: 'PLAN_NOT_ACTIVE',
  INSUFFICIENT_FUNDS: 'INSUFFICIENT_FUNDS',
  AGREEMENT_EXPIRED: 'AGREEMENT_EXPIRED',
  AGREEMENT_EXHAUSTED: 'AGREEMENT_EXHAUSTED',
  NO_ACTIVE_AGREEMENT: 'NO_ACTIVE_AGREEMENT',
};
```

### Handling Provider Responses

```typescript
async handleResponse(response: Response): Promise<DinResponse> {
  // Check for balance warning
  const balanceWarning = response.headers.get('X-DIN-Balance-Warning');
  if (balanceWarning) {
    this.emit('balance-warning', parseBalanceWarning(balanceWarning));
  }

  // Check for agreement errors
  const dinError = response.headers.get('X-DIN-Error');
  if (dinError) {
    switch (dinError) {
      case 'agreement_expired':
        throw new AgreementError(
          'Agreement has expired',
          ErrorCodes.AGREEMENT_EXPIRED
        );
      case 'insufficient_balance':
        throw new AgreementError(
          'Insufficient balance',
          ErrorCodes.AGREEMENT_EXHAUSTED
        );
    }
  }

  return this.parseResponse(response);
}
```
