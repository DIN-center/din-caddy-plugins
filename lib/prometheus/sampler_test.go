package prometheus

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestWithinGroupSampling verifies that we sample approximately 25% of requests within each label group
func TestWithinGroupSampling(t *testing.T) {
	sampler := NewHybridSampler(0.25, 1.0)

	testCases := []struct {
		name       string
		labels     []string
		iterations int
	}{
		{
			name:       "simple labels",
			labels:     []string{"ethereum", "eth_call", "infura", "200"},
			iterations: 10000, // More iterations for better statistical accuracy
		},
		{
			name:       "complex labels",
			labels:     []string{"polygon", "eth_getBalance", "alchemy", "provider1", "host1.example.com", "200", "healthy", "machine-123", "production"},
			iterations: 10000,
		},
		{
			name:       "empty label",
			labels:     []string{"", "method", "provider"},
			iterations: 10000,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Sample many times with the same labels
			// Each call should get a different decision due to timestamp
			sampledCount := 0

			for i := 0; i < tc.iterations; i++ {
				if sampler.ShouldSample(false, tc.labels...) {
					sampledCount++
				}
				// No sleep needed - UnixNano() changes between calls
			}

			// Should sample approximately 25% of requests
			expectedRate := 0.25
			actualRate := float64(sampledCount) / float64(tc.iterations)

			// With 10000 iterations, we can use tighter tolerance
			// Standard deviation for binomial distribution: sqrt(n * p * (1-p))
			// For n=10000, p=0.25: stddev = sqrt(10000 * 0.25 * 0.75) = ~43.3
			// 3 standard deviations = 130, so tolerance = 130/10000 = 0.013 (1.3%)
			// We'll use 2% to be safe
			tolerance := 0.02

			assert.InDelta(t, expectedRate, actualRate, tolerance,
				"Expected ~25%% sampling for labels %v, got %.2f%% (sampled %d/%d)",
				tc.labels, actualRate*100, sampledCount, tc.iterations)
		})
	}
}

// TestHybridSamplerErrorBoosting verifies that errors are sampled at a higher rate
func TestHybridSamplerErrorBoosting(t *testing.T) {
	sampler := NewHybridSampler(0.25, 1.0) // 25% normal, 100% errors

	// Test with the same labels to verify error boosting works
	errorCount := 0
	normalCount := 0
	iterations := 1000
	labels := []string{"ethereum", "eth_call", "provider", "200"}

	for i := 0; i < iterations; i++ {
		if sampler.ShouldSample(true, labels...) {
			errorCount++
		}
		if sampler.ShouldSample(false, labels...) {
			normalCount++
		}
		// No sleep needed - UnixNano() changes between calls
	}

	// Errors should be sampled at 100% rate
	assert.Equal(t, iterations, errorCount, "All errors should be sampled when error rate is 1.0")

	// Normal requests should be sampled at ~25% rate
	expectedNormal := float64(iterations) * 0.25
	tolerance := float64(iterations) * 0.05 // 5% tolerance

	assert.InDelta(t, expectedNormal, normalCount, tolerance,
		"Normal requests should be sampled at approximately 25%% rate, got %d/%d (%.2f%%)",
		normalCount, iterations, float64(normalCount)/float64(iterations)*100)
}

// TestHybridSamplerRates verifies sampling with different rate configurations
func TestHybridSamplerRates(t *testing.T) {
	testCases := []struct {
		name          string
		baseRate      float64
		errorRate     float64
		expectedBase  float64
		expectedError float64
	}{
		{
			name:          "zero base rate",
			baseRate:      0.0,
			errorRate:     1.0,
			expectedBase:  0.0,
			expectedError: 1.0,
		},
		{
			name:          "100% both rates",
			baseRate:      1.0,
			errorRate:     1.0,
			expectedBase:  1.0,
			expectedError: 1.0,
		},
		{
			name:          "50% sampling",
			baseRate:      0.5,
			errorRate:     0.5,
			expectedBase:  0.5,
			expectedError: 0.5,
		},
		{
			name:          "10% base, 90% error",
			baseRate:      0.1,
			errorRate:     0.9,
			expectedBase:  0.1,
			expectedError: 0.9,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sampler := NewHybridSampler(tc.baseRate, tc.errorRate)

			normalCount := 0
			errorCount := 0
			iterations := 10000

			for i := 0; i < iterations; i++ {
				labels := []string{"test", fmt.Sprintf("%d", i)}

				if sampler.ShouldSample(false, labels...) {
					normalCount++
				}
				if sampler.ShouldSample(true, labels...) {
					errorCount++
				}
			}

			// Check if sampling rates are approximately correct
			actualBase := float64(normalCount) / float64(iterations)
			actualError := float64(errorCount) / float64(iterations)

			tolerance := 0.05 // 5% tolerance
			assert.InDelta(t, tc.expectedBase, actualBase, tolerance,
				"Base rate mismatch: expected %.2f, got %.2f", tc.expectedBase, actualBase)
			assert.InDelta(t, tc.expectedError, actualError, tolerance,
				"Error rate mismatch: expected %.2f, got %.2f", tc.expectedError, actualError)
		})
	}
}

// TestShouldSampleRequest verifies request-specific sampling logic
func TestShouldSampleRequest(t *testing.T) {
	sampler := NewHybridSampler(0.25, 1.0)

	testCases := []struct {
		name           string
		responseStatus int
		expectError    bool
		description    string
	}{
		{
			name:           "success response",
			responseStatus: 200,
			expectError:    false,
			description:    "200 OK should not be treated as error",
		},
		{
			name:           "client error",
			responseStatus: 404,
			expectError:    true,
			description:    "4xx errors should be sampled at error rate",
		},
		{
			name:           "server error",
			responseStatus: 500,
			expectError:    true,
			description:    "5xx errors should be sampled at error rate",
		},
		{
			name:           "rate limited",
			responseStatus: 429,
			expectError:    true,
			description:    "429 rate limit should be sampled at error rate",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test multiple times to ensure consistent behavior
			labels := []string{"test", tc.name, fmt.Sprintf("%d", tc.responseStatus)}

			sampledCount := 0
			iterations := 100

			for i := 0; i < iterations; i++ {
				uniqueLabels := append(labels, fmt.Sprintf("iter-%d", i))
				if sampler.ShouldSampleRequest(tc.responseStatus, uniqueLabels...) {
					sampledCount++
				}
			}

			if tc.expectError {
				// Error cases should be sampled at 100%
				assert.Equal(t, iterations, sampledCount, tc.description)
			} else {
				// Normal cases should be sampled at ~25%
				assert.LessOrEqual(t, sampledCount, 40, tc.description+" - should sample ~25%")
			}
		})
	}
}

// TestShouldSampleHealthCheck verifies health check-specific sampling logic
func TestShouldSampleHealthCheck(t *testing.T) {
	sampler := NewHybridSampler(0.5, 1.0) // 50% normal, 100% errors

	testCases := []struct {
		name           string
		responseStatus int
		expectError    bool
	}{
		{
			name:           "healthy check",
			responseStatus: 200,
			expectError:    false,
		},
		{
			name:           "client error check",
			responseStatus: 400,
			expectError:    true,
		},
		{
			name:           "server error check",
			responseStatus: 503,
			expectError:    true,
		},
		{
			name:           "redirect",
			responseStatus: 302,
			expectError:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			labels := []string{"network", "status", "machine", "env"}

			sampledCount := 0
			iterations := 100

			for i := 0; i < iterations; i++ {
				uniqueLabels := append(labels, fmt.Sprintf("iter-%d", i))
				if sampler.ShouldSampleHealthCheck(tc.responseStatus, uniqueLabels...) {
					sampledCount++
				}
			}

			if tc.expectError {
				// Error cases should be sampled at 100%
				assert.Equal(t, iterations, sampledCount, "Error health checks should always be sampled")
			} else {
				// Normal cases should be sampled at ~50%
				assert.InDelta(t, 50, sampledCount, 15, "Normal health checks should be sampled at ~50%")
			}
		})
	}
}

// TestLabelCollisionResistance verifies that the sampler handles different label combinations
// Note: Due to timestamp-based sampling, results are non-deterministic within label groups
func TestLabelCollisionResistance(t *testing.T) {
	sampler := NewHybridSampler(0.5, 1.0)

	// Test potential collision scenarios - verify different labels don't cause issues
	collisionTests := []struct {
		labels1 []string
		labels2 []string
	}{
		{
			labels1: []string{"ab", "cd"},
			labels2: []string{"a", "bcd"},
		},
		{
			labels1: []string{"foo", "bar"},
			labels2: []string{"foobar", ""},
		},
		{
			labels1: []string{"", "test", ""},
			labels2: []string{"test", "", ""},
		},
	}

	for i, test := range collisionTests {
		t.Run(fmt.Sprintf("collision_test_%d", i), func(t *testing.T) {
			// Add unique suffix to ensure different hash values
			labels1 := append(test.labels1, "unique1")
			labels2 := append(test.labels2, "unique2")

			// Sample multiple times to verify the sampler doesn't crash or panic
			// Results will vary due to timestamp-based sampling
			for j := 0; j < 10; j++ {
				sampler.ShouldSample(false, labels1...)
				sampler.ShouldSample(false, labels2...)
			}
			// If we got here without panicking, the test passes
		})
	}
}

// TestEdgeCaseSamplingRates verifies edge cases for sampling rates
func TestEdgeCaseSamplingRates(t *testing.T) {
	testCases := []struct {
		name          string
		baseRate      float64
		errorRate     float64
		expectAlways  bool // true if should always sample
		expectNever   bool // true if should never sample
	}{
		{
			name:         "negative base rate (clamped to 0.0)",
			baseRate:     -0.1,
			errorRate:    1.0,
			expectNever:  true, // negative rate is clamped to 0.0, should never sample
			expectAlways: false,
		},
		{
			name:         "rate above 1.0 (clamped to 1.0)",
			baseRate:     1.5,
			errorRate:    2.0,
			expectAlways: true, // rates > 1.0 are clamped to 1.0, should always sample
			expectNever:  false,
		},
		{
			name:         "exactly 0 rate",
			baseRate:     0.0,
			errorRate:    0.0,
			expectNever:  true,
			expectAlways: false,
		},
		{
			name:         "exactly 1.0 rate",
			baseRate:     1.0,
			errorRate:    1.0,
			expectAlways: true,
			expectNever:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sampler := NewHybridSampler(tc.baseRate, tc.errorRate)
			labels := []string{"test", "labels"}

			// Test normal sampling
			normalCount := 0
			iterations := 100
			for i := 0; i < iterations; i++ {
				uniqueLabels := append(labels, fmt.Sprintf("iter-%d", i))
				if sampler.ShouldSample(false, uniqueLabels...) {
					normalCount++
				}
			}

			if tc.expectAlways {
				assert.Equal(t, iterations, normalCount, "Should always sample when rate >= 1.0")
			}
			if tc.expectNever {
				assert.Equal(t, 0, normalCount, "Should never sample when rate <= 0")
			}

			// Test error sampling
			errorCount := 0
			for i := 0; i < iterations; i++ {
				uniqueLabels := append(labels, fmt.Sprintf("iter-%d", i))
				if sampler.ShouldSample(true, uniqueLabels...) {
					errorCount++
				}
			}

			// Since we clamp rates, check against clamped values
			clampedErrorRate := tc.errorRate
			if clampedErrorRate < 0.0 {
				clampedErrorRate = 0.0
			} else if clampedErrorRate > 1.0 {
				clampedErrorRate = 1.0
			}

			if clampedErrorRate >= 1.0 {
				assert.Equal(t, iterations, errorCount, "Error sampling should always occur when clamped error rate = 1.0")
			}
			if clampedErrorRate <= 0 {
				assert.Equal(t, 0, errorCount, "Error sampling should never occur when clamped error rate = 0")
			}
		})
	}
}

// TestRateBoundaryClamping verifies that rates are properly clamped to [0.0, 1.0]
func TestRateBoundaryClamping(t *testing.T) {
	testCases := []struct {
		name             string
		inputBaseRate    float64
		inputErrorRate   float64
		expectedBaseBehavior  string // "always", "never", or "probabilistic"
		expectedErrorBehavior string // "always", "never", or "probabilistic"
	}{
		{
			name:             "negative rates clamped to 0",
			inputBaseRate:    -0.5,
			inputErrorRate:   -1.0,
			expectedBaseBehavior:  "never",
			expectedErrorBehavior: "never",
		},
		{
			name:             "rates > 1.0 clamped to 1.0",
			inputBaseRate:    1.5,
			inputErrorRate:   10.0,
			expectedBaseBehavior:  "always",
			expectedErrorBehavior: "always",
		},
		{
			name:             "valid rates unchanged",
			inputBaseRate:    0.25,
			inputErrorRate:   0.75,
			expectedBaseBehavior:  "probabilistic",
			expectedErrorBehavior: "probabilistic",
		},
		{
			name:             "boundary values unchanged",
			inputBaseRate:    0.0,
			inputErrorRate:   1.0,
			expectedBaseBehavior:  "never",
			expectedErrorBehavior: "always",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sampler := NewHybridSampler(tc.inputBaseRate, tc.inputErrorRate)
			iterations := 100

			// Test base rate behavior
			baseCount := 0
			for i := 0; i < iterations; i++ {
				labels := []string{"test", fmt.Sprintf("iter-%d", i)}
				if sampler.ShouldSample(false, labels...) {
					baseCount++
				}
			}

			switch tc.expectedBaseBehavior {
			case "always":
				assert.Equal(t, iterations, baseCount, "Base rate clamped to 1.0 should always sample")
			case "never":
				assert.Equal(t, 0, baseCount, "Base rate clamped to 0.0 should never sample")
			case "probabilistic":
				assert.Greater(t, baseCount, 0, "Probabilistic base rate should sample some requests")
				assert.Less(t, baseCount, iterations, "Probabilistic base rate should not sample all requests")
			}

			// Test error rate behavior
			errorCount := 0
			for i := 0; i < iterations; i++ {
				labels := []string{"test", fmt.Sprintf("iter-%d", i)}
				if sampler.ShouldSample(true, labels...) {
					errorCount++
				}
			}

			switch tc.expectedErrorBehavior {
			case "always":
				assert.Equal(t, iterations, errorCount, "Error rate clamped to 1.0 should always sample")
			case "never":
				assert.Equal(t, 0, errorCount, "Error rate clamped to 0.0 should never sample")
			case "probabilistic":
				assert.Greater(t, errorCount, 0, "Probabilistic error rate should sample some requests")
				assert.Less(t, errorCount, iterations, "Probabilistic error rate should not sample all requests")
			}
		})
	}
}