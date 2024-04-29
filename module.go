package din

import (
	"fmt"
	"strings"
	"net/http"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"net/url"
	"strconv"
)

var (
	_ caddy.Module                = (*DinUpstreams)(nil)
	_ caddy.Module                = (*DinSelect)(nil)
	_ caddy.Module                = (*DinMiddleware)(nil)
	_ caddy.Provisioner           = (*DinSelect)(nil)
	_ caddy.Provisioner           = (*DinMiddleware)(nil)
	_ caddyhttp.MiddlewareHandler = (*DinMiddleware)(nil)
	_ caddyfile.Unmarshaler       = (*DinMiddleware)(nil)
	// _ caddy.Validator             = (*DinUpstreams)(nil)
	_ reverseproxy.UpstreamSource = (*DinUpstreams)(nil)
)


func init() {
	caddy.RegisterModule(DinUpstreams{})
	caddy.RegisterModule(DinSelect{})
	caddy.RegisterModule(DinMiddleware{})
	httpcaddyfile.RegisterHandlerDirective("din", parseCaddyfile)
}

type metaUpstream struct{
	HttpUrl  string `json:"http.url"`
	path     string
	Headers  map[string]string
	upstream *reverseproxy.Upstream
	Priority int
}

func urlToMetaUpstream(urlstr string) (*metaUpstream, error) {
	url, err := url.Parse(urlstr)
	if err != nil {
		return nil, err
	}
	return &metaUpstream{
		HttpUrl: urlstr,
		path: url.Path,
		Headers: make(map[string]string),
		// upstream: &reverseproxy.Upstream{Dial: fmt.Sprintf("%v://%v", url.Scheme, url.Host)},
		upstream: &reverseproxy.Upstream{Dial: url.Host},
	}, nil
}

type DinSelect struct {
	selector reverseproxy.Selector
}

func (d *DinSelect) Provision(context caddy.Context) error {
	selector := &reverseproxy.HeaderHashSelection{Field: "Din-Session-Id"}
	selector.Provision(context)
	d.selector = selector
	return nil
}

// CaddyModule returns the Caddy module information.
func (DinSelect) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.reverse_proxy.selection_policies.dinupstreams",
		New: func() caddy.Module { return new(DinSelect) },
	}
}

func (d *DinSelect) Select(pool reverseproxy.UpstreamPool, r *http.Request, rw http.ResponseWriter) *reverseproxy.Upstream {
	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
	var mus []*metaUpstream
	if v, ok := repl.Get("din.internal.upstreams"); ok {
		mus = v.([]*metaUpstream)
	}

	res := d.selector.Select(pool, r, rw)
	for _, mu := range mus {
		if res == mu.upstream {
			r.URL.RawPath = mu.path
			r.URL.Path, _ = url.PathUnescape(r.URL.RawPath)
			for k, v := range mu.Headers {
				r.Header.Add(k, v)
			}
			break
		}
	}
	return res
}

func (d *DinSelect) UnmarshalCaddyfile(dispenser *caddyfile.Dispenser) error {
	return nil
}

type DinMiddleware struct {
	Services map[string][]*metaUpstream `json:"Services"`
	Methods map[string][]*string `json:"Methods"`
}

// Gizmo is an example; put your own type here.
type DinUpstreams struct {}

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
	mus, ok := d.Services[strings.TrimPrefix(r.URL.Path, "/")]
	if !ok {
		if strings.TrimPrefix(r.URL.Path, "/") == "" {
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

// CaddyModule returns the Caddy module information.
func (DinMiddleware) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.din",
		New: func() caddy.Module { return new(DinMiddleware) },
	}
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


func (d *DinUpstreams) GetUpstreams(r *http.Request) ([]*reverseproxy.Upstream, error) {
	var mus []*metaUpstream
	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
	if v, ok := repl.Get("din.internal.upstreams"); ok {
		mus = v.([]*metaUpstream)
	}
	res := make([]*reverseproxy.Upstream, 0, len(mus))
	for priority := 0; priority < len(mus); priority++ {
		for _, u := range mus {
			if u.Priority == priority && u.upstream.Available() {
				res = append(res, u.upstream)
			}
		}
		if len(res) > 0 {
			break
		}
	}
	if len(res) == 0 {
		// Didn't find any based on priority, available, pass along all upstreams
		for _, u := range mus {
			res = append(res, u.upstream)
		}
	}
	return res, nil
}

func (d *DinUpstreams) UnmarshalCaddyfile(dispenser *caddyfile.Dispenser) error {
	return nil
}

// CaddyModule returns the Caddy module information.
func (DinUpstreams) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.reverse_proxy.upstreams.dinupstreams",
		New: func() caddy.Module { return new(DinUpstreams) },
	}
}

func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	j := new(DinMiddleware)
	err := j.UnmarshalCaddyfile(h.Dispenser)
	if err != nil {
		return nil, err
	}

	return j, nil
}