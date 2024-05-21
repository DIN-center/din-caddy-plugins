package modules

import (
	"net/http"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"github.com/openrelayxyz/din-caddy-plugins/auth/eip4361"
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
	Auth     *eip4361.EIP4361ClientAuth
}

func (u *upstreamWrapper) Available() bool {
	return u.upstream.Available() && (u.Auth == nil || u.Auth.Error() == nil)
}

// CaddyModule returns the Caddy module information.
func (DinUpstreams) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.reverse_proxy.upstreams.din_reverse_proxy_policy",
		New: func() caddy.Module { return new(DinUpstreams) },
	}
}

// GetUpstreams returns the possible upstreams for the request.
func (d *DinUpstreams) GetUpstreams(r *http.Request) ([]*reverseproxy.Upstream, error) {
	var upstreamWrappers []*upstreamWrapper

	// Get upstreams from the replacer context
	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
	if v, ok := repl.Get(DinUpstreamsContextKey); ok {
		upstreamWrappers = v.([]*upstreamWrapper)
	}

	res := make([]*reverseproxy.Upstream, 0, len(upstreamWrappers))

	// Select upstream based on priority. If no upstreams are available, pass along all upstreams
	for priority := 0; priority < len(upstreamWrappers); priority++ {
		for _, u := range upstreamWrappers {
			if u.Priority == priority && u.Available() {
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
