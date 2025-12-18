# Architecture Overview

This document describes the DIN Registry system architecture, contract hierarchy, and ownership model.

## System Components

```mermaid
graph TB
    subgraph "Smart Contracts"
        DR[DinRegistry<br/>Main Entry Point]
        NR[NetworkRegistry<br/>Network Storage]
        N1[Network<br/>Ethereum-Mainnet]
        N2[Network<br/>Polygon-Mainnet]
        P1[Provider<br/>Infura]
        P2[Provider<br/>Alchemy]
        NS1[NetworkService<br/>Infura-ETH]
        NS2[NetworkService<br/>Infura-Polygon]
        NS3[NetworkService<br/>Alchemy-ETH]
    end

    DR -->|creates internally| NR
    DR -->|creates| P1
    DR -->|creates| P2
    NR -->|creates| N1
    NR -->|creates| N2
    P1 -->|creates| NS1
    P1 -->|creates| NS2
    P2 -->|creates| NS3
    NS1 -->|references| N1
    NS2 -->|references| N2
    NS3 -->|references| N1
```

## Contract Hierarchy

### Deployment Order

```mermaid
sequenceDiagram
    participant Deployer
    participant DinRegistry
    participant NetworkRegistry
    participant Network
    participant Provider
    participant NetworkService

    Deployer->>DinRegistry: deploy(name, owner)
    DinRegistry->>NetworkRegistry: new NetworkRegistry()
    Note over NetworkRegistry: Auto-created in constructor

    Deployer->>DinRegistry: createNetwork(name, config)
    DinRegistry->>NetworkRegistry: createNetwork()
    NetworkRegistry->>Network: new Network()

    Deployer->>DinRegistry: createProvider(owner, name)
    DinRegistry->>Provider: new Provider()

    Deployer->>DinRegistry: createNetworkService(network, caps, url, provider)
    DinRegistry->>Provider: addNetworkService()
    Provider->>NetworkService: new NetworkService()
```

### Contract Responsibilities

| Contract | Location | Purpose |
|----------|----------|---------|
| **DinRegistry** | `src/DinRegistry.sol` | Central hub - main entry point for all operations |
| **NetworkRegistry** | `src/NetworkRegistry.sol` | Internal registry for Network contracts |
| **Network** | `src/Network.sol` | Represents a blockchain protocol with RPC methods |
| **Provider** | `src/Provider.sol` | Represents an RPC service provider |
| **NetworkService** | `src/NetworkService.sol` | Links a Provider to a Network with capabilities |
| **INetwork** | `src/INetwork.sol` | Interface and shared type definitions |

## Ownership Model

```mermaid
graph TD
    subgraph "External Owners"
        DO[DIN Owner<br/>EOA Address]
        PO1[Provider Owner 1<br/>EOA Address]
        PO2[Provider Owner 2<br/>EOA Address]
    end

    subgraph "Contracts"
        DR[DinRegistry<br/>dinOwner = DO]
        NR[NetworkRegistry<br/>registryOwner = DR]
        N[Network<br/>networkOwner = NR]
        P1[Provider 1<br/>providerOwner = PO1]
        P2[Provider 2<br/>providerOwner = PO2]
        NS1[NetworkService<br/>serviceOwner = PO1]
        NS2[NetworkService<br/>serviceOwner = PO2]
    end

    DO -->|controls| DR
    DR -->|owns| NR
    NR -->|owns| N
    PO1 -->|controls| P1
    PO2 -->|controls| P2
    P1 -->|creates| NS1
    P2 -->|creates| NS2
```

### Ownership Rules

| Contract | Owner Set By | Immutable? | Can Transfer? |
|----------|--------------|------------|---------------|
| DinRegistry | Constructor parameter | Yes | No |
| NetworkRegistry | DinRegistry address | Yes | No |
| Network | NetworkRegistry.registryOwner | Yes | No |
| Provider | Constructor parameter (EOA) | Yes | No |
| NetworkService | Provider.providerOwner | Yes | No |

## Data Flow

### Write Operations

```mermaid
flowchart LR
    subgraph "DIN Owner Actions"
        A1[Create Network]
        A2[Add Methods]
        A3[Create Provider]
        A4[Set Network Config]
    end

    subgraph "Provider Owner Actions"
        B1[Create NetworkService]
        B2[Update Capabilities]
        B3[Set Service Status]
    end

    A1 --> DR[DinRegistry]
    A2 --> DR
    A3 --> DR
    A4 --> DR

    B1 --> DR
    B2 --> NS[NetworkService]
    B3 --> NS
```

### Read Operations

```mermaid
flowchart LR
    subgraph "Queries"
        Q1[Get All Networks]
        Q2[Get Network Methods]
        Q3[Get Providers by Network]
        Q4[Get Service Capabilities]
    end

    subgraph "Contracts"
        DR[DinRegistry]
        NR[NetworkRegistry]
        N[Network]
        P[Provider]
        NS[NetworkService]
    end

    Q1 --> DR --> NR
    Q2 --> DR --> N
    Q3 --> DR
    Q4 --> NS --> N
```

## Storage Architecture

### DinRegistry Storage

```
┌─────────────────────────────────────────────────────────────┐
│                      DinRegistry                             │
├─────────────────────────────────────────────────────────────┤
│ IMMUTABLE:                                                   │
│   address dinOwner                                           │
│   NetworkRegistry s_networkRegistry                          │
├─────────────────────────────────────────────────────────────┤
│ ARRAYS:                                                      │
│   INetwork[] networks          ─── All network addresses     │
│   Provider[] providers         ─── All provider addresses    │
├─────────────────────────────────────────────────────────────┤
│ MAPPINGS:                                                    │
│   string => bool networkMap           ─── Network exists?    │
│   Provider => bool providerMap        ─── Provider exists?   │
│   INetwork => Provider[] network2providers ─── N:M relation  │
│   bytes32 => bool providerNetworkMap  ─── Link exists?       │
└─────────────────────────────────────────────────────────────┘
```

### Network Storage

```
┌─────────────────────────────────────────────────────────────┐
│                        Network                               │
├─────────────────────────────────────────────────────────────┤
│ IMMUTABLE:                                                   │
│   address networkOwner                                       │
├─────────────────────────────────────────────────────────────┤
│ STATE:                                                       │
│   string networkName                                         │
│   string networkDescription                                  │
│   NetworkStatus networkStatus                                │
│   NetworkOperationsConfig opsConfig                          │
│   uint256 capabilities        ─── Bitmask of methods         │
│   uint8 s_nextBit            ─── Next bit to assign (1-255)  │
├─────────────────────────────────────────────────────────────┤
│ ARRAYS:                                                      │
│   Method[] methods            ─── All method structs         │
├─────────────────────────────────────────────────────────────┤
│ MAPPINGS:                                                    │
│   address => bool authenticated   ─── Can add methods?       │
│   string => Method s_name2method  ─── Name lookup            │
│   uint8 => Method s_bit2method    ─── Bit lookup             │
└─────────────────────────────────────────────────────────────┘
```

## Integration Points

```mermaid
graph TB
    subgraph "On-Chain"
        DR[DinRegistry]
        N[Networks]
        P[Providers]
        NS[NetworkServices]
    end

    subgraph "Off-Chain Services"
        GC[Go Client<br/>din-go]
        W[Watcher<br/>Performance Monitor]
        R[Router<br/>din-caddy-plugins]
    end

    subgraph "Future: ERC-8004"
        IR[Identity Registry]
        RR[Reputation Registry]
    end

    GC -->|GetRegistryData| DR
    W -->|Monitor| NS
    W -->|Scores| R
    R -->|Route Requests| NS

    P -.->|Future| IR
    W -.->|Future| RR
```

## Key Design Decisions

### 1. Contract-per-Entity Model
Each Provider and NetworkService is a separate contract, enabling:
- Independent ownership and permissions
- Clear state isolation
- Direct interaction without proxy

**Trade-off**: Higher deployment gas costs (~500K-800K gas per entity)

### 2. Bitmask Capabilities
Methods encoded as bits (1-255) in a `uint256`:
- O(1) capability checking
- Compact storage
- Efficient bitwise operations

**Trade-off**: Limited to 255 methods per network

### 3. Immutable Ownership
All owner addresses are `immutable`:
- Prevents ownership hijacking
- Clear trust model
- No admin key risks

**Trade-off**: Cannot transfer ownership or recover from key loss

### 4. No Upgradability
Contracts are not upgradeable:
- Simpler security model
- No proxy complexity
- Immutable guarantees

**Trade-off**: Bug fixes require migration to new contracts

## File Locations

| Component | Path |
|-----------|------|
| Smart Contracts | `apps/din-sc/src/` |
| Deployment Scripts | `apps/din-sc/script/` |
| Contract Tests | `apps/din-sc/test/` |
| Go Client | `apps/din-go/lib/din/` |
| Go Handlers | `apps/din-go/pkg/dinregistry/` |
