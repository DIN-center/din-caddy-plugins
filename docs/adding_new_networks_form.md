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

### 6.2 Unique Considerations
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

---

## Implementation Guide: Adding Your Network to DIN

Once you've provided the information above, here's how we'll integrate your network into the DIN Gateway. This process ensures your blockchain network becomes a first-class citizen in our multi-network routing system.

### Step 1: Handler Creation

We'll create a new handler file in `/lib/network/` named after your network (e.g., `your_network_handler.go`). This handler will implement the NetworkHandler interface, which serves as the contract between your network's specific requirements and DIN's unified routing layer. The handler encapsulates all protocol-specific logic, making your network's unique API patterns transparent to the rest of the system.

### Step 2: Core Method Implementation

Your handler will implement several critical methods that enable seamless integration:

**GetType() and Metadata Methods**: These identify your handler and provide version information for debugging and monitoring. The type ID you provided becomes the key identifier throughout the system.

**Request Processing Pipeline**: We'll implement ValidateRequest() to ensure incoming requests meet your API's requirements, TranslatePath() to handle any necessary URL transformations, and ProcessRequest() to apply protocol-specific logic before forwarding to providers.

**Health Monitoring**: Using the health check endpoint you specified, we'll implement GetLatestBlock() to continuously monitor provider health. This enables automatic failover when providers experience issues.

**Error Handling**: Based on your error format and retry specifications, we'll implement ParseResponse() and IsRetryableError() to handle failures gracefully and determine when requests should be retried with alternative providers.

### Step 3: Protocol-Specific Adaptations

Depending on your API type, we'll implement additional specialized logic:

For **JSON-RPC APIs**, we'll handle request ID tracking, method name extraction from request bodies, and potentially batch request processing. The existing JSON-RPC utilities in the codebase will be leveraged to minimize custom code.

For **REST APIs**, we'll implement path normalization to ensure consistent metrics collection despite dynamic URL parameters. Path patterns you identified will be normalized so that `/tx/abc123` and `/tx/def456` both appear as `/tx/{txid}` in metrics.

For **GraphQL APIs**, we'll parse queries to extract operation names and implement appropriate request/response handling for your schema.

### Step 4: Authentication Integration

If your network requires authentication, we'll integrate it with DIN's existing auth system. For OAuth2/OIDC, your handler will automatically benefit from our token refresh infrastructure. For API keys, we'll ensure headers are properly added to outgoing requests. The authentication configuration will be parsed from the Caddyfile auth block and managed transparently.

### Step 5: Handler Registration

Your handler will be registered in the global handler registry during initialization. This involves adding a factory function that creates handler instances when networks of your type are configured. The registration happens automatically when the module loads, making your handler immediately available.

### Step 6: Testing Infrastructure

We'll create comprehensive tests in a corresponding test file (`your_network_handler_test.go`). These tests will cover request validation, path translation, error handling, health checks, and authentication flows. Mock servers will simulate your API's behavior to ensure reliability without requiring real network calls.

### Step 7: Configuration Templates

We'll provide Caddyfile configuration examples showing how to set up networks using your handler. These templates will include proper provider configuration, authentication setup (if needed), and any network-specific parameters. Users will be able to copy and modify these templates for their deployments.

### Step 8: Metrics and Monitoring

Your network will automatically benefit from DIN's unified metrics system. Request counts, latencies, error rates, and provider health will be tracked using consistent Prometheus metrics. The path normalization ensures meaningful aggregation of metrics for dynamic endpoints.

### Step 9: Documentation

We'll update the main documentation to include your network in the supported networks table, add configuration examples to the multi-network support guide, and create a dedicated guide if your network has unique requirements or features.

### Step 10: Integration Validation

Before release, we'll validate the integration by testing with real providers you've specified, ensuring health checks work correctly, verifying error handling and retry logic, confirming metrics are properly collected, and checking that authentication (if required) functions smoothly.

### What Makes This Process Smooth

The modular architecture means your network's implementation is isolated in its handler, preventing any impact on existing networks. The standardized NetworkHandler interface ensures all necessary functionality is implemented consistently. Existing utilities for JSON-RPC, REST, and authentication can be reused, reducing implementation complexity.

### Timeline and Next Steps

Once you submit the completed form, implementation typically takes 2-3 days for straightforward APIs or 4-5 days for complex protocols with special requirements. You'll receive a pull request for review, allowing you to validate the implementation against your requirements. After your approval and our testing, the new handler will be merged and available in the next release.

### Post-Implementation Support

After your network is added, we'll monitor its performance through our standard metrics, address any issues that arise in production, and work with you to add new features as your API evolves. Your network becomes part of our regular testing and maintenance cycles, ensuring continued compatibility.

This systematic approach ensures that adding your network to DIN is reliable, maintainable, and provides users with a consistent experience across all supported blockchains. The abstraction layers mean users can switch between networks with minimal configuration changes, while your network's unique features remain fully accessible.
