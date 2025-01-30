package reputationscore

import (
	"math"
)

// ShareOfTotalTransformer adjust the input score so the sum of all scores is 1.
// In other words, adjusted score represents the percentage of the total score.
type ShareOfTotalTransformer struct {
}

func (t *ShareOfTotalTransformer) TransformScore(scores map[string]*Score) (map[string]*Score, error) {
	if len(scores) == 0 {
		return scores, nil
	}

	sumOfScores := 0.0
	for _, score := range scores {
		if score.HasValue() {
			sumOfScores += score.Value()
		}
	}

	if sumOfScores < 0.001 { // prevent division by 0 and very small raw scores
		return scores, nil
	}

	normalizedScores := map[string]*Score{}
	for providerID, score := range scores {
		if !score.HasValue() {
			normalizedScores[providerID] = EmptyScore
			continue
		}

		normalizedScore, _ := NewScore(
			math.Round(score.Value()/sumOfScores*10000)/10000,
			score.LastUpdated(),
		)
		normalizedScores[providerID] = normalizedScore
	}

	return normalizedScores, nil
}

// CompositeTransformer is a transformer that chains multiple transformers.
// It applies the result of each transformer in the chain to the next one in the chain and returns the final result.
type CompositeTransformer struct {
	chain []ScoreTransformer
}

func (t *CompositeTransformer) TransformScore(scores map[string]*Score) (map[string]*Score, error) {
	var transformedScores = scores
	var err error
	for _, transformer := range t.chain {
		transformedScores, err = transformer.TransformScore(transformedScores)
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
}

func (t *EWMATransformer) TransformScore(scores map[string]*Score) (map[string]*Score, error) {
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
		smoothedScores[providerID], _ = NewScore(math.Round(transformedScore*10000)/10000, score.LastUpdated())
	}

	// Update the previous scores with the smoothed scores
	t.previousScores = smoothedScores

	return smoothedScores, nil
}

func NewEWMATransformer(alpha float64) *EWMATransformer {
	return &EWMATransformer{alpha: alpha}
}

// HighPassThroughTransformer is a transformer that returns the same scores it receives
// but only if the score has a value greater than a cutoff value. Otherwise, it returns a score with a value of 0.
type HighPassThroughTransformer struct {
	cutoffValue float64
}

func (t *HighPassThroughTransformer) TransformScore(scores map[string]*Score) (map[string]*Score, error) {
	transformedScores := map[string]*Score{}
	for providerID, score := range scores {
		if score.HasValue() {
			if score.Value() > t.cutoffValue {
				transformedScores[providerID] = score
			} else {
				transformedScores[providerID], _ = NewScore(0.0, score.LastUpdated())
			}
		} else {
			transformedScores[providerID] = EmptyScore
		}
	}
	return transformedScores, nil
}

func NewDefaultHighPassThroughTransformer() *HighPassThroughTransformer {
	return &HighPassThroughTransformer{cutoffValue: 0.001}
}
