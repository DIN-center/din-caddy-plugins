package modules

import (
	"testing"
	"time"

	"errors"
	"fmt"
	"strings"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
	prom "github.com/DIN-center/din-caddy-plugins/lib/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// Mock implementations
type MockHTTPClient struct {
	mock.Mock
}

func (m *MockHTTPClient) Post(url string, headers map[string]string, payload []byte, ac auth.IAuthClient) ([]byte, *int, error) {
	args := m.Called(url, headers, payload, ac)
	statusCode := args.Get(1).(int)
	// Always return a non-nil status code pointer
	return args.Get(0).([]byte), &statusCode, args.Error(2)
}

type MockPrometheusClient struct {
	mock.Mock
}

func (m *MockPrometheusClient) HandleLatestBlockMetric(data *prom.PromLatestBlockMetricData) {
	m.Called(data)
}

func (m *MockPrometheusClient) HandleRequestMetrics(data *prom.PromRequestMetricData, body []byte, duration time.Duration) {
	m.Called(data, body, duration)
}

func TestNewNetwork(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *network
	}{
		{
			name:  "creates network with default values",
			input: "ethereum",
			expected: &network{
				Name:                    "ethereum",
				HCMethod:                DefaultHCMethod,
				ChainIdMethod:           DefaultChainIdMethod,
				HCThreshold:             DefaultHCThreshold,
				HCInterval:              DefaultHCInterval,
				BlockLagLimit:           DefaultBlockLagLimit,
				BlockJumpLimit:          DefaultBlockJumpLimit,
				MaxRequestPayloadSizeKB: DefaultMaxRequestPayloadSizeKB,
				RequestAttemptCount:     DefaultRequestAttemptCount,
				BlockHistorySize:        BlockHistorySize,
				Providers:               make(map[string]*provider),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewNetwork(tt.input)
			assert.Equal(t, tt.expected.Name, result.Name)
			assert.Equal(t, tt.expected.HCMethod, result.HCMethod)
			assert.Equal(t, tt.expected.ChainIdMethod, result.ChainIdMethod)
			assert.Equal(t, tt.expected.HCThreshold, result.HCThreshold)
			assert.Equal(t, tt.expected.HCInterval, result.HCInterval)
			assert.Equal(t, tt.expected.BlockLagLimit, result.BlockLagLimit)
			assert.Equal(t, tt.expected.BlockJumpLimit, result.BlockJumpLimit)
			assert.Equal(t, tt.expected.MaxRequestPayloadSizeKB, result.MaxRequestPayloadSizeKB)
			assert.Equal(t, tt.expected.RequestAttemptCount, result.RequestAttemptCount)
			assert.Equal(t, tt.expected.BlockHistorySize, result.BlockHistorySize)
			assert.NotNil(t, result.Providers)
			assert.Len(t, result.Providers, 0)
		})
	}
}

func TestEvaluateProviderHealth(t *testing.T) {
	now := time.Now()
	twoSecondsAgo := now.Add(-2 * time.Second)
	oneSecondAgo := now.Add(-1 * time.Second)

	tests := []struct {
		name               string
		currentBlock       int64
		latestNetworkBlock int64
		initialHealth      HealthStatus
		provider           *provider
		blockHistory       []blockHistoryEntry
		chainId            string
		networkChainId     string
		networkName        string
		archiveEnabled     bool
		mockArchiveTest    error // New field to mock archive test result
		expectedStatus     HealthStatus
	}{
		{
			name:               "healthy provider within limits",
			currentBlock:       100,
			latestNetworkBlock: 101,
			initialHealth:      Healthy,
			provider: &provider{
				host: "test.com",
			},
			blockHistory: []blockHistoryEntry{
				{blockNumber: 98, statusCode: Healthy, timestamp: &twoSecondsAgo},
				{blockNumber: 99, statusCode: Healthy, timestamp: &oneSecondAgo},
				{blockNumber: 100, statusCode: Healthy, timestamp: &now},
			},
			chainId:        "1",
			networkChainId: "1",
			networkName:    "ethereum",
			expectedStatus: Healthy,
		},
		{
			name:               "provider lagging behind",
			currentBlock:       90,
			latestNetworkBlock: 100,
			initialHealth:      Healthy,
			provider: &provider{
				host: "test.com",
			},
			blockHistory: []blockHistoryEntry{
				{blockNumber: 88, statusCode: Healthy, timestamp: &twoSecondsAgo},
				{blockNumber: 89, statusCode: Healthy, timestamp: &oneSecondAgo},
				{blockNumber: 90, statusCode: Healthy, timestamp: &now},
			},
			chainId:        "1",
			networkChainId: "1",
			networkName:    "ethereum",
			expectedStatus: Warning,
		},
		{
			name:               "archive check passes",
			currentBlock:       100,
			latestNetworkBlock: 101,
			initialHealth:      Healthy,
			provider: &provider{
				host:    "test.com",
				HttpUrl: "http://test.com",
			},
			blockHistory: []blockHistoryEntry{
				{blockNumber: 98, statusCode: Healthy, timestamp: &twoSecondsAgo},
				{blockNumber: 99, statusCode: Healthy, timestamp: &oneSecondAgo},
				{blockNumber: 100, statusCode: Healthy, timestamp: &now},
			},
			chainId:         "1",
			networkChainId:  "1",
			networkName:     "ethereum",
			archiveEnabled:  true,
			mockArchiveTest: nil, // No error means test passes
			expectedStatus:  Healthy,
		},
		{
			name:               "archive check fails",
			currentBlock:       100,
			latestNetworkBlock: 101,
			initialHealth:      Healthy,
			provider: &provider{
				host:    "test.com",
				HttpUrl: "http://test.com",
			},
			blockHistory: []blockHistoryEntry{
				{blockNumber: 98, statusCode: Healthy, timestamp: &twoSecondsAgo},
				{blockNumber: 99, statusCode: Healthy, timestamp: &oneSecondAgo},
				{blockNumber: 100, statusCode: Healthy, timestamp: &now},
			},
			chainId:         "1",
			networkChainId:  "1",
			networkName:     "ethereum",
			archiveEnabled:  true,
			mockArchiveTest: errors.New("network doesn't support archive mode"),
			expectedStatus:  Unhealthy,
		},
		{
			name:               "bitcoin skips archive check",
			currentBlock:       100,
			latestNetworkBlock: 101,
			initialHealth:      Healthy,
			provider: &provider{
				host:    "test.com",
				HttpUrl: "http://test.com",
			},
			blockHistory: []blockHistoryEntry{
				{blockNumber: 98, statusCode: Healthy, timestamp: &twoSecondsAgo},
				{blockNumber: 99, statusCode: Healthy, timestamp: &oneSecondAgo},
				{blockNumber: 100, statusCode: Healthy, timestamp: &now},
			},
			chainId:         "main",
			networkChainId:  "main",
			networkName:     "bitcoin",
			archiveEnabled:  true,
			mockArchiveTest: errors.New("should not be called"),
			expectedStatus:  Healthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := NewNetwork(tt.networkName)
			n.ChainId = tt.networkChainId
			n.BlockLagLimit = 5
			n.BlockJumpLimit = 50
			n.BlockHistorySize = 3
			n.ArchiveEnabled = tt.archiveEnabled
			n.CallContractMethod = "eth_call"
			n.logger, _ = zap.NewDevelopment()

			// Mock HTTP client for archive testing
			mockHTTP := new(MockHTTPClient)

			// Set up mock for archive test
			if tt.archiveEnabled && !strings.Contains(tt.networkName, "bitcoin") && !strings.Contains(tt.networkName, "solana") {
				mockHTTP.On("Post",
					mock.Anything, // URL
					mock.Anything, // headers
					mock.MatchedBy(func(payload []byte) bool {
						// Normalize both expected and actual JSON by removing whitespace
						expectedJSON := strings.ReplaceAll(
							`{"jsonrpc":"2.0","method":"eth_call","id":1,"params":[{"input":"0x436000526004601cf3"},"0x19"]}`,
							" ", "",
						)
						actualJSON := strings.ReplaceAll(string(payload), " ", "")

						t.Logf("Expected (normalized): %s", expectedJSON)
						t.Logf("Actual (normalized): %s", actualJSON)

						return actualJSON == expectedJSON
					}),
					mock.Anything, // auth client
				).Return([]byte(`{}`), 200, tt.mockArchiveTest)
			}

			n.HttpClient = mockHTTP
			tt.provider.blockHistory = tt.blockHistory
			tt.provider.chainId = tt.chainId

			result := n.evaluateProviderHealth(tt.provider, tt.currentBlock, tt.initialHealth, tt.latestNetworkBlock)
			assert.Equal(t, tt.expectedStatus, result)

			// Verify all expected calls were made
			mockHTTP.AssertExpectations(t)
		})
	}
}

func TestGetLatestBlockNumber(t *testing.T) {
	tests := []struct {
		name           string
		httpResponse   []byte
		statusCode     int
		httpError      error
		expectedBlock  int64
		expectedHealth HealthStatus
		expectedError  bool
	}{
		{
			name:           "successful hex response",
			httpResponse:   []byte(`{"jsonrpc":"2.0","result":"0x1234"}`),
			statusCode:     200,
			httpError:      nil,
			expectedBlock:  0x1234,
			expectedHealth: Healthy,
			expectedError:  false,
		},
		{
			name:           "successful decimal response",
			httpResponse:   []byte(`{"jsonrpc":"2.0","result":1234}`),
			statusCode:     200,
			httpError:      nil,
			expectedBlock:  1234,
			expectedHealth: Healthy,
			expectedError:  false,
		},
		{
			name:           "rate limit response",
			httpResponse:   []byte(`{}`),
			statusCode:     429,
			httpError:      nil,
			expectedBlock:  0,
			expectedHealth: Warning,
			expectedError:  true,
		},
		// Add more test cases
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockHTTP := new(MockHTTPClient)
			mockHTTP.On("Post", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(tt.httpResponse, tt.statusCode, tt.httpError)

			n := NewNetwork("test")
			n.HttpClient = mockHTTP

			block, health, err := n.getLatestBlockNumber("http://test.com", nil, nil)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedBlock, block)
			assert.Equal(t, tt.expectedHealth, health)
		})
	}
}

func TestIsStalled(t *testing.T) {
	now := time.Now()
	twoSecondsAgo := now.Add(-2 * time.Second)
	oneSecondAgo := now.Add(-1 * time.Second)

	tests := []struct {
		name         string
		blockHistory []blockHistoryEntry
		expected     bool
	}{
		{
			name: "not stalled - increasing blocks",
			blockHistory: []blockHistoryEntry{
				{blockNumber: 100, statusCode: Healthy, timestamp: &twoSecondsAgo},
				{blockNumber: 101, statusCode: Healthy, timestamp: &oneSecondAgo},
				{blockNumber: 102, statusCode: Healthy, timestamp: &now},
			},
			expected: false,
		},
		{
			name: "stalled - same block number",
			blockHistory: []blockHistoryEntry{
				{blockNumber: 100, statusCode: Healthy, timestamp: &twoSecondsAgo},
				{blockNumber: 100, statusCode: Healthy, timestamp: &oneSecondAgo},
				{blockNumber: 100, statusCode: Healthy, timestamp: &now},
			},
			expected: true,
		},
		{
			name: "not stalled - insufficient history",
			blockHistory: []blockHistoryEntry{
				{blockNumber: 100, statusCode: Healthy, timestamp: &oneSecondAgo},
				{blockNumber: 100, statusCode: Healthy, timestamp: &now},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := NewNetwork("test")
			n.BlockHistorySize = 3
			p := &provider{}
			p.blockHistory = tt.blockHistory

			result := n.isStalled(p)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHandleErrorWithGracePeriod(t *testing.T) {
	tests := []struct {
		name                       string
		initialHealthStatus        HealthStatus
		consecutiveUnhealthyChecks int
		healthCheckThreshold       int
		expectedHealthStatus       HealthStatus
		expectedUnhealthyChecks    int
		blockNum                   int64
	}{
		{
			name:                       "first unhealthy check",
			initialHealthStatus:        Unhealthy,
			consecutiveUnhealthyChecks: 0,
			healthCheckThreshold:       3,
			expectedHealthStatus:       Warning,
			expectedUnhealthyChecks:    1,
			blockNum:                   100,
		},
		{
			name:                       "second unhealthy check",
			initialHealthStatus:        Unhealthy,
			consecutiveUnhealthyChecks: 1,
			healthCheckThreshold:       3,
			expectedHealthStatus:       Warning,
			expectedUnhealthyChecks:    2,
			blockNum:                   100,
		},
		{
			name:                       "exceeds threshold",
			initialHealthStatus:        Unhealthy,
			consecutiveUnhealthyChecks: 3,
			healthCheckThreshold:       3,
			expectedHealthStatus:       Unhealthy,
			expectedUnhealthyChecks:    4,
			blockNum:                   100,
		},
		{
			name:                       "warning status resets counter",
			initialHealthStatus:        Warning,
			consecutiveUnhealthyChecks: 2,
			healthCheckThreshold:       3,
			expectedHealthStatus:       Warning,
			expectedUnhealthyChecks:    0,
			blockNum:                   100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := NewNetwork("test")
			n.HCThreshold = tt.healthCheckThreshold
			n.logger, _ = zap.NewDevelopment()

			mockProm := new(MockPrometheusClient)
			mockProm.On("HandleLatestBlockMetric", mock.Anything).Return()
			n.PrometheusClient = mockProm

			p := &provider{
				host:                       "test.com",
				consecutiveUnhealthyChecks: tt.consecutiveUnhealthyChecks,
			}

			result := n.handleErrorWithGracePeriod(p, tt.initialHealthStatus, tt.blockNum)

			assert.Equal(t, tt.expectedHealthStatus, result)
			assert.Equal(t, tt.expectedUnhealthyChecks, p.consecutiveUnhealthyChecks)
		})
	}
}

func TestVerifyChainID(t *testing.T) {
	tests := []struct {
		name            string
		providerChainId string
		networkChainId  string
		expected        bool
	}{
		{
			name:            "matching chain IDs",
			providerChainId: "1",
			networkChainId:  "1",
			expected:        true,
		},
		{
			name:            "mismatched chain IDs",
			providerChainId: "1",
			networkChainId:  "2",
			expected:        false,
		},
		{
			name:            "empty provider chain ID",
			providerChainId: "",
			networkChainId:  "1",
			expected:        false,
		},
		{
			name:            "empty network chain ID",
			providerChainId: "1",
			networkChainId:  "",
			expected:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := NewNetwork("test")
			n.ChainId = tt.networkChainId
			p := &provider{chainId: tt.providerChainId}

			result := n.verifyChainID(p)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAllProvidersStalled(t *testing.T) {
	now := time.Now()
	twoSecondsAgo := now.Add(-2 * time.Second)
	oneSecondAgo := now.Add(-1 * time.Second)

	tests := []struct {
		name      string
		providers map[string]*provider
		expected  bool
	}{
		{
			name: "all providers stalled",
			providers: map[string]*provider{
				"provider1": {
					blockHistory: []blockHistoryEntry{
						{blockNumber: 100, statusCode: Healthy, timestamp: &twoSecondsAgo},
						{blockNumber: 100, statusCode: Healthy, timestamp: &oneSecondAgo},
						{blockNumber: 100, statusCode: Healthy, timestamp: &now},
					},
				},
				"provider2": {
					blockHistory: []blockHistoryEntry{
						{blockNumber: 200, statusCode: Healthy, timestamp: &twoSecondsAgo},
						{blockNumber: 200, statusCode: Healthy, timestamp: &oneSecondAgo},
						{blockNumber: 200, statusCode: Healthy, timestamp: &now},
					},
				},
			},
			expected: true,
		},
		{
			name: "one provider progressing",
			providers: map[string]*provider{
				"provider1": {
					blockHistory: []blockHistoryEntry{
						{blockNumber: 100, statusCode: Healthy, timestamp: &twoSecondsAgo},
						{blockNumber: 100, statusCode: Healthy, timestamp: &oneSecondAgo},
						{blockNumber: 100, statusCode: Healthy, timestamp: &now},
					},
				},
				"provider2": {
					blockHistory: []blockHistoryEntry{
						{blockNumber: 200, statusCode: Healthy, timestamp: &twoSecondsAgo},
						{blockNumber: 201, statusCode: Healthy, timestamp: &oneSecondAgo},
						{blockNumber: 202, statusCode: Healthy, timestamp: &now},
					},
				},
			},
			expected: false,
		},
		{
			name: "insufficient history",
			providers: map[string]*provider{
				"provider1": {
					blockHistory: []blockHistoryEntry{
						{blockNumber: 100, statusCode: Healthy, timestamp: &oneSecondAgo},
						{blockNumber: 100, statusCode: Healthy, timestamp: &now},
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := NewNetwork("test")
			n.BlockHistorySize = 3
			n.Providers = tt.providers

			result := n.allProvidersStalled()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetLatestHealthyBlock(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		providers map[string]*provider
		expected  int64
	}{
		{
			name: "healthy providers available",
			providers: map[string]*provider{
				"provider1": {
					healthStatus: Healthy,
					blockHistory: []blockHistoryEntry{
						{blockNumber: 100, statusCode: Healthy, timestamp: &now},
					},
				},
				"provider2": {
					healthStatus: Healthy,
					blockHistory: []blockHistoryEntry{
						{blockNumber: 102, statusCode: Healthy, timestamp: &now},
					},
				},
			},
			expected: 102,
		},
		{
			name: "only warning providers",
			providers: map[string]*provider{
				"provider1": {
					healthStatus: Warning,
					blockHistory: []blockHistoryEntry{
						{blockNumber: 100, statusCode: Healthy, timestamp: &now},
					},
				},
				"provider2": {
					healthStatus: Warning,
					blockHistory: []blockHistoryEntry{
						{blockNumber: 102, statusCode: Healthy, timestamp: &now},
					},
				},
			},
			expected: 102,
		},
		{
			name: "mix of health statuses",
			providers: map[string]*provider{
				"provider1": {
					healthStatus: Healthy,
					blockHistory: []blockHistoryEntry{
						{blockNumber: 100, statusCode: Healthy, timestamp: &now},
					},
				},
				"provider2": {
					healthStatus: Warning,
					blockHistory: []blockHistoryEntry{
						{blockNumber: 102, statusCode: Healthy, timestamp: &now},
					},
				},
				"provider3": {
					healthStatus: Unhealthy,
					blockHistory: []blockHistoryEntry{
						{blockNumber: 105, statusCode: Healthy, timestamp: &now},
					},
				},
			},
			expected: 100,
		},
		{
			name:      "no providers",
			providers: map[string]*provider{},
			expected:  0,
		},
		{
			name: "providers with no history",
			providers: map[string]*provider{
				"provider1": {
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

func TestStartHealthcheck(t *testing.T) {
	tests := []struct {
		name        string
		hcInterval  int
		expectQuit  bool
		runDuration time.Duration
	}{
		{
			name:        "normal operation",
			hcInterval:  1,
			expectQuit:  false,
			runDuration: 2 * time.Second,
		},
		{
			name:        "quit channel closed",
			hcInterval:  1,
			expectQuit:  true,
			runDuration: 100 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := NewNetwork("test")
			n.HCInterval = tt.hcInterval
			n.quit = make(chan struct{})
			n.logger, _ = zap.NewDevelopment()

			mockHTTP := new(MockHTTPClient)
			mockHTTP.On("Post", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return([]byte(`{"result":"0x1234"}`), 200, nil)
			n.HttpClient = mockHTTP

			mockProm := new(MockPrometheusClient)
			mockProm.On("HandleLatestBlockMetric", mock.Anything).Return()
			n.PrometheusClient = mockProm

			go n.startHealthcheck()

			if tt.expectQuit {
				close(n.quit)
			}

			time.Sleep(tt.runDuration)

			if !tt.expectQuit {
				n.close()
			}
		})
	}
}

func TestGetChainID(t *testing.T) {
	tests := []struct {
		name            string
		httpResponse    []byte
		statusCode      int
		httpError       error
		expectedChainID string
		expectedError   bool
		networkName     string
		expectedStatus  int
	}{
		{
			name:            "successful ethereum response",
			httpResponse:    []byte(`{"jsonrpc":"2.0","result":"0x1"}`),
			statusCode:      200,
			httpError:       nil,
			expectedChainID: "0x1",
			expectedError:   false,
			networkName:     "ethereum",
			expectedStatus:  200,
		},
		{
			name:            "successful bitcoin response",
			httpResponse:    []byte(`{"result":{"chain":"main"}}`),
			statusCode:      200,
			httpError:       nil,
			expectedChainID: "main",
			expectedError:   false,
			networkName:     "bitcoin",
			expectedStatus:  200,
		},
		{
			name:            "network unavailable",
			httpResponse:    []byte(``),
			statusCode:      503,
			httpError:       nil,
			expectedChainID: "",
			expectedError:   true,
			networkName:     "ethereum",
			expectedStatus:  503,
		},
		{
			name:            "invalid response format",
			httpResponse:    []byte(`{"result":123}`),
			statusCode:      200,
			httpError:       nil,
			expectedChainID: "",
			expectedError:   true,
			networkName:     "ethereum",
			expectedStatus:  200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockHTTP := new(MockHTTPClient)
			// Always return the status code from the test case
			mockHTTP.On("Post", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(tt.httpResponse, tt.statusCode, tt.httpError)

			n := NewNetwork(tt.networkName)
			n.HttpClient = mockHTTP

			chainID, statusCode, err := n.getChainID("http://test.com", nil, nil)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedChainID, chainID)
			}
			assert.Equal(t, tt.expectedStatus, statusCode)
		})
	}
}

func TestTestArchiveMode(t *testing.T) {
	tests := []struct {
		name               string
		httpResponse       []byte
		statusCode         int
		httpError          error
		quarterBlockHeight string
		expectedError      string
	}{
		{
			name:               "successful archive test",
			httpResponse:       []byte(`{"jsonrpc":"2.0","result":"0x1234"}`),
			statusCode:         200,
			httpError:          nil,
			quarterBlockHeight: "0x1234",
			expectedError:      "",
		},
		{
			name:               "network unavailable",
			httpResponse:       []byte{},
			statusCode:         503,
			httpError:          nil,
			quarterBlockHeight: "0x1234",
			expectedError:      "Network Unavailable",
		},
		{
			name:               "invalid JSON response",
			httpResponse:       []byte(`invalid json`),
			statusCode:         200,
			httpError:          nil,
			quarterBlockHeight: "0x1234",
			expectedError:      "Error unmarshalling response",
		},
		{
			name:               "network doesn't support archive",
			httpResponse:       []byte(`{"error":{"code":-32000,"message":"missing trie node"}}`),
			statusCode:         200,
			httpError:          nil,
			quarterBlockHeight: "0x1234",
			expectedError:      "network doesn't support archive mode",
		},
		{
			name:               "http client error",
			httpResponse:       []byte{},
			statusCode:         200,
			httpError:          errors.New("connection failed"),
			quarterBlockHeight: "0x1234",
			expectedError:      "Error sending POST request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockHTTP := new(MockHTTPClient)

			// Set up mock with exact payload matching
			mockHTTP.On("Post",
				mock.MatchedBy(func(url string) bool {
					return url == "http://test.com"
				}),
				mock.MatchedBy(func(headers map[string]string) bool {
					return headers == nil
				}),
				mock.MatchedBy(func(payload []byte) bool {
					// Remove all whitespace from both expected and actual payloads
					expected := strings.ReplaceAll(
						fmt.Sprintf(`{"jsonrpc":"2.0","method":"%s","id":1,"params":[{"input":"0x436000526004601cf3"},"%s"]}`,
							"eth_call",
							tt.quarterBlockHeight,
						),
						" ", "",
					)
					actual := strings.ReplaceAll(string(payload), " ", "")

					if expected != actual {
						t.Logf("Expected payload: %s", expected)
						t.Logf("Actual payload: %s", actual)
					}
					return expected == actual
				}),
				mock.Anything,
			).Return(tt.httpResponse, tt.statusCode, tt.httpError)

			n := NewNetwork("test")
			n.HttpClient = mockHTTP
			n.CallContractMethod = "eth_call"

			err := n.testArchiveMode("http://test.com", nil, nil, tt.quarterBlockHeight)

			if tt.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			}

			mockHTTP.AssertExpectations(t)
		})
	}
}
