package reputationscore

import (
	"sync"
	"time"

	"github.com/DIN-center/din-sc/apps/din-go/lib/watcher"

	"github.com/pkg/errors"
)

// A ScoreFormula defines the components that will be used to compute the score (a metric)
// and how to combine them to produce a score for a provider on a given network
type ScoreFormula struct {
	Network          string
	MetricGenerators []ProviderMetricGenerator
	MetricCombiner   ProviderMetricCombiner
	ScoreTransformer ScoreTransformer
}

// ReputationScoreManager is responsible for computing reputation scores for providers across networks
// and providing a way to get the score for a specific provider on a specific network
type ReputationScoreManager struct {
	scores   map[string]map[string]*Score // map[network]map[providerID]*ReputationScore
	formulas []ScoreFormula               // map[network]ScoreFormula
	mu       sync.RWMutex
}

func NewEmpty() *ReputationScoreManager {
	return &ReputationScoreManager{
		scores:   make(map[string]map[string]*Score),
		formulas: []ScoreFormula{},
		mu:       sync.RWMutex{},
	}
}

func NewWithBuitinFormula(networks []string, client watcher.IWatcherAPIClient) *ReputationScoreManager {

	// Make sure we only have unique networks
	uniqueNetworksMap := make(map[string]struct{})
	for _, network := range networks {
		uniqueNetworksMap[network] = struct{}{}
	}
	uniqueNetworks := make([]string, 0, len(uniqueNetworksMap))
	for network := range uniqueNetworksMap {
		uniqueNetworks = append(uniqueNetworks, network)
	}

	formulas := []ScoreFormula{}
	for _, network := range uniqueNetworks {
		formulas = append(formulas, ScoreFormula{
			Network: network,
			MetricGenerators: []ProviderMetricGenerator{
				&WatcherBlockNumberConsistency{WatcherClient: client},
				&WatcherBlockNonStateConsistency{WatcherClient: client},
				&WatcherLatency{WatcherClient: client},
			},
			MetricCombiner: &WeightedCombiner{Weights: map[string]float64{
				BlockNumberConsistencyMetricID:   0.3,
				BlockNonStateConsistencyMetricID: 0.2,
				LatencyMetricID:                  0.5,
			}},
			ScoreTransformer: &CompositeTransformer{chain: []ScoreTransformer{&NormalizeTransformer{}, &EWMATransformer{alpha: ScoreSmoothingFactor}}},
		})
	}
	return &ReputationScoreManager{
		scores:   make(map[string]map[string]*Score),
		formulas: formulas,
		mu:       sync.RWMutex{},
	}
}

func (rm *ReputationScoreManager) ComputeScores() error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Process each formula and generate the scores all available providers for each network
	for _, formula := range rm.formulas {
		network := formula.Network
		// A score may be composed of multiple metrics, so we need to collect all metrics for each provider
		metricsPerProvider := make(map[string][]*ProviderMetric)
		for _, metricGenerator := range formula.MetricGenerators {
			// A metric generator produce the same metric for all providers monitored on the network
			providerMetrics, err := metricGenerator.GenerateMetrics(network)
			if err != nil {
				return errors.Wrapf(err, "Error generating metrics for network %s", network)
			}

			// Now we need to group the metrics by provider (Provider -> [Metric1, Metric2, ...])
			for _, metric := range providerMetrics {
				metricsPerProvider[metric.ProviderID()] = append(metricsPerProvider[metric.ProviderID()], metric)
			}
		}

		// Combine the metrics for each provider by reducing them to a single score per provider
		rawScores := map[string]*Score{}
		for providerID, metrics := range metricsPerProvider {
			providerRawScore, err := formula.MetricCombiner.CombineMetrics(metrics)
			if err != nil {
				return errors.Wrapf(err, "Error combining metrics for provider %s on network %s", providerID, network)
			}
			rawScores[providerID] = providerRawScore
		}

		// Apply the score transformer to the raw scores
		transformedScores, err := formula.ScoreTransformer.TransformScore(rawScores)
		if err != nil {
			return errors.Wrapf(err, "Error transforming scores for network %s", network)
		}

		// Finally, update the scores
		for providerID, score := range transformedScores {

			// Initialize the scores map for the network if it doesn't exist
			if _, networkExists := rm.scores[network]; !networkExists {
				rm.scores[network] = make(map[string]*Score)
			}

			// Add the final score to the scores map
			rm.scores[network][providerID] = score
		}
	}
	return nil
}

func (rm *ReputationScoreManager) GetScore(network string, providerID string) *Score {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if _, networkExists := rm.scores[network]; !networkExists {
		return NewEmptyScore()
	}
	if _, providerExists := rm.scores[network][providerID]; !providerExists {
		return NewEmptyScore()
	}
	return rm.scores[network][providerID]
}

func (rm *ReputationScoreManager) StartPeriodicUpdates(frequency time.Duration) chan struct{} {
	stopChan := make(chan struct{})

	go func() {
		ticker := time.NewTicker(frequency)
		defer ticker.Stop()

		// Do initial update
		rm.ComputeScores()

		for {
			select {
			case <-ticker.C:
				rm.ComputeScores()
			case <-stopChan:
				return
			}
		}
	}()

	return stopChan
}

func (rm *ReputationScoreManager) GetAllScores(network string) map[string]*Score {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	// Copy the scores to a new map to avoid returning the underlying map
	scores := make(map[string]*Score)
	for providerID, score := range rm.scores[network] {
		scores[providerID] = score.Clone()
	}

	return scores
}
