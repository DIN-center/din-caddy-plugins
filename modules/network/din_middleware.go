package network

import (
	"bytes"
	"container/list"
	"encoding/json"
	stdliberrors "errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	ws "github.com/DIN-center/din-caddy-plugins/lib/watcherscore"
	"github.com/DIN-center/din-sc/apps/din-go/lib/din"
	"github.com/DIN-center/din-sc/apps/din-go/lib/watcher"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/DIN-center/din-caddy-plugins/lib/auth/siwe"
	dinHttp "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	networklib "github.com/DIN-center/din-caddy-plugins/lib/network"
	prom "github.com/DIN-center/din-caddy-plugins/lib/prometheus"
	"github.com/DIN-center/din-caddy-plugins/lib/utils"
	"github.com/DIN-center/din-caddy-plugins/lib/web3"
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
	_ caddy.CleanerUpper          = (*DinMiddleware)(nil)
	// _ caddy.Validator            = (*mod.DinMiddleware)(nil)
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

// Provision() is called by Caddy to prepare the middleware for use.
// It is called only once, when the server is starting.
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
	if networkObj.handler == nil && networkObj.HandlerType != "" {
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
	} else if networkObj.handler != nil {
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
	networkObj.logger = d.logger
	networkObj.PrometheusClient = d.PrometheusClient
	networkObj.machineID = d.machineID

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
		for method := range networkObj.MethodFilter.FilteredMethods {
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

	provider.upstream = &reverseproxy.Upstream{Dial: dialHost}
	// For providers with no path or root path, we want to send requests to root
	if parsedUrl.Path == "" {
		provider.path = "/"
	} else {
		provider.path = parsedUrl.Path
	}
	provider.query = parsedUrl.RawQuery

	// Note: Authentication credentials from URL (username@host) are preserved in the URL
	// and handled during request construction, not converted to Authorization headers

	// Only set host if it hasn't been set already
	// This should have been set in UnmarshalCaddyfile, but set it here as a fallback
	if provider.host == "" {
		d.logger.Warn("Provider host was empty in initializeProvider, setting it now",
			zap.String("network", networkName),
			zap.String("url", provider.HttpUrl))
		provider.host = d.ensureUniqueProviderHost(networkName, parsedUrl, provider.Headers)
	}

	// Initialize authentication
	if provider.OIDCClient != nil {
		// Initialize OIDC client
		if err := provider.OIDCClient.Start(logger.Logger); err != nil {
			d.logger.Error("Failed to start OIDC client", zap.String("provider", provider.HttpUrl), zap.Error(err))
			return err
		}
	} else if provider.Auth != nil {
		// Initialize SIWE auth
		if err := provider.Auth.Start(logger.Logger); err != nil {
			d.logger.Warn("Error starting authentication", zap.String("provider", provider.HttpUrl))
		}
	}
	provider.logger = d.logger

	// Initialize the score for the provider with an empty score
	provider.SafeUpdateScore(ws.NewEmptyScore())

	d.logger.Debug("Provider provisioned", zap.String("Provider", provider.HttpUrl), zap.String("Host", provider.host), zap.String("Name", provider.Name), zap.Int("Priority", provider.Priority), zap.Any("Headers", provider.Headers), zap.Any("Auth", provider.Auth), zap.Any("Upstream", provider.upstream), zap.String("Path", provider.path), zap.String("Query", provider.query))

	// Make sure blockHistory is initialized
	if provider.blockHistory == nil {
		provider.blockHistory = list.New()
	}

	return nil
}

// ServeHTTP is the main handler for the middleware that is ran for every request.
// It checks if the network path is defined in the networks map and sets the provider in the context.
func (d *DinMiddleware) ServeHTTP(rw http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error { //nolint:gocyclo

	// Caddy replacer is used to set the context for the request
	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)

	// Extract network path - for REST APIs, this is just the first segment
	fullPath := strings.TrimPrefix(r.URL.Path, "/")
	pathSegments := strings.Split(fullPath, "/")
	networkPath := pathSegments[0] // Get the first segment as network name

	// If the network path is empty, return an empty JSON object with a 200.
	if networkPath == "" {
		rw.WriteHeader(http.StatusOK)
		_, err := rw.Write([]byte("{}"))

		return err
	}

	// Safely access the networks map for guaranteed consistency
	// It assumes that the network object is immutable after the lock is released or
	// that any internal shared state in the network object is protected by a granular lock (at the shared state level)
	// IMPORTANT NOTE: network objects are modified by the DIN Registry (e.g. adding/removing providers, methods, etc.)
	// This means that DIN Registry cannot be enabled without a refactor of the middleware to protect the shared state at the granular level
	d.mu.RLock()
	networkObj, ok := d.Networks[networkPath]
	d.mu.RUnlock()
	if !ok {
		// If the network is not defined, return a 404.
		rw.WriteHeader(http.StatusNotFound)
		_, err := rw.Write([]byte("Not Found\n"))

		return fmt.Errorf("network undefined: %w", err)
	}

	// Ensure handler is available
	if networkObj.handler == nil {
		d.logger.Error("No handler available for network", zap.String("network", networkPath))
		rw.WriteHeader(http.StatusInternalServerError)
		_, err := rw.Write([]byte("Internal Server Error\n"))

		return fmt.Errorf("no handler available for network %s: %w", networkPath, err)
	}

	// Store network object in replacer for later use
	repl.Set("network_object", networkObj)
	if api_key := r.Header.Get("Din-Api-Key"); api_key != "" {
		repl.Set("din_api_key", d.getAPIKeyId(api_key))
		r.Header.Del("Din-Api-Key") // We don't want to pass this information to providers
	} else {
		repl.Set("din_api_key", "unspecified")
	}

	// Strip Authorization header from client requests
	// The proxy handles all authentication internally
	// NOTE: This removes clients' ability to pass their own Authorization headers to providers.
	// While there is no current use case for this capability, if future requirements emerge
	// for client-controlled authentication (e.g., provider-specific bearer tokens),
	// this behavior will need to be re-evaluated.
	r.Header.Del("Authorization")

	// Middleware focuses on request validation only
	// DinSelect will handle all REST API path processing during provider configuration

	// Process the request using the handler for validation only
	if err := networkObj.handler.ProcessRequest(r); err != nil {
		d.logger.Error("Handler failed to process request", zap.String("network", networkPath), zap.Error(err))

		var (
			statusCode int
			body       string
		)

		// Check if it's an HTTPError with specific status code
		httpErr := &networklib.HTTPError{}
		if errors.As(err, &httpErr) {
			statusCode = httpErr.StatusCode
			body = httpErr.Message + "\n"
		} else {
			statusCode = http.StatusBadRequest
			body = "Bad Request\n"
		}

		rw.WriteHeader(statusCode)
		_, writeErr := rw.Write([]byte(body))

		return stdliberrors.Join(fmt.Errorf("handler failed to process request: %w", err), writeErr)
	}

	// Read request body and save in context
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil
	}
	repl.Set(RequestBodyKey, bodyBytes)
	// Set request body back to original state
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// Check if the request payload is too large
	if (len(bodyBytes) / 1024) > int(networkObj.MaxRequestPayloadSizeKB) {
		// If the request payload is too large, return an error
		rw.WriteHeader(http.StatusRequestEntityTooLarge)
		_, err := rw.Write([]byte("Request payload too large\n"))
		if err != nil {
			return errors.Wrap(err, "request payload too large")
		}

		return fmt.Errorf("request payload too large")
	}

	// Check to see if the request is empty (only for JSON-RPC requests)
	// REST APIs like beacon chain can have empty bodies for GET requests
	if len(bodyBytes) == 0 && networkObj.handler.GetRequestType() == networklib.RequestTypeRPC {
		// if the request body is empty for JSON-RPC, do not increment the prometheus metric, return an error
		// this is specifically for OPTIONS requests and invalid JSON-RPC payload bodies
		rw.WriteHeader(http.StatusBadRequest)
		_, err := rw.Write([]byte("Request body is empty\n"))
		if err != nil {
			return fmt.Errorf("error writing response: %w", err)
		}
		return fmt.Errorf("request body is empty")
	}

	// Extract the method using the handler
	method, err := networkObj.handler.ExtractMethod(r, bodyBytes)
	if err != nil {
		d.logger.Error("Failed to extract method", zap.String("network", networkPath), zap.Error(err))
		// Don't fail the request, just set method to unknown for metrics
		method = "unknown"
	}
	repl.Set(RequestMethodKey, method)

	// Create a new response writer wrapper to capture the response body and status code
	var rww *ResponseWriterWrapper

	if networkObj.MethodFilter != nil {
		// For method filtering, we need to parse the JSON-RPC request
		// Only do this if we have a body and it's a JSON-RPC request
		var parsedRequest *dinHttp.JSONRPCRequest
		if len(bodyBytes) > 0 && networkObj.handler.GetRequestType() == networklib.RequestTypeRPC {
			var rpcReq dinHttp.JSONRPCRequest
			if err := json.Unmarshal(bodyBytes, &rpcReq); err == nil {
				parsedRequest = &rpcReq
			}
		}
		// Set the upstreams in the context for the request
		repl.Set(DinUpstreamsContextKey, networkObj.MethodFilter.FilterProviders(parsedRequest, networkObj.Providers))
	} else {
		// Set the upstreams in the context for the request
		repl.Set(DinUpstreamsContextKey, networkObj.Providers)
	}

	// Set if dynamic load balancing should be done
	repl.Set(DinScoreBasedLoadBalancingContextKey, d.DynamicLoadBalancing.Enabled)

	reqStartTime := time.Now()

	// Track if we should log metrics at the end (only for final outcomes)
	var shouldLogMetrics bool

	// Track providers excluded due to method-not-found (-32601) errors.
	// These providers are skipped on subsequent attempts so a different provider is tried.
	excludedProviders := make(map[string]struct{})

	// Retry the request if it fails up to the max attempt request count
	// Retries occur when:
	// 1. HTTP errors (non-200 status codes or upstream errors)
	// 2. Retryable JSON-RPC errors (server errors, timeouts, rate limits, etc.)
	// 3. Method-not-found errors (-32601) trigger retry on a different provider
	// Non-retryable JSON-RPC errors (invalid params, revert, etc.) will not trigger retries
	for attempt := 0; attempt < networkObj.RequestAttemptCount; attempt++ {
		if err := checkRequestContext(d.logger, r, networkPath, attempt); err != nil {
			handleContextCancellation(d.logger, d.PrometheusClient, rw, r, networkPath, attempt, err, reqStartTime, networkObj)
			return nil
		}
		rww = NewResponseWriterWrapper(rw)

		// If the request fails, reset the request body and custom header if its present to the original request state
		if attempt > 0 {
			var reqBody []byte
			if v, ok := repl.Get(RequestBodyKey); ok {
				reqBody = v.([]byte)
			}
			r.Body = io.NopCloser(bytes.NewReader(reqBody))

			// Remove the custom header if it was set
			if rww.Header().Get(DinProviderInfo) != "" {
				rww.Header().Del(DinProviderInfo)
			}
		}
		// Serve the request
		err = next.ServeHTTP(rww, r)

		// Check for success and handle response
		if err == nil {
			// Get response body
			var responseBody []byte
			if rww.body != nil {
				responseBody = rww.body.Bytes()
			}

			// Decompress gzip if necessary before passing to handler
			responseBody = decompressGzipBodyIfNecessary(rww.Header(), responseBody, d.logger, networkPath)

			// Check for application-level errors using the handler
			var appError error
			if rww.statusCode >= 200 && rww.statusCode < 300 {
				// For successful HTTP responses, check for application-level errors
				appError = networkObj.handler.ParseResponse(responseBody, rww.statusCode)
			} else {
				// For non-2xx responses, create an HTTP error.
				// NOTE: Some providers return HTTP 400 with a JSON-RPC -32601 body. In that case,
				// the error will be "HTTP error: 400" and method-level failover won't trigger.
				// This is a known limitation — most providers return HTTP 200 with the JSON-RPC error.
				appError = fmt.Errorf("HTTP error: %d", rww.statusCode)
			}
			if appError == nil {
				// Request was successful
				shouldLogMetrics = true

				// Log if this success came after a method-level failover
				if len(excludedProviders) > 0 {
					successProvider := "unknown"
					successProviderName := "unknown"
					if v, ok := repl.Get(RequestProviderKey); ok {
						if pStr, ok := v.(string); ok {
							successProvider = pStr
							if providerMap, ok := repl.Get(DinUpstreamsContextKey); ok {
								if providers, ok := providerMap.(map[string]*provider); ok {
									if p, ok := providers[successProvider]; ok {
										successProviderName = p.Name
									}
								}
							}
						}
					}
					excludedList := make([]string, 0, len(excludedProviders))
					for host := range excludedProviders {
						excludedList = append(excludedList, host)
					}
					d.logger.Info("Request succeeded after method-level failover",
						zap.String("network", networkPath),
						zap.String("provider", successProvider),
						zap.String("provider_name", successProviderName),
						zap.String("request_method", method),
						zap.Int("attempt", attempt+1),
						zap.Strings("excluded_providers", excludedList),
					)
				}

				break
			}

			// Use the method we already extracted
			var params json.RawMessage

			// Check if the error is retryable using the handler
			if !networkObj.handler.IsRetryableError(appError, rww.statusCode) {
				// Check if this error can be retried on a different provider (e.g., -32601 method not found)
				if networkObj.handler.IsRetryableOnDifferentProvider(appError, rww.statusCode) {
					// Exclude the provider that returned method-not-found
					failedProvider, ok := repl.Get(RequestProviderKey)
					if !ok {
						// Cannot identify which provider failed; fall through to non-retryable path
						d.logger.Warn("Method-not-found retry skipped: provider key not available in context",
							zap.String("network", networkPath),
							zap.Error(appError))
						logFailedAttempt(LogFailedAttemptParams{
							Reason:              "Non-retryable application error",
							Logger:              d.logger,
							NetworkPath:         networkPath,
							FailedAttemptNumber: attempt + 1,
							MaxAttempts:         networkObj.RequestAttemptCount,
							StatusCodeOfFailure: rww.statusCode,
							Error:               appError,
							Replacer:            repl,
							RequestMethod:       method,
							RequestParams:       params,
							RawResponseBody:     responseBody,
						})
						break
					}
					if providerHost, ok := failedProvider.(string); ok {
						excludedProviders[providerHost] = struct{}{}
					}

					// Check if all providers are now excluded — if so, stop retrying
					if providerMap, ok := repl.Get(DinUpstreamsContextKey); ok {
						providers := providerMap.(map[string]*provider)
						if len(excludedProviders) >= len(providers) {
							logFailedAttempt(LogFailedAttemptParams{
								Reason:              "Method not supported by any provider",
								Logger:              d.logger,
								NetworkPath:         networkPath,
								FailedAttemptNumber: attempt + 1,
								MaxAttempts:         networkObj.RequestAttemptCount,
								StatusCodeOfFailure: rww.statusCode,
								Error:               appError,
								Replacer:            repl,
								RequestMethod:       method,
								RequestParams:       params,
								RawResponseBody:     responseBody,
							})
							break
						}
					}

					// Update the context so GetUpstreams excludes failed providers on next attempt
					repl.Set(DinExcludedProvidersContextKey, excludedProviders)

					// Log at INFO level since this is expected behavior — the request will be
					// retried on a different provider and is likely to succeed.
					excludedList := make([]string, 0, len(excludedProviders))
					for host := range excludedProviders {
						excludedList = append(excludedList, host)
					}
					d.logger.Info("Method not supported by provider, trying different provider",
						zap.String("network", networkPath),
						zap.String("request_method", method),
						zap.Int("attempt", attempt+1),
						zap.Int("max_attempts", networkObj.RequestAttemptCount),
						zap.Error(appError),
						zap.Strings("excluded_providers", excludedList),
					)
					continue
				}

				// Truly non-retryable error — do not retry
				logFailedAttempt(LogFailedAttemptParams{
					Reason:              "Non-retryable application error",
					Logger:              d.logger,
					NetworkPath:         networkPath,
					FailedAttemptNumber: attempt + 1,
					MaxAttempts:         networkObj.RequestAttemptCount,
					StatusCodeOfFailure: rww.statusCode,
					Error:               appError,
					Replacer:            repl,
					RequestMethod:       method,
					RequestParams:       params,
					RawResponseBody:     responseBody,
				})
				break
			}

			// Log retryable application error
			logFailedAttempt(LogFailedAttemptParams{
				Reason:              "Retryable application error",
				Logger:              d.logger,
				NetworkPath:         networkPath,
				FailedAttemptNumber: attempt + 1,
				MaxAttempts:         networkObj.RequestAttemptCount,
				StatusCodeOfFailure: rww.statusCode,
				Error:               appError,
				Replacer:            repl,
				RequestMethod:       method,
				RequestParams:       params,
				RawResponseBody:     responseBody,
			})

			continue
		}

		// Check for HTTP/low-level errors (non-200 status codes, network issues, etc.)
		if err != nil {
			// Use the method we already extracted
			var params json.RawMessage

			// Log HTTP/low-level error
			var responseBody []byte
			if rww.body != nil {
				responseBody = rww.body.Bytes()
			}

			// Determine if this is the final attempt or not
			reason := "HTTP/network error"
			if attempt == networkObj.RequestAttemptCount-1 {
				reason = "HTTP/network error (final attempt)"
				shouldLogMetrics = true
			}

			logFailedAttempt(LogFailedAttemptParams{
				Reason:              reason,
				Logger:              d.logger,
				NetworkPath:         networkPath,
				FailedAttemptNumber: attempt + 1,
				MaxAttempts:         networkObj.RequestAttemptCount,
				StatusCodeOfFailure: rww.statusCode,
				Error:               err,
				Replacer:            repl,
				RequestMethod:       method,
				RequestParams:       params,
				RawResponseBody:     responseBody,
			})
		}
	}

	// Handle final error case (all retries exhausted)
	if err != nil {
		// Get provider for metrics
		var provider string
		if v, ok := repl.Get(RequestProviderKey); ok {
			provider = v.(string)
		}

		// Get priority for metrics
		priority := 0
		if v, ok := repl.Get(RequestProviderPriorityKey); ok {
			if pInt, ok := v.(int); ok {
				priority = pInt
			}
		}

		// Get provider name for metrics
		providerName := "unknown"
		if v, ok := networkObj.Providers[provider]; ok {
			providerName = v.Name
		}

		duration := time.Since(reqStartTime)

		// Determine appropriate status code for the failure
		statusCode := http.StatusInternalServerError // Default for HTTP errors
		if rww != nil && rww.statusCode > 0 {
			statusCode = rww.statusCode
		}

		// Log metrics for final failure (only if we haven't already marked it for logging)
		if !shouldLogMetrics && !d.testMode && d.PrometheusClient != nil {
			// Record metrics for the failed request
			d.PrometheusClient.HandleRequestMetrics(&prom.PromRequestMetricData{
				Method:         method,
				Network:        networkPath,
				Provider:       provider,
				ProviderName:   providerName,
				ApiKey:         getRequestAPIKey(repl),
				HostName:       r.Host,
				ResponseStatus: statusCode,
				HealthStatus:   "unhealthy", // All providers failed
				Priority:       priority,
				Environment:    string(d.Env),
			}, duration)
		}

		return errors.Wrap(err, "Error serving HTTP")
	}

	var provider string
	if v, ok := repl.Get(RequestProviderKey); ok {
		provider = v.(string)
	}

	duration := time.Since(reqStartTime)
	// Write the response body and status to the original response writer
	// This is done after the request is attempted multiple times if needed
	if rww != nil {

		// Copy headers from wrapper to original response writer
		for k, v := range rww.Header() {
			rw.Header()[k] = v
		}

		rww.ResponseWriter.WriteHeader(rww.statusCode)

		// Write the body
		_, err := rw.Write(rww.body.Bytes())
		if err != nil {
			d.logger.Error("Error writing response body", zap.String("network", networkPath), zap.Error(err))
			return errors.Wrap(err, "Error writing response body")
		}

		// Flush the response to ensure all data is sent to the client
		if flusher, ok := rw.(http.Flusher); ok {
			flusher.Flush()
		}
	}

	// Only log metrics if this is a final outcome (success or final failure)
	if shouldLogMetrics {
		// Post-Request Processing is now handled by the helper function
		handlePostRequestTasks(PostRequestTaskParams{
			DinMiddleware: d,
			RWWrapper:     rww,
			NetworkObj:    networkObj,
			NetworkPath:   networkPath,
			Provider:      provider,
			Replacer:      repl,
			Duration:      duration,
			OriginalReq:   r,
			ParsedReqBody: nil,
		})
	}

	return nil
}

// MaxConcurrentHealthChecks limits how many networks can run health checks simultaneously
// This prevents resource exhaustion when there are many networks
const MaxConcurrentHealthChecks = 10

// StartHealthchecks starts background goroutines to monitor all networks' health
// Health checks run asynchronously with throttled concurrency to prevent resource exhaustion
func (d *DinMiddleware) startHealthChecks() error {
	d.logger.Info("Starting healthchecks asynchronously",
		zap.Int("network_count", len(d.Networks)),
		zap.Int("max_concurrent", MaxConcurrentHealthChecks))

	// Run all health checks in a background goroutine - don't block Provision()
	go func() {
		// Use a semaphore to limit concurrent health checks
		sem := make(chan struct{}, MaxConcurrentHealthChecks)
		var wg sync.WaitGroup

		for name, net := range d.Networks {
			wg.Add(1)
			go func(networkName string, n *network) {
				defer wg.Done()

				// Acquire semaphore slot (blocks if at max concurrency)
				sem <- struct{}{}
				defer func() { <-sem }()

				d.logger.Debug("Starting healthcheck for network", zap.String("network", networkName))
				n.startHealthcheck()
			}(name, net)
		}

		// Wait for all health checks to complete their initial run
		wg.Wait()
		d.logger.Info("All network healthchecks initiated")
	}()

	return nil
}

// startRegistrySync initiates a periodic synchronization process with the registry. It retrieves data from the
// registry and processes it immediately. A ticker is started to poll the latest block number from the
// Linea network at regular intervals (default 60 seconds). If the latest block number has moved beyond
// the defined block epoch, it retrieves new registry data and processes it. The function runs in a separate
// goroutine and will terminate when a quit signal is received.
func (d *DinMiddleware) startRegistrySync() {
	// Get the initial registry data with retry
	registryData, err := d.getRegistryData()
	if err != nil {
		d.logger.Error("Failed to initialize registry sync after retries",
			zap.Error(err),
			zap.Int("max_retries", d.Registry.RetryMaxAttempts))
	}
	d.processRegistryData(registryData)

	// Start a ticker to check the linea network latest block number on a time interval of 60 seconds by default.
	ticker := time.NewTicker(time.Second * time.Duration(d.Registry.BlockCheckIntervalSec))
	go func() {
		// CRITICAL: Panic recovery to prevent application crash
		// If registry sync panics, log the error and continue running
		defer func() {
			if r := recover(); r != nil {
				d.logger.Error("CRITICAL: Registry sync goroutine panicked and recovered. Application continues running.",
					zap.Any("panic", r),
					zap.Stack("stacktrace"))
				// Clean up the ticker
				ticker.Stop()

				// Restart the sync after a configurable delay to recover from transient issues
				// This prevents the sync from being permanently dead after a panic
				time.Sleep(d.Registry.PanicRecoveryDelay)
				d.logger.Info("Attempting to restart registry sync after panic recovery",
					zap.Duration("recovery_delay", d.Registry.PanicRecoveryDelay))
				d.startRegistrySync()
			}
		}()

		for {
			select {
			case <-d.quit:
				ticker.Stop()
				d.logger.Info("Registry sync goroutine shutting down gracefully")
				return
			case <-ticker.C:
				// Wrap the sync call in a function that can recover from panics
				func() {
					defer func() {
						if r := recover(); r != nil {
							d.logger.Error("Registry sync operation panicked during sync attempt",
								zap.Any("panic", r))
						}
					}()
					d.syncRegistryWithLatestBlock(web3.NewEVMClient(d.DingoClient.GetEthereumRpcClient()))
				}()
			}
		}
	}()
}

// startWatcherScoreSync starts a background goroutine to sync the watcher scores to the middleware
func (d *DinMiddleware) startWatcherScoreSync() chan struct{} {
	syncQuit := make(chan struct{})

	// Do immediate initial sync
	// Note that syncing watcher scores immediately here may be a bit early if the score computation is not yet complete,
	// but it's ok because the watcher score manager will return empty scores until the computation is complete
	// and the middleware will not use these empty scores for load balancing
	d.SyncMiddlewareWithLatestScores()

	go func() {
		ticker := time.NewTicker(time.Duration(d.DynamicLoadBalancing.WatcherScoreSyncIntervalSec) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-syncQuit:
				d.logger.Info("[DYNAMIC_LB] Watcher score sync goroutine shutting down gracefully")
				return
			case <-ticker.C:
				d.SyncMiddlewareWithLatestScores()
			}
		}
	}()
	return syncQuit
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
			if network.quit != nil {
				close(network.quit)
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
