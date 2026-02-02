package modules

import (
	"net/http"
	"time"

	ws "github.com/DIN-center/din-caddy-plugins/lib/watcherscore"
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

// DinScoreBasedSelector is a Caddy module that selects an upstream based on the watcher score of the providers.
// The score is used to weight the selection of the upstream.
// This is intended to be used as a selection policy for the DinSelect module only!
type DinScoreBasedSelector struct {
	logger   *zap.Logger
	fallback reverseproxy.Selector
}

// ChoiceEntry is a struct that contains the upstream, provider host and provider score.
// It is used to store the choice entry in the choices array.
type ChoiceEntry struct {
	upstream *reverseproxy.Upstream
	host     string
	score    *ws.Score
}

// CaddyModule returns the Caddy module information.
func (DinScoreBasedSelector) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  DinScoreBasedSelectionCaddyModuleKey,
		New: func() caddy.Module { return new(DinScoreBasedSelector) },
	}
}

// Provision() is called by Caddy to prepare the selector for use.
// It is called only once, when the server is starting.
func (s *DinScoreBasedSelector) Provision(context caddy.Context) error {
	s.logger = context.Logger(s)
	s.logger.Debug("[DYNAMIC_LB] Provisioning called")
	s.fallback = &reverseproxy.RandomSelection{}
	return nil
}

// Select() implements the logic for selecting an upstream based on the watcher score.
func (s *DinScoreBasedSelector) Select(pool reverseproxy.UpstreamPool, r *http.Request, rw http.ResponseWriter) *reverseproxy.Upstream {

	// short circuit if there is no upstreams
	if len(pool) == 0 {
		s.logger.Warn("[DYNAMIC_LB] No upstreams available")
		return nil
	}

	// Short circuit if there is only one upstream available, return it directly regardless of the score
	// Should we return nil if the score is 0.0?
	if len(pool) == 1 {
		s.logger.Debug("[DYNAMIC_LB] Only one upstream available, returning it")
		return pool[0]
	}

	// Get Caddy replacer context
	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)

	// Look if dynamic load balancing should be done
	if scoreBasedSelection, ok := repl.Get(DinScoreBasedLoadBalancingContextKey); ok && scoreBasedSelection.(bool) {

		// Get the providers from the context
		var providers map[string]*provider
		if v, ok := repl.Get(DinUpstreamsContextKey); ok {
			providers = v.(map[string]*provider)
		}

		// If providers are available, we can select a upstream based on the watcher score
		if providers != nil {
			s.logger.Debug("[DYNAMIC_LB] Selecting upstream according to its watcher score")

			// Create a map of upstreams and associated providers
			mapUpstreamProviders := make(map[*reverseproxy.Upstream]*provider)
			for _, p := range providers {
				mapUpstreamProviders[p.Upstream] = p
			}

			// Prepare the weights for the selection
			choices := make([]randutil.Choice, len(pool))
			for i, upstream := range pool {
				// Default values
				weight := ScoreBasedSelectionProviderDefaultWeight
				providerHost := "unknown"
				providerScore := ws.NewEmptyScore()

				if provider, exists := mapUpstreamProviders[upstream]; exists {

					providerHost = provider.Host
					providerScore = provider.SafeGetScore()

					if providerScore.HasValue() {
						s.logger.Debug("[DYNAMIC_LB] Score found for provider",
							zap.String("provider", providerHost),
							zap.Any("score", providerScore))

						graceTimeStart := time.Now().UTC().Add(-StaleScoreGracePeriod)
						if providerScore.LastUpdated().After(graceTimeStart) {
							// If the score has a valid value and is not stale, we can use it to weight the selection
							weight = int(providerScore.Value() * ScoreBasedSelectionWeightBase)
						} else {
							//Score is stale, so we gradually pull the weight towards the default weight
							timeSinceStale := graceTimeStart.Sub(providerScore.LastUpdated())
							staleScore := providerScore.Value()
							adjustedStaleScore, err := ws.ExponentialPullToMidpoint(staleScore,
								timeSinceStale,
								StaleScoreConvergencePeriod,
								float64(ScoreBasedSelectionProviderDefaultWeight)/float64(ScoreBasedSelectionWeightBase))
							if err != nil {
								s.logger.Error("[DYNAMIC_LB] Error when pulling stale score towards default weight", zap.Error(err))
								return nil
							}
							weight = int(adjustedStaleScore * ScoreBasedSelectionWeightBase)
							s.logger.Debug("[DYNAMIC_LB] Score is stale for provider, adjusted towards default weight",
								zap.String("provider", providerHost),
								zap.String("score_updated_at", providerScore.LastUpdated().Format(time.RFC3339)),
								zap.String("grace_time_start", graceTimeStart.Format(time.RFC3339)),
								zap.Float64("stale_score", staleScore),
								zap.Float64("adjusted_stale_score", adjustedStaleScore),
								zap.Int("weight", weight))
						}
					} else {
						s.logger.Debug("[DYNAMIC_LB] No score found for provider",
							zap.String("provider", provider.Host))
					}

				}
				choices[i] = randutil.Choice{
					Item:   ChoiceEntry{upstream: upstream, host: providerHost, score: providerScore},
					Weight: weight,
				}
			}

			// Select the upstream based on the weights
			selected, err := randutil.WeightedChoice(choices)
			if err != nil {
				s.logger.Warn("[DYNAMIC_LB] Error when selecting upstreams, all weights are 0 (zero value)")
				return nil
			}

			// Break the selected item into its components
			selectedItem := selected.Item.(ChoiceEntry)

			s.logger.Debug("[DYNAMIC_LB] Weighted upstream selection",
				zap.String("provider", selectedItem.host),
				zap.Any("score", selectedItem.score),
				zap.Int("weight", selected.Weight))

			return selectedItem.upstream
		} else {
			s.logger.Warn("[DYNAMIC_LB] Score based load balancing enabled but there is no providers available in the context")
			return nil
		}
	}

	// Fallback to default selection
	s.logger.Debug("[DYNAMIC_LB] No score based load balancing, using fallback")
	return s.fallback.Select(pool, r, rw)
}
