package modules

import (
	"net/http"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
)

var (
	// Initializations of extended Caddy Module Interface Guards
	// https://caddyserver.com/docs/extending-caddy

	// Din Upstream Module
	_ caddy.Module                = (*DinUpstreams)(nil)
	_ reverseproxy.UpstreamSource = (*DinUpstreams)(nil)
)

type DinUpstreams struct{}

type upstreamWrapper struct {
	HttpUrl  string `json:"http.url"`
	path     string
	Headers  map[string]string
	upstream *reverseproxy.Upstream
	Priority int
}

// CaddyModule returns the Caddy module information.
func (DinUpstreams) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.reverse_proxy.upstreams.din_reverse_proxy_policy",
		New: func() caddy.Module { return new(DinUpstreams) },
	}
}

func (d *DinUpstreams) GetUpstreams(r *http.Request) ([]*reverseproxy.Upstream, error) {
	var upstreamWrappers []*upstreamWrapper
	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
	if v, ok := repl.Get("din.internal.upstreams"); ok {
		upstreamWrappers = v.([]*upstreamWrapper)
	}
	res := make([]*reverseproxy.Upstream, 0, len(upstreamWrappers))
	for priority := 0; priority < len(upstreamWrappers); priority++ {
		for _, u := range upstreamWrappers {
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
		for _, u := range upstreamWrappers {
			res = append(res, u.upstream)
		}
	}
	return res, nil
}

func (d *DinUpstreams) UnmarshalCaddyfile(dispenser *caddyfile.Dispenser) error {
	return nil
}
