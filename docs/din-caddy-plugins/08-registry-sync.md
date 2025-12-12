# Registry Sync

Registry sync enables automatic configuration updates from the DIN Registry smart contract. Networks and providers can be discovered and configured on-chain, reducing manual configuration.

## Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                    DIN Registry Contract                         │
│                   (On-chain configuration)                       │
└─────────────────────────────────────────────────────────────────┘
                              │
                     Epoch-based polling
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Registry Sync Service                          │
│  ┌───────────────┐  ┌───────────────┐  ┌───────────────────┐   │
│  │ Fetch Config  │──▶│ Diff Changes  │──▶│ Apply Updates     │   │
│  └───────────────┘  └───────────────┘  └───────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                              │
                       Update networks
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    DIN Middleware Config                         │
│               (Networks, Providers, Settings)                    │
└─────────────────────────────────────────────────────────────────┘
```

## Architecture

### RegistryConfig

```go
type RegistryConfig struct {
    // Registry endpoint URL
    Endpoint string `json:"endpoint,omitempty"`

    // Smart contract address
    ContractAddress string `json:"contract_address,omitempty"`

    // Block epoch (sync every N blocks)
    BlockEpoch int `json:"block_epoch,omitempty"`

    // Polling interval in seconds
    CheckInterval int `json:"check_interval,omitempty"`

    // Retry configuration
    RetryAttempts int `json:"retry_attempts,omitempty"`
    RetryDelay    int `json:"retry_delay,omitempty"`
}
```

### Default Values

| Parameter | Default | Description |
|-----------|---------|-------------|
| `block_epoch` | 2000 | Sync every 2000 blocks |
| `check_interval` | 60 | Check for new epochs every 60s |
| `retry_attempts` | 3 | Retry failed syncs 3 times |
| `retry_delay` | 2 | Wait 2 seconds between retries |

---

## Sync Lifecycle

### 1. Initialization

```go
func (s *dinRegistrySync) Start() error {
    // Fetch initial configuration
    if err := s.syncFromRegistry(); err != nil {
        return err
    }

    // Start background sync loop
    go s.syncLoop()
    return nil
}
```

### 2. Epoch Detection

The sync service monitors block numbers to detect epoch boundaries:

```go
func (s *dinRegistrySync) syncLoop() {
    ticker := time.NewTicker(time.Duration(s.config.CheckInterval) * time.Second)
    for {
        select {
        case <-ticker.C:
            currentBlock := s.getCurrentBlock()
            if s.isNewEpoch(currentBlock) {
                s.syncFromRegistry()
            }
        case <-s.stopCh:
            return
        }
    }
}

func (s *dinRegistrySync) isNewEpoch(block uint64) bool {
    epoch := block / uint64(s.config.BlockEpoch)
    if epoch > s.lastEpoch {
        s.lastEpoch = epoch
        return true
    }
    return false
}
```

### 3. Fetch Registry Data

```go
func (s *dinRegistrySync) fetchRegistryConfig() (*RegistryData, error) {
    // Call smart contract
    networks, err := s.contract.GetNetworks()
    if err != nil {
        return nil, err
    }

    for _, network := range networks {
        providers, err := s.contract.GetProviders(network.ChainID)
        network.Providers = providers
    }

    return &RegistryData{Networks: networks}, nil
}
```

### 4. Apply Updates

```go
func (s *dinRegistrySync) applyUpdates(data *RegistryData) error {
    for _, regNetwork := range data.Networks {
        existing := s.middleware.Services[regNetwork.Name]

        if existing == nil {
            // Create new network
            s.createNetwork(regNetwork)
        } else {
            // Update existing network
            s.updateNetwork(existing, regNetwork)
        }
    }

    // Remove networks no longer in registry
    s.removeStaleNetworks(data.Networks)

    return nil
}
```

---

## Caddyfile Flags

To prevent registry sync from overwriting manual configuration, the system tracks which fields were set via Caddyfile:

```go
type CaddyfileFlags struct {
    HandlerType    bool
    ChainId        bool
    Methods        bool
    HCInterval     bool
    HCThreshold    bool
    HCTimeout      bool
    BlockLagLimit  bool
    BlockJumpLimit bool
    ArchiveEnabled bool
    Providers      map[string]ProviderCaddyfileFlags
}
```

### Flag Checking

```go
func (s *dinRegistrySync) updateNetwork(existing *network, registry *RegistryNetwork) {
    // Only update if not set via Caddyfile
    if !existing.CaddyfileFlags.HandlerType {
        existing.HandlerType = registry.HandlerType
    }

    if !existing.CaddyfileFlags.ChainId {
        existing.ChainId = registry.ChainId
    }

    // ... similar for other fields
}
```

### Priority Order

1. **Caddyfile** - Highest priority, never overwritten
2. **Registry** - Applied if Caddyfile doesn't set the field
3. **Defaults** - Used if neither sets the field

---

## Registry Data Structure

### On-Chain Network

```go
type RegistryNetwork struct {
    Name        string
    ChainID     string
    HandlerType string
    Providers   []RegistryProvider
    Methods     []string
    Enabled     bool
}
```

### On-Chain Provider

```go
type RegistryProvider struct {
    Name     string
    Endpoint string
    Priority int
    Methods  []string
    Enabled  bool
}
```

---

## Configuration

### Caddyfile

```caddyfile
din {
    din_registry {
        endpoint https://mainnet.infura.io/v3/{$INFURA_KEY}
        contract_address 0x1234567890abcdef...
        block_epoch 2000
        check_interval 60
        retry_attempts 3
        retry_delay 2
    }

    services {
        # Can still define services manually
        # These take precedence over registry
        ethereum-mainnet {
            # ...
        }
    }
}
```

### Environment Variables

```bash
DIN_REGISTRY_ENDPOINT=https://mainnet.infura.io/v3/YOUR_KEY
DIN_REGISTRY_CONTRACT=0x1234567890abcdef...
```

---

## Sync Operations

### New Network Discovery

When registry contains a new network:

1. Create network structure
2. Set handler based on type
3. Configure providers
4. Start health checks
5. Add to global registry

```go
func (s *dinRegistrySync) createNetwork(reg *RegistryNetwork) {
    n := NewNetwork(reg.Name)
    n.HandlerType = HandlerType(reg.HandlerType)
    n.ChainId = reg.ChainID

    for _, p := range reg.Providers {
        n.Providers[p.Name] = &provider{
            HttpUrl:  p.Endpoint,
            Priority: p.Priority,
            Methods:  p.Methods,
        }
    }

    s.middleware.Services[reg.Name] = n
    s.startHealthChecks(n)
}
```

### Provider Updates

When provider configuration changes:

```go
func (s *dinRegistrySync) updateProvider(existing *provider, reg *RegistryProvider) {
    // Only update fields not set via Caddyfile
    flags := s.network.CaddyfileFlags.Providers[reg.Name]

    if !flags.Priority {
        existing.Priority = reg.Priority
    }

    if !flags.Methods {
        existing.Methods = reg.Methods
    }
}
```

### Network Removal

When network is removed from registry:

```go
func (s *dinRegistrySync) removeNetwork(name string) {
    if network, ok := s.middleware.Services[name]; ok {
        // Stop health checks
        network.stopHealthChecks()

        // Remove from global registry
        removeNetworkFromGlobalRegistry(name)

        // Remove from middleware
        delete(s.middleware.Services, name)
    }
}
```

---

## Error Handling

### Retry Logic

```go
func (s *dinRegistrySync) syncWithRetry() error {
    var lastErr error
    for i := 0; i < s.config.RetryAttempts; i++ {
        if err := s.syncFromRegistry(); err != nil {
            lastErr = err
            time.Sleep(time.Duration(s.config.RetryDelay) * time.Second)
            continue
        }
        return nil
    }
    return lastErr
}
```

### Panic Recovery

```go
func (s *dinRegistrySync) syncLoop() {
    defer func() {
        if r := recover(); r != nil {
            s.logger.Error("Registry sync panic", zap.Any("error", r))
            // Restart after delay
            time.Sleep(30 * time.Second)
            go s.syncLoop()
        }
    }()
    // ... sync logic
}
```

---

## Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `din_registry_sync_total` | Counter | status | Sync attempts |
| `din_registry_sync_duration_seconds` | Histogram | - | Sync duration |
| `din_registry_networks_total` | Gauge | - | Networks from registry |
| `din_registry_last_sync_timestamp` | Gauge | - | Last successful sync |

---

## Debugging

### Sync Logs

```
INFO: Registry sync started
  epoch=1234
  block=2468000

INFO: Network discovered from registry
  name=ethereum-mainnet
  providers=3

INFO: Provider updated from registry
  network=ethereum-mainnet
  provider=infura
  priority=0

WARN: Network removed from registry
  name=deprecated-network
```

### Force Sync

Registry sync can be triggered by restarting the service or waiting for the next epoch.

---

## DIN Smart Contract Bindings

**Location**: `upstream/github.com/DIN-center/din-sc/apps/din-go/`

The Go bindings for interacting with the DIN Registry smart contract are vendored in the project.

### Key Contract Methods

```go
// Get all registered networks
func (c *Contract) GetNetworks() ([]Network, error)

// Get providers for a network
func (c *Contract) GetProviders(chainID string) ([]Provider, error)

// Get network configuration
func (c *Contract) GetNetworkConfig(chainID string) (*NetworkConfig, error)
```

---

## Related Documentation

- [Caddyfile Configuration](./11-caddyfile-configuration.md) - Registry configuration
- [Network Configuration](./03-network-configuration.md) - Network structure
- [Overview](./00-overview.md) - Architecture overview
