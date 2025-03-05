package modules

import (
	"container/list"
	"testing"
	"time"

	"github.com/DIN-center/din-caddy-plugins/lib/auth/siwe"
	"github.com/stretchr/testify/assert"
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
				assert.Error(t, err)
				assert.Nil(t, p)
				if tt.errorMessage != "" {
					assert.Contains(t, err.Error(), tt.errorMessage)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, p)
				assert.Equal(t, tt.expectedURL, p.HttpUrl)
				assert.Equal(t, tt.expectedHost, p.host)
				assert.NotNil(t, p.Headers)
				assert.Empty(t, p.Headers)
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
			if tt.expectedResult {
				assert.NotNil(t, result)
			} else {
				assert.Nil(t, result)
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
			assert.Equal(t, tt.expectedResult, result)
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
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestBlockHistory(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name         string
		setupHistory func() *list.List
		expected     int
	}{
		{
			name: "normal history",
			setupHistory: func() *list.List {
				l := list.New()
				l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: Healthy, timestamp: &now})
				l.PushBack(blockHistoryEntry{blockNumber: 101, healthStatus: Healthy, timestamp: &now})
				return l
			},
			expected: 2,
		},
		{
			name: "empty history",
			setupHistory: func() *list.List {
				return list.New()
			},
			expected: 0,
		},
		{
			name: "single entry",
			setupHistory: func() *list.List {
				l := list.New()
				l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: Healthy, timestamp: &now})
				return l
			},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &provider{
				blockHistory: tt.setupHistory(),
			}

			result := p.BlockHistory()
			assert.Equal(t, tt.expected, len(result))

			// Verify it's a copy, not the original slice
			if len(result) > 0 {
				// Get the first element from the list
				firstElement := p.blockHistory.Front().Value.(blockHistoryEntry)
				assert.NotEqual(t, result[0].blockNumber, firstElement.blockNumber)
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
			assert.Equal(t, tt.expectedLength, len(result))

			if len(result) > 0 {
				assert.Equal(t, tt.expectedFirst, result[0].blockNumber)
				assert.Equal(t, tt.expectedLast, result[len(result)-1].blockNumber)
				assert.NotNil(t, result[len(result)-1].timestamp)
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
