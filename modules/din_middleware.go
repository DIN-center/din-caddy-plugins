package modules

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	dinHttp "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	networklib "github.com/DIN-center/din-caddy-plugins/lib/network"
	prom "github.com/DIN-center/din-caddy-plugins/lib/prometheus"
	"github.com/DIN-center/din-caddy-plugins/lib/utils"
	"github.com/DIN-center/din-sc/apps/din-go/lib/din"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"container/list"

	"encoding/json"

	"github.com/DIN-center/din-caddy-plugins/lib/auth/siwe"
)

var (
	// Initializations of extended Caddy Module Interface Guards
	// https://caddyserver.com/docs/extending-caddy

	// Din Middleware Module
	_ caddy.Module                = (*DinMiddleware)(nil)
	_ caddy.Provisioner           = (*DinMiddleware)(nil)
	_ caddyhttp.MiddlewareHandler = (*DinMiddleware)(nil)
	_ caddyfile.Unmarshaler       = (*DinMiddleware)(nil)
	// _ caddy.Validator			= (*mod.DinMiddleware)(nil)
)

type DinMiddleware struct {
	// A map of network paths to network objects
	Networks map[string]*network `json:"networks"`
	mu       sync.RWMutex
	// The current environment (prod, beta, dev)
	Env utils.Environment

	// The default siwe signer object
	DefaultSiweSigner *siwe.SigningConfig

	// The Caddy port to listen on
	CaddyPort string

	// The default siwe signer client
	SiweSignerClient siwe.ISIWESignerClient

	// The prometheus client object
	PrometheusClient *prom.PrometheusClient

	// The dingo client object
	DingoClient din.IDingoClient

	logger *logger.LoggerClient

	// The unique machine ID for the current running server instance
	machineID string

	// Test mode flag, should only be used for unit/integration testing purposes.
	testMode bool

	// Handler registry for different network types
	handlerRegistry *networklib.HandlerRegistry

	// DIN Registry configuration
	// The flag to enable or disable the din registry
	RegistryEnabled bool
	// The interval in seconds to check the latest block number from the registry
	RegistryBlockCheckIntervalSec uint64
	// The epoch in blocks to check the latest block number from the registry.
	// For example, if the epoch is 10, then the din registry will be synced every 10 blocks.
	RegistryBlockEpoch uint64
	// The block number in which the registry was updated last
	registryLastUpdatedEpochBlockNumber uint64
	// The blockchain network to pull the registry data from. ie linea-mainnet or linea-sepolia
	RegistryEndpointUrl string
	// The contract address of the registry contract
	RegistryContractAddress string
	// The priority of the registry providers
	RegistryPriority int

	// The channel to quit the goroutines
	quit chan struct{}
}

// CaddyModule returns the Caddy module information.
func (DinMiddleware) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.din",
		New: func() caddy.Module { return new(DinMiddleware) },
	}
}

// Provision() is called by Caddy to prepare the middleware for use.
// It is called only once, when the server is starting.
func (d *DinMiddleware) Provision(context caddy.Context) error {
	if len(d.Networks) == 0 && !d.RegistryEnabled {
		return fmt.Errorf("expected at least 1 network or registry to be defined")
	}

	// set the initialize the dinMiddlewareObject
	err := d.initialize(context)
	if err != nil {
		return fmt.Errorf("error initializing middleware: %v", err)
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
	if d.RegistryBlockCheckIntervalSec == 0 {
		d.RegistryBlockCheckIntervalSec = DefaultRegistryBlockCheckIntervalSec
	}
	if d.RegistryBlockEpoch == 0 {
		d.RegistryBlockEpoch = DefaultRegistryBlockEpoch
	}
	if d.RegistryPriority == 0 {
		d.RegistryPriority = DefaultRegistryPriority
	}
	if d.CaddyPort == "" {
		d.CaddyPort = DefaultPort
	}
}

// initializeDinRegistryClient initializes the DIN registry client
func (d *DinMiddleware) initializeDinRegistryClient() error {
	client, err := din.NewDinClient(d.logger.Logger, d.RegistryEndpointUrl, d.RegistryContractAddress)
	if err != nil {
		return fmt.Errorf("error initializing din client: %v", err)
	}
	d.DingoClient = client
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
		if err := d.initializeProvider(provider, httpClient, d.logger); err != nil {
			return fmt.Errorf("error initializing provider: %v", err)
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
		return fmt.Errorf("error starting healthchecks: %v", err)
	}

	// Start registry sync if enabled
	if d.RegistryEnabled {
		d.logger.Info("Din registry is enabled, pulling data from the registry")
		d.startRegistrySync()
	}

	return nil
}

// initializeProvider initializes the provider's upstream, path, logger and HTTP client
func (d *DinMiddleware) initializeProvider(provider *provider, httpClient *dinHttp.HTTPClient, logger *logger.LoggerClient) error {

	url, err := url.Parse(provider.HttpUrl)
	if err != nil {
		d.logger.Error("Error parsing provider URL",
			zap.String("http_url", provider.HttpUrl),
			zap.Error(err))
		return fmt.Errorf("error parsing provider URL: %v", err)
	}

	dialHost := url.Host
	if url.Scheme == "https" && url.Port() == "" {
		dialHost = url.Host + ":443"
	}

	provider.upstream = &reverseproxy.Upstream{Dial: dialHost}
	// For providers with no path or root path, we want to send requests to root
	if url.Path == "" {
		provider.path = "/"
	} else {
		provider.path = url.Path
	}

	// Note: Authentication credentials from URL (username@host) are preserved in the URL
	// and handled during request construction, not converted to Authorization headers

	// Only set host if it hasn't been set already
	if provider.host == "" {
		provider.host = url.Host
	}

	if provider.Auth != nil {
		if err := provider.Auth.Start(logger.Logger); err != nil {
			d.logger.Warn("Error starting authentication", zap.String("provider", provider.HttpUrl))
		}
	}
	provider.logger = d.logger
	d.logger.Debug("Provider provisioned", zap.String("Provider", provider.HttpUrl), zap.String("Host", provider.host), zap.Int("Priority", provider.Priority), zap.Any("Headers", provider.Headers), zap.Any("Auth", provider.Auth), zap.Any("Upstream", provider.upstream), zap.Any("Path", provider.path))

	// Make sure blockHistory is initialized
	if provider.blockHistory == nil {
		provider.blockHistory = list.New()
	}

	return nil
}

// ServeHTTP is the main handler for the middleware that is ran for every request.
// It checks if the network path is defined in the networks map and sets the provider in the context.
func (d *DinMiddleware) ServeHTTP(rw http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {

	d.mu.RLock()
	defer d.mu.RUnlock()

	// Caddy replacer is used to set the context for the request
	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)

	// Extract network path - for REST APIs, this is just the first segment
	fullPath := strings.TrimPrefix(r.URL.Path, "/")
	pathSegments := strings.Split(fullPath, "/")
	networkPath := pathSegments[0] // Get the first segment as network name

	networkObj, ok := d.Networks[networkPath]
	if !ok {
		// If the network is not defined, return a 404. If the network path is empty, return an empty JSON object with a 200
		if networkPath == "" {
			rw.WriteHeader(200)
			rw.Write([]byte("{}"))
			return nil
		}

		rw.WriteHeader(404)
		rw.Write([]byte("Not Found\n"))
		return fmt.Errorf("network undefined")
	}

	// Ensure handler is available
	if networkObj.handler == nil {
		d.logger.Error("No handler available for network", zap.String("network", networkPath))
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Internal Server Error\n"))
		return fmt.Errorf("no handler available for network %s", networkPath)
	}

	// Store network object in replacer for later use
	repl.Set("network_object", networkObj)

	// Middleware focuses on request validation only
	// DinSelect will handle all REST API path processing during provider configuration

	// Process the request using the handler for validation only
	if err := networkObj.handler.ProcessRequest(r); err != nil {
		d.logger.Error("Handler failed to process request", zap.String("network", networkPath), zap.Error(err))
		rw.WriteHeader(http.StatusBadRequest)
		rw.Write([]byte("Bad Request\n"))
		return fmt.Errorf("handler failed to process request: %w", err)
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
		rw.Write([]byte("Request payload too large\n"))
		return fmt.Errorf("request payload too large")
	}

	// Check to see if the request is empty (only for JSON-RPC requests)
	// REST APIs like beacon chain can have empty bodies for GET requests
	if len(bodyBytes) == 0 && networkObj.handler.GetRequestType() == networklib.RequestTypeRPC {
		// if the request body is empty for JSON-RPC, do not increment the prometheus metric, return an error
		// this is specifically for OPTIONS requests and invalid JSON-RPC payload bodies
		rw.WriteHeader(http.StatusBadRequest)
		rw.Write([]byte("Request body is empty\n"))
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

	reqStartTime := time.Now()

	// Track if we should log metrics at the end (only for final outcomes)
	var shouldLogMetrics bool

	// Retry the request if it fails up to the max attempt request count
	// Retries occur when:
	// 1. HTTP errors (non-200 status codes or upstream errors)
	// 2. Retryable JSON-RPC errors (server errors, timeouts, rate limits, etc.)
	// Non-retryable JSON-RPC errors (method not found, invalid params) will not trigger retries
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
				// For non-2xx responses, create an HTTP error
				appError = fmt.Errorf("HTTP error: %d", rww.statusCode)
			}
			if appError == nil {
				// Request was successful
				shouldLogMetrics = true
				break
			}

			// Use the method we already extracted
			var params json.RawMessage

			// Check if the error is retryable using the handler
			if !networkObj.handler.IsRetryableError(appError, rww.statusCode) {
				// Non-retryable error
				// Log this for debugging purposes since we won't retry
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
				HostName:       r.Host,
				ResponseStatus: statusCode,
				HealthStatus:   "unhealthy", // All providers failed
				Environment:    string(d.Env),
			}, duration, nil)
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

// getNetworkNames returns a slice of available network names for debugging
func (d *DinMiddleware) getNetworkNames() []string {
	names := make([]string, 0, len(d.Networks))
	for name := range d.Networks {
		names = append(names, name)
	}
	return names
}

// UnmarshalCaddyfile sets up reverse proxy provider and method data on the serve based on the configuration of the Caddyfile
func (d *DinMiddleware) UnmarshalCaddyfile(dispenser *caddyfile.Dispenser) error {
	var err error
	var caddyPort string
	if d.Networks == nil {
		d.Networks = make(map[string]*network)
	}
	d.Env = utils.GetEnv()
	siweSignerClient := siwe.NewSIWESignerClient()

	// Initialize basic logger early for deprecation warnings during parsing
	if d.logger == nil {
		d.logger = logger.NewLoggerClient(zap.NewNop(), d.Env)
	}

	// Initialize handler registry early for validation during parsing
	if d.handlerRegistry == nil {
		d.handlerRegistry = networklib.DefaultRegistry
		networklib.RegisterBuiltinHandlers()
	}
	for dispenser.Next() { // Skip the directive name
		switch dispenser.Val() {
		case "port":
			dispenser.Next()
			caddyPort = dispenser.Val()
			if caddyPort == "" {
				caddyPort = DefaultPort
			}
			d.CaddyPort = caddyPort
		case "siwe-signer":
			var key []byte
			for n1 := dispenser.Nesting(); dispenser.NextBlock(n1); {
				switch dispenser.Val() {
				case "secret_file":
					dispenser.NextBlock(n1)
					hexKeyBytes, err := ioutil.ReadFile(dispenser.Val())
					if err != nil {
						return dispenser.Errf("failed to read secret file: %v", err)
					}
					hexKey := string(hexKeyBytes)
					hexKey = strings.TrimSpace(strings.TrimPrefix(hexKey, "0x"))
					key, err = hex.DecodeString(hexKey)
					if err != nil {
						return err
					}
				case "secret":
					dispenser.NextBlock(n1)
					hexKey := dispenser.Val()
					hexKey = strings.TrimSpace(strings.TrimPrefix(hexKey, "0x"))
					key, err = hex.DecodeString(hexKey)
					if err != nil {
						return dispenser.Errf("error parsing %v: %v", hexKey, err.Error())
					}
				}
			}
			if len(key) == 0 {
				return dispenser.Errf("no key material in siwe-signer definition")
			}
			d.DefaultSiweSigner = &siwe.SigningConfig{
				PrivateKey: key,
			}
			if err := siweSignerClient.GenPrivKey(d.DefaultSiweSigner); err != nil {
				return err
			}
		case "networks":
			for n1 := dispenser.Nesting(); dispenser.NextBlock(n1); {
				networkName := dispenser.Val()
				if caddyPort == "" {
					caddyPort = DefaultPort
				}
				// Create a new network if it doesn't exist
				if _, exists := d.Networks[networkName]; !exists {
					// Create network without type - will be set explicitly via 'type' field
					newNetwork, err := NewNetwork(networkName, "", d.Env, caddyPort)
					if err != nil {
						return fmt.Errorf("failed to create network '%s': %w", networkName, err)
					}
					d.Networks[networkName] = newNetwork // Create a new network object
				}
				for nesting := dispenser.Nesting(); dispenser.NextBlock(nesting); {
					switch dispenser.Val() {
					case "methods":
						d.Networks[networkName].Methods = make([]*string, dispenser.CountRemainingArgs())
						for i := 0; i < dispenser.CountRemainingArgs(); i++ {
							d.Networks[networkName].Methods[i] = new(string)
						}
						if !dispenser.Args(d.Networks[networkName].Methods...) {
							return dispenser.Errf("invalid 'methods' argument for network %s", networkName)
						}
					case "handler":
						dispenser.Next()
						explicitType := dispenser.Val()
						d.Networks[networkName].HandlerType = HandlerType(explicitType)
						// Handler creation deferred to Provision phase for proper logger initialization
					case "routed_methods":
						methods := make([]*string, dispenser.CountRemainingArgs())
						for i := 0; i < dispenser.CountRemainingArgs(); i++ {
							methods[i] = new(string)
						}
						if !dispenser.Args(methods...) {
							return dispenser.Errf("invalid 'routed_methods' argument for network %s", networkName)
						}
						methodMap := make(map[string]struct{})
						for _, method := range methods {
							methodMap[*method] = struct{}{}
						}
						d.Networks[networkName].MethodFilter = &methodFilter{
							FilteredMethods: methodMap,
						}
					case "providers":
						for dispenser.NextBlock(nesting + 1) {
							providerObj, err := NewProvider(dispenser.Val())
							if err != nil {
								return fmt.Errorf("error creating provider: %v", err)
							}
							for dispenser.NextBlock(nesting + 2) {
								switch dispenser.Val() {
								case "methods":
									methods := make([]*string, dispenser.CountRemainingArgs())
									for i := 0; i < dispenser.CountRemainingArgs(); i++ {
										methods[i] = new(string)
									}
									if !dispenser.Args(methods...) {
										return dispenser.Errf("invalid 'methods' argument for provider %s", providerObj.HttpUrl)
									}
									providerObj.Methods = make(map[string]struct{})
									for _, method := range methods {
										providerObj.Methods[*method] = struct{}{}
									}
								case "auth":
									auth := siweSignerClient.CreateNewSIWEAuth(strings.TrimSuffix(providerObj.HttpUrl, "/")+"/auth", 16)
									for dispenser.NextBlock(nesting + 3) {
										switch dispenser.Val() {
										case "type":
											dispenser.NextBlock(nesting + 3)
											if dispenser.Val() != "siwe" {
												return fmt.Errorf("unknown auth type")
											}
										case "url":
											dispenser.NextBlock(nesting + 3)
											auth.ProviderURL = dispenser.Val()
										case "sessions":
											dispenser.NextBlock(nesting + 3)
											auth.SessionCount, err = strconv.Atoi(dispenser.Val())
											if err != nil {
												return fmt.Errorf("invalid session count: %v", err)
											}
										case "signer":
											var key []byte
											for dispenser.NextBlock(nesting + 4) {
												switch dispenser.Val() {
												case "secret_file":
													dispenser.NextBlock(nesting + 4)
													hexKeyBytes, err := os.ReadFile(dispenser.Val())
													if err != nil {
														return dispenser.Errf("failed to read secret file: %v", err)
													}
													hexKey := string(hexKeyBytes)
													hexKey = strings.TrimSpace(strings.TrimPrefix(hexKey, "0x"))
													key, err = hex.DecodeString(hexKey)
													if err != nil {
														return fmt.Errorf("failed to decode secret file: %v", err)
													}
												case "secret":
													dispenser.NextBlock(nesting + 4)
													hexKey := dispenser.Val()
													hexKey = strings.TrimSpace(strings.TrimPrefix(hexKey, "0x"))
													key, err = hex.DecodeString(hexKey)
													if err != nil {
														return fmt.Errorf("failed to decode secret: %v", err)
													}
												}
											}
											auth.Signer = &siwe.SigningConfig{
												PrivateKey: key,
											}
											if err := siweSignerClient.GenPrivKey(auth.Signer); err != nil {
												return fmt.Errorf("failed to generate private key: %v", err)
											}
										}
									}
									if auth.Signer == nil {
										if d.DefaultSiweSigner == nil {
											return dispenser.Errf("signer must be set")
										}
										auth.Signer = d.DefaultSiweSigner
									}
									providerObj.Auth = auth
								case "headers":
									for dispenser.NextBlock(nesting + 3) {
										k := dispenser.Val()
										var v string
										if dispenser.Args(&v) {
											providerObj.Headers[k] = v
										} else {
											return dispenser.Errf("header should have key and value")
										}
									}
								case "priority":
									dispenser.NextBlock(nesting + 2)
									providerObj.Priority, err = strconv.Atoi(dispenser.Val())
									if err != nil {
										return fmt.Errorf("invalid priority: %v", err)
									}
								}
							}
							// Parse the URL to get the host
							parsedUrl, err := url.Parse(providerObj.HttpUrl)
							if err != nil {
								return fmt.Errorf("error parsing provider URL: %v", err)
							}

							// Initialize provider with a unique host
							providerObj.host = d.ensureUniqueProviderHost(networkName, parsedUrl.Host)
							d.Networks[networkName].Providers[providerObj.host] = providerObj
						}
					case "healthcheck_endpoint":
						dispenser.Next()
						d.Networks[networkName].HCEndpoint = dispenser.Val()
					case "chain_id":
						dispenser.Next()
						chainId := dispenser.Val()
						if chainId == "" {
							return fmt.Errorf("chain ID cannot be empty for network %s", networkName)
						}
						d.Networks[networkName].ChainId = chainId
					case "healthcheck_threshold":
						dispenser.Next()
						d.Networks[networkName].HCThreshold, err = strconv.Atoi(dispenser.Val())
						if err != nil {
							return fmt.Errorf("invalid healthcheck threshold: %v", err)
						}
					case "healthcheck_timeout":
						dispenser.Next()
						d.Networks[networkName].HCTimeout, err = strconv.Atoi(dispenser.Val())
						if err != nil {
							return fmt.Errorf("invalid healthcheck timeout: %v", err)
						}
					case "healthcheck_interval":
						dispenser.Next()
						d.Networks[networkName].HCInterval, err = strconv.Atoi(dispenser.Val())
						if err != nil {
							return fmt.Errorf("invalid healthcheck interval: %v", err)
						}
					case "healthcheck_blocklag_limit":
						dispenser.Next()
						limit, err := strconv.Atoi(dispenser.Val())
						if err != nil {
							return fmt.Errorf("invalid healthcheck blocklag limit: %v", err)
						}
						d.Networks[networkName].BlockLagLimit = int64(limit)
					case "healthcheck_blockjump_limit":
						dispenser.Next()
						limit, err := strconv.Atoi(dispenser.Val())
						if err != nil {
							return fmt.Errorf("invalid healthcheck blockjump limit: %v", err)
						}
						d.Networks[networkName].BlockJumpLimit = int64(limit)
					case "healthcheck_provider_block_history_size":
						dispenser.Next()
						size, err := strconv.Atoi(dispenser.Val())
						if err != nil {
							return fmt.Errorf("invalid healthcheck provider block history size: %v", err)
						}
						d.Networks[networkName].ProviderBlockHistorySize = int(size)
					case "network_block_history_size":
						dispenser.Next()
						size, err := strconv.Atoi(dispenser.Val())
						if err != nil {
							return fmt.Errorf("invalid network block history size: %v", err)
						}
						d.Networks[networkName].NetworkBlockHistorySize = int(size)
					case "max_request_payload_size_kb":
						dispenser.Next()
						size, err := strconv.Atoi(dispenser.Val())
						if err != nil {
							return fmt.Errorf("invalid max request payload size: %v", err)
						}
						d.Networks[networkName].MaxRequestPayloadSizeKB = int64(size)
					case "request_attempt_count":
						dispenser.Next()
						requestAttemptCount, err := strconv.Atoi(dispenser.Val())
						if err != nil {
							return fmt.Errorf("invalid request attempt count: %v", err)
						}
						d.Networks[networkName].RequestAttemptCount = requestAttemptCount
					case "archive_enabled":
						dispenser.Next()
						archiveEnabled, err := strconv.ParseBool(dispenser.Val())
						if err != nil {
							return fmt.Errorf("invalid archive enabled: %v", err)
						}
						d.Networks[networkName].ArchiveEnabled = archiveEnabled
					default:
						return dispenser.Errf("unrecognized option: %s", dispenser.Val())
					}
				}
				// Validate that required fields are set
				if d.Networks[networkName].ChainId == "" {
					return fmt.Errorf("chain ID is not set for network %s", networkName)
				}
				if d.Networks[networkName].HandlerType == "" {
					// DEFAULT to EVM for unspecified handler types (most common blockchain type)
					d.logger.Info("No explicit handler type specified, defaulting to EVM",
						zap.String("network", networkName))
					d.Networks[networkName].HandlerType = EVMHandler
					// Handler creation deferred to Provision phase for proper logger initialization
				}
			}
		case "din_registry":
			for n1 := dispenser.Nesting(); dispenser.NextBlock(n1); {
				switch dispenser.Val() {
				case "registry_enabled":
					dispenser.Next()
					registryEnabledVal := dispenser.Val()
					// Convert string to bool
					boolValue, err := strconv.ParseBool(registryEnabledVal)
					if err != nil {
						return dispenser.Errf("Error converting string to bool: %v", err)
					}
					d.RegistryEnabled = boolValue
				case "registry_block_epoch":
					dispenser.Next()
					registryBlockEpochlVal := dispenser.Val()
					// Convert string to int64
					intValue, err := strconv.Atoi(registryBlockEpochlVal)
					if err != nil {
						return dispenser.Errf("Error converting string to int: %v", err)
					}
					d.RegistryBlockEpoch = uint64(intValue)
				case "registry_block_check_interval_sec":
					dispenser.Next()
					registryBlockCheckIntervalSecVal := dispenser.Val()
					// Convert string to int64
					intValue, err := strconv.Atoi(registryBlockCheckIntervalSecVal)
					if err != nil {
						return dispenser.Errf("Error converting string to int: %v", err)
					}
					d.RegistryBlockCheckIntervalSec = uint64(intValue)
				case "registry_endpoint_url":
					dispenser.Next()
					registryEndpointUrl := dispenser.Val()
					d.RegistryEndpointUrl = registryEndpointUrl
				case "registry_contract_address":
					dispenser.Next()
					registryContractAddress := dispenser.Val()
					d.RegistryContractAddress = registryContractAddress
				case "registry_priority":
					dispenser.Next()
					registryPriorityVal := dispenser.Val()
					intValue, err := strconv.Atoi(registryPriorityVal)
					if err != nil {
						return dispenser.Errf("Error converting string to int: %v", err)
					}
					d.RegistryPriority = intValue
				}
			}
		}

	}

	return nil
}

func (d *DinMiddleware) ParseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	err := d.UnmarshalCaddyfile(h.Dispenser)
	if err != nil {
		return nil, err
	}

	return d, nil
}

// StartHealthchecks starts a background goroutine to monitor all of the networks' overall health and the health of its providers
func (d *DinMiddleware) startHealthChecks() error {
	d.logger.Info("Starting healthchecks")
	for _, network := range d.Networks {
		d.logger.Info("Starting healthcheck for network", zap.String("network", network.Name))
		network.startHealthcheck()
	}
	return nil
}

// startRegistrySync initiates a periodic synchronization process with the registry. It retrieves data from the
// registry and processes it immediately. A ticker is started to poll the latest block number from the
// Linea network at regular intervals (default 60 seconds). If the latest block number has moved beyond
// the defined block epoch, it retrieves new registry data and processes it. The function runs in a separate
// goroutine and will terminate when a quit signal is received.
func (d *DinMiddleware) startRegistrySync() {
	// Get the initial registry data
	registryData, err := d.DingoClient.GetRegistryData()
	if err != nil {
		d.logger.Error("Failed to initialize registry sync", zap.Error(err))
	}
	d.processRegistryData(registryData)
	// Start a ticker to check the linea network latest block number on a time interval of 60 seconds by default.
	ticker := time.NewTicker(time.Second * time.Duration(d.RegistryBlockCheckIntervalSec))
	go func() {
		// Keep an index for RPC request IDs
		for i := 0; ; i++ {
			select {
			case <-d.quit:
				ticker.Stop()
				return
			case <-ticker.C:
				d.syncRegistryWithLatestBlock()
			}
		}
	}()
}

func (d *DinMiddleware) closeAll() {
	for _, network := range d.Networks {
		network.close()
	}
	d.close()
}

func (d *DinMiddleware) close() {
	close(d.quit)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
