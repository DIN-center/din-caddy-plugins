package din

import (
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	prom "github.com/openrelayxyz/din-caddy-plugins/lib/prometheus"
	mod "github.com/openrelayxyz/din-caddy-plugins/modules"
	"github.com/openrelayxyz/din-caddy-plugins/auth/eip4361"
)

func init() {
	caddy.RegisterModule(mod.DinUpstreams{})
	caddy.RegisterModule(mod.DinSelect{})
	caddy.RegisterModule(mod.DinMiddleware{})
	caddy.RegisterModule(eip4361.EIP4361AuthMiddleware{})

	m := new(mod.DinMiddleware)
	m2 := new(eip4361.EIP4361AuthMiddleware)
	httpcaddyfile.RegisterHandlerDirective("din", m.ParseCaddyfile)
	httpcaddyfile.RegisterHandlerDirective("din_auth", m2.ParseCaddyfile)

	// Register custom prometheus request metrics
	prom.RegisterMetrics()
}
