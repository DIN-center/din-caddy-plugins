package modules

import (
	"net/http"
	"testing"

	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	networklib "github.com/DIN-center/din-caddy-plugins/lib/network"
	"github.com/DIN-center/din-caddy-plugins/lib/utils"
	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// TestGetBlockByNumber tests the getBlockByNumber functionality
func TestGetBlockByNumber(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name                string
		blockNumber         int64
		caddyPort           string
		handlerSupportsGet  bool
		mockSetup           func(*networklib.MockNetworkHandler, *din_http.MockIHTTPClient)
		expectedBlockData   interface{}
		expectError         bool
		expectedErrorString string
	}{
		{
			name:               "successful_get_block_evm",
			blockNumber:        12345,
			caddyPort:          "8080",
			handlerSupportsGet: true,
			mockSetup: func(mockHandler *networklib.MockNetworkHandler, mockHTTP *din_http.MockIHTTPClient) {
				// Handler setup
				mockHandler.EXPECT().SupportsGetBlockByNumber().Return(true)
				mockHandler.EXPECT().GetBlockByNumberMethod().Return("eth_getBlockByNumber")
				mockHandler.EXPECT().CreateBlockRequest("eth_getBlockByNumber", int64(12345), false).
					Return([]byte(`{"jsonrpc":"2.0","method":"eth_getBlockByNumber","id":1,"params":["0x3039",false]}`), nil)

				// HTTP response
				responseBody := []byte(`{"jsonrpc":"2.0","id":1,"result":{"number":"0x3039","hash":"0xabc123","timestamp":"0x12345"}}`)
				statusCode := 200
				mockHTTP.EXPECT().Post(
					"http://127.0.0.1:8080/test-network",
					map[string]string{"Content-Type": "application/json"},
					gomock.Any(),
					nil,
				).Return(responseBody, &statusCode, nil)

				// Parse response
				mockHandler.EXPECT().ParseBlockResponse(responseBody).
					Return(map[string]interface{}{
						"number":    "0x3039",
						"hash":      "0xabc123",
						"timestamp": "0x12345",
					}, nil)
			},
			expectedBlockData: map[string]interface{}{
				"number":    "0x3039",
				"hash":      "0xabc123",
				"timestamp": "0x12345",
			},
			expectError: false,
		},
		{
			name:               "network_does_not_support_get_block",
			blockNumber:        12345,
			caddyPort:          "8080",
			handlerSupportsGet: false,
			mockSetup: func(mockHandler *networklib.MockNetworkHandler, mockHTTP *din_http.MockIHTTPClient) {
				mockHandler.EXPECT().SupportsGetBlockByNumber().Return(false)
			},
			expectedBlockData: nil,
			expectError:       false,
		},
		{
			name:               "no_caddy_port_set",
			blockNumber:        12345,
			caddyPort:          "",
			handlerSupportsGet: true,
			mockSetup: func(mockHandler *networklib.MockNetworkHandler, mockHTTP *din_http.MockIHTTPClient) {
				// No calls expected when port is empty
			},
			expectError:         true,
			expectedErrorString: "Caddy port is not set",
		},
		{
			name:               "no_handler_available",
			blockNumber:        12345,
			caddyPort:          "8080",
			handlerSupportsGet: true,
			mockSetup: func(mockHandler *networklib.MockNetworkHandler, mockHTTP *din_http.MockIHTTPClient) {
				// Handler will be nil, no setup
			},
			expectedBlockData: nil,
			expectError:       false,
		},
		{
			name:               "http_request_fails",
			blockNumber:        12345,
			caddyPort:          "8080",
			handlerSupportsGet: true,
			mockSetup: func(mockHandler *networklib.MockNetworkHandler, mockHTTP *din_http.MockIHTTPClient) {
				mockHandler.EXPECT().SupportsGetBlockByNumber().Return(true)
				mockHandler.EXPECT().GetBlockByNumberMethod().Return("eth_getBlockByNumber")
				mockHandler.EXPECT().CreateBlockRequest("eth_getBlockByNumber", int64(12345), false).
					Return([]byte(`{"jsonrpc":"2.0","method":"eth_getBlockByNumber","id":1,"params":["0x3039",false]}`), nil)

				// HTTP request fails
				mockHTTP.EXPECT().Post(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					nil,
				).Return(nil, nil, errors.New("connection refused"))
			},
			expectError:         true,
			expectedErrorString: "Error sending POST request",
		},
		{
			name:               "non_ok_status_code",
			blockNumber:        12345,
			caddyPort:          "8080",
			handlerSupportsGet: true,
			mockSetup: func(mockHandler *networklib.MockNetworkHandler, mockHTTP *din_http.MockIHTTPClient) {
				mockHandler.EXPECT().SupportsGetBlockByNumber().Return(true)
				mockHandler.EXPECT().GetBlockByNumberMethod().Return("eth_getBlockByNumber")
				mockHandler.EXPECT().CreateBlockRequest("eth_getBlockByNumber", int64(12345), false).
					Return([]byte(`{"jsonrpc":"2.0","method":"eth_getBlockByNumber","id":1,"params":["0x3039",false]}`), nil)

				// HTTP returns error status
				responseBody := []byte(`{"error":"block not found"}`)
				statusCode := 404
				mockHTTP.EXPECT().Post(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					nil,
				).Return(responseBody, &statusCode, nil)
			},
			expectError:         true,
			expectedErrorString: "Error getting block from response",
		},
		{
			name:               "parse_response_error",
			blockNumber:        12345,
			caddyPort:          "8080",
			handlerSupportsGet: true,
			mockSetup: func(mockHandler *networklib.MockNetworkHandler, mockHTTP *din_http.MockIHTTPClient) {
				mockHandler.EXPECT().SupportsGetBlockByNumber().Return(true)
				mockHandler.EXPECT().GetBlockByNumberMethod().Return("eth_getBlockByNumber")
				mockHandler.EXPECT().CreateBlockRequest("eth_getBlockByNumber", int64(12345), false).
					Return([]byte(`{"jsonrpc":"2.0","method":"eth_getBlockByNumber","id":1,"params":["0x3039",false]}`), nil)

				// HTTP response OK
				responseBody := []byte(`{"jsonrpc":"2.0","id":1,"result":{"invalid":"data"}}`)
				statusCode := 200
				mockHTTP.EXPECT().Post(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					nil,
				).Return(responseBody, &statusCode, nil)

				// Parse fails
				mockHandler.EXPECT().ParseBlockResponse(responseBody).
					Return(nil, errors.New("invalid block format"))
			},
			expectError:         true,
			expectedErrorString: "Error parsing block response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			mockHandler := networklib.NewMockNetworkHandler(ctrl)
			mockHTTP := din_http.NewMockIHTTPClient(ctrl)

			// Create network
			n, err := NewNetwork("test-network", "evm", utils.Environment("test"), tt.caddyPort)
			assert.NoError(t, err)

			// Set dependencies
			n.logger = logger.NewLoggerClient(zap.NewNop(), utils.EnvTest)
			n.HttpClient = mockHTTP

			// Set handler only if test expects it
			if tt.name == "no_handler_available" {
				// Explicitly set handler to nil for this test
				n.handler = nil
			} else {
				n.handler = mockHandler
			}

			// Setup mocks
			if tt.mockSetup != nil {
				tt.mockSetup(mockHandler, mockHTTP)
			}

			// Execute
			result, err := n.getBlockByNumber(tt.blockNumber)

			// Verify
			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedErrorString != "" {
					assert.Contains(t, err.Error(), tt.expectedErrorString)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedBlockData, result)
			}
		})
	}
}

// TestGetBlockByNumberIntegration tests getBlockByNumber with different network types
func TestGetBlockByNumberIntegration(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name         string
		networkType  string
		blockNumber  int64
		setupHandler func(*networklib.MockNetworkHandler)
		expectNil    bool
	}{
		{
			name:        "evm_network_supports_get_block",
			networkType: "evm",
			blockNumber: 100,
			setupHandler: func(mockHandler *networklib.MockNetworkHandler) {
				mockHandler.EXPECT().SupportsGetBlockByNumber().Return(true)
				mockHandler.EXPECT().GetBlockByNumberMethod().Return("eth_getBlockByNumber")
				mockHandler.EXPECT().CreateBlockRequest(gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]byte(`{}`), nil)
			},
			expectNil: false,
		},
		{
			name:        "beacon_network_does_not_support_json_rpc_get_block",
			networkType: "beacon-chain",
			blockNumber: 100,
			setupHandler: func(mockHandler *networklib.MockNetworkHandler) {
				// Beacon chain should return false for SupportsGetBlockByNumber
				// since it uses REST API, not JSON-RPC
				mockHandler.EXPECT().SupportsGetBlockByNumber().Return(false)
			},
			expectNil: true,
		},
		{
			name:        "solana_network_supports_get_block",
			networkType: "solana",
			blockNumber: 100,
			setupHandler: func(mockHandler *networklib.MockNetworkHandler) {
				mockHandler.EXPECT().SupportsGetBlockByNumber().Return(true)
				mockHandler.EXPECT().GetBlockByNumberMethod().Return("getBlock")
				mockHandler.EXPECT().CreateBlockRequest(gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]byte(`{}`), nil)
			},
			expectNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			mockHandler := networklib.NewMockNetworkHandler(ctrl)
			mockHTTP := din_http.NewMockIHTTPClient(ctrl)

			// Create network with specific type
			n, err := NewNetwork("test-network", tt.networkType, utils.Environment("test"), "8080")
			assert.NoError(t, err)

			// Set dependencies
			n.handler = mockHandler
			n.HttpClient = mockHTTP
			n.logger = logger.NewLoggerClient(zap.NewNop(), utils.EnvTest)

			// Setup handler expectations
			tt.setupHandler(mockHandler)

			// If the handler supports GetBlockByNumber, set up HTTP expectations
			if !tt.expectNil {
				statusCode := http.StatusOK
				mockHTTP.EXPECT().Post(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					nil,
				).Return([]byte(`{"result":{}}`), &statusCode, nil)

				mockHandler.EXPECT().ParseBlockResponse(gomock.Any()).
					Return(map[string]interface{}{}, nil)
			}

			// Execute
			result, err := n.getBlockByNumber(tt.blockNumber)

			// Verify
			assert.NoError(t, err)
			if tt.expectNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
			}
		})
	}
}
