package modules

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	din_http "github.com/openrelayxyz/din-caddy-plugins/lib/http"
	prom "github.com/openrelayxyz/din-caddy-plugins/lib/prometheus"
	"github.com/pkg/errors"
)

var (
	// Initializations of extended Caddy Module Interface Guards
	// https://caddyserver.com/docs/extending-caddy

	// Din Middleware Module
	_ caddy.Module                = (*DinMiddleware)(nil)
	_ caddy.Provisioner           = (*DinMiddleware)(nil)
	_ caddyhttp.MiddlewareHandler = (*DinMiddleware)(nil)
	_ caddyfile.Unmarshaler       = (*DinMiddleware)(nil)
	// TODO: validate provision step
	// _ caddy.Validator			= (*mod.DinMiddleware)(nil)
)

type DinMiddleware struct {
	Services         map[string]*service `json:"services"`
	PrometheusClient *prom.PrometheusClient
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
	// Initialize the prometheus client on the din middleware object
	d.PrometheusClient = prom.NewPrometheusClient()

	// Initialize the HTTP client for each service and provider
	httpClient := din_http.NewHTTPClient()
	for _, service := range d.Services {
		service.HTTPClient = httpClient

		// Initialize the provider's upstream, path, and HTTP client
		for _, provider := range service.Providers {
			url, err := url.Parse(provider.HttpUrl)
			if err != nil {
				return err
			}
			provider.upstream = &reverseproxy.Upstream{Dial: url.Host}
			provider.path = url.Path
		}
	}

	// Start the latest block number polling for each provider in each network.
	// This is done in a goroutine that sets the latest block number in the service object,
	// and updates the provider's health status accordingly.
	d.startHealthChecks()

	return nil
}

// ServeHTTP is the main handler for the middleware that is ran for every request.
// It checks if the service path is defined in the services map and sets the provider in the context.
func (d *DinMiddleware) ServeHTTP(rw http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	servicePath := strings.TrimPrefix(r.URL.Path, "/")

	service, ok := d.Services[servicePath]
	if !ok {
		if servicePath == "" {
			rw.WriteHeader(200)
			rw.Write([]byte("{}"))
			return nil
		}
		rw.WriteHeader(404)
		rw.Write([]byte("Not Found\n"))
		return fmt.Errorf("service undefined")
	}

	// Create a new response writer wrapper to capture the response body and status code
	var rww *ResponseWriterWrapper

	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
	repl.Set(DinUpstreamsContextKey, service.Providers)

	reqStartTime := time.Now()

	var err error
	// Retry the request if it fails up to the max attempt request count
	for attempt := 0; attempt < service.RequestAttemptCount; attempt++ {
		rww = NewResponseWriterWrapper(rw)

		// If the request fails, reset the request body to the original request body
		if attempt > 0 {
			var reqBody []byte
			if v, ok := repl.Get(RequestBodyKey); ok {
				reqBody = v.([]byte)
			}
			r.Body = io.NopCloser(bytes.NewReader(reqBody))
		}

		// Serve the request
		err = next.ServeHTTP(rww, r)
		if err == nil && rww.statusCode == http.StatusOK {
			// If the request was successful, break out of the loop
			break
		}
		// If the first attempt fails, log the failure and retry
		// Log the retry attempt here if needed
		// TODO: add logging via specifying levels using zap.Logger
		// log.Printf("Retrying request to %s", r.RequestURI)
	}
	if err != nil {
		return errors.Wrap(err, "Error serving HTTP")
	}

	// Write the response body and status to the original response writer
	// This is done after the request is attempted multiple times if needed
	if rww != nil {
		rww.ResponseWriter.WriteHeader(rww.statusCode)
		_, err = rw.Write(rww.body.Bytes())
		if err != nil {
			return errors.Wrap(err, "Error writing response body")
		}
	}

	latency := time.Since(reqStartTime)

	var provider string
	if v, ok := repl.Get(RequestProviderKey); ok {
		provider = v.(string)
	}

	var blockNumber int64
	checkProviderValues, _ := service.getCheckedProviderHCList(provider)
	// if !ok {
	// TODO: determine log level for this message
	// fmt.Println("Provider not found in checked providers list")
	// }
	if len(checkProviderValues) > 0 {
		blockNumber = checkProviderValues[0].blockNumber
	} else {
		blockNumber = service.LatestBlockNumber
	}

	healthStatus := service.Providers[provider].healthStatus.String()

	var reqBody []byte
	if v, ok := repl.Get(RequestBodyKey); ok {
		reqBody = v.([]byte)
	}

	// If the request body is empty, do not increment the prometheus metric. specifically for OPTIONS requests
	if len(reqBody) == 0 {
		return nil
	}

	// Increment prometheus metric based on request data
	d.PrometheusClient.HandleRequestMetric(reqBody, &prom.PromRequestMetricData{
		Service:      r.RequestURI,
		Provider:     provider,
		HostName:     r.Host,
		ResStatus:    rww.statusCode,
		ResLatency:   latency,
		HealthStatus: healthStatus,
		BlockNumber:  strconv.FormatInt(blockNumber, 10),
	})

	return nil
}

// UnmarshalCaddyfile sets up reverse proxy provider and method data on the serve based on the configuration of the Caddyfile
func (d *DinMiddleware) UnmarshalCaddyfile(dispenser *caddyfile.Dispenser) error {
	var err error
	if d.Services == nil {
		d.Services = make(map[string]*service)
	}
	for dispenser.Next() { // Skip the directive name
		switch dispenser.Val() {
		case "services":
			for n1 := dispenser.Nesting(); dispenser.NextBlock(n1); {
				serviceName := dispenser.Val()
				d.Services[serviceName] = &service{
					Name: serviceName,
					// Default health check values, to be overridden if specified in the Caddyfile
					HCMethod:            DefaultHCMethod,
					HCThreshold:         DefaultHCThreshold,
					HCInterval:          DefaultHCInterval,
					BlockLagLimit:       DefaultBlockLagLimit,
					RequestAttemptCount: DefaultRequestAttemptCount,
					CheckedProviders:    make(map[string][]healthCheckEntry),
					Providers:           make(map[string]*provider),
				}
				for nesting := dispenser.Nesting(); dispenser.NextBlock(nesting); {
					switch dispenser.Val() {
					case "methods":
						d.Services[serviceName].Methods = make([]*string, dispenser.CountRemainingArgs())
						for i := 0; i < dispenser.CountRemainingArgs(); i++ {
							d.Services[serviceName].Methods[i] = new(string)
						}
						if !dispenser.Args(d.Services[serviceName].Methods...) {
							return dispenser.Errf("invalid 'methods' argument for service %s", serviceName)
						}
					case "providers":
						for dispenser.NextBlock(nesting + 1) {
							providerObj := &provider{
								Headers: make(map[string]string),
							}
							providerObj, err := NewProvider(dispenser.Val())
							if err != nil {
								return err
							}
							for dispenser.NextBlock(nesting + 2) {
								switch dispenser.Val() {
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
							d.Services[serviceName].Providers[providerObj.host] = providerObj
						}
					case "healthcheck_method":
						dispenser.Next()
						d.Services[serviceName].HCMethod = dispenser.Val()
					case "healthcheck_threshold":
						dispenser.Next()
						d.Services[serviceName].HCThreshold, err = strconv.Atoi(dispenser.Val())
						if err != nil {
							return err
						}
					case "healthcheck_interval":
						dispenser.Next()
						d.Services[serviceName].HCInterval, err = strconv.Atoi(dispenser.Val())
						if err != nil {
							return err
						}
					case "healthcheck_blocklag_limit":
						dispenser.Next()
						limit, err := strconv.Atoi(dispenser.Val())
						if err != nil {
							return err
						}
						d.Services[serviceName].BlockLagLimit = int64(limit)
					case "request_attempt_count":
						dispenser.Next()
						requestAttemptCount, err := strconv.Atoi(dispenser.Val())
						if err != nil {
							return err
						}
						d.Services[serviceName].RequestAttemptCount = requestAttemptCount
					default:
						return dispenser.Errf("unrecognized option: %s", dispenser.Val())
					}
				}
				if len(d.Services[serviceName].Providers) == 0 {
					return dispenser.Errf("expected at least one provider for service %s", serviceName)
				}
			}
		}
	}
	return nil
}

// StartHealthchecks starts a background goroutine to monitor all of the services' overall health and the health of its providers
func (d *DinMiddleware) startHealthChecks() {
	for _, service := range d.Services {
		service.startHealthcheck()
	}
}

func (d *DinMiddleware) ParseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	err := d.UnmarshalCaddyfile(h.Dispenser)
	if err != nil {
		return nil, err
	}

	return d, nil
}

func (d DinMiddleware) closeAll() {
	for _, service := range d.Services {
		service.close()
	}
}
