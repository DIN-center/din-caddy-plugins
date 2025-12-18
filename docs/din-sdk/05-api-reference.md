# API Reference

## DinClient

Main SDK entry point.

### Constructor

```typescript
const din = new DinClient(config: DinConfig);
```

### DinConfig

```typescript
interface DinConfig {
  /**
   * Private key for wallet used in all x402 payments (USDC on Linea)
   * Used for: RPC requests, registry syncs (Linea), and score syncs (Watchers)
   * @required
   */
  privateKey: string;

  /**
   * Bootstrap Linea RPC URL (FREE, used for initial sync only)
   * After bootstrap, SDK uses providers from the registry with x402 USDC payments
   * @default 'https://linea.din.dev/rpc'
   */
  bootstrapRpcUrl?: string;

  /**
   * Network name for Watcher service in the DIN Registry
   * Watcher is accessed via x402 USDC payments like any RPC provider
   * @default 'watchers'
   */
  watcherNetworkName?: string;

  /**
   * Interval for registry data refresh (uses Linea providers + x402 USDC)
   * @default 60000 (60 seconds)
   */
  registrySyncIntervalMs?: number;

  /**
   * Interval for score refresh (uses Watchers providers + x402 USDC)
   * @default 30000 (30 seconds)
   */
  scoreSyncIntervalMs?: number;

  /**
   * Session ID for sticky routing (optional)
   * Can be overridden per-request
   */
  sessionId?: string;
}
```

### Methods

#### request()

Make a request to a DIN network provider.

```typescript
async request<T = any>(
  network: string,
  options?: RequestOptions
): Promise<DinResponse<T>>
```

**Parameters:**
- `network` - Network name (e.g., `'ethereum-mainnet'`, `'bitcoin-esplora'`)
- `options` - Request configuration (optional)

**Returns:** `DinResponse<T>`

#### getNetworks()

Get all available networks.

```typescript
async getNetworks(): Promise<Network[]>
```

#### getProviders()

Get providers for a specific network with current scores.

```typescript
async getProviders(network: string): Promise<Provider[]>
```

#### refresh()

Force refresh registry and score caches.

```typescript
async refresh(): Promise<void>
```

#### start()

Start the client (initializes sync timers). Called automatically on first request if not called manually.

```typescript
async start(): Promise<void>
```

#### stop()

Stop the client (clears sync timers).

```typescript
stop(): void
```

---

## RequestOptions

Options for individual requests.

```typescript
interface RequestOptions {
  /**
   * HTTP method
   * @default 'POST'
   */
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE';

  /**
   * Custom headers to include
   */
  headers?: Record<string, string>;

  /**
   * Request body
   * Objects are automatically JSON.stringify'd
   */
  body?: string | object;

  /**
   * Path to append to provider URL
   * Used for REST APIs (e.g., '/block/tip/height')
   */
  path?: string;

  /**
   * Session ID for sticky routing
   * Overrides client-level sessionId for this request
   */
  sessionId?: string;
}
```

---

## DinResponse

Response returned from `request()`.

```typescript
interface DinResponse<T = any> {
  /**
   * Response body (parsed JSON or text)
   */
  data: T;

  /**
   * HTTP status code
   */
  status: number;

  /**
   * Response headers
   */
  headers: Record<string, string>;

  /**
   * Provider that served the request
   */
  provider: {
    name: string;
    url: string;
  };

  /**
   * Payment details (present after successful x402 payment)
   */
  paymentInfo?: {
    amount: string;      // Amount in wei
    txHash?: string;     // Transaction hash
  };
}
```

---

## Registry Types

### Network

```typescript
interface Network {
  /**
   * Network name (e.g., 'ethereum-mainnet')
   */
  name: string;

  /**
   * Contract address
   */
  address: string;

  /**
   * URL path segment for routing
   */
  proxyName: string;

  /**
   * Network status
   */
  status: 'Active' | 'Maintenance' | 'Inactive';

  /**
   * Network configuration
   */
  config: NetworkOperationsConfig;

  /**
   * Providers for this network (keyed by address)
   */
  providers: Map<string, Provider>;
}
```

### Provider

```typescript
interface Provider {
  /**
   * Provider name
   */
  name: string;

  /**
   * Contract address
   */
  address: string;

  /**
   * Address for receiving x402 payments
   */
  paymentAddress: string;

  /**
   * Provider status
   */
  status: 'Active' | 'Inactive';

  /**
   * Network services (endpoints)
   */
  services: NetworkService[];

  /**
   * Quality score from watcher (0.0 to 1.0)
   * Updated by score sync
   */
  score?: number;
}
```

### NetworkService

```typescript
interface NetworkService {
  /**
   * Network this service belongs to
   */
  networkName: string;

  /**
   * RPC endpoint URL
   */
  url: string;

  /**
   * Bitmask of supported methods
   */
  capabilities: bigint;

  /**
   * Service status
   */
  status: 'Active' | 'Inactive';
}
```

### NetworkOperationsConfig

```typescript
interface NetworkOperationsConfig {
  handler: string;
  healthcheckIntervalSec: number;
  healthcheckThreshold: number;
  healthcheckTimeout: number;
  blockLagLimit: number;
  blockJumpLimit: number;
  requestAttemptCount: number;
  maxRequestPayloadSizeKb: number;
  registryBlockEpoch: number;
  archiveEnabled: boolean;
  providerBlockHistorySize: number;
  networkBlockHistorySize: number;
  chainId: string;
}
```

---

## Watcher Types

### Score

```typescript
interface Score {
  /**
   * Combined quality score (0.0 to 1.0)
   */
  value: number;

  /**
   * Block consistency metric
   */
  blockConsistency: number;

  /**
   * State consistency metric
   */
  stateConsistency: number;

  /**
   * Latency metric (normalized)
   */
  latency: number;

  /**
   * When this score was last updated
   */
  updatedAt: Date;
}
```

---

## Linea Registry Client (Internal)

The SDK reads from the DIN Registry smart contract on Linea directly using viem:

```typescript
// Internal client - users don't interact with this directly
interface LineaRegistryClient {
  /**
   * Get all networks from registry contract
   */
  getNetworks(): Promise<Network[]>;

  /**
   * Get providers for a specific network
   */
  getProviders(networkAddress: string): Promise<Provider[]>;
}
```

**Note:** This is handled internally by the SDK. Users only need to provide a private key for x402 payments. Registry access is permissionless via DIN-provided Linea RPC.

---

## Error Types

```typescript
interface DinError extends Error {
  code: string;
  details?: any;
}

// Error codes
type ErrorCode =
  | 'NETWORK_NOT_FOUND'      // Requested network doesn't exist
  | 'NO_PROVIDERS'           // No providers available for network
  | 'PROVIDER_ERROR'         // Provider returned an error
  | 'PAYMENT_FAILED'         // x402 payment failed
  | 'PAYMENT_TIMEOUT'        // Payment took too long
  | 'SYNC_FAILED'            // Registry/score sync failed
  | 'NOT_STARTED';           // Client not started
```
