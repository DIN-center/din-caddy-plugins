package reputationscore

import (
	"testing"
	"time"

	"github.com/DIN-center/din-sc/apps/din-go/lib/watcher"
	"go.uber.org/zap/zaptest"
)

func TestCalculateCheckMetric(t *testing.T) {
	t.Run("returns 0 when success percentage is below threshold", func(t *testing.T) {
		result := calculateCheckMetric(watcher.Status{SuccessPercentage: 89.0}, watcher.Summary{PassPercentage: 80.0})
		if result != 0 {
			t.Errorf("expected 0 for low success percentage, got %f", result)
		}
	})

	t.Run("use cases when pass percentage when success percentage is 100%", func(t *testing.T) {
		testCases := []struct {
			name           string
			passPercentage float64
			expected       float64
		}{
			{
				name:           "100% pass percentage",
				passPercentage: 100.0,
				expected:       1.0,
			},
			{
				name:           "80% pass percentage",
				passPercentage: 80.0,
				expected:       0.8,
			},
			{
				name:           "0% pass percentage",
				passPercentage: 0.0,
				expected:       0.0,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := calculateCheckMetric(watcher.Status{SuccessPercentage: 100.0}, watcher.Summary{PassPercentage: tc.passPercentage})
				if result != tc.expected {
					t.Errorf("expected %f for pass percentage %f, got %f",
						tc.expected, tc.passPercentage, result)
				}
			})
		}
	})
}

func TestCalculateLatencyMetric(t *testing.T) {
	t.Run("returns 0 when success percentage is below threshold", func(t *testing.T) {
		result := calculateLatencyMetric(watcher.Status{SuccessPercentage: 89.0}, watcher.LatencyStats{P95: 500.0})
		if result != 0 {
			t.Errorf("expected 0 for low success percentage, got %f", result)
		}
	})

	t.Run("use cases when success percentage is 100%", func(t *testing.T) {
		testCases := []struct {
			name     string
			p95      float64
			expected float64
		}{
			{
				name:     "5ms latency",
				p95:      5.0,
				expected: 1.0,
			},
			{
				name:     "50ms latency",
				p95:      50.0,
				expected: 1.0,
			},
			{
				name:     "55ms latency",
				p95:      55.0,
				expected: 0.9969,
			},
			{
				name:     "500ms latency",
				p95:      500.0,
				expected: 0.6474,
			},
			{
				name:     "950ms latency",
				p95:      950.0,
				expected: 0.081100,
			},
			{
				name:     "1000ms latency",
				p95:      1000.0,
				expected: 0.0,
			},
			{
				name:     "2000ms latency",
				p95:      2000.0,
				expected: 0.0,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := calculateLatencyMetric(watcher.Status{SuccessPercentage: 100.0}, watcher.LatencyStats{P95: tc.p95})
				if result != tc.expected {
					t.Errorf("expected %f for P95 latency %f, got %f",
						tc.expected, tc.p95, result)
				}
			})
		}
	})
}

func TestBuildMetricsForCheckQuery(t *testing.T) {
	t.Run("handles multiple providers with different pass percentages", func(t *testing.T) {
		mockClient := &MockWatcherAPIClient{
			mockCheckResponses: []watcher.Result[watcher.CheckResponse]{OK_CHECK_RESPONSE_TWO_PROVIDERS_GOOD_SCORES_TIME1},
		}

		// Call the function
		metrics, err := buildMetricsForCheckQuery(mockClient,
			watcher.CheckQueryParams{Network: "network"},
			"test_metric",
			zaptest.NewLogger(t),
		)

		// Verify no error occurred
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify we got metrics for both providers
		if len(metrics) != 2 {
			t.Fatalf("expected 2 metrics, got %d", len(metrics))
		}

		expectedMetric1, _ := NewProviderMetric(
			"test_metric",
			"network",
			"provider1",
			"https://provider1.com",
			0.95,
			time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		)
		if !metrics[0].Equal(expectedMetric1) {
			t.Errorf("expected provider1 %v, got %v", expectedMetric1, metrics[0])
		}

		expectedMetric2, _ := NewProviderMetric(
			"test_metric",
			"network",
			"provider2",
			"https://provider2.com",
			0.20,
			time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		)
		if !metrics[1].Equal(expectedMetric2) {
			t.Errorf("expected provider2 %v, got %v", expectedMetric2, metrics[1])
		}
	})
	t.Run("returns error when API call fails", func(t *testing.T) {
		// Create mock client that returns an error
		mockClient := &MockWatcherAPIClient{
			mockCheckResponses: []watcher.Result[watcher.CheckResponse]{KO_CHECK_RESPONSE_API_ERROR},
		}

		// Call the function
		_, err := buildMetricsForCheckQuery(mockClient, watcher.CheckQueryParams{}, "test_metric_check", zaptest.NewLogger(t))

		// Verify error was returned
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("returns error on invalid timestamp", func(t *testing.T) {
		// Create mock client that returns response with invalid timestamp
		mockClient := &MockWatcherAPIClient{
			mockCheckResponses: []watcher.Result[watcher.CheckResponse]{OK_CHECK_RESPONSE_WRONG_TIMESTAMP},
		}

		// Call the function
		_, err := buildMetricsForCheckQuery(mockClient, watcher.CheckQueryParams{}, "test_metric_check", zaptest.NewLogger(t))

		// Verify error was returned
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestBuildMetricsForLatencyQuery(t *testing.T) {
	t.Run("successfully builds metrics from latency response", func(t *testing.T) {
		// Create mock client that returns a successful response
		mockClient := &MockWatcherAPIClient{
			mockLatencyResponses: []watcher.Result[watcher.LatencyResponse]{OK_LATENCY_RESPONSE_TWO_PROVIDERS_GOOD_SCORE_TIME1},
		}

		// Call the function
		metrics, err := buildMetricsForLatencyQuery(mockClient,
			watcher.LatencyQueryParams{Network: "network"},
			"test_metric_latency",
			zaptest.NewLogger(t),
		)

		// Verify no error occurred
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify we got metrics for both providers
		if len(metrics) != 2 {
			t.Fatalf("expected 2 metrics, got %d", len(metrics))
		}

		expectedMetric1, _ := NewProviderMetric(
			"test_metric_latency",
			"network",
			"provider1",
			"https://provider1.com",
			0.968500,
			time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		)
		if !metrics[0].Equal(expectedMetric1) {
			t.Errorf("expected provider1 %v, got %v", expectedMetric1, metrics[0])
		}

		expectedMetric2, _ := NewProviderMetric(
			"test_metric_latency",
			"network",
			"provider2",
			"https://provider2.com",
			0.647400,
			time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		)
		if !metrics[1].Equal(expectedMetric2) {
			t.Errorf("expected provider2 %v, got %v", expectedMetric2, metrics[1])
		}
	})

	t.Run("returns error when API call fails", func(t *testing.T) {
		// Create mock client that returns an error
		mockClient := &MockWatcherAPIClient{
			mockLatencyResponses: []watcher.Result[watcher.LatencyResponse]{KO_LATENCY_RESPONSE_API_ERROR},
		}

		// Call the function
		_, err := buildMetricsForLatencyQuery(mockClient, watcher.LatencyQueryParams{}, "test_metric_latency", zaptest.NewLogger(t))

		// Verify error was returned
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("returns error on invalid timestamp", func(t *testing.T) {
		// Create mock client that returns response with invalid timestamp
		mockClient := &MockWatcherAPIClient{
			mockLatencyResponses: []watcher.Result[watcher.LatencyResponse]{OK_LATENCY_RESPONSE_WRONG_TIMESTAMP},
		}

		// Call the function
		_, err := buildMetricsForLatencyQuery(mockClient, watcher.LatencyQueryParams{}, "test_metric_latency", zaptest.NewLogger(t))

		// Verify error was returned
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
