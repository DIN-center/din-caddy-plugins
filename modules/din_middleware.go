package modules

import (
	"fmt"
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

	// Initialize the HTTP client and runtime client for each service and provider
	httpClient := din_http.NewHTTPClient()
	for _, service := range d.Services {
		runtimeClient := service.getRuntimeClient(httpClient)
		service.runtimeClient = runtimeClient

		// Initialize the provider's upstream, path, and HTTP client
		for _, provider := range service.Providers {
			url, err := url.Parse(provider.HttpUrl)
			if err != nil {
				return err
			}
			provider.upstream = &reverseproxy.Upstream{Dial: url.Host}
			provider.path = url.Path
			provider.httpClient = httpClient
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
	rww := NewResponseWriterWrapper(rw)

	// Set the providers in the context for the selector to use
	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
	repl.Set(DinUpstreamsContextKey, service.Providers)

	reqStartTime := time.Now()

	// Serve the request
	err := next.ServeHTTP(rww, r)
	if err != nil {
		return errors.Wrap(err, "Error serving HTTP")
	}

	latency := time.Since(reqStartTime)

	var provider string
	if v, ok := repl.Get(RequestProviderKey); ok {
		provider = v.(string)
	}

	var blockNumber int64
	if len(service.CheckedProviders[provider]) > 0 {
		blockNumber = service.CheckedProviders[provider][0].blockNumber
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
					Runtime:          DefaultRuntime,
					HCThreshold:      DefaultHCThreshold,
					HCInterval:       DefaultHCInterval,
					BlockLagLimit:    DefaultBlockLagLimit,
					CheckedProviders: make(map[string][]healthCheckEntry),
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
							if d.Services[serviceName].Providers == nil {
								d.Services[serviceName].Providers = make(map[string]*provider)
							}

							d.Services[serviceName].Providers[providerObj.host] = providerObj
						}
						if len(d.Services[serviceName].Providers) == 0 {
							return dispenser.Errf("expected at least one provider for service %s", serviceName)
						}
					case "runtime":
						dispenser.Next()
						d.Services[serviceName].Runtime = dispenser.Val()
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
					default:
						return dispenser.Errf("unrecognized option: %s", dispenser.Val())
					}
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
