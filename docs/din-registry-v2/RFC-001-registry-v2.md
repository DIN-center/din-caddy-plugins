# RFC-001: DIN Registry V2 - Unified Struct-Based Architecture

```
RFC Number:     RFC-001
Title:          DIN Registry V2 - Unified Struct-Based Architecture
Authors:        DIN Core Team
Status:         Draft
Created:        2025-12-03
Target:         Q1 2026 (3-month timeline)
Deployment:     Linea L2 (Greenfield)
```

---

## Abstract

This RFC proposes replacing the current multi-contract DIN Registry architecture with a unified single-contract design using struct-based storage. The new architecture:

1. **Consolidates 5 contract types into 1** upgradeable contract (UUPS pattern)
2. **Generalizes from JSON-RPC to generic services** supporting REST APIs, indexers, and future service types
3. **Reduces SDK complexity** from O(N×P×S) RPC calls to O(1) batch reads
4. **Preserves ownership model** while adding transferability and mutable service URLs
5. **Deploys as greenfield** on Linea L2 with no migration required

---

## Table of Contents

1. [Motivation](#1-motivation)
2. [Goals and Non-Goals](#2-goals-and-non-goals)
3. [Proposed Design](#3-proposed-design)
4. [Data Model](#4-data-model)
5. [Storage Layout](#5-storage-layout)
6. [API Design](#6-api-design)
7. [Authorization Model](#7-authorization-model)
8. [Risks and Mitigations](#8-risks-and-mitigations)
9. [Open Questions](#9-open-questions)
10. [Appendices](#10-appendices)

---

## 1. Motivation

### 1.1 Current Architecture Pain Points

| Issue | Quantified Impact | Reference |
|-------|-------------------|-----------|
| **Contract-per-entity deployment** | ~500K gas per Provider, ~800K gas per NetworkService | `Provider.sol`, `NetworkService.sol` |
| **O(N×P×S) RPC calls for sync** | ~229 calls for 3 networks × 5 providers × 2 services | `din-go/lib/din/client.go:34-186` |
| **5 contract types / 4 ABIs** | SDK maintenance burden, multiple handlers | Current architecture |
| **Incomplete provider unregistration** | `network2providers` mapping not cleaned up | `DinRegistry.sol:271` TODO |
| **Immutable service URLs** | Providers must recreate services to change endpoints | `NetworkService.sol` constructor |
| **255 method limit per network** | `uint8 s_nextBit` constraint | `Network.sol:61` |
| **Immutable ownership** | Lost key = lost provider forever | All contracts |
| **JSON-RPC hardcoded model** | Cannot support REST APIs or indexers | Current design |

### 1.2 Business Drivers

- **Provider onboarding friction**: Every new provider requires DIN admin intervention
- **REST API support needed**: Upcoming integrations require non-RPC services (indexers, explorers)
- **Performance issues**: Router team reports slow registry sync times
- **Technical debt**: Blocking new feature development

### 1.3 Target Deployment Context

| Environment | Gas Price | Deploy Provider | Deploy Service |
|-------------|-----------|-----------------|----------------|
| Ethereum L1 | ~30 gwei | ~$30 | ~$48 |
| **Linea L2** | ~0.05-0.5 gwei | ~$0.10 | ~$0.16 |

On Linea L2, gas costs are ~100x cheaper. The economic argument for struct-based storage is weaker, but **architectural simplicity becomes the primary driver**.

---

## 2. Goals and Non-Goals

### 2.1 Goals

| # | Goal | Success Criteria |
|---|------|------------------|
| G1 | **Single contract architecture** | All entities stored as structs in one DinRegistryV2 contract |
| G2 | **Generic service model** | Support JSON-RPC, REST API; extensible for indexers, AI agents |
| G3 | **Per-service-type capabilities** | Each service type defines its own methods/capabilities |
| G4 | **Preserve ownership model** | Provider owners control their data; DIN owner has admin override |
| G5 | **O(1) registry sync** | Single `getRegistrySnapshot()` call for full state |
| G6 | **Ownership transferability** | Allow provider ownership transfer |
| G7 | **Service URL mutability** | Allow providers to update endpoint URLs |
| G8 | **UUPS upgradeability** | Support future contract upgrades |
| G9 | **Geolocation support** | Track provider regions for geographic routing |

### 2.2 Non-Goals

| # | Non-Goal | Rationale |
|---|----------|-----------|
| NG1 | Migration from V1 | Greenfield deployment, no data migration automation |
| NG2 | On-chain reputation | Remains off-chain via Watcher (future RFC for ERC-8004 alignment) |
| NG3 | Multi-chain deployment | Linea L2 only for initial release |
| NG4 | ERC-721 Agent NFTs | Future consideration per ERC-8004 alignment doc |
| NG5 | Validation registry | Out of scope for V2 |
| NG6 | Self-registration | Admin-only for V2; design API for future support |

---

## 3. Proposed Design

### 3.1 Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         CURRENT (V1)                                     │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│   ┌─────────────┐      ┌─────────────────┐      ┌──────────────┐        │
│   │ DinRegistry │─────▶│ NetworkRegistry │─────▶│   Network    │        │
│   └─────────────┘      └─────────────────┘      │  (contract)  │        │
│          │                                       └──────────────┘        │
│          │                                              ▲                │
│          ▼                                              │                │
│   ┌──────────────┐      ┌─────────────────┐            │                │
│   │   Provider   │─────▶│ NetworkService  │────────────┘                │
│   │  (contract)  │      │   (contract)    │                             │
│   └──────────────┘      └─────────────────┘                             │
│                                                                          │
│   5 contract types • 4 ABIs • O(N×P×S) RPC calls                        │
└─────────────────────────────────────────────────────────────────────────┘

                                    ▼

┌─────────────────────────────────────────────────────────────────────────┐
│                         PROPOSED (V2)                                    │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│   ┌─────────────────────────────────────────────────────────────────┐   │
│   │                      DinRegistryV2                               │   │
│   │                    (UUPS Upgradeable)                            │   │
│   │  ┌────────────────────────────────────────────────────────────┐ │   │
│   │  │                   Primary Storage                           │ │   │
│   │  │  services[id] => ServiceData                                │ │   │
│   │  │  providers[id] => ProviderData                              │ │   │
│   │  │  providerServices[id] => ProviderServiceData                │ │   │
│   │  │  methods[id] => MethodData                                  │ │   │
│   │  └────────────────────────────────────────────────────────────┘ │   │
│   │  ┌────────────────────────────────────────────────────────────┐ │   │
│   │  │                   Secondary Indexes                         │ │   │
│   │  │  serviceNameToId, providerNameToId, ownerToProviders        │ │   │
│   │  │  serviceToProviderServices, providerToProviderServices      │ │   │
│   │  └────────────────────────────────────────────────────────────┘ │   │
│   └─────────────────────────────────────────────────────────────────┘   │
│                                                                          │
│   1 contract • 1 ABI • O(1) RPC calls                                   │
└─────────────────────────────────────────────────────────────────────────┘
```

### 3.2 Entity Hierarchy

```
Service (top-level)              Provider (organization)
├── ethereum-mainnet             ├── Infura [us-east, eu-west]
├── ethereum-sepolia             ├── Alchemy [us-west, asia]
├── base-mainnet                 ├── QuickNode [global]
├── solana-mainnet               └── ...
├── bitcoin-esplora-mainnet
└── ...
              ↘                ↙
            ProviderService (junction)
            - Infura's ethereum-mainnet endpoint
            - Alchemy's solana-mainnet endpoint
            - QuickNode's bitcoin-esplora endpoint
```

**Key Changes:**
- **Service** (was "Network"): Defines a service like `ethereum-mainnet` or `bitcoin-esplora-mainnet`
- **Provider**: Organization providing infrastructure (unchanged conceptually)
- **ProviderService** (was "NetworkService"): Junction linking Provider to Service with endpoint details

### 3.3 Service Type Abstraction

Services are categorized by type, each with its own capability definitions:

```
serviceType: "json-rpc"
├── Services: ethereum-mainnet, base-sepolia, solana-mainnet
└── Methods: eth_blockNumber, eth_getBalance, eth_call, ...

serviceType: "rest-api"
├── Services: bitcoin-esplora-mainnet, etherscan-mainnet
└── Methods: GET /blocks, GET /tx/{hash}, GET /address/{addr}/balance, ...

serviceType: "indexer" (future)
├── Services: the-graph-mainnet, covalent-mainnet
└── Methods: GraphQL queries, REST endpoints
```

### 3.4 Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Entity identity | Numeric IDs (uint256) | O(1) lookups, foreign key relationships |
| Service granularity | Flat list | Each chain is its own Service (simpler than hierarchical) |
| Contract pattern | UUPS Proxy | Upgradeable with minimal overhead |
| Authorization | Centralized modifiers | Simpler than distributed across contracts |
| Contract size | Library pattern | Stay under 24KB limit |
| Name lookups | bytes32 hash index | Gas-efficient vs string comparison |
| Method bit size | uint16 | Supports >255 methods (vs current uint8) |

---

## 4. Data Model

### 4.1 Service (replaces Network)

The top-level entity representing a service definition.

```solidity
struct ServiceData {
    uint256 id;
    string name;                    // e.g., "ethereum-mainnet", "bitcoin-esplora-mainnet"
    string description;
    bytes32 serviceType;            // keccak256("json-rpc"), keccak256("rest-api")
    ServiceStatus status;
    uint256 capabilities;           // Bitmask of all methods this service supports
    uint16 nextMethodBit;           // Next available bit (expanded from uint8)
    uint256 createdAt;
    uint256 updatedAt;

    // Operations config (from current NetworkOperationsConfig)
    uint8 healthcheckMethodBit;
    uint8 healthcheckIntervalSec;
    uint8 blockLagLimit;
    uint8 requestAttemptCount;
    uint16 maxRequestPayloadSizeKb;
    uint32 registryBlockEpoch;
}
```

### 4.2 Provider

Organization providing infrastructure for services.

```solidity
struct ProviderData {
    uint256 id;
    address owner;                  // NOW TRANSFERABLE (was immutable)
    string name;                    // e.g., "Infura", "Alchemy"
    ProviderStatus status;
    uint256 createdAt;
    uint256 updatedAt;

    // Auth config (flattened from ProviderAuthConfig)
    ProviderAuthType authType;      // None, Siwe
    string authUrl;

    // Geolocation for routing (NEW)
    string[] regions;               // e.g., ["us-east", "eu-west", "asia-pacific"]
}
```

### 4.3 ProviderService (replaces NetworkService)

Junction table linking a Provider to a Service with provider-specific details.

```solidity
struct ProviderServiceData {
    uint256 id;
    uint256 providerId;             // FK to Provider
    uint256 serviceId;              // FK to Service
    ProviderServiceStatus status;
    uint256 createdAt;
    uint256 updatedAt;

    // Provider-specific for this service
    string endpointUrl;             // NOW MUTABLE (was immutable serviceUrl)
    uint256 capabilities;           // Methods THIS provider supports (subset of Service)

    // Provider-specific metadata
    bytes metadata;                 // ABI-encoded, type-specific extras
}
```

### 4.4 Method

Method/capability definition within a service.

```solidity
struct MethodData {
    uint256 id;
    uint256 serviceId;              // FK to Service
    string name;                    // e.g., "eth_blockNumber", "GET /blocks"
    uint16 bit;                     // Bit position (expanded from uint8)
    bool deactivated;
}
```

### 4.5 Enums (preserved from V1)

```solidity
enum ServiceStatus {
    None,
    Onboarding,
    Active,
    Maintenance,
    Decommissioned,
    Retired
}

enum ProviderStatus {
    None,
    Onboarding,
    Active,
    Maintenance,
    Retired
}

enum ProviderServiceStatus {
    None,
    Onboarding,
    Active,
    Maintenance,
    Retired
}

enum ProviderAuthType {
    None,
    Siwe
}
```

### 4.6 Fields Mapping: V1 → V2

| V1 (Current) | V2 (Proposed) | Notes |
|--------------|---------------|-------|
| `Network.name` | `Service.name` | Renamed |
| `Network.description` | `Service.description` | |
| `Network.networkStatus` | `Service.status` | |
| `Network.capabilities` | `Service.capabilities` | |
| `Network.opsConfig.*` | `Service.healthcheck*`, etc. | Flattened |
| `Network.methods[]` | `MethodData[]` per Service | Separate struct |
| `Network.s_nextBit` (uint8) | `Service.nextMethodBit` (uint16) | Expanded |
| `Provider.name` | `Provider.name` | |
| `Provider.providerOwner` (immutable) | `Provider.owner` | Now transferable |
| `Provider.providerStatus` | `Provider.status` | |
| `Provider.authConfig` | `Provider.authType`, `authUrl` | Flattened |
| — | `Provider.regions` | NEW: Geolocation |
| `NetworkService.serviceUrl` (immutable) | `ProviderService.endpointUrl` | Now mutable |
| `NetworkService.capabilities` | `ProviderService.capabilities` | |
| `NetworkService.serviceStatus` | `ProviderService.status` | |
| — | `ProviderService.metadata` | NEW: Type-specific |

---

## 5. Storage Layout

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.27;

import {UUPSUpgradeable} from "@openzeppelin/contracts-upgradeable/proxy/utils/UUPSUpgradeable.sol";
import {Initializable} from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";

contract DinRegistryV2 is Initializable, UUPSUpgradeable {

    // ============ Admin ============
    address public dinOwner;

    // ============ Primary Storage ============
    mapping(uint256 => ServiceData) public services;
    mapping(uint256 => ProviderData) public providers;
    mapping(uint256 => ProviderServiceData) public providerServices;
    mapping(uint256 => MethodData) public methods;

    // ============ Counters (start at 1, 0 = not found) ============
    uint256 public nextServiceId;
    uint256 public nextProviderId;
    uint256 public nextProviderServiceId;
    uint256 public nextMethodId;

    // ============ Name Indexes ============
    mapping(bytes32 => uint256) public serviceNameToId;     // hash(name) => serviceId
    mapping(bytes32 => uint256) public providerNameToId;    // hash(name) => providerId

    // ============ Relationship Indexes ============
    mapping(address => uint256[]) public ownerToProviders;              // owner => providerIds
    mapping(uint256 => uint256[]) public serviceToProviderServices;     // serviceId => providerServiceIds
    mapping(uint256 => uint256[]) public providerToProviderServices;    // providerId => providerServiceIds
    mapping(uint256 => uint256[]) public serviceToMethods;              // serviceId => methodIds

    // ============ Composite Key Indexes ============
    mapping(bytes32 => uint256) public providerServiceKey;  // hash(providerId, serviceId) => providerServiceId

    // ============ Type Filtering ============
    mapping(bytes32 => uint256[]) public serviceTypeToServices;  // "json-rpc" => serviceIds

    // ============ Method Lookups per Service ============
    mapping(uint256 => mapping(bytes32 => uint256)) public serviceMethodNameToId;  // serviceId => hash(name) => methodId
    mapping(uint256 => mapping(uint16 => uint256)) public serviceMethodBitToId;    // serviceId => bit => methodId
}
```

### 5.1 Index Diagram

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          PRIMARY STORAGE                                 │
├─────────────────────────────────────────────────────────────────────────┤
│  services[1] = {id:1, name:"ethereum-mainnet", type:"json-rpc", ...}    │
│  services[2] = {id:2, name:"bitcoin-esplora-mainnet", type:"rest-api"}  │
│                                                                          │
│  providers[1] = {id:1, owner:0xAAA, name:"Infura", regions:["us-east"]} │
│  providers[2] = {id:2, owner:0xBBB, name:"Alchemy", regions:["us-west"]}│
│                                                                          │
│  providerServices[1] = {providerId:1, serviceId:1, url:"https://..."}   │
│  providerServices[2] = {providerId:2, serviceId:1, url:"https://..."}   │
│                                                                          │
│  methods[1] = {serviceId:1, name:"eth_blockNumber", bit:1}              │
│  methods[2] = {serviceId:1, name:"eth_getBalance", bit:2}               │
└─────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────┐
│                          SECONDARY INDEXES                               │
├─────────────────────────────────────────────────────────────────────────┤
│  serviceNameToId:                                                        │
│    hash("ethereum-mainnet") => 1                                         │
│    hash("bitcoin-esplora-mainnet") => 2                                  │
│                                                                          │
│  providerNameToId:                                                       │
│    hash("Infura") => 1                                                   │
│    hash("Alchemy") => 2                                                  │
│                                                                          │
│  ownerToProviders:                                                       │
│    0xAAA => [1]                                                          │
│    0xBBB => [2]                                                          │
│                                                                          │
│  serviceToProviderServices:                                              │
│    1 => [1, 2]  (ethereum-mainnet providers)                             │
│                                                                          │
│  providerToProviderServices:                                             │
│    1 => [1]     (Infura's services)                                      │
│    2 => [2]     (Alchemy's services)                                     │
│                                                                          │
│  providerServiceKey:                                                     │
│    hash(1, 1) => 1  (Infura + ethereum-mainnet)                          │
│    hash(2, 1) => 2  (Alchemy + ethereum-mainnet)                         │
│                                                                          │
│  serviceTypeToServices:                                                  │
│    hash("json-rpc") => [1]                                               │
│    hash("rest-api") => [2]                                               │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 6. API Design

### 6.1 Initialization (UUPS)

```solidity
/// @notice Initializes the registry (replaces constructor for UUPS)
/// @param _dinOwner The address of the DIN administrator
function initialize(address _dinOwner) external initializer {
    __UUPSUpgradeable_init();
    dinOwner = _dinOwner;
    nextServiceId = 1;
    nextProviderId = 1;
    nextProviderServiceId = 1;
    nextMethodId = 1;
}

/// @notice Authorizes upgrades (UUPS requirement)
function _authorizeUpgrade(address newImplementation) internal override onlyDinOwner {}
```

### 6.2 Service Management (DIN Owner Only)

```solidity
/// @notice Creates a new service
function createService(
    string calldata name,
    string calldata description,
    bytes32 serviceType,
    ServiceStatus status,
    uint8 healthcheckMethodBit,
    uint8 healthcheckIntervalSec,
    uint8 blockLagLimit,
    uint8 requestAttemptCount,
    uint16 maxRequestPayloadSizeKb
) external onlyDinOwner returns (uint256 serviceId);

/// @notice Updates service status
function setServiceStatus(uint256 serviceId, ServiceStatus status) external onlyDinOwner;

/// @notice Updates service operations config
function setServiceOpsConfig(
    uint256 serviceId,
    uint8 healthcheckMethodBit,
    uint8 healthcheckIntervalSec,
    uint8 blockLagLimit,
    uint8 requestAttemptCount,
    uint16 maxRequestPayloadSizeKb,
    uint32 registryBlockEpoch
) external onlyDinOwner;

/// @notice Adds a method to a service
function addMethod(uint256 serviceId, string calldata methodName) external onlyDinOwner returns (uint16 bit);

/// @notice Adds multiple methods to a service
function addMethods(uint256 serviceId, string[] calldata methodNames) external onlyDinOwner returns (uint256 capabilities);

/// @notice Deactivates a method (soft delete)
function deactivateMethod(uint256 methodId) external onlyDinOwner;
```

### 6.3 Provider Management

```solidity
/// @notice Creates a new provider (DIN Owner only for V2)
function createProvider(
    address owner,
    string calldata name,
    ProviderAuthType authType,
    string calldata authUrl,
    string[] calldata regions,
    ProviderStatus status
) external onlyDinOwner returns (uint256 providerId);

/// @notice Updates provider status
function setProviderStatus(uint256 providerId, ProviderStatus status)
    external onlyProviderOwnerOrDin(providerId);

/// @notice Updates provider auth config
function setProviderAuth(uint256 providerId, ProviderAuthType authType, string calldata authUrl)
    external onlyProviderOwnerOrDin(providerId);

/// @notice Updates provider regions
function setProviderRegions(uint256 providerId, string[] calldata regions)
    external onlyProviderOwnerOrDin(providerId);

/// @notice Transfers provider ownership (owner only, not DIN)
function transferProviderOwnership(uint256 providerId, address newOwner)
    external onlyProviderOwner(providerId);

/// @notice Deletes a provider and all its provider services
function deleteProvider(uint256 providerId) external onlyProviderOwnerOrDin(providerId);
```

### 6.4 ProviderService Management

```solidity
/// @notice Creates a provider service (links provider to service)
function createProviderService(
    uint256 providerId,
    uint256 serviceId,
    string calldata endpointUrl,
    uint256 initialCapabilities,
    ProviderServiceStatus status,
    bytes calldata metadata
) external onlyProviderOwnerOrDin(providerId) returns (uint256 providerServiceId);

/// @notice Updates provider service endpoint URL (NOW MUTABLE)
function setProviderServiceUrl(uint256 providerServiceId, string calldata endpointUrl)
    external onlyProviderServiceOwnerOrDin(providerServiceId);

/// @notice Updates provider service capabilities
function setProviderServiceCapabilities(uint256 providerServiceId, uint256 capabilities)
    external onlyProviderServiceOwnerOrDin(providerServiceId);

/// @notice Updates provider service status
function setProviderServiceStatus(uint256 providerServiceId, ProviderServiceStatus status)
    external onlyProviderServiceOwnerOrDin(providerServiceId);

/// @notice Updates provider service metadata
function setProviderServiceMetadata(uint256 providerServiceId, bytes calldata metadata)
    external onlyProviderServiceOwnerOrDin(providerServiceId);

/// @notice Deletes a provider service
function deleteProviderService(uint256 providerServiceId)
    external onlyProviderServiceOwnerOrDin(providerServiceId);
```

### 6.5 Query Functions

```solidity
// ============ By ID (Primary - Most Efficient) ============
function getService(uint256 id) external view returns (ServiceData memory);
function getProvider(uint256 id) external view returns (ProviderData memory);
function getProviderService(uint256 id) external view returns (ProviderServiceData memory);
function getMethod(uint256 id) external view returns (MethodData memory);

// ============ By Name (Human-Friendly) ============
function getServiceByName(string calldata name) external view returns (ServiceData memory);
function getProviderByName(string calldata name) external view returns (ProviderData memory);

// ============ By Relationship ============
function getProvidersByOwner(address owner) external view returns (ProviderData[] memory);
function getProviderServicesByService(uint256 serviceId) external view returns (ProviderServiceData[] memory);
function getProviderServicesByProvider(uint256 providerId) external view returns (ProviderServiceData[] memory);
function getMethodsByService(uint256 serviceId) external view returns (MethodData[] memory);
function getServicesByType(bytes32 serviceType) external view returns (ServiceData[] memory);

// ============ Composite Lookup ============
function getProviderService(uint256 providerId, uint256 serviceId)
    external view returns (ProviderServiceData memory);

// ============ Method Lookups ============
function getMethodByName(uint256 serviceId, string calldata name) external view returns (MethodData memory);
function getMethodByBit(uint256 serviceId, uint16 bit) external view returns (MethodData memory);
function isMethodSupported(uint256 providerServiceId, uint16 methodBit) external view returns (bool);

// ============ Batch Read (Critical for SDK) ============
struct RegistrySnapshot {
    ServiceData[] services;
    ProviderData[] providers;
    ProviderServiceData[] providerServices;
    MethodData[] methods;
}

function getRegistrySnapshot() external view returns (RegistrySnapshot memory);

// ============ Paginated Queries ============
function getServicesPaginated(uint256 offset, uint256 limit)
    external view returns (ServiceData[] memory, uint256 total);
function getProvidersPaginated(uint256 offset, uint256 limit)
    external view returns (ProviderData[] memory, uint256 total);
function getProviderServicesPaginated(uint256 offset, uint256 limit)
    external view returns (ProviderServiceData[] memory, uint256 total);
```

### 6.6 Events

```solidity
// ============ Service Events ============
event ServiceCreated(uint256 indexed id, string name, bytes32 indexed serviceType);
event ServiceUpdated(uint256 indexed id, string field);
event ServiceDeleted(uint256 indexed id);

// ============ Method Events ============
event MethodAdded(uint256 indexed serviceId, uint256 methodId, string name, uint16 bit);
event MethodDeactivated(uint256 indexed serviceId, uint256 methodId);

// ============ Provider Events ============
event ProviderCreated(uint256 indexed id, address indexed owner, string name);
event ProviderUpdated(uint256 indexed id, string field);
event ProviderOwnershipTransferred(uint256 indexed id, address indexed from, address indexed to);
event ProviderDeleted(uint256 indexed id);

// ============ ProviderService Events ============
event ProviderServiceCreated(
    uint256 indexed id,
    uint256 indexed providerId,
    uint256 indexed serviceId,
    string endpointUrl
);
event ProviderServiceUpdated(uint256 indexed id, string field);
event ProviderServiceDeleted(uint256 indexed id);
```

---

## 7. Authorization Model

### 7.1 Roles and Permissions

| Role | Create | Update | Delete | Transfer |
|------|--------|--------|--------|----------|
| **DIN Owner** | Services, Methods, Providers, ProviderServices | All entities | All entities | — |
| **Provider Owner** | ProviderServices (own) | Own provider, own ProviderServices | Own ProviderServices | Own provider |

### 7.2 Modifier Implementations

```solidity
modifier onlyDinOwner() {
    require(msg.sender == dinOwner, "Not DIN owner");
    _;
}

modifier onlyProviderOwner(uint256 providerId) {
    require(msg.sender == providers[providerId].owner, "Not provider owner");
    _;
}

modifier onlyProviderOwnerOrDin(uint256 providerId) {
    require(
        msg.sender == providers[providerId].owner || msg.sender == dinOwner,
        "Not authorized"
    );
    _;
}

modifier onlyProviderServiceOwnerOrDin(uint256 providerServiceId) {
    uint256 providerId = providerServices[providerServiceId].providerId;
    require(
        msg.sender == providers[providerId].owner || msg.sender == dinOwner,
        "Not authorized"
    );
    _;
}

modifier serviceExists(uint256 serviceId) {
    require(services[serviceId].id != 0, "Service not found");
    _;
}

modifier providerExists(uint256 providerId) {
    require(providers[providerId].id != 0, "Provider not found");
    _;
}
```

---

## 8. Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| **Contract exceeds 24KB** | Medium | High | Use libraries extensively; split into facade pattern if needed |
| **Go client migration breaks consumers** | Medium | Medium | Version both clients; deprecation period |
| **New data model confuses providers** | Low | Medium | Documentation; admin tooling; migration guide |
| **Performance regression in batch reads** | Low | High | Gas profiling; pagination as fallback |
| **Linea L2-specific issues** | Low | Medium | Testnet validation |
| **Timeline slip** | Medium | Medium | Buffer in Phase 3; prioritize core functionality |
| **Index consistency bugs** | Medium | High | Comprehensive test coverage; formal verification consideration |
| **UUPS upgrade authorization bypass** | Low | Critical | Security audit; use OpenZeppelin battle-tested implementation |

---

## 9. Open Questions

### 9.1 Method Bit Size

**Question**: Should method bits be uint8 (255 max) or uint16 (65535 max)?

**Recommendation**: uint16 - minimal gas impact, future-proofs for large method sets

### 9.2 Service Type Governance

**Question**: Who can create new service types?

| Option | Pros | Cons |
|--------|------|------|
| DIN owner only | Controlled, consistent | Centralized |
| Governance vote | Decentralized | Slower, complex |

**Recommendation**: DIN owner only for V2; consider governance for V3

### 9.3 Metadata Encoding

**Question**: How should type-specific metadata be stored?

| Option | Pros | Cons |
|--------|------|------|
| ABI-encoded bytes | Flexible, efficient | Requires decoding |
| JSON string | Human-readable | Larger storage, parsing overhead |
| IPFS hash | Unlimited size | Requires external resolution |

**Recommendation**: ABI-encoded bytes with well-documented schemas per service type

### 9.4 REST API Capability Model

**Question**: How should REST API endpoints map to capabilities?

**Recommendation**: Defer detailed REST API design to follow-up RFC; V2 focuses on JSON-RPC with extensibility

### 9.5 Self-Registration Timeline

**Question**: When should self-registration be enabled?

**Recommendation**: Design API for future support; enable via upgrade after governance/reputation mechanisms are in place

---

## 10. Appendices

### 10.1 Gas Estimates (Linea L2)

| Operation | Estimated Gas | Est. Cost (0.5 gwei) |
|-----------|---------------|----------------------|
| Deploy V2 (with proxy) | ~3,000,000 | ~$0.0015 |
| Create Service | ~200,000 | ~$0.0001 |
| Add 10 methods | ~300,000 | ~$0.00015 |
| Create Provider | ~150,000 | ~$0.000075 |
| Create ProviderService | ~200,000 | ~$0.0001 |
| getRegistrySnapshot (50 entities) | ~500,000 | ~$0.00025 |

### 10.2 Contract Size Mitigation

If the contract exceeds 24KB, use the library pattern:

```solidity
library ServiceLib {
    function create(...) internal returns (uint256) { ... }
    function update(...) internal { ... }
}

library ProviderLib {
    function create(...) internal returns (uint256) { ... }
    function transfer(...) internal { ... }
}

library ProviderServiceLib {
    function create(...) internal returns (uint256) { ... }
    function update(...) internal { ... }
}

contract DinRegistryV2 is UUPSUpgradeable {
    using ServiceLib for *;
    using ProviderLib for *;
    using ProviderServiceLib for *;
    // ... storage and modifiers only
}
```

### 10.3 References

| Document | Description |
|----------|-------------|
| [11-architectural-critique.md](./11-architectural-critique.md) | Detailed analysis of V1 issues |
| [01-architecture-overview.md](./01-architecture-overview.md) | Current V1 architecture |
| [03-data-model.md](./03-data-model.md) | Current V1 data model |
| [08-erc8004-alignment.md](./08-erc8004-alignment.md) | Future ERC-8004 considerations |
| [09-scalability-recommendations.md](./09-scalability-recommendations.md) | Scalability analysis |

### 10.4 Glossary

| Term | Definition |
|------|------------|
| **Service** | A service definition (e.g., ethereum-mainnet, bitcoin-esplora-mainnet) |
| **Provider** | Organization providing infrastructure (e.g., Infura, Alchemy) |
| **ProviderService** | Junction linking a Provider to a Service with endpoint details |
| **Capabilities** | Bitmask representing supported methods |
| **Service Type** | Category of services (json-rpc, rest-api, etc.) |
| **UUPS** | Universal Upgradeable Proxy Standard |

---

## Approval

| Role | Name | Date | Signature |
|------|------|------|-----------|
| Author | | | |
| Reviewer | | | |
| Approver | | | |

---

*This RFC is a living document. Updates will be tracked in version control.*
