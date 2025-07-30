# New Network Requirements Form - Bitcoin Example

This is a completed example showing the level of detail needed.

## 1. Basic Information

**Network Name**: Bitcoin (Esplora API)  
**Handler Type ID**: bitcoin-esplora  
**API Documentation**: https://github.com/blockstream/esplora/blob/master/API.md

## 2. API Type

**Type** (check one):
- [ ] **JSON-RPC** (like Ethereum)
  - Version: [ ] 2.0 [ ] 1.0
  - Example method: `eth_blockNumber`
- [x] **REST API** (like Bitcoin Esplora)
  - Base path: `/api`
- [ ] **GraphQL**

## 3. Authentication

**Auth Required** (check all that apply):
- [x] **None** (public API)
- [ ] **API Key**
  - Header: _____________________ 
- [x] **OAuth2** (Enterprise endpoints only)
  - Token URL: https://login.blockstream.com/realms/blockstream-public/protocol/openid-connect/token
  - Grant type: [x] client_credentials [ ] other: _____

## 4. Critical Endpoints

### 4.1 Health Check (Get Latest Block)
**Purpose**: Monitor provider health and get current block number

**Endpoint**: `/api/blocks/tip/height`  
**Method**: [x] GET [ ] POST  

**Response Example**:
```json
// Plain integer response
838530
```

### 4.2 Get Block by Number
**Purpose**: Retrieve block details by height/number

**Endpoint**: `/api/block-height/{number}` then `/api/block/{hash}`  
**Supported**: [ ] Yes [ ] No [x] Requires 2 calls (number→hash→block)

**Response Example**:
```json
// First call returns hash as plain text:
"00000000000000000001c59b27e09b0a0a74e3e03c6f4519d1e4268d451b6645"

// Second call returns block:
{
  "id": "00000000000000000001c59b27e09b0a0a74e3e03c6f4519d1e4268d451b6645",  // <- block hash
  "height": 838530,  // <- block number
  "timestamp": 1712920703
}
```

### 4.3 Get Chain ID (if applicable)
**Endpoint**: N/A - Bitcoin doesn't have dynamic chain ID  
**Static Chain IDs**: `bitcoin:mainnet`, `bitcoin:testnet`, `bitcoin:regtest`

## 5. Error Handling

### 5.1 Error Format
```json
// 404 errors
{
  "error": "Block not found"
}
```

### 5.2 Retry Logic
**Retry on** (check all that apply):
- [x] HTTP 5xx errors
- [x] HTTP 429 (rate limit)
- [x] Timeout/connection errors
- [ ] Specific error codes: _____________________

**Don't retry on**:
- [x] HTTP 4xx errors (except 429)
- [x] Auth failures
- [x] Invalid method errors

## 6. Special Requirements

### 6.1 Path Patterns (REST APIs only)
List dynamic paths that need normalization for metrics:
- `/api/tx/{txid}` - Transaction by ID
- `/api/address/{address}` - Address info
- `/api/block/{hash}` - Block by hash
- `/api/block-height/{height}` - Block by height

### 6.2 Rate Limits
- Limit: 100 requests per minute
- Rate limit header: `X-RateLimit-Remaining`

### 6.3 Unique Considerations
List any special behavior or limitations:
- Some endpoints return plain text instead of JSON (block height, block hash by height)
- Must get block hash before getting block by height (two-step process)

## 7. Test Data

### 7.1 Example Requests
Provide 2-3 real examples we can use for testing:

**Health check**:
```
GET https://blockstream.info/api/blocks/tip/height
Response: 838530
```

**Get block**:
```
// Step 1: Get hash by height
GET https://blockstream.info/api/block-height/838530
Response: "00000000000000000001c59b27e09b0a0a74e3e03c6f4519d1e4268d451b6645"

// Step 2: Get block by hash
GET https://blockstream.info/api/block/00000000000000000001c59b27e09b0a0a74e3e03c6f4519d1e4268d451b6645
Response: {
  "id": "00000000000000000001c59b27e09b0a0a74e3e03c6f4519d1e4268d451b6645",
  "height": 838530,
  "timestamp": 1712920703,
  "tx_count": 3188
}
```

### 7.2 Configuration Example
```caddyfile
bitcoin-esplora-mainnet {
    type bitcoin-esplora
    chain_id "bitcoin:mainnet"
    
    # OAuth2 for enterprise endpoint
    custom_config {
        oauth2_client_id "your-client-id"
        oauth2_client_secret "your-secret"
        oauth2_token_url "https://login.blockstream.com/realms/blockstream-public/protocol/openid-connect/token"
    }
    
    providers {
        https://enterprise.blockstream.info {
            priority 0
            auth_type oauth2
        }
        https://blockstream.info {
            priority 1
        }
    }
}
```