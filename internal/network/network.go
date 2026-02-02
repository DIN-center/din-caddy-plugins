package network

import (
	"container/list"
	"encoding/json"
	"fmt"
	"sync"

	"go.uber.org/zap"

	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	networklib "github.com/DIN-center/din-caddy-plugins/lib/network"
	prom "github.com/DIN-center/din-caddy-plugins/lib/prometheus"
	"github.com/DIN-center/din-caddy-plugins/lib/utils"
)

// CaddyfileConfigFlags tracks which configuration fields were explicitly set via Caddyfile.
// When a field's flag is true, it means the value was configured in the Caddyfile and should
// not be overridden by registry sync. This ensures Caddyfile settings take priority.
type CaddyfileConfigFlags struct {
	HandlerTypeSetInCaddyfile              bool
	ChainIdSetInCaddyfile                  bool
	HCIntervalSetInCaddyfile               bool
	HCThresholdSetInCaddyfile              bool
	HCTimeoutSetInCaddyfile                bool
	BlockLagLimitSetInCaddyfile            bool
	BlockJumpLimitSetInCaddyfile           bool
	MaxRequestPayloadSizeKBSetInCaddyfile  bool
	RequestAttemptCountSetInCaddyfile      bool
	ProviderBlockHistorySizeSetInCaddyfile bool
	NetworkBlockHistorySizeSetInCaddyfile  bool
	ArchiveEnabledSetInCaddyfile           bool
}

// HandlerType represents the type of network handler.
// This is a string alias to allow easy comparison with Caddyfile values.
type HandlerType string

const (
	// Handler types
	EVMHandler            HandlerType = "evm"
	BeaconHandler         HandlerType = "beacon-chain"
	StarknetHandler       HandlerType = "starknet"
	SolanaHandler         HandlerType = "solana"
	BitcoinHandler        HandlerType = "bitcoin"
	BitcoinEsploraHandler HandlerType = "bitcoin-esplora"
	TronHandler           HandlerType = "tron-full-node"
)

// Default values for network configuration
const (
	DefaultHCMethod                = "eth_blockNumber"
	DefaultChainIdMethod           = "eth_chainId"
	DefaultCallContractMethod      = "eth_call"
	DefaultGetBlockByNumberMethod  = "eth_getBlockByNumber"
	DefaultHCThreshold             = 2
	DefaultHCTimeout               = 5
	DefaultHCInterval              = 5
	DefaultBlockLagLimit           = int64(15)
	DefaultBlockLagPeriodMs        = 13000 // 13 seconds in milliseconds for dynamic block lag calculation
	DefaultBlockJumpLimit          = int64(100)
	DefaultMaxRequestPayloadSizeKB = int64(4096)
	DefaultRequestAttemptCount     = 5
	DefaultArchiveEnabled          = false

	DefaultProviderBlockHistorySize = 10
	DefaultNetworkBlockHistorySize  = 128
)

var _ json.Unmarshaler = (*Network)(nil)

// Network represents a blockchain network configuration with providers and health checking.
type Network struct {
	Name             string
	HandlerType      HandlerType `json:"handler"` // Network handler type for handler registry
	Quit             chan struct{}
	HttpClient       din_http.IHTTPClient
	PrometheusClient prom.IPrometheusClient
	CaddyPort        string
	Logger           *logger.LoggerClient
	MachineID        string
	Environment      utils.Environment

	// Handler reference for network-specific operations
	Handler networklib.NetworkHandler

	// CaddyfileFlags tracks which fields were set via Caddyfile (not via registry)
	CaddyfileFlags *CaddyfileConfigFlags `json:"-"` // Don't serialize to JSON

	// internal health check values
	HCThreshold              int
	HCTimeout                int
	HCEndpoint               string `json:"healthcheck_endpoint,omitempty"` // REST endpoint for health checks
	ProviderBlockHistorySize int
	NetworkBlockHistorySize  int
	BlockHistory             *list.List
	BlockHistoryMu           sync.RWMutex

	// MethodFilter can be used to route requests based on the method.
	// Using interface{} to avoid circular dependencies; cast to concrete type in modules/.
	MethodFilter interface{}

	// Registry configuration values
	Providers map[string]*Provider `json:"providers"`
	Methods   []*string            `json:"methods"`
	ChainId   string               `json:"chain_id"`

	HCInterval              int   `json:"healthcheck_interval_seconds"`
	BlockLagLimit           int64 `json:"healthcheck_blocklag_limit"`
	BlockJumpLimit          int64 `json:"healthcheck_blockjump_limit"`
	MaxRequestPayloadSizeKB int64 `json:"max_request_payload_size_kb"`
	RequestAttemptCount     int   `json:"request_attempt_count"`
	ArchiveEnabled          bool  `json:"archive_enabled"`

	// Custom configuration passed from Caddyfile
	CustomConfig map[string]interface{} `json:"custom_config,omitempty"`
}

// GetName returns the network name
func (n *Network) GetName() string {
	return n.Name
}

// GetHandlerType returns the handler type string
func (n *Network) GetHandlerType() string {
	return string(n.HandlerType)
}

// GetHandler returns the network handler
func (n *Network) GetHandler() networklib.NetworkHandler {
	return n.Handler
}

// GetProviders returns the providers map
func (n *Network) GetProviders() map[string]*Provider {
	return n.Providers
}

// Stop signals the network to stop all background goroutines
func (n *Network) Stop() {
	close(n.Quit)
}

// NewNetwork creates a new network with the given name and handler type.
// Only put values in the struct definition that are constant.
// Don't kick off any background processes here.
func NewNetwork(name string, handlerType HandlerType, environment utils.Environment, caddyPort string) (*Network, error) {
	n := &Network{
		Name:        name,
		HandlerType: handlerType, // Used for handler selection
		Quit:        make(chan struct{}),
		// Default health check values, to be overridden if specified in the Caddyfile
		HCThreshold:              DefaultHCThreshold,
		HCTimeout:                DefaultHCTimeout,
		HCInterval:               DefaultHCInterval,
		BlockLagLimit:            DefaultBlockLagLimit,
		BlockJumpLimit:           DefaultBlockJumpLimit,
		MaxRequestPayloadSizeKB:  DefaultMaxRequestPayloadSizeKB,
		RequestAttemptCount:      DefaultRequestAttemptCount,
		ProviderBlockHistorySize: DefaultProviderBlockHistorySize,
		NetworkBlockHistorySize:  DefaultNetworkBlockHistorySize,
		BlockHistory:             list.New(),
		ArchiveEnabled:           DefaultArchiveEnabled,
		Environment:              environment,
		Providers:                make(map[string]*Provider),
		CaddyPort:                caddyPort,
		// Initialize Caddyfile flags tracking
		CaddyfileFlags: &CaddyfileConfigFlags{
			// If handlerType is provided (not empty), mark it as set in Caddyfile
			HandlerTypeSetInCaddyfile: handlerType != "",
		},
	}

	// Note: Handler initialization is deferred to avoid duplicate initialization.
	// The handler will be set later when:
	// 1. The "type" field is parsed in Caddyfile configuration
	// 2. The network defaults to EVM if no type is specified
	// This ensures the handler is only initialized once with complete configuration
	// including ChainID and other network-specific settings.

	return n, nil
}

// SetHandler sets the network's handler. This is used when:
// 1. Creating a network with an explicit type
// 2. Setting type in Caddyfile configuration
// 3. Defaulting to EVM when no type is specified
func (n *Network) SetHandler(handler networklib.NetworkHandler) error {
	if handler == nil {
		return fmt.Errorf("cannot set nil handler")
	}

	// Only set if different to avoid redundant updates
	if n.Handler == handler {
		return nil
	}

	n.Handler = handler

	// Only log if logger is available (it's initialized during Provision)
	if n.Logger != nil {
		n.Logger.Debug("Network handler set",
			zap.String("network", n.Name),
			zap.String("handler_type", handler.GetType()))
	}

	return nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (n *Network) UnmarshalJSON(data []byte) error {
	type Alias Network
	alias := &struct {
		*Alias
	}{
		Alias: (*Alias)(n),
	}
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}

	n.Quit = make(chan struct{})

	return nil
}
