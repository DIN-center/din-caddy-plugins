package modules

import (
	"container/list"
	"testing"

	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	prom "github.com/DIN-center/din-caddy-plugins/lib/prometheus"
	"github.com/DIN-center/din-caddy-plugins/lib/utils"
	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
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
				HandleHealthCheckMetric(gomock.Any()).
				AnyTimes()

			p := &provider{
				consecutiveUnhealthyChecks: tt.consecutiveUnhealthy,
				host:                       "test.com",
			}

			n := NewNetwork("test", utils.Environment("test"))
			n.HCThreshold = tt.healthThreshold
			n.PrometheusClient = mockPrometheus
			n.logger = logger.NewLoggerClient(zap.NewNop(), utils.Environment("test"))

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
			n := NewNetwork("test", utils.Environment("test"))
			n.ChainId = tt.networkChainID

			result := n.verifyChainID(tt.providerChainID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsStalled(t *testing.T) {
	tests := []struct {
		name         string
		historySize  int
		blockHistory []blockHistoryEntry
		expected     bool
	}{
		{
			name:         "empty history",
			historySize:  3,
			blockHistory: []blockHistoryEntry{},
			expected:     false,
		},
		{
			name:        "single entry",
			historySize: 3,
			blockHistory: []blockHistoryEntry{
				{blockNumber: 100, healthStatus: Healthy},
			},
			expected: false,
		},
		{
			name:        "multiple entries, not stalled",
			historySize: 3,
			blockHistory: []blockHistoryEntry{
				{blockNumber: 100, healthStatus: Healthy},
				{blockNumber: 101, healthStatus: Healthy},
				{blockNumber: 102, healthStatus: Healthy},
			},
			expected: false,
		},
		{
			name:        "multiple entries, stalled",
			historySize: 3,
			blockHistory: []blockHistoryEntry{
				{blockNumber: 100, healthStatus: Healthy},
				{blockNumber: 100, healthStatus: Healthy},
				{blockNumber: 100, healthStatus: Healthy},
			},
			expected: true,
		},
		{
			name:        "not enough history",
			historySize: 3,
			blockHistory: []blockHistoryEntry{
				{blockNumber: 100, healthStatus: Healthy},
				{blockNumber: 100, healthStatus: Healthy},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := NewNetwork("test", utils.Environment("test"))
			n.BlockHistorySize = tt.historySize

			p := &provider{blockHistory: func() *list.List {
				l := list.New()
				for _, entry := range tt.blockHistory {
					l.PushBack(entry)
				}
				return l
			}()}

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
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: Healthy})
						return l
					}(),
				},
				"p2": {
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 90, healthStatus: Healthy})
						return l
					}(),
				},
			},
			expected: 100,
		},
		{
			name: "warning provider used when no healthy",
			providers: map[string]*provider{
				"p1": {
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: Warning})
						return l
					}(),
				},
				"p2": {
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 110, healthStatus: Warning})
						return l
					}(),
				},
			},
			expected: 110,
		},
		{
			name: "empty history returns 0",
			providers: map[string]*provider{
				"p1": {
					blockHistory: list.New(),
				},
			},
			expected: 0,
		},
		{
			name: "healthy provider preferred over higher warning block",
			providers: map[string]*provider{
				"p1": {
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: Healthy})
						return l
					}(),
				},
				"p2": {
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 150, healthStatus: Warning})
						return l
					}(),
				},
			},
			expected: 100,
		},
		{
			name: "warning provider preferred over higher unhealthy block",
			providers: map[string]*provider{
				"p1": {
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: Warning})
						return l
					}(),
				},
				"p2": {
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 150, healthStatus: Unhealthy})
						return l
					}(),
				},
			},
			expected: 100,
		},
		{
			name: "mixed health statuses with multiple entries",
			providers: map[string]*provider{
				"p1": {
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 99, healthStatus: Healthy})
						l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: Healthy})
						return l
					}(),
				},
				"p2": {
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 110, healthStatus: Warning})
						l.PushBack(blockHistoryEntry{blockNumber: 109, healthStatus: Warning})
						return l
					}(),
				},
				"p3": {
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 120, healthStatus: Unhealthy})
						l.PushBack(blockHistoryEntry{blockNumber: 119, healthStatus: Unhealthy})
						return l
					}(),
				},
			},
			expected: 100,
		},
		{
			name: "no providers with block history",
			providers: map[string]*provider{
				"p1": {
					blockHistory: list.New(),
				},
				"p2": {
					blockHistory: list.New(),
				},
			},
			expected: 0,
		},
		{
			name: "only unhealthy providers with blocks",
			providers: map[string]*provider{
				"p1": {
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: Unhealthy})
						return l
					}(),
				},
				"p2": {
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 110, healthStatus: Unhealthy})
						return l
					}(),
				},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := NewNetwork("test", utils.Environment("test"))
			n.Providers = tt.providers

			result := n.getLatestHealthyBlock()
			assert.Equal(t, tt.expected, result)
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
			n := NewNetwork("test", utils.Environment("test"))
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

func TestArchiveModeCheck(t *testing.T) {
	tests := []struct {
		name                string
		httpResponse        []byte
		statusCode          int
		httpError           error
		quarterBlockHeight  string
		requestAttemptCount int
		expectError         bool
		errorContains       string
	}{
		{
			name:                "successful archive test",
			httpResponse:        []byte(`{"jsonrpc":"2.0","result":"0x1234"}`),
			statusCode:          200,
			quarterBlockHeight:  "0x1234",
			requestAttemptCount: 1,
			expectError:         false,
		},
		{
			name:                "service unavailable",
			httpResponse:        []byte{},
			statusCode:          503,
			quarterBlockHeight:  "0x1234",
			requestAttemptCount: 1,
			expectError:         true,
			errorContains:       "Network Unavailable",
		},
		{
			name:                "invalid json response",
			httpResponse:        []byte(`invalid json`),
			statusCode:          200,
			quarterBlockHeight:  "0x1234",
			requestAttemptCount: 1,
			expectError:         true,
			errorContains:       "Error unmarshalling response",
		},
		{
			name:                "archive mode not supported",
			httpResponse:        []byte(`{"error":{"code":-32000,"message":"missing trie node"}}`),
			statusCode:          200,
			quarterBlockHeight:  "0x1234",
			requestAttemptCount: 1,
			expectError:         true,
			errorContains:       "network doesn't support archive mode",
		},
		{
			name:                "http client error",
			httpResponse:        []byte{},
			statusCode:          200,
			httpError:           errors.New("connection failed"),
			quarterBlockHeight:  "0x1234",
			requestAttemptCount: 1,
			expectError:         true,
			errorContains:       "Error sending POST request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockHTTPClient := din_http.NewMockIHTTPClient(ctrl)

			// Set up the mock to expect exactly one call
			mockHTTPClient.EXPECT().
				Post(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(tt.httpResponse, &tt.statusCode, tt.httpError)

			n := NewNetwork("test", utils.Environment("test"))
			n.HttpClient = mockHTTPClient
			n.CallContractMethod = "eth_call"
			n.RequestAttemptCount = tt.requestAttemptCount

			err := n.archiveModeCheck("http://test.com", nil, nil, tt.quarterBlockHeight)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetChainID(t *testing.T) {
	tests := []struct {
		name                string
		networkName         string
		httpResponses       [][]byte
		statusCodes         []int
		httpErrors          []error
		requestAttemptCount int
		expectedChainID     string
		expectError         bool
		errorContains       string
		expectedAttempts    int
	}{
		{
			name:                "successful ethereum response on first attempt",
			httpResponses:       [][]byte{[]byte(`{"jsonrpc":"2.0","result":"0x1"}`)},
			statusCodes:         []int{200},
			httpErrors:          []error{nil},
			requestAttemptCount: 3,
			expectedChainID:     "eip155:0x1",
			expectError:         false,
			expectedAttempts:    1,
		},
		{
			name:                "successful bitcoin response on first attempt",
			networkName:         "bitcoin",
			httpResponses:       [][]byte{[]byte(`{"jsonrpc":"2.0","result":{"chain":"main"}}`)},
			statusCodes:         []int{200},
			httpErrors:          []error{nil},
			requestAttemptCount: 3,
			expectedChainID:     "bip122:main",
			expectError:         false,
			expectedAttempts:    1,
		},
		{
			name:                "successful solana response on first attempt",
			networkName:         "solana",
			httpResponses:       [][]byte{[]byte(`{"jsonrpc":"2.0","result":"mainnet-beta"}`)},
			statusCodes:         []int{200},
			httpErrors:          []error{nil},
			requestAttemptCount: 3,
			expectedChainID:     "solana:mainnet-beta",
			expectError:         false,
			expectedAttempts:    1,
		},
		{
			name:                "successful starknet response on first attempt",
			networkName:         "starknet",
			httpResponses:       [][]byte{[]byte(`{"jsonrpc":"2.0","result":"SN_MAIN"}`)},
			statusCodes:         []int{200},
			httpErrors:          []error{nil},
			requestAttemptCount: 3,
			expectedChainID:     "starknet:SN_MAIN",
			expectError:         false,
			expectedAttempts:    1,
		},
		{
			name:                "success after one failure",
			httpResponses:       [][]byte{nil, []byte(`{"jsonrpc":"2.0","result":"0x1"}`)},
			statusCodes:         []int{0, 200},
			httpErrors:          []error{errors.New("connection failed"), nil},
			requestAttemptCount: 3,
			expectedChainID:     "eip155:0x1",
			expectError:         false,
			expectedAttempts:    2,
		},
		{
			name:                "non-200 status code with retries exhausted",
			httpResponses:       [][]byte{[]byte{}, []byte{}, []byte{}},
			statusCodes:         []int{503, 503, 503},
			httpErrors:          []error{nil, nil, nil},
			requestAttemptCount: 3,
			expectedChainID:     "",
			expectError:         true,
			errorContains:       "Error getting chain ID from response",
			expectedAttempts:    3,
		},
		{
			name:                "invalid json response with retries exhausted",
			httpResponses:       [][]byte{[]byte(`invalid json`), []byte(`invalid json`), []byte(`invalid json`)},
			statusCodes:         []int{200, 200, 200},
			httpErrors:          []error{nil, nil, nil},
			requestAttemptCount: 3,
			expectedChainID:     "",
			expectError:         true,
			errorContains:       "Error unmarshalling response",
			expectedAttempts:    3,
		},
		{
			name:                "missing result field with retries exhausted",
			httpResponses:       [][]byte{[]byte(`{"jsonrpc":"2.0"}`), []byte(`{"jsonrpc":"2.0"}`), []byte(`{"jsonrpc":"2.0"}`)},
			statusCodes:         []int{200, 200, 200},
			httpErrors:          []error{nil, nil, nil},
			requestAttemptCount: 3,
			expectedChainID:     "",
			expectError:         true,
			errorContains:       "Error getting chain ID from response",
			expectedAttempts:    3,
		},
		{
			name:                "http client error with retries exhausted",
			httpResponses:       [][]byte{nil, nil, nil},
			statusCodes:         []int{0, 0, 0},
			httpErrors:          []error{errors.New("connection failed"), errors.New("connection failed"), errors.New("connection failed")},
			requestAttemptCount: 3,
			expectedChainID:     "",
			expectError:         true,
			errorContains:       "Error sending POST request",
			expectedAttempts:    3,
		},
		{
			name:                "invalid bitcoin response with retries exhausted",
			networkName:         "bitcoin",
			httpResponses:       [][]byte{[]byte(`{"jsonrpc":"2.0","result":{}}`), []byte(`{"jsonrpc":"2.0","result":{}}`), []byte(`{"jsonrpc":"2.0","result":{}}`)},
			statusCodes:         []int{200, 200, 200},
			httpErrors:          []error{nil, nil, nil},
			requestAttemptCount: 3,
			expectedChainID:     "",
			expectError:         true,
			errorContains:       "Error getting chain ID from response",
			expectedAttempts:    3,
		},
		{
			name:                "invalid result type with retries exhausted",
			httpResponses:       [][]byte{[]byte(`{"jsonrpc":"2.0","result":123}`), []byte(`{"jsonrpc":"2.0","result":123}`), []byte(`{"jsonrpc":"2.0","result":123}`)},
			statusCodes:         []int{200, 200, 200},
			httpErrors:          []error{nil, nil, nil},
			requestAttemptCount: 3,
			expectedChainID:     "",
			expectError:         true,
			errorContains:       "Error getting chain ID from response",
			expectedAttempts:    3,
		},
		{
			name:                "mixed errors with success on final attempt",
			httpResponses:       [][]byte{nil, []byte(`invalid json`), []byte(`{"jsonrpc":"2.0","result":"0x1"}`)},
			statusCodes:         []int{0, 200, 200},
			httpErrors:          []error{errors.New("connection failed"), nil, nil},
			requestAttemptCount: 3,
			expectedChainID:     "eip155:0x1",
			expectError:         false,
			expectedAttempts:    3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockHTTPClient := din_http.NewMockIHTTPClient(ctrl)

			// Set up expectations for each attempt
			for i := 0; i < tt.expectedAttempts; i++ {
				mockHTTPClient.EXPECT().
					Post(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(tt.httpResponses[i], &tt.statusCodes[i], tt.httpErrors[i])
			}

			networkName := tt.networkName
			if networkName == "" {
				networkName = "test"
			}

			n := NewNetwork("test", utils.Environment("test"))
			n.HttpClient = mockHTTPClient
			n.ChainIdMethod = "eth_chainId"
			n.RequestAttemptCount = tt.requestAttemptCount

			chainID, err := n.getChainID("http://test.com", nil, nil)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				assert.Equal(t, tt.expectedChainID, chainID)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedChainID, chainID)
			}
		})
	}
}
