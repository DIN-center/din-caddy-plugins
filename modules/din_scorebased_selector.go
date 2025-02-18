package modules

import (
	"net/http"
	"strings"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"github.com/jmcvetta/randutil"
	"go.uber.org/zap"
)

var (
	// Initializations of extended Caddy Module Interface Guards
	// https://caddyserver.com/docs/extending-caddy

	// Din Score Based Selector Module
	_ caddy.Module      = (*DinScoreBasedSelector)(nil)
	_ caddy.Provisioner = (*DinScoreBasedSelector)(nil)
)

// DinScoreBasedSelector is a Caddy module that selects an upstream based on the reputation score of the providers.
// The score is used to weight the selection of the upstream.
// This is intended to be used as a selection policy for the DinSelect module only!
type DinScoreBasedSelector struct {
	logger   *zap.Logger
	fallback reverseproxy.Selector
}

// CaddyModule returns the Caddy module information.
func (DinScoreBasedSelector) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.reverse_proxy.selection_policies.din_score_based_selector",
		New: func() caddy.Module { return new(DinScoreBasedSelector) },
	}
}

// Provision() is called by Caddy to prepare the selector for use.
// It is called only once, when the server is starting.
func (s *DinScoreBasedSelector) Provision(context caddy.Context) error {
	s.logger = context.Logger(s)
	s.logger.Debug("[SMART_ROUTING] Provisioning called")
	s.fallback = &reverseproxy.RandomSelection{}
	return nil
}

// Select() implements the logic for selecting an upstream based on the reputation score.
func (s *DinScoreBasedSelector) Select(pool reverseproxy.UpstreamPool, r *http.Request, rw http.ResponseWriter) *reverseproxy.Upstream {

	// short circuit if there is no upstreams
	if len(pool) == 0 {
		s.logger.Warn("[SMART_ROUTING] No upstreams available")
		return nil
	}

	// Get the providers from the context
	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)

	// Look if routing should be done based on reputation score
	if scoreBasedRouting, ok := repl.Get(DinScoreBasedRoutingContextKey); ok && scoreBasedRouting.(bool) {

		// Get the providers from the context
		var providers map[string]*provider
		if backedProviders, ok := repl.Get(DinUpstreamsContextKey); ok {
			providers = backedProviders.(map[string]*provider)
		}

		// Check if providers are available
		if providers != nil {
			s.logger.Debug("[SMART_ROUTING] Selecting upstream according to its reputation score")

			choices := make([]randutil.Choice, len(pool))
			for i, upstream := range pool {
				weight := ProvidersDefaultWeight
				// Try to get the provider score (if available)
				providerKey := strings.Split(upstream.Dial, ":")[0]
				if provider, exists := providers[providerKey]; exists {
					providerScore := provider.Score
					s.logger.Debug("[SMART_ROUTING] Provider score found:",
						zap.String("provider key", providerKey),
						zap.Any("score", providerScore))

					if providerScore.HasValue() && providerScore.LastUpdated().After(time.Now().UTC().Add(-time.Minute*StaleScoreGracePeriodInMinutes)) {
						weight = int(providerScore.Value() * 100)
					}
				}
				choices[i] = randutil.Choice{
					Item:   upstream,
					Weight: weight,
				}
			}

			selected, err := randutil.WeightedChoice(choices)
			if err != nil {
				s.logger.Warn("[SMART_ROUTING] Error when selecting upstreams, all weights are 0 (zero value)")
				return nil
			}
			return selected.Item.(*reverseproxy.Upstream)
		} else {
			s.logger.Warn("[SMART_ROUTING] Score based routing enabled but there is no providers score")
			return nil
		}
	}
	// Fallback to random selection
	s.logger.Debug("[SMART_ROUTING] No score based routing, using fallback")
	return s.fallback.Select(pool, r, rw)
}
