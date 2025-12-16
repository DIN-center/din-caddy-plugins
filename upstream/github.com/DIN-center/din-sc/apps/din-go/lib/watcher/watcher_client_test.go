package watcher

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetCheck(t *testing.T) {
	tests := []struct {
		name       string
		params     CheckQueryParams
		serverFunc func(w http.ResponseWriter, r *http.Request)
		wantErr    bool
	}{
		{
			name: "successful request",
			params: CheckQueryParams{
				CheckID:  "blockNumberConsistency",
				Network:  "ethereum-holesky",
				Interval: "1h",
			},
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				// Verify request
				if r.URL.Path != CheckResourcePath {
					t.Errorf("Expected path %s, got %s", CheckResourcePath, r.URL.Path)
				}
				if r.Header.Get("X-API-KEY") == "" {
					t.Error("Missing API key header")
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{
					"watcher_id": "test-watcher",
					"origin": "test",
					"network": "ethereum-holesky",
					"cycle_start": "2024-01-01T00:00:00Z",
					"cycle_end": "2024-01-01T01:00:00Z",
					"cycle_duration": 3600,
					"total_providers": 1,
					"providers": [{
						"provider": "test-provider",
						"provider_id": "test-provider-id",
						"provider_location": "test-location",
						"endpoint_url": "https://test.com",
						"block_number_from": 1000,
						"block_number_to": 2000,
						"latest_check_ts": "2024-01-01T01:00:00Z",
						"check_summary": {
							"total": 100,
							"pass": 95,
							"fail": 5,
							"pass_percentage": 95.0,
							"fail_percentage": 5.0
						},
						"response_status": {
							"total": 100,
							"success": 95,
							"error": 5,
							"timeout": 0,
							"success_percentage": 95.0
						}
					}]
				}`))
			},
			wantErr: false,
		},
		{
			name: "invalid parameters",
			params: CheckQueryParams{
				CheckID: "invalid",
				Network: "ethereum-holesky",
			},
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverFunc))
			defer server.Close()

			client := NewClient(server.URL, "test-key")
			result := client.GetCheck(tt.params)

			if tt.wantErr && !result.IsErr() {
				t.Error("Expected error, got success")
			}
			if !tt.wantErr && result.IsErr() {
				t.Errorf("Expected success, got error: %v", result.UnwrapErr())
			}

			if !tt.wantErr {
				response := result.Unwrap()
				if response.WatcherID != "test-watcher" {
					t.Errorf("Expected watcher_id 'test-watcher', got '%s'", response.WatcherID)
				}
				if response.Origin != "test" {
					t.Errorf("Expected origin 'test', got '%s'", response.Origin)
				}
				if response.Network != "ethereum-holesky" {
					t.Errorf("Expected network 'ethereum-holesky', got '%s'", response.Network)
				}
				if response.CycleStart != "2024-01-01T00:00:00Z" {
					t.Errorf("Expected cycle_start '2024-01-01T00:00:00Z', got '%s'", response.CycleStart)
				}
				if response.CycleEnd != "2024-01-01T01:00:00Z" {
					t.Errorf("Expected cycle_end '2024-01-01T01:00:00Z', got '%s'", response.CycleEnd)
				}
				if response.CycleDuration != 3600 {
					t.Errorf("Expected cycle_duration 3600, got %d", response.CycleDuration)
				}
				if response.TotalProviders != 1 {
					t.Errorf("Expected total_providers 1, got %d", response.TotalProviders)
				}
				if len(response.Providers) != 1 {
					t.Errorf("Expected 1 provider, got %d", len(response.Providers))
				}
				if response.Providers[0].Provider != "test-provider" {
					t.Errorf("Expected provider 'test-provider', got '%s'", response.Providers[0].Provider)
				}
				if response.Providers[0].ProviderID != "test-provider-id" {
					t.Errorf("Expected provider_id 'test-provider-id', got '%s'", response.Providers[0].ProviderID)
				}
				if response.Providers[0].ProviderLocation != "test-location" {
					t.Errorf("Expected provider_location 'test-location', got '%s'", response.Providers[0].ProviderLocation)
				}
				if response.Providers[0].EndpointURL != "https://test.com" {
					t.Errorf("Expected endpoint_url 'https://test.com', got '%s'", response.Providers[0].EndpointURL)
				}
				if response.Providers[0].BlockNumberFrom != 1000 {
					t.Errorf("Expected block_number_from 1000, got %d", response.Providers[0].BlockNumberFrom)
				}
				if response.Providers[0].BlockNumberTo != 2000 {
					t.Errorf("Expected block_number_to 2000, got %d", response.Providers[0].BlockNumberTo)
				}
				if response.Providers[0].LatestCheckTimestamp != "2024-01-01T01:00:00Z" {
					t.Errorf("Expected latest_check_ts '2024-01-01T01:00:00Z', got '%s'", response.Providers[0].LatestCheckTimestamp)
				}
				if response.Providers[0].CheckSummary.Total != 100 {
					t.Errorf("Expected check_summary.total 100, got %d", response.Providers[0].CheckSummary.Total)
				}
				if response.Providers[0].CheckSummary.Pass != 95 {
					t.Errorf("Expected check_summary.pass 95, got %d", response.Providers[0].CheckSummary.Pass)
				}
				if response.Providers[0].CheckSummary.Fail != 5 {
					t.Errorf("Expected check_summary.fail 5, got %d", response.Providers[0].CheckSummary.Fail)
				}
				if response.Providers[0].CheckSummary.PassPercentage != 95.0 {
					t.Errorf("Expected check_summary.pass_percentage 95.0, got %f", response.Providers[0].CheckSummary.PassPercentage)
				}
				if response.Providers[0].CheckSummary.FailPercentage != 5.0 {
					t.Errorf("Expected check_summary.fail_percentage 5.0, got %f", response.Providers[0].CheckSummary.FailPercentage)
				}
				if response.Providers[0].ResponseStatus.Total != 100 {
					t.Errorf("Expected response_status.total 100, got %d", response.Providers[0].ResponseStatus.Total)
				}
				if response.Providers[0].ResponseStatus.Success != 95 {
					t.Errorf("Expected response_status.success 95, got %d", response.Providers[0].ResponseStatus.Success)
				}
				if response.Providers[0].ResponseStatus.Error != 5 {
					t.Errorf("Expected response_status.error 5, got %d", response.Providers[0].ResponseStatus.Error)
				}
				if response.Providers[0].ResponseStatus.Timeout != 0 {
					t.Errorf("Expected response_status.timeout 0, got %d", response.Providers[0].ResponseStatus.Timeout)
				}
				if response.Providers[0].ResponseStatus.SuccessPercentage != 95.0 {
					t.Errorf("Expected response_status.success_percentage 95.0, got %f", response.Providers[0].ResponseStatus.SuccessPercentage)
				}
			}
		})
	}
}

func TestGetLatency(t *testing.T) {
	tests := []struct {
		name       string
		params     LatencyQueryParams
		serverFunc func(w http.ResponseWriter, r *http.Request)
		wantErr    bool
	}{
		{
			name: "successful request",
			params: LatencyQueryParams{
				Network:  "ethereum-holesky",
				Interval: "1h",
			},
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != LatencyResourcePath {
					t.Errorf("Expected path %s, got %s", LatencyResourcePath, r.URL.Path)
				}
				if r.Header.Get("X-API-KEY") == "" {
					t.Error("Missing API key header")
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{
					"watcher_id": "test-watcher",
					"origin": "test",
					"network": "ethereum-holesky",
					"cycle_start": "2024-01-01T00:00:00Z",
					"cycle_end": "2024-01-01T01:00:00Z",
					"cycle_duration": 3600,
					"total_providers": 1,
					"providers": [{
						"provider": "test-provider",
						"provider_id": "test-provider-id",
						"provider_location": "test-location",
						"endpoint_url": "https://test.com",
						"last_request_ts": "2024-01-01T01:00:00Z",
						"max_rps": 100.0,
						"latency": {
							"min": 50.0,
							"avg": 75.0,
							"max": 100.0,
							"p50": 75.0,
							"p75": 85.0,
							"p90": 90.0,
							"p95": 95.0,
							"p99": 99.0
						},
						"response_status": {
							"total": 100,
							"success": 95,
							"error": 5,
							"timeout": 0,
							"success_percentage": 95.0
						}
					}]
				}`))
			},
			wantErr: false,
		},
		{
			name: "invalid parameters",
			params: LatencyQueryParams{
				Network:  "",
				Interval: "invalid",
			},
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverFunc))
			defer server.Close()

			client := NewClient(server.URL, "test-key")
			result := client.GetLatency(tt.params)

			if tt.wantErr && !result.IsErr() {
				t.Error("Expected error, got success")
			}
			if !tt.wantErr && result.IsErr() {
				t.Errorf("Expected success, got error: %v", result.UnwrapErr())
			}

			if !tt.wantErr {
				response := result.Unwrap()
				if response.WatcherID != "test-watcher" {
					t.Errorf("Expected watcher_id 'test-watcher', got '%s'", response.WatcherID)
				}
				if response.CycleStart != "2024-01-01T00:00:00Z" {
					t.Errorf("Expected cycle_start '2024-01-01T00:00:00Z', got '%s'", response.CycleStart)
				}
				if response.CycleEnd != "2024-01-01T01:00:00Z" {
					t.Errorf("Expected cycle_end '2024-01-01T01:00:00Z', got '%s'", response.CycleEnd)
				}
				if response.CycleDuration != 3600 {
					t.Errorf("Expected cycle_duration 3600, got %d", response.CycleDuration)
				}
				if response.TotalProviders != 1 {
					t.Errorf("Expected total_providers 1, got %d", response.TotalProviders)
				}
				if len(response.Providers) != 1 {
					t.Errorf("Expected 1 provider, got %d", len(response.Providers))
				}
				if response.Providers[0].Provider != "test-provider" {
					t.Errorf("Expected provider 'test-provider', got '%s'", response.Providers[0].Provider)
				}
				if response.Providers[0].ProviderID != "test-provider-id" {
					t.Errorf("Expected provider_id 'test-provider-id', got '%s'", response.Providers[0].ProviderID)
				}
				if response.Providers[0].ProviderLocation != "test-location" {
					t.Errorf("Expected provider_location 'test-location', got '%s'", response.Providers[0].ProviderLocation)
				}
				if response.Providers[0].EndpointURL != "https://test.com" {
					t.Errorf("Expected endpoint_url 'https://test.com', got '%s'", response.Providers[0].EndpointURL)
				}
				if response.Providers[0].LastRequestTimestamp != "2024-01-01T01:00:00Z" {
					t.Errorf("Expected last_request_ts '2024-01-01T01:00:00Z', got '%s'", response.Providers[0].LastRequestTimestamp)
				}
				if response.Providers[0].MaxRPS != 100.0 {
					t.Errorf("Expected max_rps 100.0, got %f", response.Providers[0].MaxRPS)
				}
				if response.Providers[0].Latency.Min != 50.0 {
					t.Errorf("Expected latency.min 50.0, got %f", response.Providers[0].Latency.Min)
				}
				if response.Providers[0].Latency.Avg != 75.0 {
					t.Errorf("Expected latency.avg 75.0, got %f", response.Providers[0].Latency.Avg)
				}
				if response.Providers[0].Latency.Max != 100.0 {
					t.Errorf("Expected latency.max 100.0, got %f", response.Providers[0].Latency.Max)
				}
				if response.Providers[0].Latency.P50 != 75.0 {
					t.Errorf("Expected latency.p50 75.0, got %f", response.Providers[0].Latency.P50)
				}
				if response.Providers[0].Latency.P75 != 85.0 {
					t.Errorf("Expected latency.p75 85.0, got %f", response.Providers[0].Latency.P75)
				}
				if response.Providers[0].Latency.P90 != 90.0 {
					t.Errorf("Expected latency.p90 90.0, got %f", response.Providers[0].Latency.P90)
				}
				if response.Providers[0].Latency.P95 != 95.0 {
					t.Errorf("Expected latency.p95 95.0, got %f", response.Providers[0].Latency.P95)
				}
				if response.Providers[0].Latency.P99 != 99.0 {
					t.Errorf("Expected latency.p99 99.0, got %f", response.Providers[0].Latency.P99)
				}
				if response.Providers[0].ResponseStatus.Total != 100 {
					t.Errorf("Expected response_status.total 100, got %d", response.Providers[0].ResponseStatus.Total)
				}
				if response.Providers[0].ResponseStatus.Success != 95 {
					t.Errorf("Expected response_status.success 95, got %d", response.Providers[0].ResponseStatus.Success)
				}
				if response.Providers[0].ResponseStatus.Error != 5 {
					t.Errorf("Expected response_status.error 5, got %d", response.Providers[0].ResponseStatus.Error)
				}
				if response.Providers[0].ResponseStatus.Timeout != 0 {
					t.Errorf("Expected response_status.timeout 0, got %d", response.Providers[0].ResponseStatus.Timeout)
				}
				if response.Providers[0].ResponseStatus.SuccessPercentage != 95.0 {
					t.Errorf("Expected response_status.success_percentage 95.0, got %f", response.Providers[0].ResponseStatus.SuccessPercentage)
				}
			}
		})
	}
}
