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
	var err error
	d.machineID = utils.GetMachineId()
	loggerClient := logger.NewLoggerClient(context.Logger(d), d.Env)
	d.logger = loggerClient
	// Initialize the prometheus client on the din middleware object
	promClient := prom.NewPrometheusClient(loggerClient, d.machineID)
	d.PrometheusClient = promClient
	d.SiweSignerClient = siwe.NewSIWESignerClient()
	d.quit = make(chan struct{})

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

	// Initialize the din registry configuration values
	d.DingoClient, err = din.NewDinClient(loggerClient.Logger, d.RegistryEndpointUrl, d.RegistryContractAddress)
	if err != nil {
		return fmt.Errorf("error initializing din client: %v", err)
	}

	// Initialize the HTTP client for each network and provider
	httpClient := dinHttp.NewHTTPClient()
	for networkName, network := range d.Networks {
		d.logger.Debug("Registered network", zap.String("name", networkName))
		network.HttpClient = httpClient
		network.logger = loggerClient
		network.PrometheusClient = promClient
		network.machineID = d.machineID

		// Initialize the provider's upstream, path, and HTTP client
		for _, provider := range network.Providers {
			err := d.initializeProvider(provider, httpClient, loggerClient)
			if err != nil {
				return fmt.Errorf("error initializing provider: %v", err)
			}
		}
		if network.MethodFilter != nil {
			for method, _ := range network.MethodFilter.FilteredMethods {
				match := false
				for _, provider := range network.Providers {
					if _, ok := provider.Methods[method]; ok {
						match = true
						break
					}
				}
				if !match {
					d.logger.Warn("Method marked as routed, but not offered by any providers", zap.String("network", networkName), zap.String("method", method))
				}
			}
		}
	}

	d.logger.Info("Din middleware provisioned")

	// Start the latest block number polling for each provider in each network.
	// This is done in a goroutine that sets the latest block number in the network object,
	// and updates the provider's health status accordingly.
	// Skips if test mode is enabled.
	if !d.testMode {
		// Start the latest block number polling for each provider in each network.
		// This is done in a goroutine that sets the latest block number in the network object,
		// and updates the provider's health status accordingly.
		err := d.startHealthChecks()
		if err != nil {
			return fmt.Errorf("error starting healthchecks: %v", err)
		}

		// Pull data from the din registry
		// This will pull the latest networks and providers from the din registry and update the networks and providers in the middleware object
		// This is done in a goroutine that sets the latest networks and providers in the network map
		if d.RegistryEnabled {
			d.logger.Info("Din registry is enabled, pulling data from the registry")
			d.startRegistrySync()
		}
	}

	return nil
}

// initializeProvider initializes the provider's upstream, path, logger and HTTP client
func (d *DinMiddleware) initializeProvider(provider *provider, httpClient *dinHttp.HTTPClient, logger *logger.LoggerClient) error {
	url, err := url.Parse(provider.HttpUrl)
	if err != nil {
		return fmt.Errorf("error parsing provider URL: %v", err)
	}

	dialHost := url.Host
	if url.Scheme == "https" && url.Port() == "" {
		dialHost = url.Host + ":443"
	}

	provider.upstream = &reverseproxy.Upstream{Dial: dialHost}
	provider.path = url.Path
	// Only set host if it hasn't been set already
	if provider.host == "" {
		provider.host = url.Host
	}
	provider.httpClient = httpClient
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

	networkPath := strings.TrimPrefix(r.URL.Path, "/")
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

	// Check to see if the request is empty,
	if len(bodyBytes) == 0 {
		// if the request body is empty, do not increment the prometheus metric, return an error
		// this is specifically for OPTIONS requests and invalid requests payload bodies
		rw.WriteHeader(http.StatusBadRequest)
		rw.Write([]byte("Request body is empty\n"))
		return fmt.Errorf("request body is empty")
	}

	requestBody, err := getRequestBody(repl)
	if err != nil {
		d.logger.Error("Failed to get request body", zap.String("network", networkPath), zap.Error(err))
		return fmt.Errorf("failed to get request body: %w", err)
	}
	repl.Set(RequestMethodKey, requestBody.Method)

	// Create a new response writer wrapper to capture the response body and status code
	var rww *ResponseWriterWrapper

	if networkObj.MethodFilter != nil {
		// Set the upstreams in the context for the request
		repl.Set(DinUpstreamsContextKey, networkObj.MethodFilter.FilterProviders(requestBody, networkObj.Providers))
	} else {
		// Set the upstreams in the context for the request
		repl.Set(DinUpstreamsContextKey, networkObj.Providers)
	}

	reqStartTime := time.Now()

	// Retry the request if it fails up to the max attempt request count
	// Retries occur when:
	// 1. HTTP errors (non-200 status codes or upstream errors)
	// 2. Retryable JSON-RPC errors (server errors, timeouts, rate limits, etc.)
	// Non-retryable JSON-RPC errors (method not found, invalid params) will not trigger retries
	for attempt := 0; attempt < networkObj.RequestAttemptCount; attempt++ {
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

		// Check for success: no HTTP error and 200 status
		if err == nil && rww.statusCode == http.StatusOK {
			// Check for JSON-RPC errors in the response body (only once)
			var responseBody []byte
			if rww.body != nil {
				responseBody = rww.body.Bytes()
			}

			jsonRPCError := checkForJSONRPCError(responseBody)
			if jsonRPCError == nil {
				// If the request was successful (no HTTP error and no JSON-RPC error), break out of the loop
				break
			}

			// Check if this JSON-RPC error is retryable
			if !isJSONRPCErrorRetryable(jsonRPCError) {
				// Non-retryable JSON-RPC error (e.g., method not found, invalid params)
				break
			}

			// Log the failed attempt with JSON-RPC error information
			logFailedAttemptAsync(
				d.logger,
				networkPath,
				attempt+1,
				networkObj.RequestAttemptCount,
				rww.statusCode,
				nil, // no upstream error for JSON-RPC errors
				repl,
				requestBody,
				jsonRPCError, // Pass the JSON-RPC error
			)
		} else {
			// Log non 200 status failed attempt with HTTP error information
			logFailedAttemptAsync(
				d.logger,
				networkPath,
				attempt+1,
				networkObj.RequestAttemptCount,
				rww.statusCode,
				err, // upstream error from next.ServeHTTP
				repl,
				requestBody,
				nil, // no JSON-RPC error for HTTP errors
			)
		}
	}
	if err != nil {
		d.logger.Error("Error serving HTTP", zap.String("network", networkPath), zap.Error(err))
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
		rww.ResponseWriter.WriteHeader(rww.statusCode)
		_, err = rw.Write(rww.body.Bytes())
		if err != nil {
			d.logger.Error("Error writing response body", zap.String("network", networkPath), zap.Error(err))
			return errors.Wrap(err, "Error writing response body")
		}
	}
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
		ParsedReqBody: requestBody,
	})

	return nil
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
				d.Networks[networkName] = NewNetwork(networkName, d.Env, caddyPort) // Create a new network object
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
					case "healthcheck_method":
						dispenser.Next()
						d.Networks[networkName].HCMethod = dispenser.Val()
					case "chainid_method":
						dispenser.Next()
						d.Networks[networkName].ChainIdMethod = dispenser.Val()
					case "chain_id":
						dispenser.Next()
						chainId := dispenser.Val()
						if chainId == "" {
							return fmt.Errorf("chain ID cannot be empty for network %s", networkName)
						}
						d.Networks[networkName].ChainId = chainId
					case "call_contract_method":
						dispenser.Next()
						d.Networks[networkName].CallContractMethod = dispenser.Val()
					case "get_block_by_number_method":
						dispenser.Next()
						d.Networks[networkName].GetBlockByNumberMethod = dispenser.Val()
					case "healthcheck_threshold":
						dispenser.Next()
						d.Networks[networkName].HCThreshold, err = strconv.Atoi(dispenser.Val())
						if err != nil {
							return fmt.Errorf("invalid healthcheck threshold: %v", err)
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
				if d.Networks[networkName].ChainId == "" {
					return fmt.Errorf("chain ID is not set for network %s", networkName)
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
