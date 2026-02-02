package modules

import (
	"container/list"
	"fmt"
	"net/url"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"go.uber.org/zap"

	"github.com/DIN-center/din-sc/apps/din-go/lib/din"

	"github.com/DIN-center/din-caddy-plugins/lib/auth/siwe"
	dinHttp "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	networklib "github.com/DIN-center/din-caddy-plugins/lib/network"
	prom "github.com/DIN-center/din-caddy-plugins/lib/prometheus"
	"github.com/DIN-center/din-caddy-plugins/lib/utils"
	ws "github.com/DIN-center/din-caddy-plugins/lib/watcherscore"
)

// Provision implements caddy.Provisioner
func (d *DinMiddleware) Provision(ctx caddy.Context) error {
	if len(d.Networks) == 0 && !d.Registry.Enabled {
		return fmt.Errorf("expected at least 1 network or registry to be defined")
	}

	// set the initialize the dinMiddlewareObject
	err := d.initialize(ctx)
	if err != nil {
		return fmt.Errorf("error initializing middleware: %w", err)
	}

	d.logger.Info("Din middleware provisioned")
	return nil
}

// initialize initializes the din middleware object with the necessary configuration values
func (d *DinMiddleware) initialize(context caddy.Context) error {
	// Initialize core services
	if err := d.initializeCoreServices(context); err != nil {
		return fmt.Errorf("failed to initialize core services: %w", err)
	}

	// Initialize default configuration values
	d.initializeDefaults()

	// Initialize DIN registry client
	if err := d.initializeDinRegistryClient(); err != nil {
		return fmt.Errorf("failed to initialize DIN registry: %w", err)
	}

	// Initialize network handler registry
	d.initializeHandlerRegistry()

	// Initialize all networks
	if err := d.initializeNetworks(); err != nil {
		return fmt.Errorf("failed to initialize networks: %w", err)
	}

	// If dynamic load balancing is enabled, initialize the score backend
	if d.DynamicLoadBalancing.Enabled {
		d.logger.Info("[DYNAMIC_LB] Dynamic load balancing activated, initializing watcher score manager")
		d.logger.Debug("[DYNAMIC_LB] Dynamic load balancing settings:",
			zap.Uint64("watcher_scores_sync_interval_secs", d.DynamicLoadBalancing.WatcherScoreSyncIntervalSec),
			zap.String("watcher_endpoint", d.DynamicLoadBalancing.WatcherApiEndpoint))

		//list of networks to compute scores
		networks := make([]string, 0, len(d.Networks))
		for network := range d.Networks {
			networks = append(networks, network)
		}

		//initialize the watcher score manager for provisioned networks
		d.DynamicLoadBalancing.watcherScoreManager = ws.NewWithBuiltInFormula(networks, d.GetOrCreateWatcherClient(), d.logger.Logger)

	}

	d.logger.Info("Din middleware provisioned")

	// Start background services if not in test mode
	if !d.testMode {
		if err := d.startBackgroundServices(); err != nil {
			return fmt.Errorf("failed to start background services: %w", err)
		}
	}

	return nil
}

// initializeCoreServices initializes core services like logger, prometheus, and SIWE
func (d *DinMiddleware) initializeCoreServices(context caddy.Context) error {
	d.machineID = utils.GetMachineId()

	// Initialize logger
	loggerClient := logger.NewLoggerClient(context.Logger(d), d.Env)
	d.logger = loggerClient

	// Initialize prometheus client
	promClient := prom.NewPrometheusClient(loggerClient, d.machineID)
	d.PrometheusClient = promClient

	// Initialize SIWE signer client
	d.SiweSignerClient = siwe.NewSIWESignerClient()

	// Initialize quit channel
	d.quit = make(chan struct{})

	return nil
}

// initializeDefaults sets default values for configuration
func (d *DinMiddleware) initializeDefaults() {
	if d.Registry.BlockCheckIntervalSec == 0 {
		d.Registry.BlockCheckIntervalSec = uint64(DefaultRegistryBlockCheckIntervalSec)
	}
	if d.Registry.BlockEpoch == 0 {
		d.Registry.BlockEpoch = DefaultRegistryBlockEpoch
	}
	if d.Registry.Priority == 0 {
		d.Registry.Priority = DefaultRegistryPriority
	}
	if d.CaddyPort == "" {
		d.CaddyPort = DefaultPort
	}
	// Set retry defaults
	if d.Registry.RetryMaxAttempts == 0 {
		d.Registry.RetryMaxAttempts = DefaultRegistryRetryMaxAttempts
	}
	if d.Registry.RetryDelay == 0 {
		d.Registry.RetryDelay = DefaultRegistryRetryDelay
	}
	if d.Registry.PanicRecoveryDelay == 0 {
		d.Registry.PanicRecoveryDelay = DefaultRegistryPanicRecoveryDelay
	}
}

// initializeDinRegistryClient initializes the DIN registry client
func (d *DinMiddleware) initializeDinRegistryClient() error {
	if d.Registry.Enabled {
		// DinClient is only initialized if the registry is enabled
		d.logger.Info("DIN registry is enabled, initializing DIN client to connect to the registry",
			zap.String("registry_endpoint_url", d.Registry.EndpointUrl),
			zap.String("registry_contract_address", d.Registry.ContractAddress))

		client, err := din.NewDinClient(d.logger.Logger, d.Registry.EndpointUrl, d.Registry.ContractAddress)
		if err != nil {
			return fmt.Errorf("error initializing DIN client: %w", err)
		}
		d.DingoClient = client
	}

	return nil
}

// initializeHandlerRegistry initializes the network handler registry
func (d *DinMiddleware) initializeHandlerRegistry() {
	d.handlerRegistry = networklib.DefaultRegistry
	networklib.RegisterBuiltinHandlers()
}

// initializeNetworks initializes all configured networks
func (d *DinMiddleware) initializeNetworks() error {
	for networkName := range d.Networks {
		if err := d.initializeNetwork(networkName); err != nil {
			return fmt.Errorf("failed to initialize network '%s': %w", networkName, err)
		}
	}
	return nil
}

// initializeNetwork initializes a single network
func (d *DinMiddleware) initializeNetwork(networkName string) error {
	networkObj := d.Networks[networkName]

	// Initialize handler if needed
	if err := d.initializeNetworkHandler(networkName, networkObj); err != nil {
		return err
	}

	// Initialize network services
	if err := d.initializeNetworkServices(networkName, networkObj); err != nil {
		return err
	}

	// Validate network configuration
	if err := d.validateNetworkConfiguration(networkName, networkObj); err != nil {
		return err
	}

	// Register network in global registry
	RegisterNetwork(networkName, networkObj)

	return nil
}

// initializeNetworkHandler initializes the handler for a network if needed
func (d *DinMiddleware) initializeNetworkHandler(networkName string, networkObj *network) error {
	// Check if handler needs to be initialized
	// Handler may be nil if:
	// 1. Configuration was loaded from JSON (handlers are not serialized)
	// 2. Network was created programmatically without going through UnmarshalCaddyfile
	if networkObj.Handler == nil && networkObj.HandlerType != "" {
		d.logger.Debug("Initializing handler during provision",
			zap.String("network", networkName),
			zap.String("handler_type", string(networkObj.HandlerType)))

		// Configure the handler with complete configuration including ChainID
		config := &networklib.NetworkConfig{
			Name:           networkName,
			Type:           string(networkObj.HandlerType),
			ChainID:        networkObj.ChainId,
			MaxPayloadSize: networkObj.MaxRequestPayloadSizeKB * 1024,
			RequestTimeout: time.Duration(networkObj.HCTimeout) * time.Second,
			Logger:         d.logger,
			Custom:         make(map[string]interface{}),
		}

		handler, err := d.handlerRegistry.GetHandler(string(networkObj.HandlerType), config)
		if err != nil {
			return fmt.Errorf("failed to get handler for network '%s' handler_type '%s': %w", networkName, networkObj.HandlerType, err)
		}

		// Set the network's handler
		if err := networkObj.SetHandler(handler); err != nil {
			return fmt.Errorf("failed to set handler for network '%s': %w", networkName, err)
		}
	} else if networkObj.Handler != nil {
		d.logger.Debug("Handler already initialized, skipping",
			zap.String("network", networkName),
			zap.String("handler_type", string(networkObj.HandlerType)))
	}

	return nil
}

// initializeNetworkServices initializes HTTP client and providers for a network
func (d *DinMiddleware) initializeNetworkServices(networkName string, networkObj *network) error {
	// Initialize the HTTP client for the network
	httpClient := dinHttp.NewHTTPClient(time.Duration(networkObj.HCTimeout) * time.Second)
	d.logger.Debug("Registered network", zap.String("name", networkName))

	// Set network dependencies
	networkObj.HttpClient = httpClient
	networkObj.Logger = d.logger
	networkObj.PrometheusClient = d.PrometheusClient
	networkObj.MachineID = d.machineID

	// Initialize providers
	for _, provider := range networkObj.Providers {
		if err := d.initializeProvider(networkName, provider, httpClient, d.logger); err != nil {
			return fmt.Errorf("error initializing provider: %w", err)
		}
	}
	return nil
}

// validateNetworkConfiguration validates the network's method filter configuration
func (d *DinMiddleware) validateNetworkConfiguration(networkName string, networkObj *network) error {
	// Validate that all routed methods are offered by at least one provider
	if networkObj.MethodFilter != nil {
		// Type assert to *methodFilter to access FilteredMethods
		if mf, ok := networkObj.MethodFilter.(*methodFilter); ok {
			for method := range mf.FilteredMethods {
				match := false
				for _, provider := range networkObj.Providers {
					if _, ok := provider.Methods[method]; ok {
						match = true
						break
					}
				}
				if !match {
					d.logger.Warn("Method marked as routed, but not offered by any providers",
						zap.String("network", networkName),
						zap.String("method", method))
				}
			}
		}
	}
	return nil
}

// startBackgroundServices starts health checks and registry sync
func (d *DinMiddleware) startBackgroundServices() error {
	// Start health checks
	if err := d.startHealthChecks(); err != nil {
		return fmt.Errorf("error starting healthchecks: %w", err)
	}

	// Start registry sync if enabled
	if d.Registry.Enabled {
		d.logger.Info("Din registry is enabled, pulling data from the registry")
		d.startRegistrySync()
	}

	// Check if we need to start the periodic updates for the watcher scores
	if d.DynamicLoadBalancing.Enabled {
		d.logger.Info("[DYNAMIC_LB] Dynamic load balancing enabled, starting periodic updates for watcher scores", zap.Duration("frequency_interval", WatcherScoreUpdateInterval))
		d.DynamicLoadBalancing.watcherScoreComputeQuit = d.DynamicLoadBalancing.watcherScoreManager.StartPeriodicUpdates(WatcherScoreUpdateInterval)
		// If the sync interval is greater than 0, start the watcher score sync goroutine
		if d.DynamicLoadBalancing.WatcherScoreSyncIntervalSec > 0 {
			d.logger.Info("[DYNAMIC_LB] Dynamic load balancing enabled, syncing watcher scores to the middleware", zap.Duration("frequency_interval", time.Duration(d.DynamicLoadBalancing.WatcherScoreSyncIntervalSec)*time.Second))
			d.DynamicLoadBalancing.watcherScoreSyncQuit = d.startWatcherScoreSync()
		}
	}

	return nil
}

// initializeProvider initializes the provider's upstream, path, logger and HTTP client
func (d *DinMiddleware) initializeProvider(networkName string, provider *provider, httpClient *dinHttp.HTTPClient, logger *logger.LoggerClient) error {

	parsedUrl, err := url.Parse(provider.HttpUrl)
	if err != nil {
		d.logger.Error("Error parsing provider URL",
			zap.String("http_url", provider.HttpUrl),
			zap.Error(err))
		return fmt.Errorf("error parsing provider URL: %w", err)
	}

	dialHost := parsedUrl.Host
	if parsedUrl.Scheme == "https" && parsedUrl.Port() == "" {
		dialHost = parsedUrl.Host + ":443"
	}

	provider.Upstream = &reverseproxy.Upstream{Dial: dialHost}
	// For providers with no path or root path, we want to send requests to root
	if parsedUrl.Path == "" {
		provider.Path = "/"
	} else {
		provider.Path = parsedUrl.Path
	}
	provider.Query = parsedUrl.RawQuery

	// Note: Authentication credentials from URL (username@host) are preserved in the URL
	// and handled during request construction, not converted to Authorization headers

	// Only set host if it hasn't been set already
	// This should have been set in UnmarshalCaddyfile, but set it here as a fallback
	if provider.Host == "" {
		d.logger.Warn("Provider host was empty in initializeProvider, setting it now",
			zap.String("network", networkName),
			zap.String("url", provider.HttpUrl))
		provider.Host = d.ensureUniqueProviderHost(networkName, parsedUrl, provider.Headers)
	}

	// Initialize authentication
	if authClient := provider.AuthClient(); authClient != nil {
		if err := authClient.Start(logger.Logger); err != nil {
			d.logger.Error("Failed to start auth client", zap.String("provider", provider.HttpUrl), zap.Error(err))
			return err
		}
	}
	provider.Logger = d.logger

	// Initialize the score for the provider with an empty score
	provider.SafeUpdateScore(ws.NewEmptyScore())

	d.logger.Debug("Provider provisioned", zap.String("Provider", provider.HttpUrl), zap.String("Host", provider.Host), zap.String("Name", provider.Name), zap.Int("Priority", provider.Priority), zap.Any("Headers", provider.Headers), zap.Bool("HasAuth", provider.AuthClient() != nil), zap.Any("Upstream", provider.Upstream), zap.String("Path", provider.Path), zap.String("Query", provider.Query))

	// Make sure blockHistory is initialized
	if provider.BlockHistory == nil {
		provider.BlockHistory = list.New()
	}

	return nil
}

// Cleanup implements caddy.CleanerUpper and is called when Caddy shuts down or reloads.
// It ensures all goroutines are properly terminated and resources are cleaned up.
func (d *DinMiddleware) Cleanup() error {
	var cleanupErr error

	d.cleanupOnce.Do(func() {
		d.logger.Info("Starting graceful shutdown of DIN middleware")

		// Close all network healthcheck goroutines
		for name, network := range d.Networks {
			d.logger.Debug("Closing network resources", zap.String("network", name))
			if network.Quit != nil {
				close(network.Quit)
			}
		}

		// Close middleware-level goroutines (registry sync)
		if d.quit != nil {
			d.logger.Debug("Signaling shutdown to registry sync goroutine")
			close(d.quit)
		}

		// Close middleware-level goroutine that syncs the watcher scores to the middleware
		if d.DynamicLoadBalancing.watcherScoreSyncQuit != nil {
			d.logger.Debug("Signaling shutdown to watcher score sync goroutine")
			close(d.DynamicLoadBalancing.watcherScoreSyncQuit)
		}

		// Close middleware-level goroutine that computes the watcher scores
		if d.DynamicLoadBalancing.watcherScoreComputeQuit != nil {
			d.logger.Debug("Signaling shutdown to watcher score compute goroutine")
			close(d.DynamicLoadBalancing.watcherScoreComputeQuit)
		}

		d.logger.Info("DIN middleware shutdown complete")
	})

	return cleanupErr
}
