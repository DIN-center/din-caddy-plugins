package modules

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetCheck(t *testing.T) {
	tests := []struct {
		name       string
		params     CheckParams
		serverFunc func(w http.ResponseWriter, r *http.Request)
		wantErr    bool
	}{
		{
			name: "successful request",
			params: CheckParams{
				CheckID:  "blockNumberConsistency",
				Network:  "ethereum-holesky",
				Interval: "1h",
			},
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				// Verify request
				if r.URL.Path != CHECK_RESOURCE_PATH {
					t.Errorf("Expected path %s, got %s", CHECK_RESOURCE_PATH, r.URL.Path)
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
						"provider_location": "test-location",
						"endpoint_url": "https://test.com",
						"block_number_from": 1000,
						"block_number_to": 2000,
						"lastest_check_ts": "2024-01-01T01:00:00Z",
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
			params: CheckParams{
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
				if r.URL.Path != LATENCY_RESOURCE_PATH {
					t.Errorf("Expected path %s, got %s", LATENCY_RESOURCE_PATH, r.URL.Path)
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
						"provider_location": "test-location",
						"endpoint_url": "https://test.com",
						"lastest_ping_ts": "2024-01-01T01:00:00Z",
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
			}
		})
	}
}
