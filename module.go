package din

import (
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"

	_ "github.com/DIN-center/din-caddy-plugins/lib/auth/oidc"
	"github.com/DIN-center/din-caddy-plugins/lib/auth/siwe"
	prom "github.com/DIN-center/din-caddy-plugins/lib/prometheus"
	mod "github.com/DIN-center/din-caddy-plugins/modules/network"
	aimod "github.com/DIN-center/din-caddy-plugins/modules/ai"
)

func init() {
	caddy.RegisterModule(mod.DinUpstreams{})
	caddy.RegisterModule(mod.DinSelect{})
	caddy.RegisterModule(mod.DinScoreBasedSelector{})
	caddy.RegisterModule(&mod.DinMiddleware{})
	caddy.RegisterModule(siwe.SIWEAuthMiddleware{})
	caddy.RegisterModule(&aimod.DinAIMiddleware{})

	m := new(mod.DinMiddleware)
	m2 := new(siwe.SIWEAuthMiddleware)
	m3 := new(aimod.DinAIMiddleware)
	httpcaddyfile.RegisterHandlerDirective("din", m.ParseCaddyfile)
	httpcaddyfile.RegisterHandlerDirective("din_auth", m2.ParseCaddyfile)
	httpcaddyfile.RegisterHandlerDirective("din_ai", m3.ParseCaddyfile)

	// Register custom prometheus request metrics
	prom.RegisterMetrics()
	aimod.RegisterAIMetrics()
}
