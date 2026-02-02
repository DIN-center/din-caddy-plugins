package modules

import (
	"sync"
	"time"

	ws "github.com/DIN-center/din-caddy-plugins/lib/watcherscore"
	"github.com/DIN-center/din-sc/apps/din-go/lib/din"
	"github.com/DIN-center/din-sc/apps/din-go/lib/watcher"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"

	"github.com/DIN-center/din-caddy-plugins/lib/auth/siwe"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	networklib "github.com/DIN-center/din-caddy-plugins/lib/network"
	prom "github.com/DIN-center/din-caddy-plugins/lib/prometheus"
	"github.com/DIN-center/din-caddy-plugins/lib/utils"
)

var (
	// Initializations of extended Caddy Module Interface Guards
	// https://caddyserver.com/docs/extending-caddy

	// Din Middleware Module
	_ caddy.Module                = (*DinMiddleware)(nil)
	_ caddy.Provisioner           = (*DinMiddleware)(nil)
	_ caddy.CleanerUpper          = (*DinMiddleware)(nil)
	_ caddyhttp.MiddlewareHandler = (*DinMiddleware)(nil)
	_ caddyfile.Unmarshaler       = (*DinMiddleware)(nil)
)

// RegistryConfig contains all DIN Registry configuration settings
type RegistryConfig struct {
	// Core configuration
	Enabled         bool   `json:"enabled"`
	EndpointUrl     string `json:"endpoint_url"`
	ContractAddress string `json:"contract_address"`

	// Sync configuration
	BlockCheckIntervalSec uint64 `json:"block_check_interval_sec"`
	BlockEpoch            uint64 `json:"block_epoch"`
	Priority              int    `json:"priority"`

	// Retry and recovery configuration
	RetryMaxAttempts   int           `json:"retry_max_attempts"`
	RetryDelay         time.Duration `json:"retry_delay"`
	PanicRecoveryDelay time.Duration `json:"panic_recovery_delay"`

	// Internal state (not exposed in JSON)
	lastUpdatedEpochBlockNumber uint64
}

// DynamicLoadBalancingConfig contains configuration for dynamic load balancing
type DynamicLoadBalancingConfig struct {
	// The flag to enable or disable the dynamic load balancing
	Enabled bool
	// The endpoint of the watcher API
	WatcherApiEndpoint string
	// The API key for the watcher API
	WatcherApiKey string

	// Defines the interval in seconds to sync the watcher scores to the middleware
	WatcherScoreSyncIntervalSec uint64
	// The last time the watcher scores were synced to the middleware
	WatcherScoreLastSyncTime time.Time

	//The backend to manage score (Watcher score)
	watcherScoreManager ws.IWatcherScoreManager

	//The watcher client for dynamic load balancing
	watcherClient watcher.IWatcherAPIClient

	// The channel to quit the goroutine that computes the watcher scores
	watcherScoreComputeQuit chan struct{}

	// The channel to quit the goroutine that syncs the watcher scores to the middleware
	watcherScoreSyncQuit chan struct{}
}

// DinMiddleware is the main middleware struct for the DIN proxy
type DinMiddleware struct {
	// A map of network paths to network objects
	Networks map[string]*network `json:"networks"`
	mu       sync.RWMutex
	// The current environment (prod, beta, dev)
	Env utils.Environment

	// cleanupOnce ensures Cleanup is only executed once
	cleanupOnce sync.Once

	// The default siwe signer object
	DefaultSiweSigner *siwe.SigningConfig

	// The Caddy port to listen on
	CaddyPort string

	// The default siwe signer client
	SiweSignerClient siwe.ISIWESignerClient

	// The prometheus client object
	PrometheusClient *prom.PrometheusClient

	// The dingo client object
	DingoClient din.IDinClient

	logger *logger.LoggerClient

	// The unique machine ID for the current running server instance
	machineID string

	// Test mode flag, should only be used for unit/integration testing purposes.
	testMode bool

	// Handler registry for different network types
	handlerRegistry *networklib.HandlerRegistry

	// DIN Registry configuration
	Registry RegistryConfig `json:"registry"`

	// Internal registry tracking - this is not exposed in config
	registryLastUpdatedEpochBlockNumber uint64

	// The channel to quit the goroutines
	quit chan struct{}

	// Map for associating API keys with users
	ApiKeys map[string]string
	ApiSalt string

	// Dynamic load balancing configuration
	DynamicLoadBalancing DynamicLoadBalancingConfig
}

// CaddyModule returns the Caddy module information.
func (*DinMiddleware) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.din",
		New: func() caddy.Module { return new(DinMiddleware) },
	}
}
