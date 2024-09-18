package din

import (
	"github.com/DIN-center/din-caddy-plugins/lib/auth/siwe"
	prom "github.com/DIN-center/din-caddy-plugins/lib/prometheus"
	mod "github.com/DIN-center/din-caddy-plugins/modules"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
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
