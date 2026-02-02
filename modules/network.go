package modules

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

// caddyfileConfigFlags tracks which configuration fields were explicitly set via Caddyfile.
// When a field's flag is true, it means the value was configured in the Caddyfile and should
// not be overridden by registry sync. This ensures Caddyfile settings take priority.
type caddyfileConfigFlags struct {
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

var _ json.Unmarshaler = (*network)(nil)

type network struct {
	Name             string
	HandlerType      HandlerType `json:"handler"` // Network handler type for handler registry
	quit             chan struct{}
	HttpClient       din_http.IHTTPClient
	PrometheusClient prom.IPrometheusClient
	CaddyPort        string
	logger           *logger.LoggerClient
	machineID        string
	Environment      utils.Environment

	// NEW: Handler reference for network-specific operations
	handler networklib.NetworkHandler

	// CaddyfileFlags tracks which fields were set via Caddyfile (not via registry)
	CaddyfileFlags *caddyfileConfigFlags `json:"-"` // Don't serialize to JSON

	// internal health check values
	HCThreshold              int
	HCTimeout                int
	HCEndpoint               string `json:"healthcheck_endpoint,omitempty"` // REST endpoint for health checks
	ProviderBlockHistorySize int
	NetworkBlockHistorySize  int
	blockHistory             *list.List
	blockHistoryMu           sync.RWMutex

	// MethodFilter can be used to route requests based on the method. It implements
	// the ProviderFilter interface, but for now is the only implementation.
	MethodFilter *methodFilter

	// Registry configuration values
	Providers map[string]*provider `json:"providers"`
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

// NewNetwork creates a new network with the given name and handler type
// Only put values in the struct definition that are constant
// Don't kick off any Background processes here
func NewNetwork(name string, handlerType HandlerType, environment utils.Environment, caddyPort string) (*network, error) {
	n := &network{
		Name:        name,
		HandlerType: handlerType, // Used for handler selection
		quit:        make(chan struct{}),
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
		blockHistory:             list.New(),
		ArchiveEnabled:           DefaultArchiveEnabled,
		Environment:              environment,
		Providers:                make(map[string]*provider),
		CaddyPort:                caddyPort,
		// Initialize Caddyfile flags tracking
		CaddyfileFlags: &caddyfileConfigFlags{
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
func (n *network) SetHandler(handler networklib.NetworkHandler) error {
	if handler == nil {
		return fmt.Errorf("cannot set nil handler")
	}

	// Only set if different to avoid redundant updates
	if n.handler == handler {
		return nil
	}

	n.handler = handler

	// Only log if logger is available (it's initialized during Provision)
	if n.logger != nil {
		n.logger.Debug("Network handler set",
			zap.String("network", n.Name),
			zap.String("handler_type", handler.GetType()))
	}

	return nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (n *network) UnmarshalJSON(data []byte) error {
	type Alias network
	alias := &struct {
		*Alias
	}{
		Alias: (*Alias)(n),
	}
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}

	n.quit = make(chan struct{})

	return nil
}
