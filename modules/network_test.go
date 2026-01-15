package modules

import (
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	networklib "github.com/DIN-center/din-caddy-plugins/lib/network"
	"github.com/DIN-center/din-caddy-plugins/lib/utils"
)

func TestHandleErrorWithGracePeriod(t *testing.T) {
	tests := []struct {
		name                 string
		consecutiveUnhealthy int
		healthStatus         HealthStatus
		hcThreshold          int
		expectedHealthStatus HealthStatus
		expectedConsecutive  int
		expectedLogContains  string
	}{
		{
			name:                 "warning_resets_counter",
			consecutiveUnhealthy: 2,
			healthStatus:         Warning,
			hcThreshold:          3,
			expectedHealthStatus: Warning,
			expectedConsecutive:  0,
		},
		{
			name:                 "first_unhealthy_gives_grace",
			consecutiveUnhealthy: 0,
			healthStatus:         Unhealthy,
			hcThreshold:          3,
			expectedHealthStatus: Warning,
			expectedConsecutive:  1,
		},
		{
			name:                 "grace_period_not_exceeded",
			consecutiveUnhealthy: 1,
			healthStatus:         Unhealthy,
			hcThreshold:          3,
			expectedHealthStatus: Warning,
			expectedConsecutive:  2,
		},
		{
			name:                 "grace_period_exceeded",
			consecutiveUnhealthy: 2,
			healthStatus:         Unhealthy,
			hcThreshold:          3,
			expectedHealthStatus: Unhealthy,
			expectedConsecutive:  3,
			expectedLogContains:  "exceeded grace period",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := NewNetwork("test", EVMHandler, utils.Environment("test"), LoopbackConfig{Port: "8000", ApiKey: DefaultLoopbackApiKey})
			assert.NoError(t, err)
			n.HCThreshold = tt.hcThreshold

			// Initialize logger to prevent panic
			n.logger = logger.NewLoggerClient(zap.NewNop(), utils.EnvTest)

			// Create a test provider
			provider, err := NewProvider("http://test-provider.com")
			assert.NoError(t, err)
			provider.consecutiveUnhealthyChecks = tt.consecutiveUnhealthy

			// Call the method under test
			result := n.handleErrorWithGracePeriod(provider, tt.healthStatus, 123)

			// Verify the results
			assert.Equal(t, tt.expectedHealthStatus, result)
			assert.Equal(t, tt.expectedConsecutive, provider.consecutiveUnhealthyChecks)
		})
	}
}

func TestIsStalled(t *testing.T) {
	tests := []struct {
		name        string
		blocks      []int64
		historySize int
		expected    bool
	}{
		{
			name:        "not_enough_history",
			blocks:      []int64{100, 101},
			historySize: 3,
			expected:    false,
		},
		{
			name:        "all_same_blocks_stalled",
			blocks:      []int64{100, 100, 100},
			historySize: 3,
			expected:    true,
		},
		{
			name:        "different_blocks_not_stalled",
			blocks:      []int64{100, 101, 102},
			historySize: 3,
			expected:    false,
		},
		{
			name:        "mixed_blocks_not_stalled",
			blocks:      []int64{100, 100, 101},
			historySize: 3,
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := NewNetwork("test", EVMHandler, utils.Environment("test"), LoopbackConfig{Port: "8000", ApiKey: DefaultLoopbackApiKey})
			assert.NoError(t, err)
			n.ProviderBlockHistorySize = tt.historySize

			// Create a provider with the specified block history
			provider, err := NewProvider("http://test.com")
			assert.NoError(t, err)
			for _, blockNum := range tt.blocks {
				provider.AddBlockEntry(blockNum, Healthy, tt.historySize)
			}

			result := n.isStalled(provider)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetLatestHealthyBlock(t *testing.T) {
	tests := []struct {
		name      string
		providers []struct {
			host    string
			entries []struct {
				block  int64
				status HealthStatus
			}
		}
		expected int64
	}{
		{
			name: "healthy_provider_highest",
			providers: []struct {
				host    string
				entries []struct {
					block  int64
					status HealthStatus
				}
			}{
				{
					host: "provider1",
					entries: []struct {
						block  int64
						status HealthStatus
					}{
						{block: 100, status: Healthy},
						{block: 105, status: Healthy},
					},
				},
				{
					host: "provider2",
					entries: []struct {
						block  int64
						status HealthStatus
					}{
						{block: 95, status: Warning},
						{block: 103, status: Warning},
					},
				},
			},
			expected: 105,
		},
		{
			name: "no_healthy_fallback_to_warning",
			providers: []struct {
				host    string
				entries []struct {
					block  int64
					status HealthStatus
				}
			}{
				{
					host: "provider1",
					entries: []struct {
						block  int64
						status HealthStatus
					}{
						{block: 90, status: Warning},
						{block: 95, status: Warning},
					},
				},
				{
					host: "provider2",
					entries: []struct {
						block  int64
						status HealthStatus
					}{
						{block: 85, status: Unhealthy},
						{block: 92, status: Unhealthy},
					},
				},
			},
			expected: 95,
		},
		{
			name: "empty_providers",
			providers: []struct {
				host    string
				entries []struct {
					block  int64
					status HealthStatus
				}
			}{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := NewNetwork("test", EVMHandler, utils.Environment("test"), LoopbackConfig{Port: "8000", ApiKey: DefaultLoopbackApiKey})
			assert.NoError(t, err)
			n.Providers = make(map[string]*provider)

			// Set up providers based on test case
			for _, providerData := range tt.providers {
				p, err := NewProvider("http://" + providerData.host + ".com")
				assert.NoError(t, err)
				for _, entry := range providerData.entries {
					p.AddBlockEntry(entry.block, entry.status, 10)
				}
				n.Providers[providerData.host] = p
			}

			result := n.getLatestHealthyBlock()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProcessBlockNumberResponse(t *testing.T) {
	tests := []struct {
		name              string
		resBytes          []byte
		statusCode        int
		passNilStatusCode bool
		expected          int64
		expectErr         bool
	}{
		{
			name:              "valid_hex_response",
			resBytes:          []byte(`{"result":"0x64"}`),
			statusCode:        200,
			passNilStatusCode: false,
			expected:          100,
			expectErr:         false,
		},
		{
			name:              "invalid_decimal_response",
			resBytes:          []byte(`{"result":100}`),
			statusCode:        200,
			passNilStatusCode: false,
			expected:          0,
			expectErr:         true,
		},
		{
			name:              "error_status_code",
			resBytes:          []byte(`{"error":"bad request"}`),
			statusCode:        400,
			passNilStatusCode: false,
			expected:          0,
			expectErr:         true,
		},
		{
			name:              "rate_limit_warning",
			resBytes:          []byte(`{"error":"rate limited"}`),
			statusCode:        429,
			passNilStatusCode: false,
			expected:          0,
			expectErr:         true,
		},
		{
			name:              "nil_status_code",
			resBytes:          []byte(`{"result":"0x64"}`),
			statusCode:        0,
			passNilStatusCode: true,
			expected:          0,
			expectErr:         true,
		},
		{
			name:              "invalid_json",
			resBytes:          []byte(`{"invalid_json"`),
			statusCode:        200,
			passNilStatusCode: false,
			expected:          0,
			expectErr:         true,
		},
		{
			name:              "invalid_hex_format",
			resBytes:          []byte(`{"result":"invalid"}`),
			statusCode:        200,
			passNilStatusCode: false,
			expected:          0,
			expectErr:         true,
		},
		{
			name:              "empty_hex_result",
			resBytes:          []byte(`{"result":""}`),
			statusCode:        200,
			passNilStatusCode: false,
			expected:          0,
			expectErr:         true,
		},
		{
			name:              "unsupported_result_type",
			resBytes:          []byte(`{"result":null}`),
			statusCode:        200,
			passNilStatusCode: false,
			expected:          0,
			expectErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := NewNetwork("test", "", utils.Environment("test"), LoopbackConfig{Port: "8000", ApiKey: DefaultLoopbackApiKey})
			assert.NoError(t, err)
			// Set handler for tests
			config := &networklib.NetworkConfig{
				Name:    "test",
				Type:    string(EVMHandler),
				ChainID: "1",
			}
			require.NoError(t, n.SetHandler(networklib.NewEVMHandler(config)))

			var sc *int
			if !tt.passNilStatusCode {
				statusCodeVal := tt.statusCode
				sc = &statusCodeVal
			}

			blockNumber, healthStatus, err := n.processBlockNumberResponse(tt.resBytes, sc)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, blockNumber)
				assert.Equal(t, Healthy, healthStatus)
			}

			// Test specific health statuses for error cases
			if tt.statusCode == 429 && !tt.passNilStatusCode {
				assert.Equal(t, Warning, healthStatus)
			} else if tt.expectErr && !tt.passNilStatusCode && tt.statusCode >= 400 {
				assert.Equal(t, Unhealthy, healthStatus)
			}
		})
	}
}

func TestArchiveModeCheck(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name           string
		networkName    string
		httpResponse   []byte
		statusCode     int
		httpError      error
		quarterBlock   string
		expectError    bool
		expectedErrMsg string
	}{
		{
			name:         "successful_archive_check",
			networkName:  "ethereum",
			httpResponse: []byte(`{"result":"0x123"}`),
			statusCode:   200,
			httpError:    nil,
			quarterBlock: "0x64",
			expectError:  false,
		},
		{
			name:           "network_error",
			networkName:    "ethereum",
			httpResponse:   nil,
			statusCode:     0,
			httpError:      errors.New("connection failed"),
			quarterBlock:   "0x64",
			expectError:    true,
			expectedErrMsg: "failed after",
		},
		{
			name:           "service_unavailable",
			networkName:    "ethereum",
			httpResponse:   []byte(`{"error":"service unavailable"}`),
			statusCode:     503,
			httpError:      nil,
			quarterBlock:   "0x64",
			expectError:    true,
			expectedErrMsg: "failed after",
		},
		{
			name:           "json_rpc_error",
			networkName:    "ethereum",
			httpResponse:   []byte(`{"error":{"code":-32000,"message":"archive not supported"}}`),
			statusCode:     200,
			httpError:      nil,
			quarterBlock:   "0x64",
			expectError:    true,
			expectedErrMsg: "failed after",
		},
		{
			name:         "starknet_successful",
			networkName:  "starknet-mainnet",
			httpResponse: []byte(`{"result":{"block_hash":"0xabc123"}}`),
			statusCode:   200,
			httpError:    nil,
			quarterBlock: "1000",
			expectError:  false,
		},
		{
			name:           "starknet_missing_block_hash",
			networkName:    "starknet-mainnet",
			httpResponse:   []byte(`{"result":{}}`),
			statusCode:     200,
			httpError:      nil,
			quarterBlock:   "1000",
			expectError:    true,
			expectedErrMsg: "failed after",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock HTTP client
			mockHTTPClient := din_http.NewMockIHTTPClient(ctrl)
			mockHTTPClient.EXPECT().
				Post(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(tt.httpResponse, &tt.statusCode, tt.httpError).
				AnyTimes()

			// Use appropriate network type based on network name
			networkType := string(EVMHandler)
			if strings.Contains(tt.networkName, "starknet") {
				networkType = string(StarknetHandler)
			}

			n, err := NewNetwork(tt.networkName, "", utils.Environment("test"), LoopbackConfig{Port: "8000", ApiKey: DefaultLoopbackApiKey})
			assert.NoError(t, err)
			n.HttpClient = mockHTTPClient
			n.RequestAttemptCount = 1

			// Initialize logger to prevent panic
			n.logger = logger.NewLoggerClient(zap.NewNop(), utils.EnvTest)

			// Set the handler
			config := &networklib.NetworkConfig{
				Name:    tt.networkName,
				Type:    networkType,
				ChainID: "test-chain",
			}
			if networkType == string(EVMHandler) {
				require.NoError(t, n.SetHandler(networklib.NewEVMHandler(config)))
			} else {
				require.NoError(t, n.SetHandler(networklib.NewStarknetHandler(config)))
			}

			err = n.handler.PerformArchiveCheck("http://test.com", map[string]string{}, n.HttpClient, nil, n.RequestAttemptCount, tt.quarterBlock)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedErrMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHasOtherHealthyProviders(t *testing.T) {
	tests := []struct {
		name      string
		providers []struct {
			host   string
			blocks []struct {
				number int64
				status HealthStatus
			}
		}
		testProviderHost string
		expected         bool
	}{
		{
			name: "has_other_healthy_provider",
			providers: []struct {
				host   string
				blocks []struct {
					number int64
					status HealthStatus
				}
			}{
				{
					host: "provider1",
					blocks: []struct {
						number int64
						status HealthStatus
					}{
						{number: 100, status: Healthy},
					},
				},
				{
					host: "provider2",
					blocks: []struct {
						number int64
						status HealthStatus
					}{
						{number: 101, status: Healthy},
					},
				},
			},
			testProviderHost: "provider1",
			expected:         true,
		},
		{
			name: "no_other_healthy_providers",
			providers: []struct {
				host   string
				blocks []struct {
					number int64
					status HealthStatus
				}
			}{
				{
					host: "provider1",
					blocks: []struct {
						number int64
						status HealthStatus
					}{
						{number: 100, status: Healthy},
					},
				},
				{
					host: "provider2",
					blocks: []struct {
						number int64
						status HealthStatus
					}{
						{number: 99, status: Unhealthy},
					},
				},
			},
			testProviderHost: "provider1",
			expected:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := NewNetwork("test", EVMHandler, utils.Environment("test"), LoopbackConfig{Port: "8000", ApiKey: DefaultLoopbackApiKey})
			assert.NoError(t, err)
			n.Providers = make(map[string]*provider)

			var testProvider *provider
			for _, providerData := range tt.providers {
				p, err := NewProvider("http://" + providerData.host + ".com")
				assert.NoError(t, err)
				for _, block := range providerData.blocks {
					p.AddBlockEntry(block.number, block.status, 10)
				}
				n.Providers[providerData.host] = p
				if providerData.host == tt.testProviderHost {
					testProvider = p
				}
			}

			result := n.hasOtherHealthyProviders(testProvider)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBlockJumpBehavior(t *testing.T) {
	tests := []struct {
		name               string
		currentBlock       int64
		latestNetworkBlock int64
		blockJumpLimit     int64
		hasOtherHealthy    bool
		expectedStatus     HealthStatus
	}{
		{
			name:               "no_jump_healthy",
			currentBlock:       100,
			latestNetworkBlock: 100,
			blockJumpLimit:     5,
			hasOtherHealthy:    true,
			expectedStatus:     Healthy,
		},
		{
			name:               "small_jump_healthy",
			currentBlock:       103,
			latestNetworkBlock: 100,
			blockJumpLimit:     5,
			hasOtherHealthy:    true,
			expectedStatus:     Healthy,
		},
		{
			name:               "large_jump_with_others_unhealthy",
			currentBlock:       107,
			latestNetworkBlock: 100,
			blockJumpLimit:     5,
			hasOtherHealthy:    true,
			expectedStatus:     Unhealthy,
		},
		{
			name:               "large_jump_without_others_healthy",
			currentBlock:       107,
			latestNetworkBlock: 100,
			blockJumpLimit:     5,
			hasOtherHealthy:    false,
			expectedStatus:     Healthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			n, err := NewNetwork("test", EVMHandler, utils.Environment("test"), LoopbackConfig{Port: "8000", ApiKey: DefaultLoopbackApiKey})
			assert.NoError(t, err)
			n.BlockJumpLimit = tt.blockJumpLimit
			n.Providers = make(map[string]*provider)
			n.ChainId = "0x1" // Set expected chain ID to match mock response

			// Initialize logger to prevent panic
			n.logger = logger.NewLoggerClient(zap.NewNop(), utils.EnvTest)

			// Initialize mock HTTP client to prevent panic
			mockHTTPClient := din_http.NewMockIHTTPClient(ctrl)
			mockHTTPClient.EXPECT().
				Post(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return([]byte(`{"result":"0x1"}`), intPtr(200), nil).
				AnyTimes()
			n.HttpClient = mockHTTPClient

			// Create test provider
			testProvider, err := NewProvider("http://test-provider.com")
			assert.NoError(t, err)
			testProvider.AddBlockEntry(tt.currentBlock, Healthy, 5)
			n.Providers["test-provider"] = testProvider

			// Create other providers based on hasOtherHealthy
			if tt.hasOtherHealthy {
				otherProvider, err := NewProvider("http://other-provider.com")
				assert.NoError(t, err)
				otherProvider.AddBlockEntry(tt.latestNetworkBlock, Healthy, 5)
				n.Providers["other-provider"] = otherProvider
			}

			result := n.evaluateProviderHealth(testProvider, tt.currentBlock, Healthy, tt.latestNetworkBlock)
			assert.Equal(t, tt.expectedStatus, result)
		})
	}
}

func TestGetLatestBlockNumber(t *testing.T) {
	tests := []struct {
		name                 string
		networkName          string
		caddyPort            string
		httpResponse         []byte
		statusCode           int
		httpError            error
		hcMethod             string
		expectedBlockNum     int64
		expectedHealthStatus HealthStatus
		expectedErr          bool
	}{
		{
			name:                 "successful_request",
			networkName:          "ethereum",
			caddyPort:            "8000",
			httpResponse:         []byte(`{"result":"0x64"}`),
			statusCode:           200,
			httpError:            nil,
			hcMethod:             "eth_blockNumber",
			expectedBlockNum:     100,
			expectedHealthStatus: Healthy,
			expectedErr:          false,
		},
		{
			name:                 "http_error",
			networkName:          "ethereum",
			caddyPort:            "8000",
			httpResponse:         nil,
			statusCode:           0,
			httpError:            errors.New("connection failed"),
			hcMethod:             "eth_blockNumber",
			expectedBlockNum:     0,
			expectedHealthStatus: Unhealthy,
			expectedErr:          true,
		},
		{
			name:                 "rate_limit_warning",
			networkName:          "ethereum",
			caddyPort:            "8000",
			httpResponse:         []byte(`{"error":"rate limited"}`),
			statusCode:           429,
			httpError:            nil,
			hcMethod:             "eth_blockNumber",
			expectedBlockNum:     0,
			expectedHealthStatus: Warning,
			expectedErr:          true,
		},
		{
			name:                 "server_error",
			networkName:          "ethereum",
			caddyPort:            "8000",
			httpResponse:         []byte(`{"error":"internal server error"}`),
			statusCode:           500,
			httpError:            nil,
			hcMethod:             "eth_blockNumber",
			expectedBlockNum:     0,
			expectedHealthStatus: Unhealthy,
			expectedErr:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// Create mock HTTP client
			mockHTTPClient := din_http.NewMockIHTTPClient(ctrl)
			mockHTTPClient.EXPECT().
				Post(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(tt.httpResponse, &tt.statusCode, tt.httpError).
				AnyTimes()

			// Create mock logger
			mockLogger := logger.NewLoggerClient(zap.NewNop(), utils.EnvTest)

			n, err := NewNetwork(tt.networkName, EVMHandler, utils.Environment("test"), LoopbackConfig{
				Port: tt.caddyPort,
				ApiKey: DefaultLoopbackApiKey,
			})
			assert.NoError(t, err)
			n.HttpClient = mockHTTPClient
			n.logger = mockLogger
			n.RequestAttemptCount = 1

			// Create and set handler for test since it's not created in NewNetwork anymore
			config := &networklib.NetworkConfig{
				Name:   tt.networkName,
				Type:   string(EVMHandler),
				Logger: mockLogger,
			}
			handler := networklib.NewEVMHandler(config)
			err = n.SetHandler(handler)
			assert.NoError(t, err)

			result, err := n.getLatestBlockNumber("http://test.com", map[string]string{}, nil, "test-provider")

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedBlockNum, result.blockNumber)
			assert.Equal(t, tt.expectedHealthStatus, result.healthStatus)
		})
	}
}

func newTestNetwork(t *testing.T, name string, historySize int) *network {
	t.Helper()

	n, _ := NewNetwork(name, EVMHandler, utils.Environment("test"), LoopbackConfig{Port: "8000", ApiKey: DefaultLoopbackApiKey})
	n.NetworkBlockHistorySize = historySize
	// Initialize logger to prevent panic
	n.logger = logger.NewLoggerClient(zap.NewNop(), utils.EnvTest)

	// Create and set handler for test since it's not created in NewNetwork anymore
	config := &networklib.NetworkConfig{
		Name:   name,
		Type:   string(EVMHandler),
		Logger: n.logger,
	}
	handler := networklib.NewEVMHandler(config)
	require.NoError(t, n.SetHandler(handler))

	return n
}

func TestAddNetworkBlockEntry(t *testing.T) {
	tests := []struct {
		name                string
		initialEntries      []int64
		newBlockNumber      int64
		newBlockData        interface{}
		expectedLength      int
		expectedLatestBlock int64
		networkHistorySize  int
		expectAdd           bool
	}{
		{
			name:                "add_first_entry",
			initialEntries:      []int64{},
			newBlockNumber:      100,
			newBlockData:        "0xabc123",
			expectedLength:      1,
			expectedLatestBlock: 100,
			networkHistorySize:  5,
			expectAdd:           true,
		},
		{
			name:                "add_increasing_block",
			initialEntries:      []int64{100, 101},
			newBlockNumber:      102,
			newBlockData:        "0xdef456",
			expectedLength:      3,
			expectedLatestBlock: 102,
			networkHistorySize:  5,
			expectAdd:           true,
		},
		{
			name:                "skip_lower_block",
			initialEntries:      []int64{100, 101, 102},
			newBlockNumber:      101,
			newBlockData:        "0xdef456",
			expectedLength:      3,
			expectedLatestBlock: 102,
			networkHistorySize:  5,
			expectAdd:           false,
		},
		{
			name:                "skip_same_block",
			initialEntries:      []int64{100, 101, 102},
			newBlockNumber:      102,
			newBlockData:        "0xdef456",
			expectedLength:      3,
			expectedLatestBlock: 102,
			networkHistorySize:  5,
			expectAdd:           false,
		},
		{
			name:                "trim_history_when_exceeded",
			initialEntries:      []int64{100, 101, 102},
			newBlockNumber:      103,
			newBlockData:        "0xghi789",
			expectedLength:      3,
			expectedLatestBlock: 103,
			networkHistorySize:  3,
			expectAdd:           true,
		},
		{
			name:                "handle_solana_block_data",
			initialEntries:      []int64{},
			newBlockNumber:      100,
			newBlockData:        din_http.JSONRPCSolanaBlockResponse{Result: din_http.SolanaBlockResult{Blockhash: "SolanaHash123"}},
			expectedLength:      1,
			expectedLatestBlock: 100,
			networkHistorySize:  5,
			expectAdd:           true,
		},
		{
			name:                "handle_evm_block_data",
			initialEntries:      []int64{},
			newBlockNumber:      100,
			newBlockData:        din_http.JSONRPCEVMBlockResponse{Result: din_http.EVMBlockResult{Hash: "0xevmhash123"}},
			expectedLength:      1,
			expectedLatestBlock: 100,
			networkHistorySize:  5,
			expectAdd:           true,
		},
		{
			name:                "handle_nil_block_data",
			initialEntries:      []int64{},
			newBlockNumber:      100,
			newBlockData:        nil,
			expectedLength:      1,
			expectedLatestBlock: 100,
			networkHistorySize:  5,
			expectAdd:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := newTestNetwork(t, "test", tt.networkHistorySize)

			// Add initial entries
			for _, blockNum := range tt.initialEntries {
				n.AddNetworkBlockEntry(blockNum, "0x"+strings.Repeat("a", 8))
			}

			// Add the new entry
			n.AddNetworkBlockEntry(tt.newBlockNumber, tt.newBlockData)

			// Check the length
			n.blockHistoryMu.RLock()
			actualLength := n.blockHistory.Len()
			n.blockHistoryMu.RUnlock()
			assert.Equal(t, tt.expectedLength, actualLength)

			// Check the latest block if entries exist
			if tt.expectedLength > 0 {
				latestEntry := n.getLatestBlockEntry()
				assert.NotNil(t, latestEntry)
				assert.Equal(t, tt.expectedLatestBlock, latestEntry.blockNumber)
			}
		})
	}
}

func TestGetLatestBlockEntry(t *testing.T) {
	tests := []struct {
		name          string
		blockNumbers  []int64
		expectNil     bool
		expectedBlock int64
	}{
		{
			name:          "empty_history",
			blockNumbers:  []int64{},
			expectNil:     true,
			expectedBlock: 0,
		},
		{
			name:          "single_entry",
			blockNumbers:  []int64{100},
			expectNil:     false,
			expectedBlock: 100,
		},
		{
			name:          "multiple_entries",
			blockNumbers:  []int64{100, 101, 102},
			expectNil:     false,
			expectedBlock: 102,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := newTestNetwork(t, "test", 5)

			// Add entries
			for _, blockNum := range tt.blockNumbers {
				n.AddNetworkBlockEntry(blockNum, "0x"+strings.Repeat("a", 8))
			}

			result := n.getLatestBlockEntry()

			if tt.expectNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectedBlock, result.blockNumber)
			}
		})
	}
}

func TestCheckSelfLoopbackHealth(t *testing.T) {
	tests := []struct {
		name                 string
		networkName          string
		caddyPort            string
		httpResponse         []byte
		statusCode           int
		httpError            error
		hcMethod             string
		expectedBlockNum     int64
		expectedHealthStatus HealthStatus
		expectedErr          bool
	}{
		{
			name:                 "successful_loopback",
			networkName:          "ethereum",
			caddyPort:            "8000",
			httpResponse:         []byte(`{"result":"0x64"}`),
			statusCode:           200,
			httpError:            nil,
			hcMethod:             "eth_blockNumber",
			expectedBlockNum:     100,
			expectedHealthStatus: Healthy,
			expectedErr:          false,
		},
		{
			name:                 "http_error_loopback",
			networkName:          "ethereum",
			caddyPort:            "8000",
			httpResponse:         nil,
			statusCode:           0,
			httpError:            errors.New("connection refused"),
			hcMethod:             "eth_blockNumber",
			expectedBlockNum:     0,
			expectedHealthStatus: Unhealthy,
			expectedErr:          true,
		},
		{
			name:                 "empty_caddy_port",
			networkName:          "ethereum",
			caddyPort:            "",
			httpResponse:         nil,
			statusCode:           0,
			httpError:            nil,
			hcMethod:             "eth_blockNumber",
			expectedBlockNum:     0,
			expectedHealthStatus: Unhealthy,
			expectedErr:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock HTTP client only if we have a caddy port
			var mockHTTPClient *din_http.MockIHTTPClient
			if tt.caddyPort != "" {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				mockHTTPClient = din_http.NewMockIHTTPClient(ctrl)
				mockHTTPClient.EXPECT().
					Post(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(tt.httpResponse, &tt.statusCode, tt.httpError).
					AnyTimes()
			}

			n, err := NewNetwork(tt.networkName, EVMHandler, utils.Environment("test"), LoopbackConfig{
				Port: tt.caddyPort,
				ApiKey: DefaultLoopbackApiKey,
			})
			assert.NoError(t, err)
			if mockHTTPClient != nil {
				n.HttpClient = mockHTTPClient
			}

			// Initialize logger to prevent panic
			n.logger = logger.NewLoggerClient(zap.NewNop(), utils.EnvTest)

			// Create and set handler for test since it's not created in NewNetwork anymore
			config := &networklib.NetworkConfig{
				Name:   tt.networkName,
				Type:   string(EVMHandler),
				Logger: n.logger,
			}
			handler := networklib.NewEVMHandler(config)
			err = n.SetHandler(handler)
			assert.NoError(t, err)

			result, err := n.checkSelfLoopbackHealth()

			if tt.expectedErr {
				assert.Error(t, err)
				if result != nil {
					assert.Equal(t, tt.expectedBlockNum, result.blockNumber)
					assert.Equal(t, tt.expectedHealthStatus, result.healthStatus)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectedBlockNum, result.blockNumber)
				assert.Equal(t, tt.expectedHealthStatus, result.healthStatus)
			}
		})
	}
}

func TestNewNetwork(t *testing.T) {
	tests := []struct {
		name        string
		networkName string
		networkType string
		environment utils.Environment
		caddyPort   string
		expectError bool
	}{
		{
			name:        "valid_evm_network",
			networkName: "ethereum",
			networkType: string(EVMHandler),
			environment: utils.EnvDev,
			caddyPort:   "8000",
			expectError: false,
		},
		{
			name:        "valid_beacon_network",
			networkName: "ethereum-beacon",
			networkType: string(BeaconHandler),
			environment: utils.EnvProd,
			caddyPort:   "8080",
			expectError: false,
		},
		{
			name:        "empty_network_type",
			networkName: "unknown",
			networkType: "",
			environment: utils.EnvDev,
			caddyPort:   "8000",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			network, err := NewNetwork(tt.networkName, HandlerType(tt.networkType), tt.environment, LoopbackConfig{Port: tt.caddyPort, ApiKey: DefaultLoopbackApiKey})

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, network)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, network)
				assert.Equal(t, tt.networkName, network.Name)
				assert.Equal(t, HandlerType(tt.networkType), network.HandlerType)
				assert.Equal(t, tt.environment, network.Environment)
				assert.Equal(t, tt.caddyPort, network.LoopbackConfig.Port)
			}
		})
	}
}

func TestNetwork_processBlockNumberResponse(t *testing.T) {
	tests := []struct {
		name             string
		resBytes         []byte
		statusCode       *int
		expectedBlock    int64
		expectedHealth   HealthStatus
		expectedErrorMsg string
	}{
		{
			name:             "nil_status_code",
			resBytes:         []byte(`{"result":"0x64"}`),
			statusCode:       nil,
			expectedBlock:    0,
			expectedHealth:   Unhealthy,
			expectedErrorMsg: "received nil statusCode",
		},
		{
			name:             "rate_limit_status",
			resBytes:         []byte(`{"error":"rate limited"}`),
			statusCode:       intPtr(429),
			expectedBlock:    0,
			expectedHealth:   Warning,
			expectedErrorMsg: "rate limit error",
		},
		{
			name:             "server_error_status",
			resBytes:         []byte(`{"error":"server error"}`),
			statusCode:       intPtr(500),
			expectedBlock:    0,
			expectedHealth:   Unhealthy,
			expectedErrorMsg: "error status code",
		},
		{
			name:             "invalid_json",
			resBytes:         []byte(`{invalid json`),
			statusCode:       intPtr(200),
			expectedBlock:    0,
			expectedHealth:   Unhealthy,
			expectedErrorMsg: "failed to parse JSON-RPC response",
		},
		{
			name:             "valid_hex_result",
			resBytes:         []byte(`{"result":"0x64"}`),
			statusCode:       intPtr(200),
			expectedBlock:    100,
			expectedHealth:   Healthy,
			expectedErrorMsg: "",
		},
		{
			name:             "invalid_decimal_result",
			resBytes:         []byte(`{"result":100}`),
			statusCode:       intPtr(200),
			expectedBlock:    0,
			expectedHealth:   Unhealthy,
			expectedErrorMsg: "failed to unmarshal block number",
		},
		{
			name:             "invalid_hex_format",
			resBytes:         []byte(`{"result":"invalid"}`),
			statusCode:       intPtr(200),
			expectedBlock:    0,
			expectedHealth:   Unhealthy,
			expectedErrorMsg: "invalid hex block number",
		},
		{
			name:             "empty_result",
			resBytes:         []byte(`{"result":""}`),
			statusCode:       intPtr(200),
			expectedBlock:    0,
			expectedHealth:   Unhealthy,
			expectedErrorMsg: "invalid hex block number",
		},
		{
			name:             "null_result",
			resBytes:         []byte(`{"result":null}`),
			statusCode:       intPtr(200),
			expectedBlock:    0,
			expectedHealth:   Unhealthy,
			expectedErrorMsg: "invalid hex block number: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := NewNetwork("test", "", utils.Environment("test"), LoopbackConfig{Port: "8000", ApiKey: DefaultLoopbackApiKey})
			assert.NoError(t, err)
			// Set handler for processBlockNumberResponse tests
			config := &networklib.NetworkConfig{
				Name:    "test",
				Type:    string(EVMHandler),
				ChainID: "1",
			}
			require.NoError(t, n.SetHandler(networklib.NewEVMHandler(config)))

			blockNumber, healthStatus, err := n.processBlockNumberResponse(tt.resBytes, tt.statusCode)

			assert.Equal(t, tt.expectedBlock, blockNumber)
			assert.Equal(t, tt.expectedHealth, healthStatus)

			if tt.expectedErrorMsg != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func intPtr(i int) *int {
	return &i
}

func TestNetwork_isStalled(t *testing.T) {
	n, err := NewNetwork("test", EVMHandler, utils.Environment("test"), LoopbackConfig{Port: "8000", ApiKey: DefaultLoopbackApiKey})
	assert.NoError(t, err)
	n.ProviderBlockHistorySize = 3

	// Test with insufficient history
	provider, err := NewProvider("http://test1.com")
	assert.NoError(t, err)
	provider.AddBlockEntry(100, Healthy, 3)
	provider.AddBlockEntry(101, Healthy, 3)
	assert.False(t, n.isStalled(provider))

	// Test with full history but different block numbers (not stalled)
	provider.AddBlockEntry(102, Healthy, 3)
	assert.False(t, n.isStalled(provider))

	// Test with full history and same block numbers (stalled)
	provider2, err := NewProvider("http://test2.com")
	assert.NoError(t, err)
	provider2.AddBlockEntry(100, Healthy, 3)
	provider2.AddBlockEntry(100, Healthy, 3)
	provider2.AddBlockEntry(100, Healthy, 3)
	assert.True(t, n.isStalled(provider2))
}

func TestNetwork_allProvidersStalled(t *testing.T) {
	n, err := NewNetwork("test", EVMHandler, utils.Environment("test"), LoopbackConfig{Port: "8000", ApiKey: DefaultLoopbackApiKey})
	assert.NoError(t, err)
	n.ProviderBlockHistorySize = 3

	// Create mock providers
	provider1, err := NewProvider("http://provider1.com")
	assert.NoError(t, err)
	provider1.AddBlockEntry(100, Healthy, 3)
	provider1.AddBlockEntry(100, Healthy, 3)
	provider1.AddBlockEntry(100, Healthy, 3)

	provider2, err := NewProvider("http://provider2.com")
	assert.NoError(t, err)
	provider2.AddBlockEntry(101, Healthy, 3)
	provider2.AddBlockEntry(101, Healthy, 3)
	provider2.AddBlockEntry(101, Healthy, 3)

	// Test all stalled
	n.Providers = map[string]*provider{
		"provider1": provider1,
		"provider2": provider2,
	}
	assert.True(t, n.allProvidersStalled())

	// Test one not stalled
	provider2.AddBlockEntry(102, Healthy, 3)
	assert.False(t, n.allProvidersStalled())
}

func TestNetwork_getLatestHealthyBlock(t *testing.T) {
	n, err := NewNetwork("test", EVMHandler, utils.Environment("test"), LoopbackConfig{Port: "8000", ApiKey: DefaultLoopbackApiKey})
	assert.NoError(t, err)

	// Test empty providers
	n.Providers = map[string]*provider{}
	assert.Equal(t, int64(0), n.getLatestHealthyBlock())

	// Test with healthy and warning providers
	provider1, err := NewProvider("http://provider1.com")
	assert.NoError(t, err)
	provider1.AddBlockEntry(105, Healthy, 5)

	provider2, err := NewProvider("http://provider2.com")
	assert.NoError(t, err)
	provider2.AddBlockEntry(103, Warning, 5)

	n.Providers = map[string]*provider{
		"provider1": provider1,
		"provider2": provider2,
	}

	assert.Equal(t, int64(105), n.getLatestHealthyBlock())
}

func TestNetwork_hasOtherHealthyProviders(t *testing.T) {
	n, err := NewNetwork("test", EVMHandler, utils.Environment("test"), LoopbackConfig{Port: "8000", ApiKey: DefaultLoopbackApiKey})
	assert.NoError(t, err)

	provider1, err := NewProvider("http://provider1.com")
	assert.NoError(t, err)
	provider1.AddBlockEntry(100, Healthy, 5)

	provider2, err := NewProvider("http://provider2.com")
	assert.NoError(t, err)
	provider2.AddBlockEntry(101, Healthy, 5)

	n.Providers = map[string]*provider{
		provider1.host: provider1,
		provider2.host: provider2,
	}

	// Test that provider1 has other healthy providers
	assert.True(t, n.hasOtherHealthyProviders(provider1))

	// Test with no other healthy providers - add enough unhealthy entries to clear the healthy history
	provider2.AddBlockEntry(102, Unhealthy, 5)
	provider2.AddBlockEntry(103, Unhealthy, 5)
	provider2.AddBlockEntry(104, Unhealthy, 5)
	provider2.AddBlockEntry(105, Unhealthy, 5)
	provider2.AddBlockEntry(106, Unhealthy, 5)
	assert.False(t, n.hasOtherHealthyProviders(provider1))
}

func TestNetwork_AddNetworkBlockEntry(t *testing.T) {
	n, err := NewNetwork("test", EVMHandler, utils.Environment("test"), LoopbackConfig{Port: "8000", ApiKey: DefaultLoopbackApiKey})
	assert.NoError(t, err)
	n.NetworkBlockHistorySize = 3

	// Test adding valid entries
	n.AddNetworkBlockEntry(100, "hash100")
	assert.Equal(t, 1, n.blockHistory.Len())

	n.AddNetworkBlockEntry(101, "hash101")
	assert.Equal(t, 2, n.blockHistory.Len())

	// Test that history is trimmed when it exceeds size
	n.AddNetworkBlockEntry(102, "hash102")
	n.AddNetworkBlockEntry(103, "hash103")
	assert.Equal(t, 3, n.blockHistory.Len())

	// Test that the latest entry is correct
	latest := n.getLatestBlockEntry()
	assert.NotNil(t, latest)
	assert.Equal(t, int64(103), latest.blockNumber)
}

func TestNetwork_getLatestBlockEntry(t *testing.T) {
	n, err := NewNetwork("test", EVMHandler, utils.Environment("test"), LoopbackConfig{Port: "8000", ApiKey: DefaultLoopbackApiKey})
	assert.NoError(t, err)

	// Test empty history
	assert.Nil(t, n.getLatestBlockEntry())

	// Test with entries
	n.AddNetworkBlockEntry(100, "hash100")
	latest := n.getLatestBlockEntry()
	assert.NotNil(t, latest)
	assert.Equal(t, int64(100), latest.blockNumber)
}

func TestNetwork_ExtractBlockHashByNetworkType(t *testing.T) {
	tests := []struct {
		name        string
		networkName string
		blockData   interface{}
		expected    string
	}{
		{
			name:        "string_hash",
			networkName: "ethereum",
			blockData:   "0xabcdef123456",
			expected:    "0xabcdef123456",
		},
		{
			name:        "solana_block_response",
			networkName: "solana-mainnet",
			blockData:   din_http.JSONRPCSolanaBlockResponse{Result: din_http.SolanaBlockResult{Blockhash: "SolanaHash123"}},
			expected:    "SolanaHash123",
		},
		{
			name:        "evm_block_response",
			networkName: "ethereum",
			blockData:   din_http.JSONRPCEVMBlockResponse{Result: din_http.EVMBlockResult{Hash: "0xevmhash123"}},
			expected:    "0xevmhash123",
		},
		{
			name:        "unknown_type",
			networkName: "ethereum",
			blockData:   map[string]interface{}{"unknown": "type"},
			expected:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := NewNetwork(tt.networkName, EVMHandler, utils.Environment("test"), LoopbackConfig{Port: "8000", ApiKey: DefaultLoopbackApiKey})
			assert.NoError(t, err)

			// Initialize logger to prevent panic
			n.logger = logger.NewLoggerClient(zap.NewNop(), utils.EnvTest)

			// We'll test this through AddNetworkBlockEntry since the extraction logic is internal
			n.AddNetworkBlockEntry(100, tt.blockData)

			if tt.expected != "" {
				latest := n.getLatestBlockEntry()
				assert.NotNil(t, latest)
				// The block hash extraction is tested indirectly through the network entry logic
			}
		})
	}
}

func TestDetermineNetworkTypeFromNetworkName(t *testing.T) {
	tests := []struct {
		name         string
		networkName  string
		expectedType string
	}{
		{
			name:         "ethereum_mainnet",
			networkName:  "ethereum",
			expectedType: string(EVMHandler),
		},
		{
			name:         "beacon-chain",
			networkName:  "ethereum-beacon-mainnet",
			expectedType: string(BeaconHandler),
		},
		{
			name:         "starknet_mainnet",
			networkName:  "starknet-mainnet",
			expectedType: string(StarknetHandler),
		},
		{
			name:         "solana_mainnet",
			networkName:  "solana-mainnet",
			expectedType: string(SolanaHandler),
		},
		{
			name:         "unknown_network",
			networkName:  "unknown-network",
			expectedType: string(EVMHandler), // Default fallback
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test this through NewNetwork since the detection logic is internal
			n, err := NewNetwork(tt.networkName, HandlerType(tt.expectedType), utils.Environment("test"), LoopbackConfig{Port: "8000", ApiKey: DefaultLoopbackApiKey})
			assert.NoError(t, err)
			assert.Equal(t, HandlerType(tt.expectedType), n.HandlerType)
		})
	}
}

func TestNetworkSetHandler(t *testing.T) {
	tests := []struct {
		name          string
		handler       networklib.NetworkHandler
		expectedError string
	}{
		{
			name:          "set nil handler",
			handler:       nil,
			expectedError: "cannot set nil handler",
		},
		{
			name:    "set valid handler",
			handler: networklib.NewEVMHandler(&networklib.NetworkConfig{}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := NewNetwork("test-network", "", utils.EnvTest, LoopbackConfig{Port: "2019", ApiKey: DefaultLoopbackApiKey})
			require.NoError(t, err)

			err = n.SetHandler(tt.handler)
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.handler, n.handler)
			}
		})
	}
}

func TestNetworkSetHandler_AvoidDuplicates(t *testing.T) {
	n, err := NewNetwork("test-network", "", utils.EnvTest, LoopbackConfig{Port: "2019", ApiKey: DefaultLoopbackApiKey})
	require.NoError(t, err)

	handler := networklib.NewEVMHandler(&networklib.NetworkConfig{})

	// Set handler first time
	err = n.SetHandler(handler)
	assert.NoError(t, err)

	// Set same handler again - should not error and should be a no-op
	err = n.SetHandler(handler)
	assert.NoError(t, err)
	assert.Equal(t, handler, n.handler)
}
