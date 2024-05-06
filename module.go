package din

import (
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	prom "github.com/openrelayxyz/din-caddy-plugins/lib/prometheus"
	mod "github.com/openrelayxyz/din-caddy-plugins/modules"
)

func init() {
	caddy.RegisterModule(mod.DinUpstreams{})
	caddy.RegisterModule(mod.DinSelect{})
	caddy.RegisterModule(mod.DinMiddleware{})

	m := new(mod.DinMiddleware)
	httpcaddyfile.RegisterHandlerDirective("din", m.ParseCaddyfile)

	// Register custom prometheus request metrics
	prom.RegisterMetrics()
}
