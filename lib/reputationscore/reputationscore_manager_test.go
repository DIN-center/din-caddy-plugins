package reputationscore

import (
	"testing"
	"time"

	"github.com/DIN-center/din-sc/apps/din-go/lib/watcher"
	"go.uber.org/zap/zaptest"
)

func TestReputationScoreManager(t *testing.T) {
	t.Run("empty manager returns empty scores", func(t *testing.T) {
		rm := NewEmpty(zaptest.NewLogger(t))
		scores := rm.GetAllScores("mainnet")
		if len(scores) != 0 {
			t.Errorf("expected 0 scores, got %d", len(scores))
		}
	})

	t.Run("empty manager returns ReputationScore with no value", func(t *testing.T) {
		rm := NewEmpty(zaptest.NewLogger(t))
		score := rm.GetScore("mainnet", "provider1")
		if score.HasValue() {
			t.Errorf("expected score has value to be false, got true")
		}
		if !score.LastUpdated().IsZero() {
			t.Errorf("expected last updated to be zero time, got %v", score.LastUpdated())
		}
	})

	t.Run("manager with a single network formula containing a single metric generator for a single provider returns ReputationScore with value 0.95", func(t *testing.T) {
		rm := NewEmpty(zaptest.NewLogger(t))
		rm.AddNetworkFormula("gyro", ScoreFormula{
			network: "gyro",
			metricGenerators: []ProviderMetricGenerator{
				&WatcherBlockNumberConsistency{
					WatcherClient: &MockWatcherAPIClient{mockCheckResponses: []watcher.Result[watcher.CheckResponse]{OK_CHECK_RESPONSE_ONE_PROVIDER_GOOD_SCORE}},
					Logger:        zaptest.NewLogger(t),
				},
			},
			metricCombiner: MustCreateWeightedCombiner(map[string]float64{
				BlockNumberConsistencyMetricID: 1.0,
			}),
			scoreTransformer: NewDefaultHighPassThroughTransformer(),
		})

		rm.ComputeScores()

		score := rm.GetScore("gyro", "provider1.com")
		if !score.HasValue() {
			t.Errorf("expected score has value to be true, got false")
		}
		if !Float64Equal(score.Value(), 0.95) {
			t.Errorf("expected score value to be 0.95, got %f", score.Value())
		}
	})

	t.Run("manager with a single network formula containing two metric generators for a single provider returns ReputationScore with value 0.915", func(t *testing.T) {
		rm := NewEmpty(zaptest.NewLogger(t))
		rm.AddNetworkFormula("gyro", ScoreFormula{
			network: "gyro",
			metricGenerators: []ProviderMetricGenerator{
				&WatcherBlockNumberConsistency{
					WatcherClient: &MockWatcherAPIClient{mockCheckResponses: []watcher.Result[watcher.CheckResponse]{OK_CHECK_RESPONSE_ONE_PROVIDER_GOOD_SCORE}},
					Logger:        zaptest.NewLogger(t),
				},
				&WatcherLatency{
					WatcherClient: &MockWatcherAPIClient{mockLatencyResponses: []watcher.Result[watcher.LatencyResponse]{OK_LATENCY_RESPONSE_ONE_PROVIDER_GOOD_SCORE}},
					Logger:        zaptest.NewLogger(t),
				},
			},
			metricCombiner: MustCreateWeightedCombiner(map[string]float64{
				BlockNumberConsistencyMetricID: 0.3,
				LatencyMetricID:                0.7,
			}),
			scoreTransformer: NewDefaultHighPassThroughTransformer(),
		})

		rm.ComputeScores()

		score := rm.GetScore("gyro", "provider1.com")
		if !score.HasValue() {
			t.Errorf("expected score has value to be true, got false")
		}
		if !Float64Equal(score.Value(), 0.86712) {
			t.Errorf("expected score value to be 0.86712, got %f", score.Value())
		}
	})

	t.Run("manager with a single network formula containing two metric generators for two providers returns P1 score 0.6906 and P2 score 0.3094", func(t *testing.T) {
		rm := NewEmpty(zaptest.NewLogger(t))
		rm.AddNetworkFormula("gyro", ScoreFormula{
			network: "gyro",
			metricGenerators: []ProviderMetricGenerator{
				&WatcherBlockNumberConsistency{
					WatcherClient: &MockWatcherAPIClient{mockCheckResponses: []watcher.Result[watcher.CheckResponse]{OK_CHECK_RESPONSE_TWO_PROVIDERS_GOOD_SCORES_TIME1}},
					Logger:        zaptest.NewLogger(t),
				},
				&WatcherLatency{
					WatcherClient: &MockWatcherAPIClient{mockLatencyResponses: []watcher.Result[watcher.LatencyResponse]{OK_LATENCY_RESPONSE_TWO_PROVIDERS_GOOD_SCORE_TIME1}},
					Logger:        zaptest.NewLogger(t),
				},
			},
			metricCombiner: MustCreateWeightedCombiner(map[string]float64{
				BlockNumberConsistencyMetricID: 0.3,
				LatencyMetricID:                0.7,
			}),
			scoreTransformer: NewDefaultHighPassThroughTransformer(),
		})

		rm.ComputeScores()

		scoreP1 := rm.GetScore("gyro", "provider1.com")
		scoreP2 := rm.GetScore("gyro", "provider2.com")

		//Math details: Note that each metric can go up to 1.0, weights sum up to 1.0, so max raw score is 1.0
		//    Metric(BlockNumberConsistency) * W(BlockNumberConsistency) + Metric(Latency) * W(Latency))
		//P1:                 0.95           *              0.3         +      0.9        *       0.7 = Raw Score => 0.915
		//P2:                 0.2            *              0.3         +      0.5         *       0.7 = Raw Score => 0.41

		if !scoreP1.HasValue() || !Float64Equal(scoreP1.Value(), 0.86712) {
			t.Errorf("expected score value to be 0.86712, got %f", scoreP1.Value())
		}
		if !scoreP2.HasValue() || !Float64Equal(scoreP2.Value(), 0.25516) {
			t.Errorf("expected score value to be 0.25516, got %f", scoreP2.Value())
		}
	})

	t.Run("manager is idempotent as long as the metrics are the same", func(t *testing.T) {
		rm := NewEmpty(zaptest.NewLogger(t))
		rm.AddNetworkFormula("gyro", ScoreFormula{
			network: "gyro",
			metricGenerators: []ProviderMetricGenerator{
				&WatcherBlockNumberConsistency{
					WatcherClient: &MockWatcherAPIClient{
						mockCheckResponses: []watcher.Result[watcher.CheckResponse]{
							OK_CHECK_RESPONSE_TWO_PROVIDERS_GOOD_SCORES_TIME1,
							OK_CHECK_RESPONSE_TWO_PROVIDERS_GOOD_SCORES_TIME2,
						},
					},
					Logger: zaptest.NewLogger(t),
				},
				&WatcherLatency{
					WatcherClient: &MockWatcherAPIClient{
						mockLatencyResponses: []watcher.Result[watcher.LatencyResponse]{
							OK_LATENCY_RESPONSE_TWO_PROVIDERS_GOOD_SCORE_TIME1,
							OK_LATENCY_RESPONSE_TWO_PROVIDERS_GOOD_SCORE_TIME2,
						},
					},
					Logger: zaptest.NewLogger(t),
				},
			},
			metricCombiner: MustCreateWeightedCombiner(map[string]float64{
				BlockNumberConsistencyMetricID: 0.3,
				LatencyMetricID:                0.7,
			}),
			scoreTransformer: NewDefaultHighPassThroughTransformer(),
		})

		// Run it once
		rm.ComputeScores()

		firstScoreP1 := rm.GetScore("gyro", "provider1.com")
		firstScoreP2 := rm.GetScore("gyro", "provider2.com")

		if !firstScoreP1.HasValue() || !Float64Equal(firstScoreP1.Value(), 0.86712) || !firstScoreP1.LastUpdated().Equal(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("expected first score value to be Value=0.86712 and Time=2023-01-01T00:00:00Z, got Value=%f and Time=%v", firstScoreP1.Value(), firstScoreP1.LastUpdated())
		}
		if !firstScoreP2.HasValue() || !Float64Equal(firstScoreP2.Value(), 0.25516) || !firstScoreP2.LastUpdated().Equal(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("expected first score value to be Value=0.25516 and Time=2023-01-01T00:00:00Z, got Value=%f and Time=%v", firstScoreP2.Value(), firstScoreP2.LastUpdated())
		}

		// Run it again
		rm.ComputeScores()

		secondScoreP1 := rm.GetScore("gyro", "provider1.com")
		secondScoreP2 := rm.GetScore("gyro", "provider2.com")

		if !secondScoreP1.HasValue() || !Float64Equal(secondScoreP1.Value(), 0.86712) || !secondScoreP1.LastUpdated().Equal(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("expected second score value to be Value=0.86712 and Time=2024-01-01T00:00:00Z, got Value=%f and Time=%v", secondScoreP1.Value(), secondScoreP1.LastUpdated())
		}
		if !secondScoreP2.HasValue() || !Float64Equal(secondScoreP2.Value(), 0.25516) || !secondScoreP2.LastUpdated().Equal(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("expected second score value to be Value=0.25516 and Time=2024-01-01T00:00:00Z, got Value=%f and Time=%v", secondScoreP2.Value(), secondScoreP2.LastUpdated())
		}
	})

	t.Run("manager behaviour nicely when the metrics fails to be generated", func(t *testing.T) {
		rm := NewEmpty(zaptest.NewLogger(t))
		rm.AddNetworkFormula("gyro", ScoreFormula{
			network: "gyro",
			metricGenerators: []ProviderMetricGenerator{
				&WatcherBlockNumberConsistency{
					WatcherClient: &MockWatcherAPIClient{mockCheckResponses: []watcher.Result[watcher.CheckResponse]{KO_CHECK_RESPONSE_API_ERROR}},
					Logger:        zaptest.NewLogger(t),
				},
				&WatcherLatency{
					WatcherClient: &MockWatcherAPIClient{mockLatencyResponses: []watcher.Result[watcher.LatencyResponse]{OK_LATENCY_RESPONSE_TWO_PROVIDERS_GOOD_SCORE_TIME1}},
					Logger:        zaptest.NewLogger(t),
				},
			},
			metricCombiner: MustCreateWeightedCombiner(map[string]float64{
				BlockNumberConsistencyMetricID: 0.3,
				LatencyMetricID:                0.7,
			}),
			scoreTransformer: NewDefaultHighPassThroughTransformer(),
		})

		rm.ComputeScores()

		scoreP1 := rm.GetScore("gyro", "provider1.com")
		scoreP2 := rm.GetScore("gyro", "provider2.com")

		if scoreP1.HasValue() {
			t.Errorf("expected score has value to be false, got true")
		}
		if scoreP2.HasValue() {
			t.Errorf("expected score has value to be false, got true")
		}
	})

	t.Run("manager behaviour nicely when one metric has only zero values and another has a score", func(t *testing.T) {
		rm := NewEmpty(zaptest.NewLogger(t))
		rm.AddNetworkFormula("gyro", ScoreFormula{
			network: "gyro",
			metricGenerators: []ProviderMetricGenerator{
				&WatcherBlockNumberConsistency{
					WatcherClient: &MockWatcherAPIClient{mockCheckResponses: []watcher.Result[watcher.CheckResponse]{OK_CHECK_RESPONSE_TWO_PROVIDERS_BAD_SCORES_TIME1}},
					Logger:        zaptest.NewLogger(t),
				},
				&WatcherLatency{
					WatcherClient: &MockWatcherAPIClient{mockLatencyResponses: []watcher.Result[watcher.LatencyResponse]{OK_LATENCY_RESPONSE_TWO_PROVIDERS_BAD_SCORE_TIME1}},
					Logger:        zaptest.NewLogger(t),
				},
			},
			metricCombiner: MustCreateWeightedCombiner(map[string]float64{
				BlockNumberConsistencyMetricID: 0.3,
				LatencyMetricID:                0.7,
			}),
			scoreTransformer: NewDefaultHighPassThroughTransformer(),
		})

		rm.ComputeScores()

		scoreP1 := rm.GetScore("gyro", "provider1.com")
		scoreP2 := rm.GetScore("gyro", "provider2.com")

		if !scoreP1.HasValue() || scoreP1.Value() != 0.0 {
			t.Errorf("expected score has value to be 0.0, got %f", scoreP1.Value())
		}
		if !scoreP2.HasValue() || scoreP2.Value() != 0.435160 {
			t.Errorf("expected score has value to be 0.435160, got %f", scoreP2.Value())
		}
	})

	t.Run("manager behaviour nicely when the metrics has only zero values", func(t *testing.T) {
		rm := NewEmpty(zaptest.NewLogger(t))
		rm.AddNetworkFormula("gyro", ScoreFormula{
			network: "gyro",
			metricGenerators: []ProviderMetricGenerator{
				&WatcherBlockNumberConsistency{
					WatcherClient: &MockWatcherAPIClient{mockCheckResponses: []watcher.Result[watcher.CheckResponse]{OK_CHECK_RESPONSE_ONE_PROVIDER_BAD_SCORE}},
					Logger:        zaptest.NewLogger(t),
				},
				&WatcherLatency{
					WatcherClient: &MockWatcherAPIClient{mockLatencyResponses: []watcher.Result[watcher.LatencyResponse]{OK_LATENCY_RESPONSE_ONE_PROVIDER_BAD_SCORE}},
					Logger:        zaptest.NewLogger(t),
				},
			},
			metricCombiner: MustCreateWeightedCombiner(map[string]float64{
				BlockNumberConsistencyMetricID: 0.3,
				LatencyMetricID:                0.7,
			}),
			scoreTransformer: NewDefaultHighPassThroughTransformer(),
		})

		rm.ComputeScores()

		scoreP1 := rm.GetScore("gyro", "provider1.com")

		if !scoreP1.HasValue() || scoreP1.Value() != 0.0 {
			t.Errorf("expected score has value to be 0.0, got %f", scoreP1.Value())
		}

	})

	t.Run("manager prevents very small scores", func(t *testing.T) {
		rm := NewEmpty(zaptest.NewLogger(t))
		rm.AddNetworkFormula("gyro", ScoreFormula{
			network: "gyro",
			metricGenerators: []ProviderMetricGenerator{
				&WatcherBlockNumberConsistency{
					WatcherClient: &MockWatcherAPIClient{mockCheckResponses: []watcher.Result[watcher.CheckResponse]{OK_CHECK_RESPONSE_ONE_PROVIDER_GOOD_SCORE}},
					Logger:        zaptest.NewLogger(t),
				},
				&WatcherLatency{
					WatcherClient: &MockWatcherAPIClient{mockLatencyResponses: []watcher.Result[watcher.LatencyResponse]{OK_LATENCY_RESPONSE_ONE_PROVIDER_BAD_SCORE}},
					Logger:        zaptest.NewLogger(t),
				},
			},
			metricCombiner: MustCreateWeightedCombiner(map[string]float64{
				BlockNumberConsistencyMetricID: 0.0001,
				LatencyMetricID:                0.9999,
			}),
			scoreTransformer: NewDefaultHighPassThroughTransformer(),
		})

		rm.ComputeScores()
		//Raw score should be 0.95 * 0.0001 = 0,000095

		score := rm.GetScore("gyro", "provider1.com")
		if !score.HasValue() || score.Value() != 0.0 {
			t.Errorf("expected score has value to be 0.0, got %f", score.Value())
		}
	})

	t.Run("manager update scores as long as metrics changes over time", func(t *testing.T) {
		rm := NewEmpty(zaptest.NewLogger(t))
		rm.AddNetworkFormula("gyro", ScoreFormula{
			network: "gyro",
			metricGenerators: []ProviderMetricGenerator{
				&WatcherBlockNumberConsistency{
					WatcherClient: &MockWatcherAPIClient{
						mockCheckResponses: []watcher.Result[watcher.CheckResponse]{
							OK_CHECK_RESPONSE_TWO_PROVIDERS_GOOD_SCORES_TIME1,
							OK_CHECK_RESPONSE_TWO_PROVIDERS_BAD_SCORES_TIME1,
						},
					},
					Logger: zaptest.NewLogger(t),
				},
			},
			metricCombiner: MustCreateWeightedCombiner(map[string]float64{
				BlockNumberConsistencyMetricID: 1.0,
			}),
			scoreTransformer: NewDefaultHighPassThroughTransformer(),
		})
		//Run scores once
		rm.ComputeScores()

		firstScoreP1 := rm.GetScore("gyro", "provider1.com")
		firstScoreP2 := rm.GetScore("gyro", "provider2.com")
		if !firstScoreP1.HasValue() || firstScoreP1.Value() != 0.95 {
			t.Errorf("expected score has value to be 0.95, got %f", firstScoreP1.Value())
		}
		if !firstScoreP2.HasValue() || firstScoreP2.Value() != 0.2 {
			t.Errorf("expected score has value to be 0.2, got %f", firstScoreP2.Value())
		}

		//Run scores again
		rm.ComputeScores()

		secondScoreP1 := rm.GetScore("gyro", "provider1.com")
		secondScoreP2 := rm.GetScore("gyro", "provider2.com")
		if !secondScoreP1.HasValue() || secondScoreP1.Value() != 0.0 {
			t.Errorf("expected score has value to be 0.0, got %f", secondScoreP1.Value())
		}
		if !secondScoreP2.HasValue() || secondScoreP2.Value() != 0.8 {
			t.Errorf("expected score has value to be 0.8, got %f", secondScoreP2.Value())
		}
	})

	t.Run("manager update scores smoothly when the metrics changes abruptly over time (using EWMA transform)", func(t *testing.T) {
		rm := NewEmpty(zaptest.NewLogger(t))
		rm.AddNetworkFormula("gyro", ScoreFormula{
			network: "gyro",
			metricGenerators: []ProviderMetricGenerator{
				&WatcherBlockNumberConsistency{
					WatcherClient: &MockWatcherAPIClient{
						mockCheckResponses: []watcher.Result[watcher.CheckResponse]{
							OK_CHECK_RESPONSE_TWO_PROVIDERS_GOOD_SCORES_TIME1,
							OK_CHECK_RESPONSE_TWO_PROVIDERS_BAD_SCORES_TIME1,
						},
					},
					Logger: zaptest.NewLogger(t),
				},
			},
			metricCombiner: MustCreateWeightedCombiner(map[string]float64{
				BlockNumberConsistencyMetricID: 1.0,
			}),
			scoreTransformer: NewCompositeTransformer(&EWMATransformer{alpha: 0.7}, NewDefaultHighPassThroughTransformer()),
		})
		//Run scores once
		rm.ComputeScores()

		firstScoreP1 := rm.GetScore("gyro", "provider1.com")
		firstScoreP2 := rm.GetScore("gyro", "provider2.com")
		if !firstScoreP1.HasValue() || firstScoreP1.Value() != 0.95 {
			t.Errorf("expected score has value to be 0.95, got %f", firstScoreP1.Value())
		}
		if !firstScoreP2.HasValue() || firstScoreP2.Value() != 0.2 {
			t.Errorf("expected score has value to be 0.2, got %f", firstScoreP2.Value())
		}

		//Run scores again
		rm.ComputeScores()

		secondScoreP1 := rm.GetScore("gyro", "provider1.com")
		secondScoreP2 := rm.GetScore("gyro", "provider2.com")
		if !secondScoreP1.HasValue() || secondScoreP1.Value() != 0.285 {
			t.Errorf("expected score has value to be 0.285, got %f", secondScoreP1.Value())
		}
		if !secondScoreP2.HasValue() || secondScoreP2.Value() != 0.62 {
			t.Errorf("expected score has value to be 0.62, got %f", secondScoreP2.Value())
		}

	})

	t.Run("manager removes network and its scores when RemoveNetwork is called", func(t *testing.T) {
		rm := NewEmpty(zaptest.NewLogger(t))
		rm.AddNetworkFormula("gyro", ScoreFormula{
			network: "gyro",
			metricGenerators: []ProviderMetricGenerator{
				&WatcherBlockNumberConsistency{
					WatcherClient: &MockWatcherAPIClient{
						mockCheckResponses: []watcher.Result[watcher.CheckResponse]{
							OK_CHECK_RESPONSE_TWO_PROVIDERS_GOOD_SCORES_TIME1,
						},
					},
					Logger: zaptest.NewLogger(t),
				},
			},
			metricCombiner: MustCreateWeightedCombiner(map[string]float64{
				BlockNumberConsistencyMetricID: 1.0,
			}),
			scoreTransformer: NewDefaultHighPassThroughTransformer(),
		})

		// Compute initial scores
		rm.ComputeScores()

		// Verify scores exist before removal
		initialScoreP1 := rm.GetScore("gyro", "provider1.com")
		if !initialScoreP1.HasValue() {
			t.Error("expected score to have value before network removal")
		}

		// Remove the network
		rm.RemoveNetwork("gyro")

		// Verify formula was removed
		if _, exists := rm.formulas["gyro"]; exists {
			t.Error("expected formula to be removed")
		}

		// Verify scores were cleared
		scoreAfterRemoval := rm.GetScore("gyro", "provider1.com")
		if scoreAfterRemoval.HasValue() {
			t.Error("expected score to be empty after network removal")
		}
	})
}
