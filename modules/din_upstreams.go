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

// CaddyModule returns the Caddy module information.
func (DinUpstreams) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.reverse_proxy.upstreams.din_reverse_proxy_policy",
		New: func() caddy.Module { return new(DinUpstreams) },
	}
}

// GetUpstreams returns the possible upstream endpoints for the request.
func (d *DinUpstreams) GetUpstreams(r *http.Request) ([]*reverseproxy.Upstream, error) {
	var providers map[string]*provider

	// Get upstreams from the replacer context
	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
	if v, ok := repl.Get(DinUpstreamsContextKey); ok {
		providers = v.(map[string]*provider)
	}

	upstreamPool := make([]*reverseproxy.Upstream, 0, len(providers))

	// Select upstream based on priority. If no upstreams are available, pass along all upstreams
	for priority := 0; priority < len(providers); priority++ {
		for _, p := range providers {
			if p.Priority == priority && p.Available() {
				upstreamPool = append(upstreamPool, p.upstream)
			}
		}
		if len(upstreamPool) > 0 {
			break
		}
	}

	// TODO: set config based on customer's request header
	// Didn't find any based on priority, available, find all upstreams that are in warning status by priority.
	if len(upstreamPool) == 0 {
		for priority := 0; priority < len(providers); priority++ {
			for _, p := range providers {
				if p.Priority == priority && p.IsAvailableWithWarning() {
					upstreamPool = append(upstreamPool, p.upstream)
				}
			}
			if len(upstreamPool) > 0 {
				break
			}
		}
	}

	// If no upstreams are available, pass along no upstreams
	return upstreamPool, nil
}

func (d *DinUpstreams) UnmarshalCaddyfile(dispenser *caddyfile.Dispenser) error {
	return nil
}
