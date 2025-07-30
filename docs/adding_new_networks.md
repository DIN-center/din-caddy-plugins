# New Network Requirements Form

Please provide the following information to add support for your blockchain network.

## 1. Basic Information

**Network Name**: _____________________ (e.g., "Bitcoin", "Polygon")  
**Handler Type ID**: _____________________ (e.g., "bitcoin-esplora", "polygon") - lowercase with hyphens  
**API Documentation**: _____________________ (URL)

## 2. API Type

**Type** (check one):
- [ ] **JSON-RPC** (like Ethereum)
  - Version: [ ] 2.0 [ ] 1.0
  - Example method: `eth_blockNumber`
- [ ] **REST API** (like Bitcoin Esplora)
  - Base path: _____________________ (e.g., `/api/v1`)
- [ ] **GraphQL**

## 3. Authentication

**Auth Required** (check all that apply):
- [ ] **None** (public API)
- [ ] **API Key**
  - Header: _____________________ (e.g., `X-API-Key`)
- [ ] **OAuth2**
  - Token URL: _____________________
  - Grant type: [ ] client_credentials [ ] other: _____

## 4. Critical Endpoints

### 4.1 Health Check (Get Latest Block)
**Purpose**: Monitor provider health and get current block number

**Endpoint**: _____________________ (e.g., `/blocks/latest` or `eth_blockNumber`)  
**Method**: [ ] GET [ ] POST  

**Response Example**:
```json
// Show where the block number is located
{
  "height": 123456,  // <- block number here
  "hash": "0xabc..."
}
```

### 4.2 Get Block by Number
**Purpose**: Retrieve block details by height/number

**Endpoint**: _____________________ (e.g., `/blocks/{number}` or `eth_getBlockByNumber`)  
**Supported**: [ ] Yes [ ] No [ ] Requires 2 calls (number→hash→block)

**Response Example**:
```json
{
  "number": 123456,     // <- block number
  "hash": "0xabc...",   // <- block hash (required)
  "timestamp": 1234567  // <- if available
}
```

### 4.3 Get Chain ID (if applicable)
**Endpoint**: _____________________ (e.g., `eth_chainId`)  
**Static Chain IDs**: _____________________ (e.g., `bitcoin:mainnet`, `evm:1`)

## 5. Error Handling

### 5.1 Error Format
```json
// Provide your API's error format
{
  "error": {
    "code": -32000,
    "message": "Error description"
  }
}
```

### 5.2 Retry Logic
**Retry on** (check all that apply):
- [ ] HTTP 5xx errors
- [ ] HTTP 429 (rate limit)
- [ ] Timeout/connection errors
- [ ] Specific error codes: _____________________

**Don't retry on**:
- [ ] HTTP 4xx errors (except 429)
- [ ] Auth failures
- [ ] Invalid method errors

## 6. Special Requirements

### 6.1 Path Patterns (REST APIs only)
List dynamic paths that need normalization for metrics:
- `/tx/{txid}` - Transaction by ID
- `/address/{address}` - Address info
- `/block/{height}` - Block by height
- _____________________

### 6.2 Rate Limits
- Limit: _____ requests per _____
- Rate limit header: _____________________ (e.g., `X-RateLimit-Remaining`)

### 6.3 Unique Considerations
List any special behavior or limitations:
- _____________________
- _____________________

## 7. Test Data

### 7.1 Example Requests
Provide 2-3 real examples we can use for testing:

**Health check**:
```
GET https://api.example.com/blocks/latest
Response: {"height": 838530}
```

**Get block**:
```
POST https://api.example.com/
{"jsonrpc": "2.0", "method": "getBlock", "params": [123456], "id": 1}
Response: {"result": {"hash": "0xabc...", "number": 123456}}
```

### 7.2 Configuration Example
```caddyfile
network-mainnet {
    type your-network-type
    chain_id "network:mainnet"
    
    providers {
        https://api.example.com {
            priority 0
            # Add auth if needed
        }
    }
}
```

---

## Quick Reference: Required Handler Methods

Your handler will implement these key methods based on your API type:

**All handlers need**:
- Health check endpoint
- Block retrieval 
- Error handling
- Request validation

**JSON-RPC specific**:
- Method name mapping
- Request ID handling
- Batch support (optional)

**REST API specific**:
- Path normalization
- URL parameter handling
- Path stripping for routing

---

**Submission**: Email completed form to: _____________________ or create a GitHub issue