package reputationscore

import (
	"testing"
	"time"
)

func TestWeightedCombiner(t *testing.T) {
	now := time.Now()

	t.Run("combines metrics using all weights", func(t *testing.T) {
		combiner := WeightedCombiner{
			Weights: map[string]float64{
				"metric1": 0.3,
				"metric2": 0.7,
			},
		}

		time1 := now
		time2 := time1.Add(time.Hour)
		metrics := []*ProviderMetric{
			{
				metricID:    "metric1",
				providerID:  "provider1",
				value:       0.9,
				lastUpdated: time1,
			},
			{
				metricID:    "metric2",
				providerID:  "provider1",
				value:       0.1,
				lastUpdated: time2,
			},
		}

		score, err := combiner.CombineMetrics(metrics)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expectedValue := 0.9*0.3 + 0.1*0.7
		if score.Value() != expectedValue {
			t.Errorf("expected score value %f, got %f", expectedValue, score.Value())
		}

		if !score.HasValue() {
			t.Error("expected HasValue to be true")
		}

		if !score.LastUpdated().Equal(time2) {
			t.Errorf("expected LastUpdated to be %v, got %v", time2, score.LastUpdated())
		}
	})

	t.Run("returns error when weight missing for metric", func(t *testing.T) {
		combiner := WeightedCombiner{
			Weights: map[string]float64{
				"metric1": 0.3,
				// metric2 weight missing!
			},
		}

		metrics := []*ProviderMetric{
			{
				metricID:    "metric1",
				providerID:  "provider1",
				value:       0.9,
				lastUpdated: now,
			},
			{
				metricID:    "metric2",
				providerID:  "provider1",
				value:       0.1,
				lastUpdated: now,
			},
		}

		_, err := combiner.CombineMetrics(metrics)
		if err == nil {
			t.Fatal("expected error when weight missing, got nil")
		}

		expectedErr := "no weight for metric metric2"
		if err.Error() != expectedErr {
			t.Errorf("expected error message %q, got %q", expectedErr, err.Error())
		}
	})
}
