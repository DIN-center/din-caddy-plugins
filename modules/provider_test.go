package modules

import (
	"container/list"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/DIN-center/din-caddy-plugins/lib/auth/siwe"
	ws "github.com/DIN-center/din-caddy-plugins/lib/watcherscore"
)

func TestNewProvider(t *testing.T) {
	tests := []struct {
		name         string
		urlStr       string
		expectError  bool
		expectedURL  string
		expectedHost string
		errorMessage string
	}{
		{
			name:         "valid http url",
			urlStr:       "http://example.com",
			expectError:  false,
			expectedURL:  "http://example.com",
			expectedHost: "example.com",
		},
		{
			name:         "valid https url with port",
			urlStr:       "https://example.com:8545",
			expectError:  false,
			expectedURL:  "https://example.com:8545",
			expectedHost: "example.com:8545",
		},
		{
			name:         "invalid url",
			urlStr:       "not-a-url",
			expectError:  true,
			errorMessage: "invalid URL: missing host",
		},
		{
			name:         "empty url",
			urlStr:       "",
			expectError:  true,
			errorMessage: "empty URL",
		},
		{
			name:         "missing scheme",
			urlStr:       "example.com",
			expectError:  true,
			errorMessage: "invalid URL: missing host",
		},
		{
			name:        "empty url",
			urlStr:      "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewProvider(tt.urlStr)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, but got nil")
				}
				if p != nil {
					t.Errorf("expected nil provider, but got %v", p)
				}
				if tt.errorMessage != "" && (err == nil || !reflect.DeepEqual(err.Error(), tt.errorMessage)) {
					t.Errorf("expected error message %q, but got %q", tt.errorMessage, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, but got %v", err)
				}
				if p == nil {
					t.Errorf("expected non-nil provider, but got nil")
				}
				if p != nil && p.HttpUrl != tt.expectedURL {
					t.Errorf("expected URL %q, but got %q", tt.expectedURL, p.HttpUrl)
				}
				if p != nil && p.host != tt.expectedHost {
					t.Errorf("expected host %q, but got %q", tt.expectedHost, p.host)
				}
				if p != nil && p.Headers == nil {
					t.Errorf("expected non-nil headers, but got nil")
				}
				if p != nil && len(p.Headers) != 0 {
					t.Errorf("expected empty headers, but got %v", p.Headers)
				}
				if p != nil && p.Score != ws.EmptyScore {
					t.Errorf("expected to be empty score, but got %v", p.Score)
				}
			}
		})
	}
}

func TestAuthClient(t *testing.T) {
	tests := []struct {
		name           string
		auth           *siwe.SIWEClientAuth
		expectedResult bool
	}{
		{
			name:           "auth client configured",
			auth:           &siwe.SIWEClientAuth{},
			expectedResult: true,
		},
		{
			name:           "no auth client",
			auth:           nil,
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &provider{
				Auth: tt.auth,
			}

			result := p.AuthClient()
			if tt.expectedResult && result == nil {
				t.Errorf("expected non-nil result, but got nil")
			}
			if !tt.expectedResult && result != nil {
				t.Errorf("expected nil result, but got %v", result)
			}
		})
	}
}

func TestHealthy(t *testing.T) {
	tests := []struct {
		name           string
		healthStatus   HealthStatus
		expectedResult bool
	}{
		{
			name:           "healthy status",
			healthStatus:   Healthy,
			expectedResult: true,
		},
		{
			name:           "warning status",
			healthStatus:   Warning,
			expectedResult: false,
		},
		{
			name:           "unhealthy status",
			healthStatus:   Unhealthy,
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			p := &provider{
				blockHistory: list.New(),
			}
			entry := blockHistoryEntry{blockNumber: 100, healthStatus: tt.healthStatus, timestamp: &now}
			p.blockHistory.PushBack(entry)

			result := p.Healthy()
			if result != tt.expectedResult {
				t.Errorf("expected result %v, but got %v", tt.expectedResult, result)
			}
		})
	}
}

func TestWarning(t *testing.T) {
	tests := []struct {
		name           string
		healthStatus   HealthStatus
		expectedResult bool
	}{
		{
			name:           "warning status",
			healthStatus:   Warning,
			expectedResult: true,
		},
		{
			name:           "healthy status",
			healthStatus:   Healthy,
			expectedResult: false,
		},
		{
			name:           "unhealthy status",
			healthStatus:   Unhealthy,
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			p := &provider{
				blockHistory: list.New(),
			}
			entry := blockHistoryEntry{blockNumber: 100, healthStatus: tt.healthStatus, timestamp: &now}
			p.blockHistory.PushBack(entry)

			result := p.Warning()
			if result != tt.expectedResult {
				t.Errorf("expected result %v, but got %v", tt.expectedResult, result)
			}
		})
	}
}

func TestAddBlockEntry(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name           string
		setupHistory   func() *list.List
		newBlock       int64
		newStatus      HealthStatus
		historySize    int
		expectedLength int
		expectedFirst  int64
		expectedLast   int64
	}{
		{
			name: "add to empty history",
			setupHistory: func() *list.List {
				return list.New()
			},
			newBlock:       100,
			newStatus:      Healthy,
			historySize:    3,
			expectedLength: 1,
			expectedFirst:  100,
			expectedLast:   100,
		},
		{
			name: "add within size limit",
			setupHistory: func() *list.List {
				l := list.New()
				l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: Healthy, timestamp: &now})
				return l
			},
			newBlock:       101,
			newStatus:      Healthy,
			historySize:    3,
			expectedLength: 2,
			expectedFirst:  100,
			expectedLast:   101,
		},
		{
			name: "exceed size limit",
			setupHistory: func() *list.List {
				l := list.New()
				l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: Healthy, timestamp: &now})
				l.PushBack(blockHistoryEntry{blockNumber: 101, healthStatus: Healthy, timestamp: &now})
				l.PushBack(blockHistoryEntry{blockNumber: 102, healthStatus: Healthy, timestamp: &now})
				return l
			},
			newBlock:       103,
			newStatus:      Healthy,
			historySize:    3,
			expectedLength: 3,
			expectedFirst:  101,
			expectedLast:   103,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &provider{
				blockHistory: tt.setupHistory(),
			}

			p.AddBlockEntry(tt.newBlock, tt.newStatus, tt.historySize)

			result := p.BlockHistory()
			if len(result) != tt.expectedLength {
				t.Errorf("expected length %v, but got %v", tt.expectedLength, len(result))
			}

			if len(result) > 0 {
				if result[0].blockNumber != tt.expectedFirst {
					t.Errorf("expected first block number %v, but got %v", tt.expectedFirst, result[0].blockNumber)
				}
				if result[len(result)-1].blockNumber != tt.expectedLast {
					t.Errorf("expected last block number %v, but got %v", tt.expectedLast, result[len(result)-1].blockNumber)
				}
				if result[len(result)-1].timestamp == nil {
					t.Errorf("expected non-nil timestamp, but got nil")
				}
			}
		})
	}
}

func TestGetLatestHealthyBlockEntry(t *testing.T) {
	tests := []struct {
		name          string
		setupHistory  func() *list.List
		expectedBlock *blockHistoryEntry
	}{
		{
			name: "empty history returns nil",
			setupHistory: func() *list.List {
				return list.New()
			},
			expectedBlock: nil,
		},
		{
			name: "single healthy entry returns that entry",
			setupHistory: func() *list.List {
				l := list.New()
				l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: Healthy})
				return l
			},
			expectedBlock: &blockHistoryEntry{blockNumber: 100, healthStatus: Healthy},
		},
		{
			name: "single unhealthy entry returns nil",
			setupHistory: func() *list.List {
				l := list.New()
				l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: Unhealthy})
				return l
			},
			expectedBlock: nil,
		},
		{
			name: "multiple entries returns latest healthy",
			setupHistory: func() *list.List {
				l := list.New()
				l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: Healthy})
				l.PushBack(blockHistoryEntry{blockNumber: 101, healthStatus: Unhealthy})
				l.PushBack(blockHistoryEntry{blockNumber: 102, healthStatus: Healthy})
				l.PushBack(blockHistoryEntry{blockNumber: 103, healthStatus: Unhealthy})
				return l
			},
			expectedBlock: &blockHistoryEntry{blockNumber: 102, healthStatus: Healthy},
		},
		{
			name: "all unhealthy entries returns nil",
			setupHistory: func() *list.List {
				l := list.New()
				l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: Unhealthy})
				l.PushBack(blockHistoryEntry{blockNumber: 101, healthStatus: Unhealthy})
				l.PushBack(blockHistoryEntry{blockNumber: 102, healthStatus: Unhealthy})
				return l
			},
			expectedBlock: nil,
		},
		{
			name: "latest entry is healthy returns that entry",
			setupHistory: func() *list.List {
				l := list.New()
				l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: Unhealthy})
				l.PushBack(blockHistoryEntry{blockNumber: 101, healthStatus: Unhealthy})
				l.PushBack(blockHistoryEntry{blockNumber: 102, healthStatus: Healthy})
				return l
			},
			expectedBlock: &blockHistoryEntry{blockNumber: 102, healthStatus: Healthy},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &provider{
				blockHistory: tt.setupHistory(),
			}

			got := p.getLatestHealthyBlockEntry()

			if tt.expectedBlock == nil {
				if got != nil {
					t.Errorf("getLatestHealthyBlockEntry() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Errorf("getLatestHealthyBlockEntry() = nil, want %v", tt.expectedBlock)
				return
			}

			if got.blockNumber != tt.expectedBlock.blockNumber {
				t.Errorf("getLatestHealthyBlockEntry() blockNumber = %v, want %v",
					got.blockNumber, tt.expectedBlock.blockNumber)
			}

			if got.healthStatus != tt.expectedBlock.healthStatus {
				t.Errorf("getLatestHealthyBlockEntry() healthStatus = %v, want %v",
					got.healthStatus, tt.expectedBlock.healthStatus)
			}
		})
	}
}

func TestProviderBlockHistory(t *testing.T) {
	// Helper function to create a time pointer
	timePtr := func(t time.Time) *time.Time {
		return &t
	}

	now := time.Now()
	pastTime1 := now.Add(-1 * time.Hour)
	pastTime2 := now.Add(-2 * time.Hour)
	pastTime3 := now.Add(-3 * time.Hour)

	tests := []struct {
		name          string
		blockHistory  func() *list.List
		expectedItems []blockHistoryEntry
	}{
		{
			name: "Empty history",
			blockHistory: func() *list.List {
				return list.New()
			},
			expectedItems: []blockHistoryEntry{},
		},
		{
			name: "Single entry with timestamp",
			blockHistory: func() *list.List {
				l := list.New()
				l.PushBack(blockHistoryEntry{
					blockNumber:  100,
					healthStatus: Healthy,
					timestamp:    timePtr(now),
				})
				return l
			},
			expectedItems: []blockHistoryEntry{
				{
					blockNumber:  100,
					healthStatus: Healthy,
					timestamp:    timePtr(now),
				},
			},
		},
		{
			name: "Single entry without timestamp",
			blockHistory: func() *list.List {
				l := list.New()
				l.PushBack(blockHistoryEntry{
					blockNumber:  200,
					healthStatus: Unhealthy,
					timestamp:    nil,
				})
				return l
			},
			expectedItems: []blockHistoryEntry{
				{
					blockNumber:  200,
					healthStatus: Unhealthy,
					timestamp:    nil,
				},
			},
		},
		{
			name: "Multiple entries with mixed timestamps",
			blockHistory: func() *list.List {
				l := list.New()
				l.PushBack(blockHistoryEntry{
					blockNumber:  100,
					healthStatus: Healthy,
					timestamp:    timePtr(pastTime3),
				})
				l.PushBack(blockHistoryEntry{
					blockNumber:  101,
					healthStatus: Warning,
					timestamp:    nil,
				})
				l.PushBack(blockHistoryEntry{
					blockNumber:  102,
					healthStatus: Unhealthy,
					timestamp:    timePtr(pastTime1),
				})
				return l
			},
			expectedItems: []blockHistoryEntry{
				{
					blockNumber:  100,
					healthStatus: Healthy,
					timestamp:    timePtr(pastTime3),
				},
				{
					blockNumber:  101,
					healthStatus: Warning,
					timestamp:    nil,
				},
				{
					blockNumber:  102,
					healthStatus: Unhealthy,
					timestamp:    timePtr(pastTime1),
				},
			},
		},
		{
			name: "Multiple entries with various health statuses",
			blockHistory: func() *list.List {
				l := list.New()
				l.PushBack(blockHistoryEntry{
					blockNumber:  200,
					healthStatus: Healthy,
					timestamp:    timePtr(pastTime3),
				})
				l.PushBack(blockHistoryEntry{
					blockNumber:  201,
					healthStatus: Warning,
					timestamp:    timePtr(pastTime2),
				})
				l.PushBack(blockHistoryEntry{
					blockNumber:  202,
					healthStatus: Unhealthy,
					timestamp:    timePtr(pastTime1),
				})
				return l
			},
			expectedItems: []blockHistoryEntry{
				{
					blockNumber:  200,
					healthStatus: Healthy,
					timestamp:    timePtr(pastTime3),
				},
				{
					blockNumber:  201,
					healthStatus: Warning,
					timestamp:    timePtr(pastTime2),
				},
				{
					blockNumber:  202,
					healthStatus: Unhealthy,
					timestamp:    timePtr(pastTime1),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &provider{
				blockHistory: tt.blockHistory(),
			}

			history := p.BlockHistory()

			// Check if the length matches
			if len(history) != len(tt.expectedItems) {
				t.Errorf("BlockHistory() returned %d items, expected %d", len(history), len(tt.expectedItems))
				return
			}

			// Check each item
			for i, expected := range tt.expectedItems {
				got := history[i]

				// Check block number
				if got.blockNumber != expected.blockNumber {
					t.Errorf("BlockHistory()[%d].blockNumber = %d, expected %d", i, got.blockNumber, expected.blockNumber)
				}

				// Check health status
				if got.healthStatus != expected.healthStatus {
					t.Errorf("BlockHistory()[%d].healthStatus = %v, expected %v", i, got.healthStatus, expected.healthStatus)
				}

				// Check timestamp
				if (expected.timestamp == nil && got.timestamp != nil) ||
					(expected.timestamp != nil && got.timestamp == nil) {
					t.Errorf("BlockHistory()[%d].timestamp nil status doesn't match: got %v, expected %v",
						i, got.timestamp != nil, expected.timestamp != nil)
				} else if expected.timestamp != nil && got.timestamp != nil {
					if !expected.timestamp.Equal(*got.timestamp) {
						t.Errorf("BlockHistory()[%d].timestamp = %v, expected %v",
							i, *got.timestamp, *expected.timestamp)
					}

					// Verify deep copy by checking the pointer addresses are different
					if reflect.ValueOf(got.timestamp).Pointer() == reflect.ValueOf(expected.timestamp).Pointer() {
						t.Errorf("BlockHistory()[%d].timestamp is not a deep copy, got same pointer", i)
					}
				}
			}
		})
	}
}

func TestSafeExtractMainDomainWithPSL(t *testing.T) {
	tests := []struct {
		name         string
		urlStr       string
		expectedName string
	}{
		{
			name:         "valid url",
			urlStr:       "https://example.com",
			expectedName: "example",
		},
		{
			name:         "valid url with port",
			urlStr:       "https://example.com:8545",
			expectedName: "example",
		},
		{
			name:         "valid url with path",
			urlStr:       "https://example.com/path",
			expectedName: "example",
		},
		{
			name:         "valid url with query params",
			urlStr:       "https://example.com/path?query=value",
			expectedName: "example",
		},
		{
			name:         "valid url with two level domain",
			urlStr:       "https://example.com.au",
			expectedName: "example",
		},
		{
			name:         "valid url multple subdomains and two level domain",
			urlStr:       "https://subdomain.buying-spree.example.com.au",
			expectedName: "example",
		},
		{
			name:         "valid url with suffix",
			urlStr:       "https://invalid.domain.io-XHG",
			expectedName: "domain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, err := url.Parse(tt.urlStr)
			if err != nil {
				t.Errorf("failed to parse url: %v", err)
			}

			name := safeExtractMainDomainWithPSL(url)
			if name != tt.expectedName {
				t.Errorf("expected name %q, but got %q", tt.expectedName, name)
			}
		})
	}
}
