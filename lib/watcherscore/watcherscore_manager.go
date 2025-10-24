package watcherscore

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

// WatcherScoreManager is responsible for computing watcher scores for providers across networks
// and providing a way to get the score for a specific provider on a specific network
type WatcherScoreManager struct {
	scores   map[string]map[string]*Score // map[network]map[providerID]*WatcherScore
	formulas map[string]ScoreFormula      // map[network]ScoreFormula
	logger   *zap.Logger
	mu       sync.RWMutex
}

func NewEmpty(logger *zap.Logger) *WatcherScoreManager {
	return &WatcherScoreManager{
		scores:   make(map[string]map[string]*Score),
		formulas: make(map[string]ScoreFormula),
		logger:   logger,
		mu:       sync.RWMutex{},
	}
}

func NewWithBuiltInFormula(networks []string, client watcher.IWatcherAPIClient, logger *zap.Logger) *WatcherScoreManager {

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

func (rm *WatcherScoreManager) ComputeScores() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.logger.Info("[WATCHER_SCORE] Computing watcher scores...")

	// Process each formula that defines how to compute the score for a network
	for _, formula := range rm.formulas {
		network := formula.network

		// A score may be composed of multiple metrics, so we need to collect all metrics for each provider
		metricsPerProvider := make(map[string][]*ProviderMetric)
		allMetricsCompleted := true
		for _, metricGenerator := range formula.metricGenerators {

			// A metric generator produces the same metric type for all providers monitored on the network
			providerMetrics, err := metricGenerator.GenerateMetrics(network)
			if err != nil {
				rm.logger.Error("[WATCHER_SCORE] Error while generating metrics for network", zap.String("network", network), zap.Error(err))
				// If one metric generator fails, we are not able to compute the scores for this network
				allMetricsCompleted = false
				break
			}

			// If no metrics are generated, we are not able to compute the scores for this network
			if len(providerMetrics) == 0 {
				rm.logger.Error("[WATCHER_SCORE] Empty metrics generated for network", zap.String("network", network))
				allMetricsCompleted = false
				break
			}

			// Group the metrics by provider (Provider -> [Metric1, Metric2, ...])
			for _, metric := range providerMetrics {
				metricsPerProvider[metric.ProviderID()] = append(metricsPerProvider[metric.ProviderID()], metric)
			}
		}

		if !allMetricsCompleted {
			rm.logger.Error("[WATCHER_SCORE] Not all metrics completed for network, skipping network", zap.String("network", network))
			continue
		}

		// Combine the metrics for each provider by reducing them to a single score per provider
		rawScoresPerProvider := map[string]*Score{}
		for providerID, metrics := range metricsPerProvider {
			providerRawScore, err := formula.metricCombiner.CombineMetrics(metrics)
			if err != nil {
				rm.logger.Error("[WATCHER_SCORE] Error while combining metrics for provider", zap.String("network", network), zap.String("providerID", providerID), zap.Error(err))
				// If one provider fails, we just skip it and continue with the next one
				continue
			}
			rawScoresPerProvider[providerID] = providerRawScore
		}

		// Apply the score transformer
		transformedScores, err := formula.scoreTransformer.TransformScore(network, rawScoresPerProvider)
		if err != nil {
			rm.logger.Error("[WATCHER_SCORE] Error while transforming scores for network", zap.String("network", network), zap.Error(err))
			// If final score transformation fails, we just skip the entire network
			continue
		}

		// Finally, update the manager with the new scores
		for providerID, score := range transformedScores {

			// Initialize the scores map for the network if it doesn't exist
			if _, networkExists := rm.scores[network]; !networkExists {
				rm.scores[network] = make(map[string]*Score)
			}

			// Add the final score to the scores map
			rm.logger.Info("[WATCHER_SCORE] Provider score",
				zap.String("network", network),
				zap.String("provider", providerID),
				zap.Bool("isValid", score.HasValue()),
				zap.Float64("score", score.Value()),
				zap.Time("lastUpdated", score.LastUpdated()))
			rm.scores[network][providerID] = score
		}
	}
}

func (rm *WatcherScoreManager) GetScore(network string, providerID string) *Score {
	// Lock the manager to prevent race condition when reading scores
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

func (rm *WatcherScoreManager) StartPeriodicUpdates(frequency time.Duration) chan struct{} {
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
				rm.logger.Info("[WATCHER_SCORE] Watcher compute score sync goroutine shutting down gracefully")
				return
			}
		}
	}()

	return stopChan
}

func (rm *WatcherScoreManager) GetAllScores(network string) map[string]*Score {

	// Lock the manager to prevent race condition when reading scores
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	// Copy the scores to a new map to avoid returning the underlying map
	scores := make(map[string]*Score)
	for providerID, score := range rm.scores[network] {
		scores[providerID] = score.Clone()
	}

	return scores
}

func (rm *WatcherScoreManager) AddNetworkWithBuiltInFormula(network string, client watcher.IWatcherAPIClient) error {

	combiner, err := NewWeightedCombiner(map[string]float64{
		BlockNumberConsistencyMetricID:   BlockNumberConsistencyWeight,
		BlockNonStateConsistencyMetricID: BlockNonStateConsistencyWeight,
		LatencyMetricID:                  LatencyWeight,
	}, rm.logger)
	if err != nil {
		rm.logger.Error("[WATCHER_SCORE] Error while creating weighted combiner", zap.Error(err))
		return errors.Wrapf(err, "Error while creating weighted combiner")
	}

	builtInFormula := ScoreFormula{
		network: network,
		metricGenerators: []ProviderMetricGenerator{
			&WatcherBlockNumberConsistency{WatcherClient: client, Logger: rm.logger},
			&WatcherBlockNonStateConsistency{WatcherClient: client, Logger: rm.logger},
			&WatcherLatency{WatcherClient: client, Logger: rm.logger},
		},
		metricCombiner:   combiner,
		scoreTransformer: NewCompositeTransformer(NewEWMATransformer(ScoreSmoothingFactor, rm.logger), NewDefaultHighPassThroughTransformer(rm.logger)),
	}
	rm.AddNetworkFormula(network, builtInFormula)
	return nil
}

func (rm *WatcherScoreManager) AddNetworkFormula(network string, formula ScoreFormula) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.logger.Info("[WATCHER_SCORE] Adding network with custom formula", zap.String("network", network))

	// Check if the network already exists
	if _, exists := rm.formulas[network]; exists {
		rm.logger.Warn("[WATCHER_SCORE] A formula for this network already exists, overriding formula", zap.String("network", network))
	}

	rm.formulas[network] = formula
}

func (rm *WatcherScoreManager) GetNetworkFormula(network string) *ScoreFormula {
	if formula, exists := rm.formulas[network]; exists {
		return &formula
	}
	return nil
}

func (rm *WatcherScoreManager) RemoveNetwork(network string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.logger.Info("[WATCHER_SCORE] Removing network from watcher score manager", zap.String("network", network))

	// Remove from formulas so no more scores will be computed for this network
	delete(rm.formulas, network)

	// Remove from already computed scores
	delete(rm.scores, network)
}
