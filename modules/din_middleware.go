package modules

import (
	"fmt"
	"encoding/hex"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"io/ioutil"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"

	"github.com/openrelayxyz/din-caddy-plugins/auth/eip4361"
	"go.uber.org/zap"
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
	logger *zap.Logger
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
	d.logger = context.Logger(d) 
	for _, upstreamWrappers := range d.Services {
		for _, upstreamWrapper := range upstreamWrappers {
			url, err := url.Parse(upstreamWrapper.HttpUrl)
			if err != nil {
				return err
			}
			// upstreamWrapper.upstream = &reverseproxy.Upstream{Dial: fmt.Sprintf("%v://%v", url.Scheme, url.Host)}
			upstreamWrapper.upstream = &reverseproxy.Upstream{Dial: url.Host}
			upstreamWrapper.path = url.Path
			if upstreamWrapper.Auth != nil {
				if err := upstreamWrapper.Auth.Start(context.Logger(d)); err != nil {
					d.logger.Warn("Error starting authentication", zap.String("provider", upstreamWrapper.HttpUrl))
				}
			}
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
							ms, err := urlToUpstreamWrapper(dispenser.Val())
							if err != nil {
								return err
							}
							for dispenser.NextBlock(nesting + 2) {
								switch dispenser.Val() {
								case "auth":
									auth := &eip4361.EIP4361ClientAuth{
										ProviderURL: strings.TrimSuffix(ms.HttpUrl, "/") + "/auth",
										SessionCount: 16,
									}
									for dispenser.NextBlock(nesting + 3) {
										switch dispenser.Val() {
										case "type":
											dispenser.NextBlock(nesting + 3)
											if dispenser.Val() != "eip4361" {
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
													key, err = ioutil.ReadFile(dispenser.Val())
													if err != nil {
														return dispenser.Errf("failed to read secret file: %v", err)
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
											auth.Signer = &eip4361.SigningConfig{
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
									ms.Auth = auth
								case "headers":
									for dispenser.NextBlock(nesting + 3) {
										k := dispenser.Val()
										var v string
										if dispenser.Args(&v) {
											ms.Headers[k] = v
										} else {
											return dispenser.Errf("header should have key and value")
										}
									}
								case "priority":
									dispenser.NextBlock(nesting + 2)
									ms.Priority, err = strconv.Atoi(dispenser.Val())
									if err != nil {
										return err
									}
								}
							}
							d.Services[serviceName] = append(d.Services[serviceName], ms)
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

func (d *DinMiddleware) ParseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	err := d.UnmarshalCaddyfile(h.Dispenser)
	if err != nil {
		return nil, err
	}

	return d, nil
}
