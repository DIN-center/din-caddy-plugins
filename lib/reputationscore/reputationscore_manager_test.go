package reputationscore

import (
	"testing"
	"time"

	"github.com/DIN-center/din-sc/apps/din-go/lib/watcher"
)

func TestReputationScoreManager(t *testing.T) {
	t.Run("empty manager returns empty scores", func(t *testing.T) {
		rm := NewEmpty()
		scores := rm.GetAllScores("mainnet")
		if len(scores) != 0 {
			t.Errorf("expected 0 scores, got %d", len(scores))
		}
	})

	t.Run("empty manager returns ReputationScore with no value", func(t *testing.T) {
		rm := NewEmpty()
		score := rm.GetScore("mainnet", "provider1")
		if score.HasValue() {
			t.Errorf("expected score has value to be false, got true")
		}
		if !score.LastUpdated().IsZero() {
			t.Errorf("expected last updated to be zero time, got %v", score.LastUpdated())
		}
	})

	t.Run("manager with a single network formula containing a single metric generator for a single provider returns ReputationScore with value 1.0", func(t *testing.T) {
		rm := NewEmpty()
		rm.formulas = []ScoreFormula{
			{
				Network: "gyro",
				MetricGenerators: []ProviderMetricGenerator{
					&WatcherBlockNumberConsistency{WatcherClient: &MockWatcherAPIClient{mockCheckResponses: []watcher.Result[watcher.CheckResponse]{OK_CHECK_RESPONSE_ONE_PROVIDER_GOOD_SCORE}}},
				},
				MetricCombiner: &WeightedCombiner{Weights: map[string]float64{
					BlockNumberConsistencyMetricID: 1.0,
				}},
				ScoreTransformer: &NormalizeTransformer{},
			},
		}

		rm.ComputeScores()

		score := rm.GetScore("gyro", "provider1")
		if !score.HasValue() {
			t.Errorf("expected score has value to be true, got false")
		}
		if score.Value() != 1.0 {
			t.Errorf("expected score value to be 1.0, got %f", score.Value())
		}
	})

	t.Run("manager with a single network formula containing two metric generators for a single provider returns ReputationScore with value 1.0", func(t *testing.T) {
		rm := NewEmpty()
		rm.formulas = []ScoreFormula{
			{
				Network: "gyro",
				MetricGenerators: []ProviderMetricGenerator{
					&WatcherBlockNumberConsistency{
						WatcherClient: &MockWatcherAPIClient{mockCheckResponses: []watcher.Result[watcher.CheckResponse]{OK_CHECK_RESPONSE_ONE_PROVIDER_GOOD_SCORE}}},
					&WatcherLatency{
						WatcherClient: &MockWatcherAPIClient{mockLatencyResponses: []watcher.Result[watcher.LatencyResponse]{OK_LATENCY_RESPONSE_ONE_PROVIDER_GOOD_SCORE}}},
				},
				MetricCombiner: &WeightedCombiner{Weights: map[string]float64{
					BlockNumberConsistencyMetricID: 0.3,
					LatencyMetricID:                0.7,
				}},
				ScoreTransformer: &NormalizeTransformer{},
			},
		}

		rm.ComputeScores()

		score := rm.GetScore("gyro", "provider1")
		if !score.HasValue() {
			t.Errorf("expected score has value to be true, got false")
		}
		if score.Value() != 1.0 {
			t.Errorf("expected score value to be 1.0, got %f", score.Value())
		}
	})

	t.Run("manager with a single network formula containing two metric generators for two providers returns P1 score 0.6906 and P2 score 0.3094", func(t *testing.T) {
		rm := NewEmpty()
		rm.formulas = []ScoreFormula{
			{
				Network: "gyro",
				MetricGenerators: []ProviderMetricGenerator{
					&WatcherBlockNumberConsistency{
						WatcherClient: &MockWatcherAPIClient{mockCheckResponses: []watcher.Result[watcher.CheckResponse]{OK_CHECK_RESPONSE_TWO_PROVIDERS_GOOD_SCORES_TIME1}},
					},
					&WatcherLatency{
						WatcherClient: &MockWatcherAPIClient{mockLatencyResponses: []watcher.Result[watcher.LatencyResponse]{OK_LATENCY_RESPONSE_TWO_PROVIDERS_GOOD_SCORE_TIME1}}},
				},
				MetricCombiner: &WeightedCombiner{Weights: map[string]float64{
					BlockNumberConsistencyMetricID: 0.3,
					LatencyMetricID:                0.7,
				}},
				ScoreTransformer: &NormalizeTransformer{},
			},
		}

		rm.ComputeScores()

		scoreP1 := rm.GetScore("gyro", "provider1")
		scoreP2 := rm.GetScore("gyro", "provider2")

		//Math details: Note that each metric can go up to 1.0, weights sum up to 1.0, so max raw score is 1.0
		//    Metric(BlockNumberConsistency) * W(BlockNumberConsistency) + Metric(Latency) * W(Latency))
		//P1:                 0.95           *              0.3         +      0.9        *       0.7 = Raw Score => 0.915
		//P2:                 0.2            *              0.3         +      0.5         *       0.7 = Raw Score => 0.41

		//Normalized the scores: raw score / (sum of raw scores of all providers), always rounded to 4 decimal places
		//P1: 0.915 / (0.915 + 0.41) = 0.6906
		//P2: 0.41 / (0.915 + 0.41) = 0.3094
		if !scoreP1.HasValue() || scoreP1.Value() != 0.6906 {
			t.Errorf("expected score value to be 0.6906, got %f", scoreP1.Value())
		}
		if !scoreP2.HasValue() || scoreP2.Value() != 0.3094 {
			t.Errorf("expected score value to be 0.3094, got %f", scoreP2.Value())
		}
	})

	t.Run("manager is idempotent as long as the metrics are the same", func(t *testing.T) {
		rm := NewEmpty()
		rm.formulas = []ScoreFormula{
			{
				Network: "gyro",
				MetricGenerators: []ProviderMetricGenerator{
					&WatcherBlockNumberConsistency{
						WatcherClient: &MockWatcherAPIClient{mockCheckResponses: []watcher.Result[watcher.CheckResponse]{OK_CHECK_RESPONSE_TWO_PROVIDERS_GOOD_SCORES_TIME1, OK_CHECK_RESPONSE_TWO_PROVIDERS_GOOD_SCORES_TIME2}},
					},
					&WatcherLatency{
						WatcherClient: &MockWatcherAPIClient{mockLatencyResponses: []watcher.Result[watcher.LatencyResponse]{OK_LATENCY_RESPONSE_TWO_PROVIDERS_GOOD_SCORE_TIME1, OK_LATENCY_RESPONSE_TWO_PROVIDERS_GOOD_SCORE_TIME2}},
					},
				},
				MetricCombiner: &WeightedCombiner{Weights: map[string]float64{
					BlockNumberConsistencyMetricID: 0.3,
					LatencyMetricID:                0.7,
				}},
				ScoreTransformer: &NormalizeTransformer{},
			},
		}

		// Run it once
		rm.ComputeScores()

		firstScoreP1 := rm.GetScore("gyro", "provider1")
		firstScoreP2 := rm.GetScore("gyro", "provider2")

		if !firstScoreP1.HasValue() || firstScoreP1.Value() != 0.6906 || !firstScoreP1.LastUpdated().Equal(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("expected first score value to be Value=0.6906 and Time=2023-01-01T00:00:00Z, got Value=%f and Time=%v", firstScoreP1.Value(), firstScoreP1.LastUpdated())
		}
		if !firstScoreP2.HasValue() || firstScoreP2.Value() != 0.3094 || !firstScoreP2.LastUpdated().Equal(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("expected first score value to be Value=0.3094 and Time=2023-01-01T00:00:00Z, got Value=%f and Time=%v", firstScoreP2.Value(), firstScoreP2.LastUpdated())
		}

		// Run it again
		rm.ComputeScores()

		secondScoreP1 := rm.GetScore("gyro", "provider1")
		secondScoreP2 := rm.GetScore("gyro", "provider2")

		if !secondScoreP1.HasValue() || secondScoreP1.Value() != 0.6906 || !secondScoreP1.LastUpdated().Equal(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("expected second score value to be Value=0.6906 and Time=2024-01-01T00:00:00Z, got Value=%f and Time=%v", secondScoreP1.Value(), secondScoreP1.LastUpdated())
		}
		if !secondScoreP2.HasValue() || secondScoreP2.Value() != 0.3094 || !secondScoreP2.LastUpdated().Equal(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("expected second score value to be Value=0.3094 and Time=2024-01-01T00:00:00Z, got Value=%f and Time=%v", secondScoreP2.Value(), secondScoreP2.LastUpdated())
		}
	})

	t.Run("manager behaviour nicely when the metrics fails to be generated", func(t *testing.T) {
		rm := NewEmpty()
		rm.formulas = []ScoreFormula{
			{
				Network: "gyro",
				MetricGenerators: []ProviderMetricGenerator{
					&WatcherBlockNumberConsistency{
						WatcherClient: &MockWatcherAPIClient{mockCheckResponses: []watcher.Result[watcher.CheckResponse]{KO_CHECK_RESPONSE_API_ERROR}},
					},
					&WatcherLatency{
						WatcherClient: &MockWatcherAPIClient{mockLatencyResponses: []watcher.Result[watcher.LatencyResponse]{OK_LATENCY_RESPONSE_TWO_PROVIDERS_GOOD_SCORE_TIME1}},
					},
				},
				MetricCombiner: &WeightedCombiner{Weights: map[string]float64{
					BlockNumberConsistencyMetricID: 0.3,
					LatencyMetricID:                0.7,
				}},
				ScoreTransformer: &NormalizeTransformer{},
			},
		}

		rm.ComputeScores()

		scoreP1 := rm.GetScore("gyro", "provider1")
		scoreP2 := rm.GetScore("gyro", "provider2")

		if scoreP1.HasValue() {
			t.Errorf("expected score has value to be false, got true")
		}
		if scoreP2.HasValue() {
			t.Errorf("expected score has value to be false, got true")
		}
	})

	t.Run("manager behaviour nicely when one metric has only zero values and another has a score", func(t *testing.T) {
		rm := NewEmpty()
		rm.formulas = []ScoreFormula{
			{
				Network: "gyro",
				MetricGenerators: []ProviderMetricGenerator{
					&WatcherBlockNumberConsistency{
						WatcherClient: &MockWatcherAPIClient{mockCheckResponses: []watcher.Result[watcher.CheckResponse]{OK_CHECK_RESPONSE_TWO_PROVIDERS_BAD_SCORES_TIME1}},
					},
					&WatcherLatency{
						WatcherClient: &MockWatcherAPIClient{mockLatencyResponses: []watcher.Result[watcher.LatencyResponse]{OK_LATENCY_RESPONSE_TWO_PROVIDERS_BAD_SCORE_TIME1}},
					},
				},
				MetricCombiner: &WeightedCombiner{Weights: map[string]float64{
					BlockNumberConsistencyMetricID: 0.3,
					LatencyMetricID:                0.7,
				}},
				ScoreTransformer: &NormalizeTransformer{},
			},
		}

		rm.ComputeScores()

		scoreP1 := rm.GetScore("gyro", "provider1")
		scoreP2 := rm.GetScore("gyro", "provider2")

		if !scoreP1.HasValue() || scoreP1.Value() != 0.0 {
			t.Errorf("expected score has value to be 0.0, got %f", scoreP1.Value())
		}
		if !scoreP2.HasValue() || scoreP2.Value() != 1.0 {
			t.Errorf("expected score has value to be 1.0, got %f", scoreP2.Value())
		}
	})

	t.Run("manager behaviour nicely when the metrics has only zero values", func(t *testing.T) {
		rm := NewEmpty()
		rm.formulas = []ScoreFormula{
			{
				Network: "gyro",
				MetricGenerators: []ProviderMetricGenerator{
					&WatcherBlockNumberConsistency{
						WatcherClient: &MockWatcherAPIClient{mockCheckResponses: []watcher.Result[watcher.CheckResponse]{OK_CHECK_RESPONSE_ONE_PROVIDER_BAD_SCORE}},
					},
					&WatcherLatency{
						WatcherClient: &MockWatcherAPIClient{mockLatencyResponses: []watcher.Result[watcher.LatencyResponse]{OK_LATENCY_RESPONSE_ONE_PROVIDER_BAD_SCORE}},
					},
				},
				MetricCombiner: &WeightedCombiner{Weights: map[string]float64{
					BlockNumberConsistencyMetricID: 0.3,
					LatencyMetricID:                0.7,
				}},
				ScoreTransformer: &NormalizeTransformer{},
			},
		}

		rm.ComputeScores()

		scoreP1 := rm.GetScore("gyro", "provider1")

		if scoreP1.HasValue() {
			t.Errorf("expected score has value to be false, got true")
		}

	})

	t.Run("manager prevents very small scores", func(t *testing.T) {
		rm := NewEmpty()
		rm.formulas = []ScoreFormula{
			{
				Network: "gyro",
				MetricGenerators: []ProviderMetricGenerator{
					&WatcherBlockNumberConsistency{WatcherClient: &MockWatcherAPIClient{mockCheckResponses: []watcher.Result[watcher.CheckResponse]{OK_CHECK_RESPONSE_ONE_PROVIDER_GOOD_SCORE}}},
				},
				MetricCombiner: &WeightedCombiner{Weights: map[string]float64{
					BlockNumberConsistencyMetricID: 0.0001,
				}},
				ScoreTransformer: &NormalizeTransformer{},
			},
		}

		rm.ComputeScores()
		//Raw score should be 0.95 * 0.0001 = 0,000095
		//Normalized score should be 0,000095 / 0,000095 = 1.0 but raw score is too small to be considered a score

		score := rm.GetScore("gyro", "provider1")
		if score.HasValue() {
			t.Errorf("expected score has value to be false, got true")
		}
	})

	t.Run("manager update scores as long as metrics changes over time", func(t *testing.T) {
		rm := NewEmpty()
		rm.formulas = []ScoreFormula{
			{
				Network: "gyro",
				MetricGenerators: []ProviderMetricGenerator{
					&WatcherBlockNumberConsistency{
						WatcherClient: &MockWatcherAPIClient{mockCheckResponses: []watcher.Result[watcher.CheckResponse]{OK_CHECK_RESPONSE_TWO_PROVIDERS_GOOD_SCORES_TIME1, OK_CHECK_RESPONSE_TWO_PROVIDERS_BAD_SCORES_TIME1}},
					},
				},
				MetricCombiner: &WeightedCombiner{Weights: map[string]float64{
					BlockNumberConsistencyMetricID: 1.0,
				}},
				ScoreTransformer: &NormalizeTransformer{},
			},
		}
		//Run scores once
		rm.ComputeScores()

		firstScoreP1 := rm.GetScore("gyro", "provider1")
		firstScoreP2 := rm.GetScore("gyro", "provider2")
		if !firstScoreP1.HasValue() || firstScoreP1.Value() != 0.8261 {
			t.Errorf("expected score has value to be 0.8261, got %f", firstScoreP1.Value())
		}
		if !firstScoreP2.HasValue() || firstScoreP2.Value() != 0.1739 {
			t.Errorf("expected score has value to be 0.1739, got %f", firstScoreP2.Value())
		}

		//Run scores again
		rm.ComputeScores()

		secondScoreP1 := rm.GetScore("gyro", "provider1")
		secondScoreP2 := rm.GetScore("gyro", "provider2")
		if !secondScoreP1.HasValue() || secondScoreP1.Value() != 0.0 {
			t.Errorf("expected score has value to be 0.0, got %f", secondScoreP1.Value())
		}
		if !secondScoreP2.HasValue() || secondScoreP2.Value() != 1.0 {
			t.Errorf("expected score has value to be 1.0, got %f", secondScoreP2.Value())
		}
	})

	t.Run("manager update scores smoothly when the metrics changes abruptly over time (using EWMA transform)", func(t *testing.T) {
		rm := NewEmpty()
		rm.formulas = []ScoreFormula{
			{
				Network: "gyro",
				MetricGenerators: []ProviderMetricGenerator{
					&WatcherBlockNumberConsistency{
						WatcherClient: &MockWatcherAPIClient{mockCheckResponses: []watcher.Result[watcher.CheckResponse]{OK_CHECK_RESPONSE_TWO_PROVIDERS_GOOD_SCORES_TIME1, OK_CHECK_RESPONSE_TWO_PROVIDERS_BAD_SCORES_TIME1}},
					},
				},
				MetricCombiner: &WeightedCombiner{Weights: map[string]float64{
					BlockNumberConsistencyMetricID: 1.0,
				}},
				ScoreTransformer: &CompositeTransformer{chain: []ScoreTransformer{&NormalizeTransformer{}, &EWMATransformer{alpha: 0.7}}},
			},
		}
		//Run scores once
		rm.ComputeScores()

		firstScoreP1 := rm.GetScore("gyro", "provider1")
		firstScoreP2 := rm.GetScore("gyro", "provider2")
		if !firstScoreP1.HasValue() || firstScoreP1.Value() != 0.8261 {
			t.Errorf("expected score has value to be 0.8261, got %f", firstScoreP1.Value())
		}
		if !firstScoreP2.HasValue() || firstScoreP2.Value() != 0.1739 {
			t.Errorf("expected score has value to be 0.1739, got %f", firstScoreP2.Value())
		}

		//Run scores again
		rm.ComputeScores()

		secondScoreP1 := rm.GetScore("gyro", "provider1")
		secondScoreP2 := rm.GetScore("gyro", "provider2")
		if !secondScoreP1.HasValue() || secondScoreP1.Value() != 0.2478 {
			t.Errorf("expected score has value to be 0.2478, got %f", secondScoreP1.Value())
		}
		if !secondScoreP2.HasValue() || secondScoreP2.Value() != 0.7522 {
			t.Errorf("expected score has value to be 0.7522, got %f", secondScoreP2.Value())
		}

	})
}
