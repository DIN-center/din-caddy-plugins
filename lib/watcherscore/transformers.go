package watcherscore

import (
	"math"

	"go.uber.org/zap"
)

// CompositeTransformer is a transformer that chains multiple transformers.
// It applies the result of each transformer in the chain to the next one in the chain and returns the final result.
type CompositeTransformer struct {
	chain []ScoreTransformer
}

func (t *CompositeTransformer) TransformScore(network string, scores map[string]*Score) (map[string]*Score, error) {
	var transformedScores = scores
	var err error
	for _, transformer := range t.chain {
		transformedScores, err = transformer.TransformScore(network, transformedScores)
		if err != nil {
			return nil, err
		}
	}
	return transformedScores, nil
}

func NewCompositeTransformer(chain ...ScoreTransformer) *CompositeTransformer {
	return &CompositeTransformer{chain: chain}
}

// EWMATransformer is a transformer that applies an Exponential Weighted Moving Average (EWMA) to the scores.
// It uses a smoothing factor (called alpha) to control the rate of change of the score.
// A lower value of alpha means less change and more smoothing; a higher value means more rapid adaptation to new provider situation.
// See https://en.wikipedia.org/wiki/Exponential_smoothing#Basic_(simple)_exponential_smoothing
type EWMATransformer struct {
	alpha             float64
	previousScores    map[string]*Score
	hasPreviousScores bool
	logger            *zap.Logger
}

func (t *EWMATransformer) TransformScore(network string, scores map[string]*Score) (map[string]*Score, error) {
	// If this is the first time we're running the transformer, initialize the previousScores and return same scores
	if !t.hasPreviousScores {
		t.previousScores = scores
		t.hasPreviousScores = true
		return scores, nil
	}

	// Apply EWMA to the scores using the previous scores as the T-1 scores
	smoothedScores := map[string]*Score{}
	for providerID, score := range scores {
		if !score.HasValue() {
			smoothedScores[providerID] = EmptyScore
			continue
		}

		previousScore, exists := t.previousScores[providerID]
		if !exists || !previousScore.HasValue() {
			smoothedScores[providerID] = EmptyScore
			continue
		}

		//The update formula is:
		// S_t = alpha * S_t + (1 - alpha) * S_t-1
		transformedScore := score.Value()*t.alpha + (1-t.alpha)*previousScore.Value()
		t.logger.Info("[WATCHER_SCORE] EWMA transformed score",
			zap.String("network", network),
			zap.String("provider", providerID),
			zap.Float64("alpha", t.alpha),
			zap.Float64("currentScore", score.Value()),
			zap.Float64("previousScore", previousScore.Value()),
			zap.Float64("newScore", transformedScore))
		smoothedScores[providerID], _ = NewScore(math.Round(transformedScore*10000)/10000, score.LastUpdated())
	}

	// Update the previous scores with the smoothed scores
	t.previousScores = smoothedScores

	return smoothedScores, nil
}

func NewEWMATransformer(alpha float64, logger *zap.Logger) *EWMATransformer {
	return &EWMATransformer{alpha: alpha, logger: logger}
}

// HighPassThroughTransformer is a transformer that returns the same scores it receives
// but only if the score has a value greater than a cutoff value. Otherwise, it returns a score with a value of 0.
type HighPassThroughTransformer struct {
	cutoffValue float64
	logger      *zap.Logger
}

func (t *HighPassThroughTransformer) TransformScore(network string, scores map[string]*Score) (map[string]*Score, error) {
	transformedScores := map[string]*Score{}
	for providerID, score := range scores {
		if score.HasValue() {
			if score.Value() > t.cutoffValue {
				transformedScores[providerID] = score
			} else {
				t.logger.Debug("[WATCHER_SCORE] Score below cutoff value",
					zap.String("network", network),
					zap.String("provider", providerID),
					zap.Float64("score", score.Value()),
					zap.Float64("cutoffValue", t.cutoffValue))
				transformedScores[providerID], _ = NewScore(0.0, score.LastUpdated())
			}
		} else {
			transformedScores[providerID] = EmptyScore
		}
	}
	return transformedScores, nil
}

func NewDefaultHighPassThroughTransformer(logger *zap.Logger) *HighPassThroughTransformer {
	return &HighPassThroughTransformer{cutoffValue: 0.001, logger: logger}
}
