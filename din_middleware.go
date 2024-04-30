package din

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
)

type DinMiddleware struct {
	Services map[string][]*metaUpstream `json:"services"`
	Methods  map[string][]*string       `json:"methods"`
}

// CaddyModule returns the Caddy module information.
func (DinMiddleware) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.din",
		New: func() caddy.Module { return new(DinMiddleware) },
	}
}

func (d *DinMiddleware) Provision(context caddy.Context) error {
	for _, upstreams := range d.Services {
		for _, metaUpstream := range upstreams {
			url, err := url.Parse(metaUpstream.HttpUrl)
			if err != nil {
				return err
			}
			// metaUpstream.upstream = &reverseproxy.Upstream{Dial: fmt.Sprintf("%v://%v", url.Scheme, url.Host)}
			metaUpstream.upstream = &reverseproxy.Upstream{Dial: url.Host}
			metaUpstream.path = url.Path
		}
	}
	return nil
}

func (d *DinMiddleware) ServeHTTP(rw http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	servicePath := strings.TrimPrefix(r.URL.Path, "/")

	mus, ok := d.Services[servicePath]
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
	repl.Set("din.internal.upstreams", mus)
	return next.ServeHTTP(rw, r)
}

func (d *DinMiddleware) UnmarshalCaddyfile(dispenser *caddyfile.Dispenser) error {
	if d.Methods == nil {
		d.Methods = make(map[string][]*string)
	}
	if d.Services == nil {
		d.Services = make(map[string][]*metaUpstream)
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
							ms, err := urlToMetaUpstream(dispenser.Val())
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

func urlToMetaUpstream(urlstr string) (*metaUpstream, error) {
	url, err := url.Parse(urlstr)
	if err != nil {
		return nil, err
	}
	return &metaUpstream{
		HttpUrl: urlstr,
		path:    url.Path,
		Headers: make(map[string]string),
		// upstream: &reverseproxy.Upstream{Dial: fmt.Sprintf("%v://%v", url.Scheme, url.Host)},
		upstream: &reverseproxy.Upstream{Dial: url.Host},
	}, nil
}