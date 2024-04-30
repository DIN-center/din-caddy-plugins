package din

import (
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"github.com/prometheus/client_golang/prometheus"
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
	// _ caddy.Validator			= (*DinMiddleware)(nil)

	// Din Upstream Module
	_ caddy.Module                = (*DinUpstreams)(nil)
	_ reverseproxy.UpstreamSource = (*DinUpstreams)(nil)

	// Din Select Module
	_ caddy.Module      = (*DinSelect)(nil)
	_ caddy.Provisioner = (*DinSelect)(nil)
)

func init() {
	caddy.RegisterModule(DinUpstreams{})
	caddy.RegisterModule(DinSelect{})
	caddy.RegisterModule(DinMiddleware{})
	httpcaddyfile.RegisterHandlerDirective("din", parseCaddyfile)

	// Register custom prometheus request metrics
	prometheus.MustRegister(dinRequestCount)
}

func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	j := new(DinMiddleware)
	err := j.UnmarshalCaddyfile(h.Dispenser)
	if err != nil {
		return nil, err
	}

	return j, nil
}

// prometheus metric initialization
var dinRequestCount = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "din_http_request_count",
		Help: "",
	},
	[]string{"service", "method", "provider"},
)
