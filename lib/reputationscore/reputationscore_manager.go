package reputationscore

import (
	"sync"
	"time"

	"github.com/DIN-center/din-sc/apps/din-go/lib/watcher"
	"go.uber.org/zap"

	"github.com/pkg/errors"
)

// A ScoreFormula defines the components that will be used to compute the score (a metric)
// and how to combine them to produce a score for a provider on a given network
type ScoreFormula struct {
	network          string
	metricGenerators []ProviderMetricGenerator
	metricCombiner   ProviderMetricCombiner
	scoreTransformer ScoreTransformer
}

// ReputationScoreManager is responsible for computing reputation scores for providers across networks
// and providing a way to get the score for a specific provider on a specific network
type ReputationScoreManager struct {
	scores   map[string]map[string]*Score // map[network]map[providerID]*ReputationScore
	formulas map[string]ScoreFormula      // map[network]ScoreFormula
	logger   *zap.Logger
	mu       sync.RWMutex
}

func NewEmpty(logger *zap.Logger) *ReputationScoreManager {
	return &ReputationScoreManager{
		scores:   make(map[string]map[string]*Score),
		formulas: make(map[string]ScoreFormula),
		logger:   logger,
		mu:       sync.RWMutex{},
	}
}

func NewWithBuiltInFormula(networks []string, client watcher.IWatcherAPIClient, logger *zap.Logger) *ReputationScoreManager {

	// Make sure we only have unique networks
	uniqueNetworksMap := make(map[string]struct{})
	for _, network := range networks {
		uniqueNetworksMap[network] = struct{}{}
	}

	rm := NewEmpty(logger)
	for network := range uniqueNetworksMap {
		rm.AddNetworkWithBuiltInFormula(network, client)
	}
	return rm
}

func (rm *ReputationScoreManager) ComputeScores() error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.logger.Debug("[REPUTATION_SCORE] Computing reputation scores...")

	// Process each formula that defines how to compute the score for a network
	for _, formula := range rm.formulas {
		network := formula.network

		// A score may be composed of multiple metrics, so we need to collect all metrics for each provider
		metricsPerProvider := make(map[string][]*ProviderMetric)
		for _, metricGenerator := range formula.metricGenerators {

			// A metric generator produces the same metric type for all providers monitored on the network
			providerMetrics, err := metricGenerator.GenerateMetrics(network)
			if err != nil {
				rm.logger.Error("[REPUTATION_SCORE] Error generating metrics for network", zap.String("network", network), zap.Error(err))
				return errors.Wrapf(err, "Error generating metrics for network %s", network)
			}

			// Group the metrics by provider (Provider -> [Metric1, Metric2, ...])
			for _, metric := range providerMetrics {
				metricsPerProvider[metric.ProviderID()] = append(metricsPerProvider[metric.ProviderID()], metric)
			}
		}

		// Combine the metrics for each provider by reducing them to a single score per provider
		rawScores := map[string]*Score{}
		for providerID, metrics := range metricsPerProvider {
			providerRawScore, err := formula.metricCombiner.CombineMetrics(metrics)
			if err != nil {
				rm.logger.Error("[REPUTATION_SCORE] Error whilecombining metrics for provider", zap.String("network", network), zap.String("providerID", providerID), zap.Error(err))
				return errors.Wrapf(err, "Error while combining metrics for provider %s on network %s", providerID, network)
			}
			rawScores[providerID] = providerRawScore
		}

		// Apply the score transformer
		transformedScores, err := formula.scoreTransformer.TransformScore(rawScores)
		if err != nil {
			rm.logger.Error("[REPUTATION_SCORE] Error while transforming scores for network", zap.String("network", network), zap.Error(err))
			return errors.Wrapf(err, "Error while transforming scores for network %s", network)
		}

		// Finally, update the manager with the new scores
		for providerID, score := range transformedScores {

			// Initialize the scores map for the network if it doesn't exist
			if _, networkExists := rm.scores[network]; !networkExists {
				rm.scores[network] = make(map[string]*Score)
			}

			// Add the final score to the scores map
			rm.logger.Debug("[REPUTATION_SCORE] Adding score for provider", zap.String("network", network), zap.String("providerID", providerID), zap.Any("score", score))
			rm.scores[network][providerID] = score
		}
	}
	return nil
}

func (rm *ReputationScoreManager) GetScore(network string, providerID string) *Score {
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

	// Do immediate initial update
	rm.ComputeScores()

	go func() {
		ticker := time.NewTicker(frequency)
		defer ticker.Stop()

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

	// Copy the scores to a new map to avoid returning the underlying map
	scores := make(map[string]*Score)
	for providerID, score := range rm.scores[network] {
		scores[providerID] = score.Clone()
	}

	return scores
}

func (rm *ReputationScoreManager) AddNetworkWithBuiltInFormula(network string, client watcher.IWatcherAPIClient) {

	builtInFormula := ScoreFormula{
		network: network,
		metricGenerators: []ProviderMetricGenerator{
			&WatcherBlockNumberConsistency{WatcherClient: client, Logger: rm.logger},
			&WatcherBlockNonStateConsistency{WatcherClient: client, Logger: rm.logger},
			&WatcherLatency{WatcherClient: client, Logger: rm.logger},
		},
		metricCombiner: &WeightedCombiner{Weights: map[string]float64{
			BlockNumberConsistencyMetricID:   BlockNumberConsistencyWeight,
			BlockNonStateConsistencyMetricID: BlockNonStateConsistencyWeight,
			LatencyMetricID:                  LatencyWeight,
		}},
		scoreTransformer: NewCompositeTransformer(&EWMATransformer{alpha: ScoreSmoothingFactor}, NewDefaultHighPassThroughTransformer()),
	}
	rm.AddNetworkFormula(network, builtInFormula)
}

func (rm *ReputationScoreManager) AddNetworkFormula(network string, formula ScoreFormula) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.logger.Info("[REPUTATION_SCORE] Adding network with custom formula", zap.String("network", network))

	// Check if the network already exists
	if _, exists := rm.formulas[network]; exists {
		rm.logger.Warn("[REPUTATION_SCORE] A formula for this network already exists, overriding formula", zap.String("network", network))
	}

	rm.formulas[network] = formula
}

func (rm *ReputationScoreManager) GetNetworkFormula(network string) *ScoreFormula {
	if formula, exists := rm.formulas[network]; exists {
		return &formula
	}
	return nil
}

func (rm *ReputationScoreManager) RemoveNetwork(network string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.logger.Info("[REPUTATION_SCORE] Removing network from reputation score manager", zap.String("network", network))

	// Remove from formulas so no more scores will be computed for this network
	delete(rm.formulas, network)

	// Remove from already computed scores
	delete(rm.scores, network)
}
