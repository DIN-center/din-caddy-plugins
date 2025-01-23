package reputationscore

import (
	"fmt"
	"math"
)

// NormalizeTransform adjust the input score so the sum of all scores is 1.
// In other words, it adjusted score reprensents the percentage of the total score.
type NormalizeTransformer struct {
}

func (t *NormalizeTransformer) TransformScore(scores map[string]*Score) (map[string]*Score, error) {
	if len(scores) == 0 {
		return scores, nil
	}

	sumOfScores := 0.0
	for _, score := range scores {
		if score.HasValue() {
			sumOfScores += score.Value()
		}
	}

	normalizedScores := map[string]*Score{}
	for providerID, score := range scores {
		if !score.HasValue() {
			normalizedScores[providerID] = EmptyScore
			continue
		}

		var normalizedScore *Score
		if sumOfScores < 0.0001 { // prevent division by 0 and very small raw scores to be considered a score
			normalizedScore = EmptyScore
		} else {
			normalizedScore, _ = NewScore(
				math.Round(score.Value()/sumOfScores*10000)/10000,
				score.LastUpdated(),
			)
		}
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

func NewCompositeTransformer(chain []ScoreTransformer) *CompositeTransformer {
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

	//ensure sum of scores is 1
	sumOfScores := 0.0
	for _, score := range scores {
		if score.HasValue() {
			sumOfScores += score.Value()
		}
	}
	if sumOfScores != 1.0 {
		return nil, fmt.Errorf("sum of scores given to EWMA transformer should be 1.0, got %f", sumOfScores)
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
