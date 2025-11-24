package watcherscore

import (
	"testing"
	"time"

	"go.uber.org/zap/zaptest"
)

var TIME1 = time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

func TestEWMATransformer(t *testing.T) {
	t.Run("first call returns unchanged scores", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		transformer := NewEWMATransformer(0.7, logger)
		scores := map[string]*Score{
			"provider1": MustCreateScore(0.8, TIME1),
			"provider2": MustCreateScore(0.2, TIME1),
		}

		transformedScores, err := transformer.TransformScore("network", scores)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if transformedScores["provider1"].Value() != 0.8 {
			t.Errorf("expected provider1 score to be 0.8, got %v", transformedScores["provider1"].Value())
		}
		if transformedScores["provider2"].Value() != 0.2 {
			t.Errorf("expected provider2 score to be 0.2, got %v", transformedScores["provider2"].Value())
		}
	})

	t.Run("applies EWMA formula correctly", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		transformer := NewEWMATransformer(0.7, logger)
		scoresTMinusOne := map[string]*Score{
			"provider1": MustCreateScore(1.0, TIME1),
			"provider2": MustCreateScore(0.0, TIME1),
		}
		scoresT := map[string]*Score{
			"provider1": MustCreateScore(0.9, TIME1),
			"provider2": MustCreateScore(0.1, TIME1),
		}

		// First call initializes previous scores
		_, _ = transformer.TransformScore("network", scoresTMinusOne)

		// Second call should apply EWMA
		transformedScores, err := transformer.TransformScore("network", scoresT)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Expected: 0.9 * 0.7 + 1.0 * 0.3 = 0.93
		expectedScore := 0.93
		if transformedScores["provider1"].Value() != expectedScore {
			t.Errorf("expected score to be %v, got %v", expectedScore, transformedScores["provider1"].Value())
		}
	})

	t.Run("previous scores valid, current scores empty, then result is empty", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		transformer := NewEWMATransformer(0.7, logger)
		scores1 := map[string]*Score{
			"provider1": MustCreateScore(1.0, TIME1),
		}
		scores2 := map[string]*Score{
			"provider1": NewEmptyScore(),
		}

		// First call initializes previous scores
		_, _ = transformer.TransformScore("network", scores1)

		// Second call : current score is empty, previous score is valid, so the result is empty
		transformedScores, err := transformer.TransformScore("network", scores2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if transformedScores["provider1"].HasValue() {
			t.Error("expected empty score for provider1")
		}

	})

	t.Run("previous scores empty, current scores valid, then result is valid", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		transformer := NewEWMATransformer(0.7, logger)
		scores1 := map[string]*Score{
			"provider1": NewEmptyScore(),
		}
		scores2 := map[string]*Score{
			"provider1": MustCreateScore(1.0, TIME1),
		}

		// First call initializes previous scores
		_, _ = transformer.TransformScore("network", scores1)

		// Second call : current score is valid, previous score is empty, so the result is valid
		transformedScores, err := transformer.TransformScore("network", scores2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !transformedScores["provider1"].HasValue() {
			t.Error("expected valid score for provider1")
		}

	})

	t.Run("handles new providers", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		transformer := NewEWMATransformer(0.7, logger)
		scores1 := map[string]*Score{
			"provider1": MustCreateScore(1.0, TIME1),
		}
		scores2 := map[string]*Score{
			"provider1": MustCreateScore(0.85, TIME1),
			"provider2": MustCreateScore(0.15, TIME1), // New provider was added
		}

		transformer.TransformScore("network", scores1)
		transformedScores, err := transformer.TransformScore("network", scores2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// For provider1: 0.85 * 0.7 + 1.0 * 0.3 = 0.895
		expectedScore1 := 0.895
		if transformedScores["provider1"].Value() != expectedScore1 {
			t.Errorf("expected provider1 score to be %v, got %v", expectedScore1, transformedScores["provider1"].Value())
		}

		// For provider2 (new): just appeared, so the result is the current score
		expectedScore2 := 0.15
		if transformedScores["provider2"].Value() != expectedScore2 {
			t.Errorf("expected provider2 score to be %v, got %v", expectedScore2, transformedScores["provider2"].Value())
		}

	})
	t.Run("handles three consecutive transformations", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		transformer := NewEWMATransformer(0.7, logger)
		scores1 := map[string]*Score{
			"provider1": MustCreateScore(1.0, TIME1),
			"provider2": MustCreateScore(0.0, TIME1),
		}
		scores2 := map[string]*Score{
			"provider1": MustCreateScore(0.0, TIME1),
			"provider2": MustCreateScore(1.0, TIME1),
		}
		scores3 := map[string]*Score{
			"provider1": MustCreateScore(0.5, TIME1),
			"provider2": MustCreateScore(0.5, TIME1),
		}

		transformer.TransformScore("network", scores1)
		transformer.TransformScore("network", scores2)
		transformedScores, err := transformer.TransformScore("network", scores3)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// For provider1:
		// First transform:  1.0 (initial)
		// Second transform: 0.0 * 0.7 + 1.0 * 0.3 = 0.3
		// Third transform:  0.5 * 0.7 + 0.3 * 0.3 = 0.44
		expectedScore1 := 0.44
		if transformedScores["provider1"].Value() != expectedScore1 {
			t.Errorf("expected provider1 score to be %v, got %v", expectedScore1, transformedScores["provider1"].Value())
		}

		// For provider2:
		// First transform:  0.0 (initial)
		// Second transform: 1.0 * 0.7 + 0.0 * 0.3 = 0.7
		// Third transform:  0.5 * 0.7 + 0.7 * 0.3 = 0.56
		expectedScore2 := 0.56
		if transformedScores["provider2"].Value() != expectedScore2 {
			t.Errorf("expected provider2 score to be %v, got %v", expectedScore2, transformedScores["provider2"].Value())
		}
	})
}

func TestHighPassThroughTransformer(t *testing.T) {
	t.Run("passes through scores above cutoff value", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		transformer := NewDefaultHighPassThroughTransformer(logger)
		scores := map[string]*Score{
			"provider1": MustCreateScore(0.5, TIME1),
			"provider2": MustCreateScore(0.002, TIME1),
		}

		transformedScores, err := transformer.TransformScore("network", scores)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if transformedScores["provider1"].Value() != 0.5 {
			t.Errorf("expected provider1 score to be 0.5, got %v", transformedScores["provider1"].Value())
		}
		if transformedScores["provider2"].Value() != 0.002 {
			t.Errorf("expected provider2 score to be 0.002, got %v", transformedScores["provider2"].Value())
		}
	})

	t.Run("zeros out scores below cutoff value", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		transformer := NewDefaultHighPassThroughTransformer(logger)
		scores := map[string]*Score{
			"provider1": MustCreateScore(0.0005, TIME1),
			"provider2": MustCreateScore(0.0009, TIME1),
		}

		transformedScores, err := transformer.TransformScore("network", scores)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if transformedScores["provider1"].Value() != 0.0 {
			t.Errorf("expected provider1 score to be 0.0, got %v", transformedScores["provider1"].Value())
		}
		if transformedScores["provider2"].Value() != 0.0 {
			t.Errorf("expected provider2 score to be 0.0, got %v", transformedScores["provider2"].Value())
		}
	})

	t.Run("handles empty scores", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		transformer := NewDefaultHighPassThroughTransformer(logger)
		scores := map[string]*Score{
			"provider1": NewEmptyScore(),
			"provider2": MustCreateScore(0.5, TIME1),
		}

		transformedScores, err := transformer.TransformScore("network", scores)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if transformedScores["provider1"].HasValue() {
			t.Error("expected provider1 score to have no value")
		}
		if transformedScores["provider2"].Value() != 0.5 {
			t.Errorf("expected provider2 score to be 0.5, got %v", transformedScores["provider2"].Value())
		}
	})

	t.Run("handles empty scores map", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		transformer := NewDefaultHighPassThroughTransformer(logger)
		scores := map[string]*Score{}

		transformedScores, err := transformer.TransformScore("network", scores)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(transformedScores) != 0 {
			t.Errorf("expected empty map, got map with %d elements", len(transformedScores))
		}
	})
}
