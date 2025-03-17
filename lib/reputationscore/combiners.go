package reputationscore

import (
	"fmt"
	"time"
)

// A weighted combiner is a base implementation of combiner that uses weights to aggregate metrics.
type WeightedCombiner struct {
	weights map[string]float64
}

func NewWeightedCombiner(weights map[string]float64) (*WeightedCombiner, error) {
	if len(weights) == 0 {
		return nil, fmt.Errorf("weights map cannot be empty")
	}

	totalWeight := 0.0
	for _, weight := range weights {
		totalWeight += weight
	}

	if totalWeight != 1.0 {
		return nil, fmt.Errorf("weights must sum to 1.0, got %f", totalWeight)
	}

	return &WeightedCombiner{weights: weights}, nil
}

func (c *WeightedCombiner) CombineMetrics(metrics []*ProviderMetric) (*Score, error) {
	if len(metrics) == 0 {
		return NewEmptyScore(), fmt.Errorf("no metrics to combine")
	}

	if len(c.weights) != len(metrics) {
		return NewEmptyScore(), fmt.Errorf("weights map must have the same length as the number of metrics")
	}

	weightedValue := 0.0
	for _, metric := range metrics {
		if weight, exists := c.weights[metric.MetricID()]; exists {
			weightedValue += metric.Value() * weight
		} else {
			return NewEmptyScore(), fmt.Errorf("no weight for metric %s", metric.MetricID())
		}
	}

	// Find the most recent metric timestamp
	maxLastUpdated := time.Time{}
	for _, metric := range metrics {
		if metric.LastUpdated().After(maxLastUpdated) {
			maxLastUpdated = metric.LastUpdated()
		}
	}

	return NewScore(weightedValue, maxLastUpdated)
}
