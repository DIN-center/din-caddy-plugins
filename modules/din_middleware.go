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
	Networks         map[string]*network `json:"networks"`
	PrometheusClient *prom.PrometheusClient
	DingoClient      *dingo.DingoClient
	logger           *zap.Logger
	machineID        string

	testMode bool

	// DIN Registry configuration
	RegistryEnabled                     bool
	RegistryBlockCheckInterval          int64
	RegistryBlockEpoch                  int64
	RegistryLastUpdatedEpochBlockNumber int64
	RegistryEnv                         string

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

	// Initialize the dinRegistry
	d.DingoClient, err = dingo.NewDingoClient(d.logger)
	if err != nil {
		return err
	}
	d.RegistryBlockCheckInterval = DefaultRegistryBlockCheckInterval
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
		// This is done in a goroutine that sets the latest block number in the service object,
		// and updates the provider's health status accordingly.
		d.startHealthChecks()

		// Pull data from the din registry
		// This will pull the latest services and providers from the din registry and update the services and providers in the middleware object
		// This is done in a goroutine that sets the latest services and providers in the service map
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
	network, ok := d.Networks[networkPath]
	if !ok {
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
					dinRegistryBlockEpochlVal := dispenser.Val()
					// Convert string to int
					intValue, err := strconv.Atoi(dinRegistryBlockEpochlVal)
					if err != nil {
						return dispenser.Errf("Error converting string to int: %v", err)
					}
					d.RegistryBlockEpoch = int64(intValue)
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
func (d *DinMiddleware) startHealthChecks() {
	d.logger.Info("Starting healthchecks", zap.String("machine_id", d.machineID))
	for _, network := range d.Networks {
		d.logger.Info("Starting healthcheck for network", zap.String("network", network.Name), zap.String("machine_id", d.machineID))
		network.startHealthcheck()
	}
}

func (d *DinMiddleware) startRegistrySync() {
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
				if d.Services[d.RegistryEnv] == nil {
					d.logger.Error("Service not found in middleware object", zap.String("service", d.RegistryEnv))
					continue
				}

				// Get the latest block number from the linea network
				latestBlockNumber := d.Services[d.RegistryEnv].LatestBlockNumber

				// Calculate the latest block floor by epoch. for example if the current block number is 55 and the epoch is 10, then the latest block floor by epoch is 50.
				latestBlockFloorByEpoch := latestBlockNumber - (latestBlockNumber % d.RegistryBlockEpoch)

				d.logger.Debug("Checking block number for registry sync", zap.Int64("block_epoch", d.RegistryBlockEpoch), zap.Int64("latest_linea_block_number", latestBlockNumber), zap.Int64("latest_block_floor_by_epoch", latestBlockFloorByEpoch), zap.Int64("last_updated_block_number", d.RegistryLastUpdatedEpochBlockNumber), zap.Int64("difference", latestBlockFloorByEpoch-d.RegistryLastUpdatedEpochBlockNumber), zap.Int64("mod", (latestBlockNumber-d.RegistryLastUpdatedEpochBlockNumber)%d.RegistryBlockEpoch))

				// If the difference between the latest block floor by epoch and the last updated block number is greater than or equal to the epoch, then update the services and providers.
				if latestBlockFloorByEpoch-d.RegistryLastUpdatedEpochBlockNumber%d.RegistryBlockEpoch >= 1 {
					registryData, err := d.DingoClient.GetDataFromRegistry()
					if err != nil {
						d.logger.Error("Failed to get data from registry", zap.Error(err))
					}
					d.processRegistryData(registryData, latestBlockFloorByEpoch)
				}
			}
		}
	}()
}

// TODO: finish this.
func (d *DinMiddleware) processRegistryData(registryData *dinsdk.DinRegistryData, latestBlockNumber int64) {
	d.logger.Debug("Processing registry data")
	d.RegistryLastUpdatedEpochBlockNumber = latestBlockNumber
}

func (d *DinMiddleware) ParseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	err := d.UnmarshalCaddyfile(h.Dispenser)
	if err != nil {
		return nil, err
	}

	return d, nil
}

func (d DinMiddleware) closeAll() {
	for _, network := range d.Networks {
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

func (d DinMiddleware) close() {
	close(d.quit)
}
