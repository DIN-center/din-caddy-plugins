package reputationscore

import (
	"fmt"
	"sync"
	"time"

	"github.com/DIN-center/din-sc/apps/din-go/lib/watcher"
	"go.uber.org/zap"
)

// MockWatcherAPIClient is a mock implementation of the IWatcherAPIClient interface to be used in tests
type MockWatcherAPIClient struct {
	mockCheckResponses   []watcher.Result[watcher.CheckResponse]
	checkCallsCount      int
	mockLatencyResponses []watcher.Result[watcher.LatencyResponse]
	latencyCallsCount    int
}

func (m *MockWatcherAPIClient) GetCheck(params watcher.CheckQueryParams) watcher.Result[watcher.CheckResponse] {
	m.checkCallsCount++
	if m.checkCallsCount > len(m.mockCheckResponses) {
		return watcher.Err[watcher.CheckResponse](fmt.Errorf("no more mock check responses available"))
	}
	return m.mockCheckResponses[m.checkCallsCount-1]
}

func (m *MockWatcherAPIClient) GetLatency(params watcher.LatencyQueryParams) watcher.Result[watcher.LatencyResponse] {
	m.latencyCallsCount++
	if m.latencyCallsCount > len(m.mockLatencyResponses) {
		return watcher.Err[watcher.LatencyResponse](fmt.Errorf("no more mock latency responses available"))
	}
	return m.mockLatencyResponses[m.latencyCallsCount-1]
}

// Mock data for check response API
var OK_CHECK_RESPONSE_ONE_PROVIDER_GOOD_SCORE = watcher.Ok(watcher.CheckResponse{
	Providers: []watcher.CheckProviderData{
		{
			Provider:    "provider1",
			EndpointURL: "https://provider1.com",
			ResponseStatus: watcher.Status{
				SuccessPercentage: 100.0,
			},
			CheckSummary: watcher.Summary{
				PassPercentage: 95.0, // Score is 0.95 because response status is 100% and pass percentage is 95%
			},
			LatestCheckTimestamp: "2023-01-01T00:00:00Z",
		},
	},
})

var OK_CHECK_RESPONSE_ONE_PROVIDER_BAD_SCORE = watcher.Ok(watcher.CheckResponse{
	Providers: []watcher.CheckProviderData{
		{
			Provider:    "provider1",
			EndpointURL: "https://provider1.com",
			ResponseStatus: watcher.Status{
				SuccessPercentage: 55.0,
			},
			CheckSummary: watcher.Summary{
				PassPercentage: 100.0, // Score is 0 because response status is below threshold
			},
			LatestCheckTimestamp: "2023-01-01T00:00:00Z",
		},
	},
})

var OK_CHECK_RESPONSE_TWO_PROVIDERS_GOOD_SCORES_TIME1 = watcher.Ok(watcher.CheckResponse{
	Providers: []watcher.CheckProviderData{
		{
			Provider:    "provider1",
			EndpointURL: "https://provider1.com",
			ResponseStatus: watcher.Status{
				SuccessPercentage: 100.0,
			},
			CheckSummary: watcher.Summary{
				PassPercentage: 95.0, //Score is 0.95
			},
			LatestCheckTimestamp: "2023-01-01T00:00:00Z",
		},
		{
			Provider:    "provider2",
			EndpointURL: "https://provider2.com",
			ResponseStatus: watcher.Status{
				SuccessPercentage: 100.0,
			},
			CheckSummary: watcher.Summary{
				PassPercentage: 20.0, //Score is 0.2
			},
			LatestCheckTimestamp: "2023-01-01T00:00:00Z",
		},
	},
})

var OK_CHECK_RESPONSE_TWO_PROVIDERS_GOOD_SCORES_TIME2 = watcher.Ok(watcher.CheckResponse{
	Providers: []watcher.CheckProviderData{
		{
			Provider:    "provider1",
			EndpointURL: "https://provider1.com",
			ResponseStatus: watcher.Status{
				SuccessPercentage: 100.0,
			},
			CheckSummary: watcher.Summary{
				PassPercentage: 95.0, //Score is 0.95
			},
			LatestCheckTimestamp: "2024-01-01T00:00:00Z",
		},
		{
			Provider:    "provider2",
			EndpointURL: "https://provider2.com",
			ResponseStatus: watcher.Status{
				SuccessPercentage: 100.0,
			},
			CheckSummary: watcher.Summary{
				PassPercentage: 20.0, //Score is 0.2
			},
			LatestCheckTimestamp: "2024-01-01T00:00:00Z",
		},
	},
})

var OK_CHECK_RESPONSE_TWO_PROVIDERS_BAD_SCORES_TIME1 = watcher.Ok(watcher.CheckResponse{
	Providers: []watcher.CheckProviderData{
		{
			Provider:    "provider1",
			EndpointURL: "https://provider1.com",
			ResponseStatus: watcher.Status{
				SuccessPercentage: 55.0,
			},
			CheckSummary: watcher.Summary{
				PassPercentage: 100.0, // Score is 0 because response status is below threshold
			},
			LatestCheckTimestamp: "2023-01-01T00:00:00Z",
		},
		{
			Provider:    "provider2",
			EndpointURL: "https://provider2.com",
			ResponseStatus: watcher.Status{
				SuccessPercentage: 100.0,
			},
			CheckSummary: watcher.Summary{
				PassPercentage: 80.0, // Score is 0.8 because response status is above threshold
			},
			LatestCheckTimestamp: "2023-01-01T00:00:00Z",
		},
	},
})

var KO_CHECK_RESPONSE_API_ERROR = watcher.Err[watcher.CheckResponse](fmt.Errorf("API error"))

var OK_CHECK_RESPONSE_WRONG_TIMESTAMP = watcher.Ok(watcher.CheckResponse{
	Providers: []watcher.CheckProviderData{
		{
			Provider: "provider1",
			ResponseStatus: watcher.Status{
				SuccessPercentage: 100.0,
			},
			CheckSummary: watcher.Summary{
				PassPercentage: 100.0,
			},
			LatestCheckTimestamp: "invalid-timestamp",
		},
	},
})

// Mock data for latency response API
var OK_LATENCY_RESPONSE_ONE_PROVIDER_GOOD_SCORE = watcher.Ok(watcher.LatencyResponse{
	Providers: []watcher.LatencyProviderData{
		{
			Provider:    "provider1",
			EndpointURL: "https://provider1.com",
			ResponseStatus: watcher.Status{
				SuccessPercentage: 100.0,
			},
			Latency: watcher.LatencyStats{
				P95: 100.0, // 100ms latency = 0.9 score
			},
			LastRequestTimestamp: "2023-01-01T00:00:00Z",
		},
	},
})

var OK_LATENCY_RESPONSE_ONE_PROVIDER_BAD_SCORE = watcher.Ok(watcher.LatencyResponse{
	Providers: []watcher.LatencyProviderData{
		{
			Provider:    "provider1",
			EndpointURL: "https://provider1.com",
			ResponseStatus: watcher.Status{
				SuccessPercentage: 55.0,
			},
			Latency: watcher.LatencyStats{
				P95: 100.0, // Score is 0 because response status is below threshold
			},
			LastRequestTimestamp: "2023-01-01T00:00:00Z",
		},
	},
})

var OK_LATENCY_RESPONSE_TWO_PROVIDERS_GOOD_SCORE_TIME1 = watcher.Ok(watcher.LatencyResponse{
	Providers: []watcher.LatencyProviderData{
		{
			Provider:    "provider1",
			EndpointURL: "https://provider1.com",
			ResponseStatus: watcher.Status{
				SuccessPercentage: 100.0,
			},
			Latency: watcher.LatencyStats{
				P95: 100.0, // 100ms latency = 0.9 score
			},
			LastRequestTimestamp: "2023-01-01T00:00:00Z",
		},
		{
			Provider:    "provider2",
			EndpointURL: "https://provider2.com",
			ResponseStatus: watcher.Status{
				SuccessPercentage: 100.0,
			},
			Latency: watcher.LatencyStats{
				P95: 500.0, // 500ms latency = 0.5 score
			},
			LastRequestTimestamp: "2023-01-01T00:00:00Z",
		},
	},
})

var OK_LATENCY_RESPONSE_TWO_PROVIDERS_GOOD_SCORE_TIME2 = watcher.Ok(watcher.LatencyResponse{
	Providers: []watcher.LatencyProviderData{
		{
			Provider:    "provider1",
			EndpointURL: "https://provider1.com",
			ResponseStatus: watcher.Status{
				SuccessPercentage: 100.0,
			},
			Latency: watcher.LatencyStats{
				P95: 100.0, // 100ms latency = 0.9 score
			},
			LastRequestTimestamp: "2024-01-01T00:00:00Z",
		},
		{
			Provider:    "provider2",
			EndpointURL: "https://provider2.com",
			ResponseStatus: watcher.Status{
				SuccessPercentage: 100.0,
			},
			Latency: watcher.LatencyStats{
				P95: 500.0, // 500ms latency = 0.5 score
			},
			LastRequestTimestamp: "2024-01-01T00:00:00Z",
		},
	},
})

var OK_LATENCY_RESPONSE_TWO_PROVIDERS_BAD_SCORE_TIME1 = watcher.Ok(watcher.LatencyResponse{
	Providers: []watcher.LatencyProviderData{
		{
			Provider:    "provider1",
			EndpointURL: "https://provider1.com",
			ResponseStatus: watcher.Status{
				SuccessPercentage: 50.0,
			},
			Latency: watcher.LatencyStats{
				P95: 100.0, // Score is 0 because response status is below threshold
			},
			LastRequestTimestamp: "2023-01-01T00:00:00Z",
		},
		{
			Provider:    "provider2",
			EndpointURL: "https://provider2.com",
			ResponseStatus: watcher.Status{
				SuccessPercentage: 100.0,
			},
			Latency: watcher.LatencyStats{
				P95: 500.0, // 500ms latency = 0.5 score
			},
			LastRequestTimestamp: "2023-01-01T00:00:00Z",
		},
	},
})

var KO_LATENCY_RESPONSE_API_ERROR = watcher.Err[watcher.LatencyResponse](fmt.Errorf("API error"))

var OK_LATENCY_RESPONSE_WRONG_TIMESTAMP = watcher.Ok(watcher.LatencyResponse{
	Providers: []watcher.LatencyProviderData{
		{
			Provider:    "provider1",
			EndpointURL: "https://provider1.com",
			ResponseStatus: watcher.Status{
				SuccessPercentage: 100.0,
			},
			Latency: watcher.LatencyStats{
				P95: 100.0,
			},
			LastRequestTimestamp: "invalid-timestamp",
		},
	},
})

// Helper function to create scores without error checking
func MustCreateScore(value float64, time time.Time) *Score {
	score, _ := NewScore(value, time)
	return score
}

func NewMock(logger *zap.Logger, scores map[string]map[string]*Score, formulas map[string]ScoreFormula) *ReputationScoreManager {
	return &ReputationScoreManager{
		scores:   scores,
		formulas: formulas,
		logger:   logger,
		mu:       sync.RWMutex{},
	}
}
