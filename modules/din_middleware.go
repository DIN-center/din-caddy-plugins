package modules

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	dingo "github.com/DIN-center/din-caddy-plugins/lib/dingo"

	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	prom "github.com/DIN-center/din-caddy-plugins/lib/prometheus"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"github.com/pkg/errors"
	"go.uber.org/zap"

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

	// The prometheus client object
	PrometheusClient *prom.PrometheusClient

	// The di-ngo client object
	DingoClient dingo.IDingoClient

	logger *zap.Logger

	// The unique machine ID for the current running server instance
	machineID string

	// Test mode flag, should only be used for unit/integration testing purposes.
	testMode bool

	// DIN Registry configuration
	// The flag to enable or disable the din registry
	RegistryEnabled bool
	// The interval in seconds to check the latest block number from the registry
	RegistryBlockCheckInterval int64
	// The epoch in blocks to check the latest block number from the registry.
	// For example, if the epoch is 10, then the din registry will be synced every 10 blocks.
	RegistryBlockEpoch int64
	// The block number in which the registry was updated last
	RegistryLastUpdatedEpochBlockNumber int64
	// The blockchain network to pull the registry data from. ie linea-mainnet or linea-sepolia
	RegistryEnv string

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
	var err error

	// TODO: abstract these default initializations to a separate function
	d.machineID = getMachineId()
	d.logger = context.Logger(d)
	// Initialize the prometheus client on the din middleware object
	promClient := prom.NewPrometheusClient(d.logger, d.machineID)
	d.PrometheusClient = promClient
	d.quit = make(chan struct{})

	// Initialize the din registry configuration values
	d.DingoClient, err = dingo.NewDingoClient(d.logger)
	if err != nil {
		return err
	}
	if d.RegistryBlockCheckInterval == 0 {
		d.RegistryBlockCheckInterval = DefaultRegistryBlockCheckInterval
	}
	if d.RegistryBlockEpoch == 0 {
		d.RegistryBlockEpoch = DefaultRegistryBlockEpoch
	}
	if d.RegistryEnv == "" {
		d.RegistryEnv = DefaultRegistryEnv
	}

	// Initialize the HTTP client for each network and provider
	httpClient := din_http.NewHTTPClient()
	for _, network := range d.Networks {
		network.HTTPClient = httpClient
		network.logger = d.logger
		network.PrometheusClient = promClient
		network.machineID = d.machineID

		// Initialize the provider's upstream, path, and HTTP client
		for _, provider := range network.Providers {
			url, err := url.Parse(provider.HttpUrl)
			if err != nil {
				return err
			}

			dialHost := url.Host
			if url.Scheme == "https" && url.Port() == "" {
				dialHost = url.Host + ":443"
			}

			provider.upstream = &reverseproxy.Upstream{Dial: dialHost}
			provider.path = url.Path
			provider.host = url.Host
			provider.httpClient = httpClient
			if provider.Auth != nil {
				if err := provider.Auth.Start(context.Logger(d)); err != nil {
					d.logger.Warn("Error starting authentication", zap.String("provider", provider.HttpUrl), zap.String("machine_id", d.machineID))
				}
			}
			provider.logger = d.logger
			d.logger.Debug("Provider provisioned", zap.String("Provider", provider.HttpUrl), zap.String("Host", provider.host), zap.Int("Priority", provider.Priority), zap.Any("Headers", provider.Headers), zap.Any("Auth", provider.Auth), zap.Any("Upstream", provider.upstream), zap.Any("Path", provider.path))
		}
	}

	d.logger.Info("Din middleware provisioned", zap.String("machine_id", d.machineID))

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
			return err
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

// ServeHTTP is the main handler for the middleware that is ran for every request.
// It checks if the network path is defined in the networks map and sets the provider in the context.
func (d *DinMiddleware) ServeHTTP(rw http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	// Caddy replacer is used to set the context for the request
	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)

	networkPath := strings.TrimPrefix(r.URL.Path, "/")
	network, err := d.GetNetwork(networkPath)
	if err != nil {
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
	if (len(bodyBytes) / 1024) > int(network.MaxRequestPayloadSizeKB) {
		// If the request payload is too large, return an error
		rw.WriteHeader(http.StatusRequestEntityTooLarge)
		rw.Write([]byte("Request payload too large\n"))
		return fmt.Errorf("request payload too large")
	}

	// Create a new response writer wrapper to capture the response body and status code
	var rww *ResponseWriterWrapper

	// Set the upstreams in the context for the request
	repl.Set(DinUpstreamsContextKey, network.Providers)

	reqStartTime := time.Now()

	// Retry the request if it fails up to the max attempt request count
	for attempt := 0; attempt < network.RequestAttemptCount; attempt++ {
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
		if err == nil && rww.statusCode == http.StatusOK {
			// If the request was successful, break out of the loop
			break
		}
		// If the first attempt fails, log the failure and retry
		d.logger.Debug("Retrying request", zap.String("network", networkPath), zap.Int("attempt", attempt), zap.Int("status", rww.statusCode))
	}
	if err != nil {
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
			return errors.Wrap(err, "Error writing response body")
		}

		if rww.statusCode != http.StatusOK {
			var bodyData []byte
			if v, ok := repl.Get(RequestBodyKey); ok {
				bodyData = v.([]byte)
			}
			d.logger.Warn("Request failed", zap.String("request_body", string(bodyData)), zap.String("network", networkPath), zap.String("provider", provider), zap.Int("status", rww.statusCode), zap.String("machine_id", d.machineID))
		}
	}
	healthStatus := network.Providers[provider].healthStatus.String()

	// If the request body is empty, do not increment the prometheus metric. specifically for OPTIONS requests
	if len(bodyBytes) == 0 {
		return nil
	}

	if d.testMode {
		return nil
	}
	// Increment prometheus metric based on request data
	// debug logging of metric is found in here.
	d.PrometheusClient.HandleRequestMetrics(&prom.PromRequestMetricData{
		Network:        r.RequestURI,
		Provider:       provider,
		HostName:       r.Host,
		ResponseStatus: rww.statusCode,
		HealthStatus:   healthStatus,
	}, bodyBytes, duration)

	return nil
}

// UnmarshalCaddyfile sets up reverse proxy provider and method data on the serve based on the configuration of the Caddyfile
func (d *DinMiddleware) UnmarshalCaddyfile(dispenser *caddyfile.Dispenser) error {
	var err error
	if d.Networks == nil {
		d.Networks = make(map[string]*network)
	}
	for dispenser.Next() { // Skip the directive name
		switch dispenser.Val() {
		case "networks":
			for n1 := dispenser.Nesting(); dispenser.NextBlock(n1); {
				networkName := dispenser.Val()
				d.Networks[networkName] = NewNetwork(networkName) // Create a new network object
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
					case "providers":
						for dispenser.NextBlock(nesting + 1) {
							providerObj, err := NewProvider(dispenser.Val())
							if err != nil {
								return err
							}
							for dispenser.NextBlock(nesting + 2) {
								switch dispenser.Val() {
								case "auth":
									auth := &siwe.SIWEClientAuth{
										ProviderURL:  strings.TrimSuffix(providerObj.HttpUrl, "/") + "/auth",
										SessionCount: 16,
									}
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
												return err
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
														return err
													}
												case "secret":
													dispenser.NextBlock(nesting + 4)
													hexKey := dispenser.Val()
													hexKey = strings.TrimPrefix(hexKey, "0x")
													key, err = hex.DecodeString(hexKey)
													if err != nil {
														return err
													}
												}
											}
											auth.Signer = &siwe.SigningConfig{
												PrivateKey: key,
											}
											if err := auth.Signer.GenPrivKey(); err != nil {
												return err
											}
										}
									}
									if auth.Signer == nil {
										return fmt.Errorf("signer must be set")
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
										return err
									}
								}
							}
							d.Networks[networkName].Providers[providerObj.host] = providerObj
						}
					case "healthcheck_method":
						dispenser.Next()
						d.Networks[networkName].HCMethod = dispenser.Val()
					case "healthcheck_threshold":
						dispenser.Next()
						d.Networks[networkName].HCThreshold, err = strconv.Atoi(dispenser.Val())
						if err != nil {
							return err
						}
					case "healthcheck_interval":
						dispenser.Next()
						d.Networks[networkName].HCInterval, err = strconv.Atoi(dispenser.Val())
						if err != nil {
							return err
						}
					case "healthcheck_blocklag_limit":
						dispenser.Next()
						limit, err := strconv.Atoi(dispenser.Val())
						if err != nil {
							return err
						}
						d.Networks[networkName].BlockLagLimit = int64(limit)
					case "max_request_payload_size_kb":
						dispenser.Next()
						size, err := strconv.Atoi(dispenser.Val())
						if err != nil {
							return err
						}
						d.Networks[networkName].MaxRequestPayloadSizeKB = int64(size)
					case "request_attempt_count":
						dispenser.Next()
						requestAttemptCount, err := strconv.Atoi(dispenser.Val())
						if err != nil {
							return err
						}
						d.Networks[networkName].RequestAttemptCount = requestAttemptCount
					default:
						return dispenser.Errf("unrecognized option: %s", dispenser.Val())
					}
				}
				if len(d.Networks[networkName].Providers) == 0 {
					return dispenser.Errf("expected at least one provider for network %s", networkName)
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
					d.RegistryBlockEpoch = int64(intValue)
				case "registry_block_check_interval":
					dispenser.Next()
					registryBlockCheckIntervalVal := dispenser.Val()
					// Convert string to int64
					intValue, err := strconv.Atoi(registryBlockCheckIntervalVal)
					if err != nil {
						return dispenser.Errf("Error converting string to int: %v", err)
					}
					d.RegistryBlockCheckInterval = int64(intValue)
				case "registry_env":
					dispenser.Next()
					registryEnvVal := dispenser.Val()
					d.RegistryEnv = registryEnvVal
				}
			}
		}

	}

	return nil
}

// StartHealthchecks starts a background goroutine to monitor all of the networks' overall health and the health of its providers
func (d *DinMiddleware) startHealthChecks() error {
	d.logger.Info("Starting healthchecks", zap.String("machine_id", d.machineID))
	networks := d.GetNetworks()
	for _, network := range networks {
		d.logger.Info("Starting healthcheck for network", zap.String("network", network.Name), zap.String("machine_id", d.machineID))
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
	registryData, err := d.DingoClient.GetDataFromRegistry()
	if err != nil {
		d.logger.Error("Failed to get data from registry", zap.Error(err))
	}
	d.processRegistryData(registryData, int64(0))
	// Start a ticker to check the linea network latest block number on a time interval of 60 seconds by default.
	ticker := time.NewTicker(time.Second * time.Duration(d.RegistryBlockCheckInterval))
	// ticker := time.NewTicker(time.Second * time.Duration(d.RegistryBlockCheckInterval))
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

func (d *DinMiddleware) syncRegistryWithLatestBlock() {
	// Check if the linea network exists in the middleware object
	network, err := d.GetNetwork(d.RegistryEnv)
	if err != nil {
		d.logger.Error("Network not found in middleware object. Registry data cannot be retrieved", zap.String("network", d.RegistryEnv))
		return
	}
	// Get the latest block number from the linea network
	latestBlockNumber := network.LatestBlockNumber

	// Calculate the latest block floor by epoch. for example if the current block number is 55 and the epoch is 10, then the latest block floor by epoch is 50.
	latestBlockFloorByEpoch := latestBlockNumber - (latestBlockNumber % d.RegistryBlockEpoch)

	d.logger.Debug("Checking block number for registry sync", zap.Int64("block_epoch", d.RegistryBlockEpoch),
		zap.Int64("latest_linea_block_number", latestBlockNumber), zap.Int64("latest_block_floor_by_epoch", latestBlockFloorByEpoch),
		zap.Int64("last_updated_block_number", d.RegistryLastUpdatedEpochBlockNumber), zap.Int64("difference", latestBlockFloorByEpoch-d.RegistryLastUpdatedEpochBlockNumber),
	)

	// If the difference between the latest block floor by epoch and the last updated block number is greater than or equal to the epoch, then update the networks and providers.
	if latestBlockFloorByEpoch-d.RegistryLastUpdatedEpochBlockNumber >= d.RegistryBlockEpoch {
		registryData, err := d.DingoClient.GetDataFromRegistry()
		if err != nil {
			d.logger.Error("Failed to get data from registry", zap.Error(err))
		}
		d.processRegistryData(registryData, latestBlockFloorByEpoch)

		// Update the last updated block number
		d.RegistryLastUpdatedEpochBlockNumber = latestBlockFloorByEpoch

	}
}

// TODO: finish this.
func (d *DinMiddleware) processRegistryData(registryData *dinsdk.DinRegistryData) {
	for _, regNetwork := range registryData.Networks {
		// Check if the network is provisioned
		if regNetwork.NetworkConfig == nil || !regNetwork.NetworkConfig.IsProvisioned {
			continue
		}
		var network *network
		// Check if the network exists in the middleware object
		network, err := d.GetNetwork(regNetwork.ProxyName)
		if err != nil {
			// If the network does not exist, create a new network object and use the registry values as the initial values
			network = NewNetwork(regNetwork.ProxyName)
		}

		// Set registry network config values.
		// network.HCMethod = regNetwork.NetworkConfig.HealthcheckMethodBit
		network.HCInterval = int(regNetwork.NetworkConfig.HealthcheckIntervalSec)
		network.BlockLagLimit = int64(regNetwork.NetworkConfig.BlockLagLimit)
		network.MaxRequestPayloadSizeKB = int64(regNetwork.NetworkConfig.MaxRequestPayloadSizeKb)
		network.RequestAttemptCount = int(regNetwork.NetworkConfig.RequestAttemptCount)

		for _, regProvider := range regNetwork.Providers {
			// Check if the provider exists in the network object
			// TODO: FINISH
			provider, ok := network.Providers[]
			if !ok {
				// If the provider does not exist, create a new provider object and use the registry values as the initial values
				provider = NewProvider(regProvider.HttpUrl)
			}

			// Set registry provider config values.
			provider.Priority = int(regProvider.Priority)
			provider.Auth = nil
			if regProvider.Auth != nil {
				provider.Auth = &siwe.SIWEClientAuth{
					ProviderURL:  regProvider.Auth.ProviderURL,
					SessionCount: int(regProvider.Auth.SessionCount),
					Signer: &siwe.SigningConfig{
						PrivateKey: regProvider.Auth.Signer.PrivateKey,
					},
				}
			}
			provider.Headers = regProvider.Headers
			provider.HttpUrl = regProvider.HttpUrl

			// Update the provider object with the new data from the registry
		}

		// Update the network object with the new data from the registry
		// TODO: use mutext to lock the network object and update the values
		d.Networks[regNetwork.ProxyName] = network
	}
	d.logger.Debug("Processing registry data")
}

func (d *DinMiddleware) ParseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	err := d.UnmarshalCaddyfile(h.Dispenser)
	if err != nil {
		return nil, err
	}

	return d, nil
}

func (d *DinMiddleware) GetNetwork(networkName string) (*network, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	n, ok := d.Networks[networkName]
	if !ok {
		return nil, fmt.Errorf("network not found")
	}
	return n, nil
}

func (d *DinMiddleware) GetNetworks() []*network {
	d.mu.RLock()
	defer d.mu.RUnlock()

	networks := make([]*network, 0)
	for _, n := range d.Networks {
		networks = append(networks, n)
	}
	return networks
}

func (d *DinMiddleware) closeAll() {
	networks := d.GetNetworks()
	for _, network := range networks {
		network.close()
	}
	d.close()
}

// getMachineId returns a unique string for the current running process
func getMachineId() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "UNKNOWN"
	}
	currentPid := os.Getpid()
	return fmt.Sprintf("@%s:%d", hostname, currentPid)
}

func (d *DinMiddleware) close() {
	close(d.quit)
}
