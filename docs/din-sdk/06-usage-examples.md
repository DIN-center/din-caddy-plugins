# Usage Examples

## Basic Setup

```typescript
import { DinClient } from '@din-center/router';

// Minimal configuration - just need a private key
const din = new DinClient({
  privateKey: process.env.PRIVATE_KEY!,
});
```

## JSON-RPC Requests

### Ethereum: Get Block Number

```typescript
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
// Block: 18543210
```

### Ethereum: Call Contract

```typescript
const response = await din.request('ethereum-mainnet', {
  body: {
    jsonrpc: '2.0',
    method: 'eth_call',
    params: [
      {
        to: '0xdAC17F958D2ee523a2206206994597C13D831ec7', // USDT
        data: '0x70a08231000000000000000000000000...',    // balanceOf(address)
      },
      'latest',
    ],
    id: 1,
  },
});

console.log('Balance:', response.data.result);
```

### Ethereum: Get Transaction Receipt

```typescript
const response = await din.request('ethereum-mainnet', {
  body: {
    jsonrpc: '2.0',
    method: 'eth_getTransactionReceipt',
    params: ['0x...txHash'],
    id: 1,
  },
});

console.log('Status:', response.data.result.status);
console.log('Gas Used:', response.data.result.gasUsed);
```

## REST API Requests

### Bitcoin Esplora: Get Block Height

```typescript
const response = await din.request('bitcoin-esplora', {
  method: 'GET',
  path: '/block/tip/height',
});

console.log('Block height:', response.data);
// Block height: 820123
```

### Bitcoin Esplora: Get Transaction

```typescript
const response = await din.request('bitcoin-esplora', {
  method: 'GET',
  path: '/tx/abc123...txid',
});

console.log('Transaction:', response.data);
```

### Bitcoin Esplora: Get Address UTXOs

```typescript
const response = await din.request('bitcoin-esplora', {
  method: 'GET',
  path: '/address/bc1q.../utxo',
});

console.log('UTXOs:', response.data);
```

## Session Affinity

### Global Session (Per Client)

```typescript
// Same provider for all requests from this client
const din = new DinClient({
  privateKey: process.env.PRIVATE_KEY!,
  sessionId: 'user-wallet-0x1234...',
});

// All requests route to same provider
await din.request('ethereum-mainnet', { body: { ... } });
await din.request('ethereum-mainnet', { body: { ... } });
```

### Per-Request Session

```typescript
const din = new DinClient({
  privateKey: process.env.PRIVATE_KEY!,
});

// Group related requests to same provider
const txId = 'tx-12345';

const tx = await din.request('ethereum-mainnet', {
  body: { jsonrpc: '2.0', method: 'eth_getTransactionByHash', params: [txId], id: 1 },
  sessionId: txId,
});

const receipt = await din.request('ethereum-mainnet', {
  body: { jsonrpc: '2.0', method: 'eth_getTransactionReceipt', params: [txId], id: 2 },
  sessionId: txId,  // Same provider as above
});
```

## Custom Headers

```typescript
const response = await din.request('ethereum-mainnet', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'X-Request-ID': 'req-12345',
    'X-Custom-Header': 'custom-value',
  },
  body: {
    jsonrpc: '2.0',
    method: 'eth_blockNumber',
    params: [],
    id: 1,
  },
});
```

## Lifecycle Management

### Explicit Start/Stop

```typescript
const din = new DinClient({
  privateKey: process.env.PRIVATE_KEY!,
  registrySyncIntervalMs: 60000,  // 60s
  scoreSyncIntervalMs: 30000,     // 30s
});

// Explicitly start (performs initial sync)
await din.start();

// Make requests...
const response = await din.request('ethereum-mainnet', { ... });

// Clean shutdown (stops sync timers)
din.stop();
```

### Auto-Start

```typescript
const din = new DinClient({
  privateKey: process.env.PRIVATE_KEY!,
});

// First request auto-starts the client
const response = await din.request('ethereum-mainnet', { ... });

// No need to call start() manually
```

### Force Refresh

```typescript
// Force refresh registry and scores
await din.refresh();
```

## Inspecting State

### List Available Networks

```typescript
const networks = await din.getNetworks();

for (const network of networks) {
  console.log(`${network.name} - ${network.status}`);
}
// ethereum-mainnet - Active
// bitcoin-esplora - Active
// solana-mainnet - Active
```

### List Providers with Scores

```typescript
const providers = await din.getProviders('ethereum-mainnet');

for (const provider of providers) {
  console.log(`${provider.name}: score=${provider.score?.toFixed(2)}`);
}
// Infura: score=0.92
// Alchemy: score=0.88
// QuickNode: score=0.85
```

## Handling Responses

### Check Payment Info

```typescript
const response = await din.request('ethereum-mainnet', { ... });

console.log('Provider:', response.provider.name);
console.log('Provider URL:', response.provider.url);

if (response.paymentInfo) {
  console.log('Paid:', response.paymentInfo.amount, 'wei');
  console.log('Tx Hash:', response.paymentInfo.txHash);
}
```

### Type-Safe Responses

```typescript
interface BlockNumberResponse {
  jsonrpc: string;
  id: number;
  result: string;
}

const response = await din.request<BlockNumberResponse>('ethereum-mainnet', {
  body: { jsonrpc: '2.0', method: 'eth_blockNumber', params: [], id: 1 },
});

// response.data is typed as BlockNumberResponse
const blockNumber = parseInt(response.data.result, 16);
```

## Error Handling

### Basic Error Handling

```typescript
try {
  const response = await din.request('ethereum-mainnet', { ... });
} catch (error) {
  if (error.code === 'NETWORK_NOT_FOUND') {
    console.error('Network does not exist');
  } else if (error.code === 'NO_PROVIDERS') {
    console.error('No providers available');
  } else if (error.code === 'PAYMENT_FAILED') {
    console.error('Payment failed - check wallet balance');
  } else {
    console.error('Request failed:', error.message);
  }
}
```

### Provider Error Handling

```typescript
const response = await din.request('ethereum-mainnet', {
  body: { jsonrpc: '2.0', method: 'eth_call', params: [...], id: 1 },
});

if (response.data.error) {
  // JSON-RPC error from provider
  console.error('RPC Error:', response.data.error.message);
  console.error('Code:', response.data.error.code);
}
```

## Testing Configuration

### Use Testnet

```typescript
const din = new DinClient({
  privateKey: process.env.TEST_PRIVATE_KEY!,
  paymentNetwork: 'base-sepolia',  // Use testnet for payments
});

const response = await din.request('ethereum-sepolia', { ... });
```

## Full Example: Token Balance Checker

```typescript
import { DinClient } from '@din-center/router';

const ERC20_BALANCE_OF = '0x70a08231';  // balanceOf(address)

async function getTokenBalance(
  din: DinClient,
  tokenAddress: string,
  walletAddress: string
): Promise<bigint> {
  // Encode call data
  const paddedAddress = walletAddress.slice(2).padStart(64, '0');
  const data = ERC20_BALANCE_OF + paddedAddress;

  const response = await din.request('ethereum-mainnet', {
    body: {
      jsonrpc: '2.0',
      method: 'eth_call',
      params: [{ to: tokenAddress, data }, 'latest'],
      id: 1,
    },
  });

  return BigInt(response.data.result);
}

// Usage
const din = new DinClient({ privateKey: process.env.PRIVATE_KEY! });

const usdtAddress = '0xdAC17F958D2ee523a2206206994597C13D831ec7';
const walletAddress = '0x...';

const balance = await getTokenBalance(din, usdtAddress, walletAddress);
console.log('USDT Balance:', balance / BigInt(10 ** 6), 'USDT');
```
