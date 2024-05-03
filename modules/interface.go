package modules

import (
	"net/http"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
)

type IDinMiddleware interface {
	CaddyModule() caddy.ModuleInfo
	Provision(context caddy.Context) error
	ServeHTTP(rw http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error
	UnmarshalCaddyfile(dispenser *caddyfile.Dispenser) error
}

type IDinSelect interface {
	CaddyModule() caddy.ModuleInfo
	Provision(context caddy.Context) error
	Select(pool reverseproxy.UpstreamPool, r *http.Request, rw http.ResponseWriter) *reverseproxy.Upstream
	handleRequestMetric(bodyBytes []byte, service string, provider string)
	UnmarshalCaddyfile(dispenser *caddyfile.Dispenser) error
}

type IDinUpstreams interface {
	CaddyModule() caddy.ModuleInfo
	GetUpstreams(r *http.Request) ([]*reverseproxy.Upstream, error)
	UnmarshalCaddyfile(dispenser *caddyfile.Dispenser) error
}
