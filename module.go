package din

import (
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/openrelayxyz/din-caddy-plugins/lib/auth/siwe"
	prom "github.com/openrelayxyz/din-caddy-plugins/lib/prometheus"
	mod "github.com/openrelayxyz/din-caddy-plugins/modules"
)

func init() {
	caddy.RegisterModule(mod.DinUpstreams{})
	caddy.RegisterModule(mod.DinSelect{})
	caddy.RegisterModule(mod.DinMiddleware{})
	caddy.RegisterModule(siwe.SIWEAuthMiddleware{})

	m := new(mod.DinMiddleware)
	m2 := new(siwe.SIWEAuthMiddleware)
	httpcaddyfile.RegisterHandlerDirective("din", m.ParseCaddyfile)
	httpcaddyfile.RegisterHandlerDirective("din_auth", m2.ParseCaddyfile)

	// Register custom prometheus request metrics
	prom.RegisterMetrics()
}
