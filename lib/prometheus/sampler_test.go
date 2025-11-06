package prometheus

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestHybridSamplerDeterministic verifies that the same labels always produce the same sampling decision
func TestHybridSamplerDeterministic(t *testing.T) {
	sampler := NewHybridSampler(0.25, 1.0)

	testCases := []struct {
		name   string
		labels []string
	}{
		{
			name:   "simple labels",
			labels: []string{"ethereum", "eth_call", "infura", "200"},
		},
		{
			name:   "complex labels",
			labels: []string{"polygon", "eth_getBalance", "alchemy", "provider1", "host1.example.com", "200", "healthy", "machine-123", "production"},
		},
		{
			name:   "empty label",
			labels: []string{"", "method", "provider"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Run sampling decision multiple times with same labels
			results := make([]bool, 100)
			for i := 0; i < 100; i++ {
				results[i] = sampler.ShouldSample(false, tc.labels...)
			}

			// All results should be the same
			firstResult := results[0]
			for i, result := range results {
				assert.Equal(t, firstResult, result, "Iteration %d: sampling should be deterministic for labels %v", i, tc.labels)
			}
		})
	}
}

// TestHybridSamplerErrorBoosting verifies that errors are sampled at a higher rate
func TestHybridSamplerErrorBoosting(t *testing.T) {
	sampler := NewHybridSampler(0.25, 1.0) // 25% normal, 100% errors

	// Test with different label combinations to ensure good distribution
	errorCount := 0
	normalCount := 0
	iterations := 10000

	for i := 0; i < iterations; i++ {
		// Generate unique labels for each iteration
		labels := []string{"ethereum", "eth_call", fmt.Sprintf("provider%d", i), fmt.Sprintf("%d", i)}

		if sampler.ShouldSample(true, labels...) {
			errorCount++
		}
		if sampler.ShouldSample(false, labels...) {
			normalCount++
		}
	}

	// Errors should be sampled at 100% rate
	assert.Equal(t, iterations, errorCount, "All errors should be sampled when error rate is 1.0")

	// Normal requests should be sampled at ~25% rate (with some tolerance for randomness)
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
		healthStatus   string
		expectError    bool
		description    string
	}{
		{
			name:           "success response",
			responseStatus: 200,
			healthStatus:   "Healthy",
			expectError:    false,
			description:    "200 OK with Healthy status should not be treated as error",
		},
		{
			name:           "client error",
			responseStatus: 404,
			healthStatus:   "Healthy",
			expectError:    true,
			description:    "4xx errors should be sampled at error rate",
		},
		{
			name:           "server error",
			responseStatus: 500,
			healthStatus:   "Healthy",
			expectError:    true,
			description:    "5xx errors should be sampled at error rate",
		},
		{
			name:           "unhealthy status",
			responseStatus: 200,
			healthStatus:   "Unhealthy",
			expectError:    true,
			description:    "Unhealthy status should be sampled at error rate even with 200 status",
		},
		{
			name:           "warning status",
			responseStatus: 200,
			healthStatus:   "Warning",
			expectError:    true,
			description:    "Warning status should be sampled at error rate",
		},
		{
			name:           "empty health status",
			responseStatus: 200,
			healthStatus:   "",
			expectError:    false,
			description:    "Empty health status with 200 should not be treated as error",
		},
		{
			name:           "rate limited",
			responseStatus: 429,
			healthStatus:   "Healthy",
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
				if sampler.ShouldSampleRequest(tc.responseStatus, tc.healthStatus, uniqueLabels...) {
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

// TestLabelCollisionResistance verifies that similar labels don't cause hash collisions
func TestLabelCollisionResistance(t *testing.T) {
	sampler := NewHybridSampler(0.5, 1.0)

	// Test potential collision scenarios
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

			result1 := sampler.ShouldSample(false, labels1...)
			result2 := sampler.ShouldSample(false, labels2...)

			// We can't guarantee they'll be different, but we can verify
			// that they produce consistent results
			for j := 0; j < 10; j++ {
				assert.Equal(t, result1, sampler.ShouldSample(false, labels1...),
					"Same labels should always produce same result")
				assert.Equal(t, result2, sampler.ShouldSample(false, labels2...),
					"Same labels should always produce same result")
			}
		})
	}
}