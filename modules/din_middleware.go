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

	"github.com/openrelayxyz/din-caddy-plugins/lib/runtime"
	"github.com/openrelayxyz/din-caddy-plugins/lib/runtime/ethereum"
	"github.com/openrelayxyz/din-caddy-plugins/lib/runtime/solana"
	"github.com/openrelayxyz/din-caddy-plugins/lib/runtime/starknet"
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
	Services map[string][]*upstreamWrapper `json:"services"`
	Methods  map[string][]*string          `json:"methods"`
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
// For each upstream wrapper object, we parse the URL and populate the upstream and path fields.
func (d *DinMiddleware) Provision(context caddy.Context) error {
	for _, upstreamWrappers := range d.Services {
		for _, upstreamWrapper := range upstreamWrappers {
			url, err := url.Parse(upstreamWrapper.HttpUrl)
			if err != nil {
				return err
			}
			// upstreamWrapper.upstream = &reverseproxy.Upstream{Dial: fmt.Sprintf("%v://%v", url.Scheme, url.Host)}
			upstreamWrapper.upstream = &reverseproxy.Upstream{Dial: url.Host}
			upstreamWrapper.path = url.Path
		}
	}
	return nil
}

// ServeHTTP is the main handler for the middleware that is ran for every request.
// It checks if the service path is defined in the services map and sets the upstreamWrapper in the context.
func (d *DinMiddleware) ServeHTTP(rw http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	servicePath := strings.TrimPrefix(r.URL.Path, "/")

	upstreamWrapperList, ok := d.Services[servicePath]
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
	repl.Set(DinUpstreamsContextKey, upstreamWrapperList)
	return next.ServeHTTP(rw, r)
}

// UnmarshalCaddyfile sets up reverse proxy upstreamWrapper and method data on the serve based on the configuration of the Caddyfile
func (d *DinMiddleware) UnmarshalCaddyfile(dispenser *caddyfile.Dispenser) error {
	var runtime string
	if d.Methods == nil {
		d.Methods = make(map[string][]*string)
	}
	if d.Services == nil {
		d.Services = make(map[string][]*upstreamWrapper)
	}
	for dispenser.Next() { // Skip the directive name
		switch dispenser.Val() {
		case "services":
			for n1 := dispenser.Nesting(); dispenser.NextBlock(n1); {
				serviceName := dispenser.Val()
				for nesting := dispenser.Nesting(); dispenser.NextBlock(nesting); {
					switch dispenser.Val() {
					case "runtime":
						dispenser.NextBlock(nesting + 2)
						runtime = dispenser.Val()
					case "methods":
						d.Methods[serviceName] = make([]*string, dispenser.CountRemainingArgs())
						for i := 0; i < dispenser.CountRemainingArgs(); i++ {
							d.Methods[serviceName][i] = new(string)
						}
						if !dispenser.Args(d.Methods[serviceName]...) {
							return dispenser.Errf("invalid 'methods' argument for service %s", serviceName)
						}
					case "providers":
						for dispenser.NextBlock(nesting + 1) {
							upstreamWrapper, err := urlToUpstreamWrapper(dispenser.Val())
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
											upstreamWrapper.Headers[k] = v
										} else {
											return dispenser.Errf("header should have key and value")
										}
									}
								case "priority":
									dispenser.NextBlock(nesting + 2)
									upstreamWrapper.Priority, err = strconv.Atoi(dispenser.Val())
									if err != nil {
										return err
									}
								}
							}

							// setup the runtime client
							// TODO: may want to set these clients up globally so they aren't attached individually to each upstreamWrapper
							upstreamWrapper.RuntimeClient, err = getRuntimeClient(runtime)
							if err != nil {
								return dispenser.Errf("error getting runtime client: %v", err)
							}

							d.Services[serviceName] = append(d.Services[serviceName], upstreamWrapper)
						}
						if len(d.Services[serviceName]) == 0 {
							return dispenser.Errf("expected at least one provider for service %s", serviceName)
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

// urlToUpstreamWrapper parses the URL and returns an upstreamWrapper object
func urlToUpstreamWrapper(urlstr string) (*upstreamWrapper, error) {
	url, err := url.Parse(urlstr)
	if err != nil {
		return nil, err
	}
	return &upstreamWrapper{
		HttpUrl: urlstr,
		path:    url.Path,
		Headers: make(map[string]string),
		// upstream: &reverseproxy.Upstream{Dial: fmt.Sprintf("%v://%v", url.Scheme, url.Host)},
		upstream: &reverseproxy.Upstream{Dial: url.Host},
	}, nil
}

func getRuntimeClient(runtime string) (runtime.IRuntimeClient, error) {
	switch runtime {
	case Ethereum:
		return ethereum.NewEthereumRuntimeClient(), nil
	case Solana:
		return solana.NewSolanaRuntimeClient(), nil
	case Starknet:
		return starknet.NewStarknetRuntimeClient(), nil
	default:
		return nil, fmt.Errorf("runtime %s not supported", runtime)
	}
}

func (d *DinMiddleware) ParseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	err := d.UnmarshalCaddyfile(h.Dispenser)
	if err != nil {
		return nil, err
	}

	return d, nil
}
