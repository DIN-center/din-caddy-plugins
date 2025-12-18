# x402 Payment Integration

## Overview

The DIN network uses **x402 micropayments** for pay-per-request access to RPC providers. The SDK handles all payment logic transparently using the `x402-axios` library.

## What is x402?

x402 is an HTTP-based micropayment protocol:

1. Client sends request to server
2. Server returns `402 Payment Required` with payment details
3. Client signs payment and retries with `X-PAYMENT` header
4. Server verifies payment and returns response

## Payment Flow

```
┌──────────┐                              ┌──────────────┐
│  SDK     │                              │   Provider   │
└────┬─────┘                              └──────┬───────┘
     │                                           │
     │  1. POST /rpc { jsonrpc: ... }            │
     │──────────────────────────────────────────▶│
     │                                           │
     │  2. 402 Payment Required                  │
     │     X-PAYMENT-REQUIRED: { ... }           │
     │◀──────────────────────────────────────────│
     │                                           │
     │  3. (SDK signs payment with wallet)       │
     │                                           │
     │  4. POST /rpc { jsonrpc: ... }            │
     │     X-PAYMENT: <signed-payment>           │
     │──────────────────────────────────────────▶│
     │                                           │
     │  5. 200 OK                                │
     │     X-PAYMENT-RESPONSE: { ... }           │
     │     { result: ... }                       │
     │◀──────────────────────────────────────────│
     │                                           │
```

## Implementation

### X402PaymentClient Class

```typescript
import { withPaymentInterceptor, decodeXPaymentResponse } from 'x402-axios';
import axios, { AxiosInstance } from 'axios';
import { privateKeyToAccount } from 'viem/accounts';

export class X402PaymentClient {
  private account: ReturnType<typeof privateKeyToAccount>;
  private axiosInstance: AxiosInstance;

  constructor(
    privateKey: string,
    network: 'base' | 'base-sepolia' = 'base'
  ) {
    // Create viem account from private key
    this.account = privateKeyToAccount(privateKey as `0x${string}`);

    // Wrap axios with x402 payment interceptor
    this.axiosInstance = withPaymentInterceptor(
      axios.create(),
      this.account
    );
  }

  async makeRequest<T>(
    url: string,
    options: {
      method: string;
      headers?: Record<string, string>;
      body?: string;
    }
  ): Promise<{
    data: T;
    status: number;
    headers: Record<string, string>;
    paymentInfo?: { amount: string; txHash?: string };
  }> {
    const response = await this.axiosInstance.request({
      url,
      method: options.method,
      headers: options.headers,
      data: options.body,
    });

    // Extract payment info from response
    let paymentInfo: { amount: string; txHash?: string } | undefined;
    const paymentResponse = response.headers['x-payment-response'];
    if (paymentResponse) {
      const decoded = decodeXPaymentResponse(paymentResponse);
      paymentInfo = {
        amount: decoded.amount,
        txHash: decoded.txHash,
      };
    }

    return {
      data: response.data,
      status: response.status,
      headers: response.headers as Record<string, string>,
      paymentInfo,
    };
  }
}
```

### How x402-axios Works

The `withPaymentInterceptor` function adds response interceptor logic:

```typescript
// Simplified view of what x402-axios does internally
axios.interceptors.response.use(
  (response) => response,  // Pass through successful responses
  async (error) => {
    if (error.response?.status === 402) {
      // 1. Parse payment requirements from header
      const requirements = parsePaymentRequired(
        error.response.headers['x-payment-required']
      );

      // 2. Sign payment with wallet
      const payment = await signPayment(account, requirements);

      // 3. Retry original request with payment header
      return axios.request({
        ...error.config,
        headers: {
          ...error.config.headers,
          'X-PAYMENT': encodePayment(payment),
        },
      });
    }
    throw error;
  }
);
```

## Payment Details

### X-PAYMENT-REQUIRED Header

When a provider requires payment, it returns:

```http
HTTP/1.1 402 Payment Required
X-PAYMENT-REQUIRED: {
  "scheme": "exact",
  "network": "base",
  "maxAmountRequired": "1000000000000000",
  "resource": "/rpc",
  "description": "RPC request",
  "mimeType": "application/json",
  "payTo": "0xProviderPaymentAddress",
  "maxTimeoutSeconds": 300,
  "asset": "0x...",
  "extra": { ... }
}
```

### X-PAYMENT Header

The SDK signs and sends:

```http
POST /rpc HTTP/1.1
X-PAYMENT: {
  "x402Version": 1,
  "scheme": "exact",
  "network": "base",
  "payload": {
    "signature": "0x...",
    "authorization": {
      "from": "0xUserAddress",
      "to": "0xProviderAddress",
      "value": "1000000000000000",
      ...
    }
  }
}
```

### X-PAYMENT-RESPONSE Header

After successful payment verification:

```http
HTTP/1.1 200 OK
X-PAYMENT-RESPONSE: {
  "success": true,
  "amount": "1000000000000000",
  "txHash": "0x..."
}
```

## Configuration

### DinConfig Payment Options

```typescript
interface DinConfig {
  privateKey: string;  // REQUIRED - wallet for signing payments

  paymentNetwork?: 'base' | 'base-sepolia';  // Default: 'base'
}
```

### Supported Networks

| Network | Chain ID | Use Case |
|---------|----------|----------|
| `base` | 8453 | Production |
| `base-sepolia` | 84532 | Testing |

## User Response

Payment info is included in the SDK response:

```typescript
interface DinResponse<T = any> {
  data: T;
  status: number;
  headers: Record<string, string>;
  provider: {
    name: string;
    url: string;
  };
  paymentInfo?: {           // Present after successful payment
    amount: string;         // Amount paid in wei
    txHash?: string;        // Transaction hash (if available)
  };
}
```

### Example Usage

```typescript
const response = await din.request('ethereum-mainnet', {
  body: { jsonrpc: '2.0', method: 'eth_blockNumber', params: [], id: 1 },
});

console.log('Result:', response.data.result);
console.log('Paid:', response.paymentInfo?.amount, 'wei');
console.log('Tx:', response.paymentInfo?.txHash);
```

## Provider Payment Address

The provider's payment address comes from the registry:

```typescript
interface Provider {
  name: string;
  address: string;          // Contract address
  paymentAddress: string;   // EOA for receiving payments ← Used by x402
  status: 'Active' | 'Inactive';
  services: NetworkService[];
}
```

The SDK doesn't need to explicitly pass this - the provider includes it in the 402 response's `payTo` field.

## Error Handling

### Payment Failures

```typescript
try {
  const response = await din.request('ethereum-mainnet', { ... });
} catch (error) {
  if (error.code === 'PAYMENT_FAILED') {
    // Insufficient funds, signature failed, etc.
  }
  if (error.code === 'PAYMENT_TIMEOUT') {
    // Payment took too long
  }
}
```

### Insufficient Funds

If the wallet doesn't have enough funds:
- x402-axios will throw an error
- SDK propagates error to user
- User needs to fund wallet on Base network

## Security Considerations

1. **Private Key Storage** - Never hardcode private keys; use environment variables
2. **Amount Verification** - x402-axios verifies amounts before signing
3. **Network Matching** - Ensure payment network matches provider expectations
4. **Replay Protection** - x402 includes nonces to prevent replay attacks

## Dependencies

```json
{
  "x402-axios": "^0.1.0",   // Payment interceptor
  "axios": "^1.6.0",        // HTTP client
  "viem": "^2.0.0"          // Wallet/signing
}
```

## Testing

For testing without real payments:

1. Use `base-sepolia` network
2. Fund test wallet with Sepolia ETH
3. Use test providers that accept testnet payments

```typescript
const din = new DinClient({
  privateKey: process.env.TEST_PRIVATE_KEY!,
  paymentNetwork: 'base-sepolia',
});
```
