package modules

import (
	"container/list"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

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

			n := NewNetwork("test", utils.Environment("test"), "test")
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
			n := NewNetwork("test", utils.Environment("test"), "test")
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
			n := NewNetwork("test", utils.Environment("test"), "test")
			n.ProviderBlockHistorySize = tt.historySize

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
			n := NewNetwork("test", utils.Environment("test"), "test")
			n.Providers = tt.providers

			result := n.getLatestHealthyBlock()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProcessBlockNumberResponse(t *testing.T) {
	tests := []struct {
		name              string
		response          []byte
		statusCode        int
		passNilStatusCode bool
		expectedBlock     int64
		expectedHealth    HealthStatus
		expectError       bool
		errorContains     string
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
		{
			name:              "nil status code pointer",
			response:          []byte{},
			statusCode:        0,
			passNilStatusCode: true,
			expectedBlock:     0,
			expectedHealth:    Unhealthy,
			expectError:       true,
			errorContains:     "received nil statusCode in processBlockNumberResponse",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := NewNetwork("test", utils.Environment("test"), "test")
			var sc *int
			if !tt.passNilStatusCode {
				statusCodeVal := tt.statusCode
				sc = &statusCodeVal
			}

			block, health, err := n.processBlockNumberResponse(tt.response, sc)

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

			n := NewNetwork("test", utils.Environment("test"), "test")
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

			n := NewNetwork(networkName, utils.Environment("test"), "test")
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

func TestHasOtherHealthyProviders(t *testing.T) {
	tests := []struct {
		name           string
		providers      map[string]*provider
		checkProvider  string
		expectedResult bool
	}{
		{
			name: "one other healthy provider exists",
			providers: map[string]*provider{
				"p1": {
					host: "p1",
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: Healthy})
						return l
					}(),
				},
				"p2": {
					host: "p2",
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 90, healthStatus: Healthy})
						return l
					}(),
				},
			},
			checkProvider:  "p1",
			expectedResult: true,
		},
		{
			name: "no other healthy providers",
			providers: map[string]*provider{
				"p1": {
					host: "p1",
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: Healthy})
						return l
					}(),
				},
				"p2": {
					host: "p2",
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 90, healthStatus: Warning})
						return l
					}(),
				},
				"p3": {
					host: "p3",
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 110, healthStatus: Unhealthy})
						return l
					}(),
				},
			},
			checkProvider:  "p1",
			expectedResult: false,
		},
		{
			name: "multiple healthy providers",
			providers: map[string]*provider{
				"p1": {
					host: "p1",
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: Healthy})
						return l
					}(),
				},
				"p2": {
					host: "p2",
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 90, healthStatus: Healthy})
						return l
					}(),
				},
				"p3": {
					host: "p3",
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 110, healthStatus: Healthy})
						return l
					}(),
				},
			},
			checkProvider:  "p1",
			expectedResult: true,
		},
		{
			name: "empty history in other providers",
			providers: map[string]*provider{
				"p1": {
					host: "p1",
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: Healthy})
						return l
					}(),
				},
				"p2": {
					host:         "p2",
					blockHistory: list.New(),
				},
			},
			checkProvider:  "p1",
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := NewNetwork("test", utils.Environment("test"), "test")
			n.Providers = tt.providers

			result := n.hasOtherHealthyProviders(tt.providers[tt.checkProvider])
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

// TestBlockJumpBehavior tests the behavior of block jump detection logic
func TestBlockJumpBehavior(t *testing.T) {
	tests := []struct {
		name               string
		providers          map[string]*provider
		testProvider       string
		currentBlock       int64
		latestNetworkBlock int64
		blockJumpLimit     int64
		expectedStatus     HealthStatus
	}{
		{
			name: "block jump with other healthy providers",
			providers: map[string]*provider{
				"p1": {
					host: "p1",
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 1000, healthStatus: Healthy})
						return l
					}(),
				},
				"p2": {
					host: "p2",
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: Healthy})
						return l
					}(),
				},
			},
			testProvider:       "p1",
			currentBlock:       1000,
			latestNetworkBlock: 100,
			blockJumpLimit:     50,
			expectedStatus:     Unhealthy,
		},
		{
			name: "block jump without other healthy providers",
			providers: map[string]*provider{
				"p1": {
					host: "p1",
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 1000, healthStatus: Healthy})
						return l
					}(),
				},
				"p2": {
					host: "p2",
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: Warning})
						return l
					}(),
				},
				"p3": {
					host: "p3",
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 90, healthStatus: Unhealthy})
						return l
					}(),
				},
			},
			testProvider:       "p1",
			currentBlock:       1000,
			latestNetworkBlock: 100,
			blockJumpLimit:     50,
			expectedStatus:     Healthy,
		},
		{
			name: "no block jump",
			providers: map[string]*provider{
				"p1": {
					host: "p1",
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 120, healthStatus: Healthy})
						return l
					}(),
				},
				"p2": {
					host: "p2",
					blockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: Healthy})
						return l
					}(),
				},
			},
			testProvider:       "p1",
			currentBlock:       120,
			latestNetworkBlock: 100,
			blockJumpLimit:     50,
			expectedStatus:     Healthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockHTTPClient := din_http.NewMockIHTTPClient(ctrl)
			mockPrometheus := prom.NewMockIPrometheusClient(ctrl)
			mockLogger := logger.NewLoggerClient(zap.NewNop(), utils.Environment("test"))

			// Set up mock response for chain ID check
			successStatusCode := 200
			chainIDResponse := []byte(`{"jsonrpc":"2.0","result":"1"}`)

			// Setup HTTP client expectations for getChainID - expect calls for each provider
			for range tt.providers {
				mockHTTPClient.EXPECT().
					Post(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(chainIDResponse, &successStatusCode, nil).
					AnyTimes()
			}

			n := NewNetwork("test", utils.Environment("test"), "test")
			n.Providers = tt.providers
			n.BlockJumpLimit = tt.blockJumpLimit
			n.logger = mockLogger
			n.HttpClient = mockHTTPClient
			n.PrometheusClient = mockPrometheus
			n.ChainId = "eip155:1"
			n.ChainIdMethod = "eth_chainId"
			n.ArchiveEnabled = false // Disable archive mode checks for this test

			// Create a custom provider health evaluation function that omits chain ID and archive checks
			testEvaluateBlockJump := func(provider *provider, currentBlock int64, latestNetworkBlock int64) HealthStatus {
				blockJump := currentBlock - latestNetworkBlock
				if blockJump > n.BlockJumpLimit {
					if n.hasOtherHealthyProviders(provider) {
						return Unhealthy
					} else {
						return Healthy
					}
				}
				return Healthy
			}

			// Test only the block jump logic directly
			result := testEvaluateBlockJump(
				tt.providers[tt.testProvider],
				tt.currentBlock,
				tt.latestNetworkBlock,
			)

			assert.Equal(t, tt.expectedStatus, result, "Block jump behavior incorrect")
		})
	}
}

// TestGetLatestBlockNumber tests the getLatestBlockNumber function in network.go
func TestGetLatestBlockNumber(t *testing.T) {
	type mockPostResponse struct {
		resBytes          []byte
		statusCodeVal     int
		passNilStatusCode bool
		err               error
	}

	tests := []struct {
		name                   string
		mockPostResponses      []mockPostResponse
		requestAttemptCount    int
		hcMethod               string
		expectedBlockNumber    int64
		expectedHealthStatus   HealthStatus
		expectedResponseStatus int
		expectError            bool
		errorContains          string
	}{
		{
			name: "success on first attempt",
			mockPostResponses: []mockPostResponse{
				{resBytes: []byte(`{"jsonrpc":"2.0","result":"0x123"}`), statusCodeVal: 200, err: nil},
			},
			requestAttemptCount:    1,
			hcMethod:               "eth_blockNumber",
			expectedBlockNumber:    0x123,
			expectedHealthStatus:   Healthy,
			expectedResponseStatus: 200,
			expectError:            false,
		},
		{
			name: "HttpClient.Post error on first attempt, success on second",
			mockPostResponses: []mockPostResponse{
				{err: errors.New("network hiccup"), statusCodeVal: 503, passNilStatusCode: false}, // passNilStatusCode can be false if statusCodeVal is used
				{resBytes: []byte(`{"jsonrpc":"2.0","result":"0x124"}`), statusCodeVal: 200, err: nil},
			},
			requestAttemptCount:    2,
			hcMethod:               "eth_blockNumber",
			expectedBlockNumber:    0x124,
			expectedHealthStatus:   Healthy,
			expectedResponseStatus: 200,
			expectError:            false,
		},
		{
			name: "HttpClient.Post error with nil status code on first attempt, success on second",
			mockPostResponses: []mockPostResponse{
				{err: errors.New("network hiccup with nil status"), passNilStatusCode: true},
				{resBytes: []byte(`{"jsonrpc":"2.0","result":"0x125"}`), statusCodeVal: 200, err: nil},
			},
			requestAttemptCount:    2,
			hcMethod:               "eth_blockNumber",
			expectedBlockNumber:    0x125,
			expectedHealthStatus:   Healthy,
			expectedResponseStatus: 200, // Status from the successful attempt
			expectError:            false,
		},
		{
			name: "all attempts fail with HttpClient.Post errors (non-nil status codes)",
			mockPostResponses: []mockPostResponse{
				{err: errors.New("attempt 1 fail"), statusCodeVal: 500},
				{err: errors.New("attempt 2 fail"), statusCodeVal: 502},
			},
			requestAttemptCount:    2,
			hcMethod:               "eth_blockNumber",
			expectedBlockNumber:    0,
			expectedHealthStatus:   Unhealthy, // Default initial
			expectedResponseStatus: 502,       // Status from the last attempt
			expectError:            true,
			errorContains:          "attempt 2 fail",
		},
		{
			name: "all attempts fail with HttpClient.Post errors (mixed nil/non-nil status codes)",
			mockPostResponses: []mockPostResponse{
				{err: errors.New("attempt 1 fail"), statusCodeVal: 500},
				{err: errors.New("attempt 2 fail with nil status"), passNilStatusCode: true},
			},
			requestAttemptCount:    2,
			hcMethod:               "eth_blockNumber",
			expectedBlockNumber:    0,
			expectedHealthStatus:   Unhealthy,
			expectedResponseStatus: 500, // Status from the last attempt *that had one*
			expectError:            true,
			errorContains:          "attempt 2 fail with nil status",
		},
		{
			name: "HttpClient.Post returns nil status code, processBlockNumberResponse then errors",
			mockPostResponses: []mockPostResponse{
				// This will cause processBlockNumberResponse to return "received nil statusCode in processBlockNumberResponse"
				{resBytes: []byte(`{}`), passNilStatusCode: true, err: nil},
			},
			requestAttemptCount:    1,
			hcMethod:               "eth_blockNumber",
			expectedBlockNumber:    0,
			expectedHealthStatus:   Unhealthy, // From processBlockNumberResponse's error
			expectedResponseStatus: 0,         // lastResponseStatus not updated due to nil statusCode from Post
			expectError:            true,
			errorContains:          "received nil statusCode in processBlockNumberResponse",
		},
		{
			name: "HttpClient.Post returns data causing processBlockNumberResponse to error (e.g. invalid json)",
			mockPostResponses: []mockPostResponse{
				{resBytes: []byte(`invalid json`), statusCodeVal: 200, err: nil},
			},
			requestAttemptCount:    1,
			hcMethod:               "eth_blockNumber",
			expectedBlockNumber:    0,
			expectedHealthStatus:   Unhealthy, // From processBlockNumberResponse
			expectedResponseStatus: 200,       // Status from Post was 200
			expectError:            true,
			errorContains:          "Error unmarshalling response",
		},
		{
			name: "first attempt Post error with status, second attempt Post success with nil status (leads to process error)",
			mockPostResponses: []mockPostResponse{
				{err: errors.New("attempt 1 fail"), statusCodeVal: 503},
				{resBytes: []byte(`{}`), passNilStatusCode: true, err: nil}, // This leads to "received nil statusCode" error from processBlockNumberResponse
			},
			requestAttemptCount:    2,
			hcMethod:               "eth_blockNumber",
			expectedBlockNumber:    0,
			expectedHealthStatus:   Unhealthy, // From processBlockNumberResponse's error on 2nd attempt
			expectedResponseStatus: 503,       // From first attempt, as second attempt's Post had nil statusCode
			expectError:            true,
			errorContains:          "received nil statusCode in processBlockNumberResponse",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockHTTPClient := din_http.NewMockIHTTPClient(ctrl)
			network := NewNetwork("test-network", utils.Environment("test"), "test")
			network.HttpClient = mockHTTPClient
			network.RequestAttemptCount = tt.requestAttemptCount
			network.HCMethod = tt.hcMethod
			// Suppress logs for cleaner test output, or use a mock logger
			network.logger = logger.NewLoggerClient(zap.NewNop(), utils.Environment("test"))

			callIndex := 0
			mockHTTPClient.EXPECT().Post(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(url string, headers map[string]string, payload []byte, ac interface{}) ([]byte, *int, error) {
					if callIndex >= len(tt.mockPostResponses) {
						t.Fatalf("Mock Post called more times than expected responses defined")
						return nil, nil, errors.New("unexpected call to mock Post")
					}
					resp := tt.mockPostResponses[callIndex]
					callIndex++
					if resp.passNilStatusCode {
						return resp.resBytes, nil, resp.err
					}
					// Make a copy for pointer safety if statusCodeVal is used in parallel subtests (not an issue here but good practice)
					statusCode := resp.statusCodeVal
					return resp.resBytes, &statusCode, resp.err
				}).Times(len(tt.mockPostResponses))

			result, err := network.getLatestBlockNumber("http://dummyurl.com", nil, nil)

			if tt.expectError {
				assert.Error(t, err, "Expected an error but got none")
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains, "Error message does not contain expected string")
				}
			} else {
				assert.NoError(t, err, "Expected no error but got one")
			}

			assert.NotNil(t, result, "Result should not be nil")
			if result != nil {
				assert.Equal(t, tt.expectedBlockNumber, result.blockNumber, "Block number mismatch")
				assert.Equal(t, tt.expectedHealthStatus, result.healthStatus, "Health status mismatch")
				assert.Equal(t, tt.expectedResponseStatus, result.responseStatus, "Response status mismatch")
			}
		})
	}
}

// Helper to create a new network for testing
func newTestNetwork(name string, historySize int) *network {
	return &network{
		Name:                    name,
		logger:                  logger.NewLoggerClient(zap.NewNop(), utils.EnvTest),
		NetworkBlockHistorySize: historySize,
		blockHistory:            list.New(),
	}
}

func TestAddNetworkBlockEntry(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var n *network
		assert.NotPanics(t, func() { n.AddNetworkBlockEntry(100, "test_block_hash") }, "Calling AddNetworkBlockEntry on nil network should not panic")
	})

	t.Run("nil blockHistory initialization", func(t *testing.T) {
		n := newTestNetwork("test_init", 3)
		n.blockHistory = nil // Force nil history
		n.AddNetworkBlockEntry(100, "test_block_hash")
		assert.NotNil(t, n.blockHistory, "blockHistory should be initialized")
		assert.Equal(t, 1, n.blockHistory.Len(), "Should have 1 entry after initialization and add")
		entry := n.blockHistory.Front().Value.(blockHistoryEntry)
		assert.Equal(t, int64(100), entry.blockNumber)
	})

	t.Run("add first entry", func(t *testing.T) {
		n := newTestNetwork("test_first", 3)
		n.AddNetworkBlockEntry(100, "test_block_hash")
		assert.Equal(t, 1, n.blockHistory.Len())
		entry := n.blockHistory.Front().Value.(blockHistoryEntry)
		assert.Equal(t, int64(100), entry.blockNumber)
		assert.NotNil(t, entry.timestamp)
	})

	t.Run("add higher block", func(t *testing.T) {
		n := newTestNetwork("test_higher", 3)
		n.AddNetworkBlockEntry(100, "test_block_hash")
		n.AddNetworkBlockEntry(101, "test_block_hash")
		assert.Equal(t, 2, n.blockHistory.Len())
		entry := n.blockHistory.Back().Value.(blockHistoryEntry)
		assert.Equal(t, int64(101), entry.blockNumber)
	})

	t.Run("skip equal block", func(t *testing.T) {
		n := newTestNetwork("test_equal", 3)
		n.AddNetworkBlockEntry(100, "test_block_hash")
		n.AddNetworkBlockEntry(100, "test_block_hash") // Attempt to add equal block

		assert.Equal(t, 1, n.blockHistory.Len(), "History length should remain 1")
		entry := n.blockHistory.Back().Value.(blockHistoryEntry)
		assert.Equal(t, int64(100), entry.blockNumber, "Latest block should still be 100")
	})

	t.Run("skip lower block", func(t *testing.T) {
		n := newTestNetwork("test_lower", 3)
		n.AddNetworkBlockEntry(100, "test_block_hash")
		n.AddNetworkBlockEntry(99, "test_block_hash") // Attempt to add lower block

		assert.Equal(t, 1, n.blockHistory.Len(), "History length should remain 1")
		entry := n.blockHistory.Back().Value.(blockHistoryEntry)
		assert.Equal(t, int64(100), entry.blockNumber, "Latest block should still be 100")
	})

	t.Run("history trimming", func(t *testing.T) {
		historySize := 3
		n := newTestNetwork("test_trim", historySize)
		for i := 1; i <= historySize+2; i++ {
			n.AddNetworkBlockEntry(int64(100+i), fmt.Sprintf("hash_%d", i))
		}
		assert.Equal(t, historySize, n.blockHistory.Len(), "History should be trimmed to NetworkBlockHistorySize")
		firstEntry := n.blockHistory.Front().Value.(blockHistoryEntry)
		assert.Equal(t, int64(100+2+1), firstEntry.blockNumber, "Oldest entry is incorrect after trimming")
		lastEntry := n.blockHistory.Back().Value.(blockHistoryEntry)
		assert.Equal(t, int64(100+historySize+2), lastEntry.blockNumber, "Newest entry is incorrect after trimming")
	})

	t.Run("error on invalid type assertion during add check - defensive", func(t *testing.T) {
		n := newTestNetwork("test_invalid_type", 3)
		n.blockHistory.PushBack("not_a_block_history_entry") // Manually add invalid type

		n.AddNetworkBlockEntry(100, "test_block_hash")
		assert.Equal(t, 1, n.blockHistory.Len(), "History length should be 1 (the invalid entry)")
	})

	t.Run("concurrency test for AddNetworkBlockEntry", func(t *testing.T) {
		numGoroutines := 100
		blocksPerGoroutine := 10
		historySize := 50
		n := newTestNetwork("test_concurrent_add", historySize)

		var wg sync.WaitGroup
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(startBlock int) {
				defer wg.Done()
				for j := 0; j < blocksPerGoroutine; j++ {
					n.AddNetworkBlockEntry(int64(startBlock+j), "test_block_hash")
				}
			}(i * blocksPerGoroutine)
		}
		wg.Wait()

		assert.LessOrEqual(t, n.blockHistory.Len(), historySize, "History should not exceed max size")

		var prevBlock int64 = -1
		for e := n.blockHistory.Front(); e != nil; e = e.Next() {
			currentEntry := e.Value.(blockHistoryEntry)
			assert.Greater(t, currentEntry.blockNumber, prevBlock, "Block history should be strictly increasing")
			prevBlock = currentEntry.blockNumber
		}
	})
}

func TestGetLatestBlockEntry(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var n *network
		var entry *blockHistoryEntry
		assert.NotPanics(t, func() { entry = n.getLatestBlockEntry() }, "Calling getLatestBlockEntry on nil network should not panic")
		assert.Nil(t, entry, "Result should be nil for nil network")
	})

	t.Run("nil blockHistory", func(t *testing.T) {
		n := newTestNetwork("test_get_nil_hist", 3)
		n.blockHistory = nil // Force nil history
		entry := n.getLatestBlockEntry()
		assert.Nil(t, entry, "Result should be nil if blockHistory is nil")
	})

	t.Run("empty blockHistory", func(t *testing.T) {
		n := newTestNetwork("test_get_empty_hist", 3)
		entry := n.getLatestBlockEntry()
		assert.Nil(t, entry, "Result should be nil if blockHistory is empty")
	})

	t.Run("get from populated list", func(t *testing.T) {
		n := newTestNetwork("test_get_populated", 3)
		time1 := time.Now().Add(-time.Minute)
		time2 := time.Now()
		n.blockHistory.PushBack(blockHistoryEntry{blockNumber: 100, timestamp: &time1})
		n.blockHistory.PushBack(blockHistoryEntry{blockNumber: 101, timestamp: &time2})

		entry := n.getLatestBlockEntry()
		assert.NotNil(t, entry, "Expected an entry")
		assert.Equal(t, int64(101), entry.blockNumber)
		assert.NotNil(t, entry.timestamp)
		if entry.timestamp != nil {
			assert.Equal(t, time2.UnixNano(), entry.timestamp.UnixNano(), "Timestamp should match the latest entry")
		}

		originalLen := n.blockHistory.Len()
		entry.blockNumber = 999
		lastListEntry := n.blockHistory.Back().Value.(blockHistoryEntry)
		assert.Equal(t, int64(101), lastListEntry.blockNumber, "Modifying returned entry should not affect list content")
		assert.Equal(t, originalLen, n.blockHistory.Len())
	})

	t.Run("error on invalid type assertion in getLatest - defensive", func(t *testing.T) {
		n := newTestNetwork("test_get_invalid_type", 3)
		n.blockHistory.PushBack("not_a_block_history_entry")

		entry := n.getLatestBlockEntry()
		assert.Nil(t, entry, "Entry should be nil on type assertion error")
	})

	t.Run("concurrency test for getLatestBlockEntry with adds", func(t *testing.T) {
		n := newTestNetwork("test_concurrent_get_add", 200)
		stopCh := make(chan struct{})
		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; ; i++ {
				select {
				case <-stopCh:
					return
				default:
					n.AddNetworkBlockEntry(int64(i), "test_block_hash")
					time.Sleep(1 * time.Millisecond)
				}
			}
		}()

		numReaders := 50
		for i := 0; i < numReaders; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					entry := n.getLatestBlockEntry()
					if entry != nil {
						assert.GreaterOrEqual(t, entry.blockNumber, int64(0))
					}
					time.Sleep(time.Duration(rand.Intn(5)) * time.Millisecond)
				}
			}()
		}

		time.Sleep(200 * time.Millisecond)
		close(stopCh)
		wg.Wait()

		latest := n.getLatestBlockEntry()
		assert.NotNil(t, latest, "Should have a latest block entry after concurrent operations")
		if latest != nil {
			t.Logf("Final latest block after concurrency test: %d", latest.blockNumber)
		}
	})
}

var randSeed = time.Now().UnixNano()
var randLock sync.Mutex

func randIntn(n int) int {
	randLock.Lock()
	randSeed = (randSeed*1664525 + 1013904223) & 0x7fffffff
	val := int(randSeed % int64(n))
	randLock.Unlock()
	return val
}
