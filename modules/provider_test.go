package modules

import (
	"testing"
	"time"

	"github.com/DIN-center/din-caddy-plugins/lib/auth/siwe"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"github.com/stretchr/testify/assert"
)

type MockUpstream reverseproxy.Upstream

func TestNewProvider(t *testing.T) {
	tests := []struct {
		name         string
		urlStr       string
		expectError  bool
		expectedURL  string
		expectedHost string
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
			name:        "invalid url",
			urlStr:      "not-a-url",
			expectError: true,
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

func TestAvailable(t *testing.T) {
	tests := []struct {
		name           string
		upstreamAvail  bool
		healthStatus   HealthStatus
		expectedResult bool
	}{
		{
			name:           "upstream available and healthy",
			upstreamAvail:  true,
			healthStatus:   Healthy,
			expectedResult: true,
		},
		{
			name:           "upstream available but unhealthy",
			upstreamAvail:  true,
			healthStatus:   Unhealthy,
			expectedResult: false,
		},
		{
			name:           "upstream unavailable but healthy",
			upstreamAvail:  false,
			healthStatus:   Healthy,
			expectedResult: false,
		},
		{
			name:           "upstream unavailable and warning",
			upstreamAvail:  false,
			healthStatus:   Warning,
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUpstream := (*reverseproxy.Upstream)(new(MockUpstream))

			p := &provider{
				upstream:     mockUpstream,
				healthStatus: tt.healthStatus,
			}

			result := p.Available()
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestIsAvailableWithWarning(t *testing.T) {
	tests := []struct {
		name           string
		upstreamAvail  bool
		healthStatus   HealthStatus
		expectedResult bool
	}{
		{
			name:           "upstream available and warning",
			upstreamAvail:  true,
			healthStatus:   Warning,
			expectedResult: true,
		},
		{
			name:           "upstream available and healthy",
			upstreamAvail:  true,
			healthStatus:   Healthy,
			expectedResult: false,
		},
		{
			name:           "upstream unavailable with warning",
			upstreamAvail:  false,
			healthStatus:   Warning,
			expectedResult: false,
		},
		{
			name:           "upstream unavailable and unhealthy",
			upstreamAvail:  false,
			healthStatus:   Unhealthy,
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUpstream := (*reverseproxy.Upstream)(new(MockUpstream))

			p := &provider{
				upstream:     mockUpstream,
				healthStatus: tt.healthStatus,
			}

			result := p.IsAvailableWithWarning()
			assert.Equal(t, tt.expectedResult, result)
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
			p := &provider{
				healthStatus: tt.healthStatus,
			}

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
			p := &provider{
				healthStatus: tt.healthStatus,
			}

			result := p.Warning()
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestBlockHistory(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name         string
		blockHistory []blockHistoryEntry
		expected     int
	}{
		{
			name: "normal history",
			blockHistory: []blockHistoryEntry{
				{blockNumber: 100, statusCode: Healthy, timestamp: &now},
				{blockNumber: 101, statusCode: Healthy, timestamp: &now},
			},
			expected: 2,
		},
		{
			name:         "empty history",
			blockHistory: []blockHistoryEntry{},
			expected:     0,
		},
		{
			name: "single entry",
			blockHistory: []blockHistoryEntry{
				{blockNumber: 100, statusCode: Healthy, timestamp: &now},
			},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &provider{
				blockHistory: tt.blockHistory,
			}

			result := p.BlockHistory()
			assert.Equal(t, tt.expected, len(result))
			assert.Equal(t, tt.blockHistory, result)

			// Verify it's a copy, not the original slice
			if len(result) > 0 {
				result[0].blockNumber = 999
				assert.NotEqual(t, result[0].blockNumber, p.blockHistory[0].blockNumber)
			}
		})
	}
}

func TestAddBlockEntry(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name           string
		initialHistory []blockHistoryEntry
		newBlock       int64
		newStatus      HealthStatus
		historySize    int
		expectedLength int
		expectedFirst  int64
		expectedLast   int64
	}{
		{
			name:           "add to empty history",
			initialHistory: []blockHistoryEntry{},
			newBlock:       100,
			newStatus:      Healthy,
			historySize:    3,
			expectedLength: 1,
			expectedFirst:  100,
			expectedLast:   100,
		},
		{
			name: "add within size limit",
			initialHistory: []blockHistoryEntry{
				{blockNumber: 100, statusCode: Healthy, timestamp: &now},
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
			initialHistory: []blockHistoryEntry{
				{blockNumber: 100, statusCode: Healthy, timestamp: &now},
				{blockNumber: 101, statusCode: Healthy, timestamp: &now},
				{blockNumber: 102, statusCode: Healthy, timestamp: &now},
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
				blockHistory: tt.initialHistory,
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

func TestProviderGetChainID(t *testing.T) {
	tests := []struct {
		name            string
		chainID         string
		expectedChainID string
	}{
		{
			name:            "normal chain ID",
			chainID:         "1",
			expectedChainID: "1",
		},
		{
			name:            "empty chain ID",
			chainID:         "",
			expectedChainID: "",
		},
		{
			name:            "hex chain ID",
			chainID:         "0x1",
			expectedChainID: "0x1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &provider{
				chainID: tt.chainID,
			}

			result := p.getChainID()
			assert.Equal(t, tt.expectedChainID, result)
		})
	}
}
