package reputationscore

import (
	"fmt"
	"time"
)

// A weighted combiner is a base implementation of combiner that uses weights to aggregate metrics.
type WeightedCombiner struct {
	Weights map[string]float64
}

func (c *WeightedCombiner) CombineMetrics(metrics []*ProviderMetric) (*Score, error) {
	weightedValue := 0.0
	for _, metric := range metrics {
		if weight, exists := c.Weights[metric.MetricID()]; exists {
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
	finalScore, err := NewScore(weightedValue, maxLastUpdated)

	return finalScore, err
}
