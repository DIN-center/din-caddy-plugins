package reputationscore

import (
	"testing"
	"time"
)

func TestWeightedCombiner(t *testing.T) {
	now := time.Now()

	t.Run("combines metrics using all weights", func(t *testing.T) {
		combiner := WeightedCombiner{
			weights: map[string]float64{
				"metric1": 0.3,
				"metric2": 0.7,
			},
		}

		time1 := now
		time2 := time1.Add(time.Hour)
		metrics := []*ProviderMetric{
			{
				metricID:     "metric1",
				providerName: "provider1",
				value:        0.9,
				lastUpdated:  time1,
			},
			{
				metricID:     "metric2",
				providerName: "provider1",
				value:        0.1,
				lastUpdated:  time2,
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
			weights: map[string]float64{
				"metric1": 0.3,
				// metric2 weight missing!
			},
		}

		metrics := []*ProviderMetric{
			{
				metricID:     "metric1",
				providerName: "provider1",
				value:        0.9,
				lastUpdated:  now,
			},
			{
				metricID:     "metric2",
				providerName: "provider1",
				value:        0.1,
				lastUpdated:  now,
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

	t.Run("returns error when weights sum is not 1", func(t *testing.T) {
		_, err := NewWeightedCombiner(map[string]float64{
			"metric1": 0.3,
			"metric2": 0.71,
		})

		if err == nil {
			t.Fatal("expected error when weights sum is not 1, got nil")
		}
	})

	t.Run("returns error when weights map is empty", func(t *testing.T) {
		_, err := NewWeightedCombiner(map[string]float64{})
		if err == nil {
			t.Fatal("expected error when weights map is empty, got nil")
		}

	})

	t.Run("returns error when weights map has different length than metrics", func(t *testing.T) {
		metrics := []*ProviderMetric{
			{
				metricID:     "metric1",
				providerName: "provider1",
				value:        0.9,
				lastUpdated:  now,
			},
		}

		combiner, _ := NewWeightedCombiner(map[string]float64{
			"metric1": 0.3,
			"metric2": 0.7,
		})

		_, err := combiner.CombineMetrics(metrics)

		if err == nil {
			t.Fatal("expected error when weights map has different length than metrics, got nil")
		}

	})
}
