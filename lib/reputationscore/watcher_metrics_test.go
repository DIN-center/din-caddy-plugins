package reputationscore

import (
	"testing"
	"time"

	"github.com/DIN-center/din-sc/apps/din-go/lib/watcher"
	"go.uber.org/zap/zaptest"
)

func TestWatcherBlockNumberConsistencyGenerateMetrics(t *testing.T) {
	t.Run("successfully builds single metric for block number consistency", func(t *testing.T) {
		mockClient := &MockWatcherAPIClient{
			mockCheckResponses: []watcher.Result[watcher.CheckResponse]{OK_CHECK_RESPONSE_ONE_PROVIDER_GOOD_SCORE},
		}

		metrics, err := (&WatcherBlockNumberConsistency{WatcherClient: mockClient, Logger: zaptest.NewLogger(t)}).GenerateMetrics("test_network")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(metrics) != 1 {
			t.Fatalf("expected 1 metric, got %d", len(metrics))
		}
		expectedMetric, _ := NewProviderMetric(
			BlockNumberConsistencyMetricID,
			"provider1",
			"https://provider1.com",
			0.95,
			time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		)
		if !metrics[0].Equal(expectedMetric) {
			t.Errorf("expected provider1 %v, got %v", expectedMetric, metrics[0])
		}

	})

	t.Run("catch error when API returns error", func(t *testing.T) {
		mockClient := &MockWatcherAPIClient{
			mockCheckResponses: []watcher.Result[watcher.CheckResponse]{KO_CHECK_RESPONSE_API_ERROR},
		}

		_, err := (&WatcherBlockNumberConsistency{WatcherClient: mockClient, Logger: zaptest.NewLogger(t)}).GenerateMetrics("test_network")
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})
}

func TestWatcherBlockNonStateConsistencyGenerateMetrics(t *testing.T) {
	t.Run("successfully builds single metric for block non-state consistency", func(t *testing.T) {
		mockClient := &MockWatcherAPIClient{
			mockCheckResponses: []watcher.Result[watcher.CheckResponse]{OK_CHECK_RESPONSE_ONE_PROVIDER_GOOD_SCORE},
		}

		metrics, err := (&WatcherBlockNonStateConsistency{WatcherClient: mockClient, Logger: zaptest.NewLogger(t)}).GenerateMetrics("test_network")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(metrics) != 1 {
			t.Fatalf("expected 1 metric, got %d", len(metrics))
		}
		expectedMetric, _ := NewProviderMetric(
			BlockNonStateConsistencyMetricID,
			"provider1",
			"https://provider1.com",
			0.95,
			time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		)
		if !metrics[0].Equal(expectedMetric) {
			t.Errorf("expected provider1 %v, got %v", expectedMetric, metrics[0])
		}
	})

	t.Run("catch error when API returns error", func(t *testing.T) {
		mockClient := &MockWatcherAPIClient{
			mockCheckResponses: []watcher.Result[watcher.CheckResponse]{KO_CHECK_RESPONSE_API_ERROR},
		}

		_, err := (&WatcherBlockNonStateConsistency{WatcherClient: mockClient, Logger: zaptest.NewLogger(t)}).GenerateMetrics("test_network")
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})
}

func TestWatcherLatencyGenerateMetrics(t *testing.T) {
	t.Run("successfully builds single metric for latency", func(t *testing.T) {
		mockClient := &MockWatcherAPIClient{
			mockLatencyResponses: []watcher.Result[watcher.LatencyResponse]{OK_LATENCY_RESPONSE_ONE_PROVIDER_GOOD_SCORE},
		}

		metrics, err := (&WatcherLatency{WatcherClient: mockClient, Logger: zaptest.NewLogger(t)}).GenerateMetrics("test_network")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(metrics) != 1 {
			t.Fatalf("expected 1 metric, got %d", len(metrics))
		}
		expectedMetric, _ := NewProviderMetric(
			LatencyMetricID,
			"provider1",
			"https://provider1.com",
			0.8316,
			time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		)
		if !metrics[0].Equal(expectedMetric) {
			t.Errorf("expected provider1 %v, got %v", expectedMetric, metrics[0])
		}
	})

	t.Run("catch error when API returns error", func(t *testing.T) {
		mockClient := &MockWatcherAPIClient{
			mockLatencyResponses: []watcher.Result[watcher.LatencyResponse]{KO_LATENCY_RESPONSE_API_ERROR},
		}

		_, err := (&WatcherLatency{WatcherClient: mockClient, Logger: zaptest.NewLogger(t)}).GenerateMetrics("test_network")
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})
}
