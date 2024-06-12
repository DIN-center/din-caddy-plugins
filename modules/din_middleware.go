package modules

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	din_http "github.com/openrelayxyz/din-caddy-plugins/lib/http"
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
	Services map[string]*service `json:"services"`
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
	httpClient := din_http.NewHTTPClient()
	for _, service := range d.Services {
		runtimeClient := service.getRuntimeClient(httpClient)
		service.runtimeClient = runtimeClient
		for _, provider := range service.Providers {
			url, err := url.Parse(provider.HttpUrl)
			if err != nil {
				return err
			}
			// upstreamWrapper.upstream = &reverseproxy.Upstream{Dial: fmt.Sprintf("%v://%v", url.Scheme, url.Host)}
			provider.upstream = &reverseproxy.Upstream{Dial: url.Host}
			provider.path = url.Path
			provider.httpClient = httpClient
		}
	}

	// Start the latest block number polling for each provider in each network.
	// This is done in a goroutine that sets the latest block number in the service object,
	// and updates the provider's health status accordingly.
	d.startHealthchecks()

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

	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
	repl.Set(DinUpstreamsContextKey, service.Providers)
	return next.ServeHTTP(rw, r)
}

// UnmarshalCaddyfile sets up reverse proxy provider and method data on the serve based on the configuration of the Caddyfile
func (d *DinMiddleware) UnmarshalCaddyfile(dispenser *caddyfile.Dispenser) error {
	// if d.Methods == nil {
	// 	d.Methods = make(map[string][]*string)
	// }
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
					Runtime:     DefaultRuntime,
					HCThreshold: DefaultHCThreshold,
					HCInterval:  DefaultHCInterval,
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
					case "healthcheck_method":
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
func (d *DinMiddleware) startHealthchecks() {
	for name, service := range d.Services {
		// TODO: This is a temporary solution to only start the healthcheck for the blast mainnet service for local debugging
		if name == "blast-mainnet" {
			service.startHealthcheck()
		}
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
