package din

import (
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	prom "github.com/openrelayxyz/din-caddy-plugins/lib/prometheus"
	mod "github.com/openrelayxyz/din-caddy-plugins/modules"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// Initializations of extended Caddy Module Interface Guards
	// https://caddyserver.com/docs/extending-caddy

	// Din Middleware Module
	_ caddy.Module                = (*mod.DinMiddleware)(nil)
	_ caddy.Provisioner           = (*mod.DinMiddleware)(nil)
	_ caddyhttp.MiddlewareHandler = (*mod.DinMiddleware)(nil)
	_ caddyfile.Unmarshaler       = (*mod.DinMiddleware)(nil)
	// TODO: validate provision step
	// _ caddy.Validator			= (*mod.DinMiddleware)(nil)

	// Din Upstream Module
	_ caddy.Module                = (*mod.DinUpstreams)(nil)
	_ reverseproxy.UpstreamSource = (*mod.DinUpstreams)(nil)

	// Din Select Module
	_ caddy.Module      = (*mod.DinSelect)(nil)
	_ caddy.Provisioner = (*mod.DinSelect)(nil)
)

func init() {
	caddy.RegisterModule(mod.DinUpstreams{})
	caddy.RegisterModule(mod.DinSelect{})
	caddy.RegisterModule(mod.DinMiddleware{})
	httpcaddyfile.RegisterHandlerDirective("din", parseCaddyfile)

	// Register custom prometheus request metrics
	prometheus.MustRegister(prom.DinRequestCount)
}

func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	j := new(mod.DinMiddleware)
	err := j.UnmarshalCaddyfile(h.Dispenser)
	if err != nil {
		return nil, err
	}

	return j, nil
}
