package modules

import (
	"testing"
	"time"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	prom "github.com/DIN-center/din-caddy-plugins/lib/prometheus"
	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestHandleErrorWithGracePeriod(t *testing.T) {
	tests := []struct {
		name                   string
		consecutiveUnhealthy   int
		healthThreshold        int
		currentStatus          HealthStatus
		expectedStatus         HealthStatus
		expectedUnhealthyCount int
	}{
		{
			name:                   "first error within threshold",
			consecutiveUnhealthy:   0,
			healthThreshold:        3,
			currentStatus:          Unhealthy,
			expectedStatus:         Warning,
			expectedUnhealthyCount: 1,
		},
		{
			name:                   "multiple errors within threshold",
			consecutiveUnhealthy:   1,
			healthThreshold:        3,
			currentStatus:          Unhealthy,
			expectedStatus:         Warning,
			expectedUnhealthyCount: 2,
		},
		{
			name:                   "errors beyond threshold",
			consecutiveUnhealthy:   2,
			healthThreshold:        3,
			currentStatus:          Unhealthy,
			expectedStatus:         Unhealthy,
			expectedUnhealthyCount: 3,
		},
		{
			name:                   "healthy status resets counter",
			consecutiveUnhealthy:   2,
			healthThreshold:        3,
			currentStatus:          Healthy,
			expectedStatus:         Healthy,
			expectedUnhealthyCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrometheus := prom.NewMockIPrometheusClient(ctrl)
			mockPrometheus.EXPECT().
				HandleLatestBlockMetric(gomock.Any()).
				AnyTimes()

			p := &provider{
				consecutiveUnhealthyChecks: tt.consecutiveUnhealthy,
				host:                       "test.com",
			}

			n := NewNetwork("test")
			n.HCThreshold = tt.healthThreshold
			n.PrometheusClient = mockPrometheus

			status := n.handleErrorWithGracePeriod(p, tt.currentStatus, 100)

			assert.Equal(t, tt.expectedStatus, status)
			assert.Equal(t, tt.expectedUnhealthyCount, p.consecutiveUnhealthyChecks)
		})
	}
}

func TestVerifyChainID(t *testing.T) {
	tests := []struct {
		name            string
		networkChainID  string
		providerChainID string
		expected        bool
	}{
		{
			name:            "matching chain IDs",
			networkChainID:  "mainnet",
			providerChainID: "mainnet",
			expected:        true,
		},
		{
			name:            "mismatched chain IDs",
			networkChainID:  "mainnet",
			providerChainID: "testnet",
			expected:        false,
		},
		{
			name:            "empty provider chain ID",
			networkChainID:  "mainnet",
			providerChainID: "",
			expected:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := NewNetwork("test")
			n.ChainId = tt.networkChainID

			result := n.verifyChainID(tt.providerChainID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsStalled(t *testing.T) {
	tests := []struct {
		name         string
		blockHistory []blockHistoryEntry
		historySize  int
		expected     bool
	}{
		{
			name: "not enough history",
			blockHistory: []blockHistoryEntry{
				{blockNumber: 100},
			},
			historySize: 2,
			expected:    false,
		},
		{
			name: "stalled blocks",
			blockHistory: []blockHistoryEntry{
				{blockNumber: 100},
				{blockNumber: 100},
				{blockNumber: 100},
			},
			historySize: 3,
			expected:    true,
		},
		{
			name: "progressing blocks",
			blockHistory: []blockHistoryEntry{
				{blockNumber: 100},
				{blockNumber: 101},
				{blockNumber: 102},
			},
			historySize: 3,
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := NewNetwork("test")
			n.BlockHistorySize = tt.historySize

			p := &provider{blockHistory: tt.blockHistory}

			result := n.isStalled(p)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetLatestHealthyBlock(t *testing.T) {
	tests := []struct {
		name      string
		providers map[string]*provider
		expected  int64
	}{
		{
			name: "healthy provider has highest block",
			providers: map[string]*provider{
				"p1": {
					healthStatus: Healthy,
					blockHistory: []blockHistoryEntry{{blockNumber: 100}},
				},
				"p2": {
					healthStatus: Warning,
					blockHistory: []blockHistoryEntry{{blockNumber: 90}},
				},
			},
			expected: 100,
		},
		{
			name: "warning provider used when no healthy",
			providers: map[string]*provider{
				"p1": {
					healthStatus: Warning,
					blockHistory: []blockHistoryEntry{{blockNumber: 100}},
				},
				"p2": {
					healthStatus: Unhealthy,
					blockHistory: []blockHistoryEntry{{blockNumber: 110}},
				},
			},
			expected: 100,
		},
		{
			name: "empty history returns 0",
			providers: map[string]*provider{
				"p1": {
					healthStatus: Healthy,
					blockHistory: []blockHistoryEntry{},
				},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := NewNetwork("test")
			n.Providers = tt.providers

			result := n.getLatestHealthyBlock()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetChainID(t *testing.T) {
	tests := []struct {
		name            string
		networkName     string
		httpResponse    []byte
		statusCode      int
		httpError       error
		expectedChainID string
		expectError     bool
		errorContains   string
	}{
		{
			name:            "successful ethereum response",
			httpResponse:    []byte(`{"jsonrpc":"2.0","result":"0x1"}`),
			statusCode:      200,
			expectedChainID: "eip155:0x1",
			expectError:     false,
		},
		{
			name:            "successful bitcoin response",
			networkName:     "bitcoin",
			httpResponse:    []byte(`{"jsonrpc":"2.0","result":{"chain":"main"}}`),
			statusCode:      200,
			expectedChainID: "bip122:main",
			expectError:     false,
		},
		{
			name:            "non-200 status code",
			httpResponse:    []byte{},
			statusCode:      503,
			expectedChainID: "",
			expectError:     true,
			errorContains:   "Error getting chain ID from response",
		},
		{
			name:            "invalid json response",
			httpResponse:    []byte(`invalid json`),
			statusCode:      200,
			expectedChainID: "",
			expectError:     true,
			errorContains:   "Error unmarshalling response",
		},
		{
			name:            "missing result field",
			httpResponse:    []byte(`{"jsonrpc":"2.0"}`),
			statusCode:      200,
			expectedChainID: "",
			expectError:     true,
			errorContains:   "Error getting chain ID from response",
		},
		{
			name:            "http client error",
			httpError:       errors.New("connection failed"),
			expectedChainID: "",
			expectError:     true,
			errorContains:   "Error sending POST request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockHTTPClient := din_http.NewMockIHTTPClient(ctrl)
			mockHTTPClient.EXPECT().
				Post(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(tt.httpResponse, &tt.statusCode, tt.httpError)

			n := NewNetwork(tt.networkName)
			n.HttpClient = mockHTTPClient
			n.ChainIdMethod = "eth_chainId"

			chainID, err := n.getChainID("http://test.com", nil, nil)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedChainID, chainID)
			}
		})
	}
}

func TestArchiveMode(t *testing.T) {
	tests := []struct {
		name               string
		httpResponse       []byte
		statusCode         int
		httpError          error
		quarterBlockHeight string
		expectError        bool
		errorContains      string
	}{
		{
			name:               "successful archive test",
			httpResponse:       []byte(`{"jsonrpc":"2.0","result":"0x1234"}`),
			statusCode:         200,
			quarterBlockHeight: "0x1234",
			expectError:        false,
		},
		{
			name:               "service unavailable",
			httpResponse:       []byte{},
			statusCode:         503,
			quarterBlockHeight: "0x1234",
			expectError:        true,
			errorContains:      "Network Unavailable",
		},
		{
			name:               "invalid json response",
			httpResponse:       []byte(`invalid json`),
			statusCode:         200,
			quarterBlockHeight: "0x1234",
			expectError:        true,
			errorContains:      "Error unmarshalling response",
		},
		{
			name:               "archive mode not supported",
			httpResponse:       []byte(`{"error":{"code":-32000,"message":"missing trie node"}}`),
			statusCode:         200,
			quarterBlockHeight: "0x1234",
			expectError:        true,
			errorContains:      "network doesn't support archive mode",
		},
		{
			name:               "http client error",
			httpResponse:       []byte{},
			statusCode:         200,
			httpError:          errors.New("connection failed"),
			quarterBlockHeight: "0x1234",
			expectError:        true,
			errorContains:      "Error sending POST request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockHTTPClient := din_http.NewMockIHTTPClient(ctrl)
			mockHTTPClient.EXPECT().
				Post(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(tt.httpResponse, &tt.statusCode, tt.httpError)

			n := NewNetwork("test")
			n.HttpClient = mockHTTPClient
			n.CallContractMethod = "eth_call"

			err := n.testArchiveMode("http://test.com", nil, nil, tt.quarterBlockHeight)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestProcessBlockNumberResponse(t *testing.T) {
	tests := []struct {
		name           string
		response       []byte
		statusCode     int
		expectedBlock  int64
		expectedHealth HealthStatus
		expectError    bool
		errorContains  string
	}{
		{
			name:           "valid hex response",
			response:       []byte(`{"jsonrpc":"2.0","result":"0x1234"}`),
			statusCode:     200,
			expectedBlock:  0x1234,
			expectedHealth: Healthy,
			expectError:    false,
		},
		{
			name:           "valid decimal response",
			response:       []byte(`{"jsonrpc":"2.0","result":1234}`),
			statusCode:     200,
			expectedBlock:  1234,
			expectedHealth: Healthy,
			expectError:    false,
		},
		{
			name:           "rate limit error",
			response:       []byte(`{"error":"rate limited"}`),
			statusCode:     429,
			expectedBlock:  0,
			expectedHealth: Warning,
			expectError:    true,
			errorContains:  "rate limit error",
		},
		{
			name:           "server error",
			response:       []byte(`{"error":"internal error"}`),
			statusCode:     500,
			expectedBlock:  0,
			expectedHealth: Unhealthy,
			expectError:    true,
			errorContains:  "error status code: 500",
		},
		{
			name:           "invalid json",
			response:       []byte(`invalid json`),
			statusCode:     200,
			expectedBlock:  0,
			expectedHealth: Unhealthy,
			expectError:    true,
			errorContains:  "Error unmarshalling response",
		},
		{
			name:           "missing hex prefix",
			response:       []byte(`{"jsonrpc":"2.0","result":"1234"}`),
			statusCode:     200,
			expectedBlock:  0,
			expectedHealth: Unhealthy,
			expectError:    true,
			errorContains:  "Invalid block number",
		},
		{
			name:           "invalid hex string",
			response:       []byte(`{"jsonrpc":"2.0","result":"0xZZZZ"}`),
			statusCode:     200,
			expectedBlock:  0,
			expectedHealth: Unhealthy,
			expectError:    true,
			errorContains:  "Error converting block number",
		},
		{
			name:           "unsupported result type",
			response:       []byte(`{"jsonrpc":"2.0","result":{"block":1234}}`),
			statusCode:     200,
			expectedBlock:  0,
			expectedHealth: Unhealthy,
			expectError:    true,
			errorContains:  "unsupported block number type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := NewNetwork("test")
			block, health, err := n.processBlockNumberResponse(tt.response, &tt.statusCode)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedBlock, block)
			assert.Equal(t, tt.expectedHealth, health)
		})
	}
}

func TestTryBlockNumberRequestWithTimeout(t *testing.T) {
	tests := []struct {
		name          string
		responseDelay time.Duration
		httpResponse  []byte
		statusCode    int
		httpError     error
		expectError   bool
		errorContains string
		expectedCalls int
	}{
		{
			name:          "successful response",
			httpResponse:  []byte(`{"result":"0x1234"}`),
			statusCode:    200,
			expectError:   false,
			expectedCalls: 1,
		},
		{
			name:          "timeout on first try, success on retry",
			responseDelay: time.Duration(BlockNumberTimeoutSeconds+1) * time.Second,
			httpResponse:  []byte(`{"result":"0x1234"}`),
			statusCode:    200,
			expectError:   true,
			errorContains: "request is taking too long after retry",
			expectedCalls: 2,
			// For timeout errors, both resBytes and statusCode should be nil
		},
		{
			name:          "http client error",
			httpError:     errors.New("connection failed"),
			statusCode:    0,
			expectError:   true,
			errorContains: "connection failed",
			expectedCalls: 1,
		},
		{
			name:          "non-200 status code",
			httpResponse:  []byte(`{"error":"server error"}`),
			statusCode:    500,
			expectError:   false, // Function only handles timeouts, not HTTP errors
			expectedCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockHTTPClient := din_http.NewMockIHTTPClient(ctrl)

			if tt.responseDelay > 0 {
				mockHTTPClient.EXPECT().
					Post(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(url string, headers map[string]string, payload []byte, ac auth.IAuthClient) ([]byte, *int, error) {
						time.Sleep(tt.responseDelay)
						return tt.httpResponse, &tt.statusCode, tt.httpError
					}).Times(tt.expectedCalls)
			} else {
				mockHTTPClient.EXPECT().
					Post(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(tt.httpResponse, &tt.statusCode, tt.httpError).
					Times(tt.expectedCalls)
			}

			n := NewNetwork("test")
			n.HttpClient = mockHTTPClient
			n.HCMethod = "eth_blockNumber"

			resBytes, statusCode, err := n.tryBlockNumberRequestWithTimeout("test-url", nil, nil)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				if tt.responseDelay > 0 {
					// For timeout errors, both resBytes and statusCode should be nil
					assert.Nil(t, resBytes)
					assert.Nil(t, statusCode)
				} else {
					// For other errors, check status code
					assert.Equal(t, &tt.statusCode, statusCode)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.httpResponse, resBytes)
				assert.Equal(t, tt.statusCode, *statusCode)
			}
		})
	}
}
