# DIN Router SDK (TypeScript)

A TypeScript SDK for dynamic routing to DIN network providers with **mandatory x402 micropayments**.

## What This SDK Does

The SDK handles the complete request flow for accessing blockchain RPC providers through the DIN network:

```
User Request → Provider Selection → x402 Payment → Request Forwarding → Response
```

**You provide:** The network name and request payload
**SDK handles:** Provider selection, payment, and request execution

## Quick Start

```typescript
import { DinClient } from '@din-center/router';

const din = new DinClient({
  privateKey: process.env.PRIVATE_KEY!,
});

// Make a request to any DIN network
const response = await din.request('ethereum-mainnet', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: {
    jsonrpc: '2.0',
    method: 'eth_blockNumber',
    params: [],
    id: 1,
  },
});

console.log('Block:', parseInt(response.data.result, 16));
console.log('Provider:', response.provider.name);
console.log('Payment:', response.paymentInfo?.amount);
```

## Key Features

| Feature | Description |
|---------|-------------|
| **Network-agnostic** | Works with any blockchain (EVM, Solana, Bitcoin, etc.) |
| **Automatic payments** | x402 micropayments handled transparently |
| **Smart routing** | Weighted selection based on provider quality scores |
| **Session affinity** | Consistent provider routing when needed |
| **Auto-sync** | Registry and scores refresh automatically |

## Documentation

| Document | Description |
|----------|-------------|
| [Architecture](./01-architecture.md) | System components and data flow |
| [Registry Sync](./02-registry-sync.md) | How registry data is fetched and cached |
| [Provider Selection](./03-provider-selection.md) | Selection algorithm details |
| [x402 Payments](./04-x402-payments.md) | Payment integration and flow |
| [API Reference](./05-api-reference.md) | Types and method signatures |
| [Usage Examples](./06-usage-examples.md) | Code examples for common scenarios |
| [Implementation Guide](./07-implementation-guide.md) | Build order and reference files |

## Dependencies

```json
{
  "dependencies": {
    "@din-center/registry": "^0.1.0",
    "x402-axios": "^0.1.0",
    "axios": "^1.6.0",
    "viem": "^2.0.0"
  }
}
```

## Design Principles

1. **SDK makes requests** - User provides payload, SDK handles everything else
2. **x402 is mandatory** - All requests include micropayments to providers
3. **Network-agnostic** - SDK forwards payloads as-is, no protocol parsing
4. **Protocol-agnostic** - Works with JSON-RPC, REST, or any HTTP API
5. **Automatic lifecycle** - Client auto-starts on first request if needed
