package watcherscore

import (
	"testing"
	"time"
)

func TestNormalizedExponentialFunction(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		k        float64
		expected float64
	}{
		{
			name:     "negative input, k < 0",
			input:    -2.0,
			k:        -1,
			expected: -10.1073,
		},
		{
			name:     "negative input, k = 0",
			input:    -2.0,
			k:        0,
			expected: -2,
		},
		{
			name:     "negative input, k > 0",
			input:    -2.0,
			k:        1,
			expected: -0.5032,
		},
		{
			name:     "positive input, k < 0",
			input:    2.0,
			k:        -1,
			expected: 0.5032,
		},
		{
			name:     "positive input, k = 0",
			input:    2.0,
			k:        0,
			expected: 2,
		},
		{
			name:     "positive input, k > 0",
			input:    2.0,
			k:        1,
			expected: 0.5032,
		},
		{
			name:     "all zeros",
			input:    0.0,
			k:        0,
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizedExponentialFunction(tt.input, tt.k)
			if Float64AlmostEqual(result, tt.expected) {
				t.Errorf("NormalizedExponentialFunction() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExponentialPullToHalf(t *testing.T) {

	tests := []struct {
		name          string
		input         float64
		elapsed       time.Duration
		tau           time.Duration
		midpoint      float64
		expected      float64
		errorExpected bool
	}{
		{
			name:          "init = 0.5, elapsed = 15 minutes, tau = 30 minutes",
			input:         0.5,
			elapsed:       15 * time.Minute,
			tau:           30 * time.Minute,
			midpoint:      0.5,
			expected:      0.5,
			errorExpected: false,
		},
		{
			name:          "elapsed < 0 (should error)",
			input:         0.5,
			elapsed:       -10 * time.Minute,
			tau:           30 * time.Minute,
			midpoint:      0.5,
			errorExpected: true,
		},
		{
			name:          "tau < 0 (should error)",
			input:         0.5,
			elapsed:       15 * time.Minute,
			tau:           -1 * time.Minute,
			midpoint:      0.5,
			errorExpected: true,
		},
		{
			name:          "input < 0 (should error)",
			input:         -0.1,
			elapsed:       10 * time.Minute,
			tau:           30 * time.Minute,
			midpoint:      0.5,
			errorExpected: true,
		},
		{
			name:          "input > 1 (should error)",
			input:         1.2,
			elapsed:       10 * time.Minute,
			tau:           30 * time.Minute,
			midpoint:      0.5,
			errorExpected: true,
		},
		{
			name:          "init = 0, elapsed = 0, tau = 30 minutes",
			input:         0.0,
			elapsed:       0,
			tau:           30 * time.Minute,
			midpoint:      0.5,
			expected:      0.0,
			errorExpected: false,
		},
		{
			name:          "init = 1, elapsed = 0, tau = 30 minutes",
			input:         1.0,
			elapsed:       0,
			tau:           30 * time.Minute,
			midpoint:      0.5,
			expected:      1.0,
			errorExpected: false,
		},
		{
			name:          "init = 1, elapsed = 30 min, tau = 30 min (capped at midpoint)",
			input:         1.0,
			elapsed:       30 * time.Minute,
			tau:           30 * time.Minute,
			midpoint:      0.5,
			expected:      0.5,
			errorExpected: false,
		},
		{
			name:          "init = 0, elapsed = 30 min, tau = 30 min (capped at midpoint)",
			input:         0.0,
			elapsed:       30 * time.Minute,
			tau:           30 * time.Minute,
			midpoint:      0.5,
			expected:      0.5,
			errorExpected: false,
		},
		{
			name:          "init = 0, elapsed = 30 min, tau = 30 min, midpoint = 0.75",
			input:         0.0,
			elapsed:       30 * time.Minute,
			tau:           30 * time.Minute,
			midpoint:      0.75,
			expected:      0.75,
			errorExpected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExponentialPullToMidpoint(tt.input, tt.elapsed, tt.tau, tt.midpoint)
			if (err != nil) != tt.errorExpected {
				t.Errorf("ExponentialPullToMidpoint() error = %v, errorExpected=%v", err, tt.errorExpected)
			}
			if !tt.errorExpected && !Float64AlmostEqual(result, tt.expected) {
				t.Errorf("ExponentialPullToMidpoint() = %v, want %v", result, tt.expected)
			}
		})
	}
}
