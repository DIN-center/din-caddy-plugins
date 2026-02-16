package network_test

import (
	"encoding/json"
	"net/http"
	"testing"

	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
	"gotest.tools/v3/golden"
)

func TestStellarRpcHandler_GetType(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	assert.Equal(t, network.StellarRpcHandlerType, handler.GetType())
}

func TestStellarRpcHandler_GetName(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	expected := "Stellar JSON-RPC Handler"
	assert.Equal(t, expected, handler.GetName())
}

func TestStellarRpcHandler_GetRequestType(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	assert.Equal(t, network.RequestTypeRPC, handler.GetRequestType())
}

//
// Request processing
//

func TestStellarRpcHandler_ValidateRequest(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	tests := []struct {
		name        string
		method      string
		contentType string
		expectError bool
	}{
		{
			name:        "valid_request",
			method:      "POST",
			contentType: "application/json",
			expectError: false,
		},
		{
			name:        "valid_request_with_charset",
			method:      "POST",
			contentType: "application/json; charset=utf-8",
			expectError: false,
		},
		{
			name:        "invalid_method_GET",
			method:      "GET",
			contentType: "application/json",
			expectError: true,
		},
		{
			name:        "invalid_content_type",
			method:      "POST",
			contentType: "text/plain",
			expectError: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req, _ := http.NewRequest(tt.method, "/test", nil)
			req.Header.Set("Content-Type", tt.contentType)

			err := handler.ValidateRequest(req)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStellarRpcHandler_ExtractMethod(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	tests := []struct {
		name           string
		body           []byte
		expectedMethod string
		expectError    bool
	}{
		{
			name:           "valid_getHealth_method",
			body:           []byte(`{"jsonrpc":"2.0","method":"getHealth","id":1}`),
			expectedMethod: "getHealth",
			expectError:    false,
		},
		{
			name:           "valid_getNetwork_method",
			body:           []byte(`{"jsonrpc":"2.0","method":"getNetwork","id":1}`),
			expectedMethod: "getNetwork",
			expectError:    false,
		},
		{
			name:           "valid_getLedgers_method",
			body:           []byte(`{"jsonrpc":"2.0","method":"getLedgers","params":{"startLedger":100},"id":1}`),
			expectedMethod: "getLedgers",
			expectError:    false,
		},
		{
			name:        "empty_body",
			body:        []byte{},
			expectError: true,
		},
		{
			name:        "invalid_json",
			body:        []byte(`{invalid json`),
			expectError: true,
		},
		{
			name:        "missing_method",
			body:        []byte(`{"jsonrpc":"2.0","id":1}`),
			expectError: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req, _ := http.NewRequest("POST", "/test", nil)
			method, err := handler.ExtractMethod(req, tt.body)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedMethod, method)
			}
		})
	}
}

//
// Block operations
//

func TestStellarRpcHandler_CreateBlockRequest(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	exp := golden.Get(t, "stellar_get_ledgers_request.json")

	act, err := handler.CreateBlockRequest("getLedgers", 36233, false)
	require.NoError(t, err)
	assert.JSONEq(t, string(exp), string(act))
}

func TestStellarRpcHandler_ParseBlockResponse(t *testing.T) {
	body := golden.Get(t, "stellar_get_ledgers_response.json")

	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	result, err := handler.ParseBlockResponse(body)
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Verify it's a valid Stellar ledger response
	stellarResponse, ok := result.(din_http.JSONRPCStellarLedgerResponse)
	require.True(t, ok)
	assert.NotNil(t, stellarResponse.Result)
}

func TestStellarRpcHandler_ParseBlockResponse_Error(t *testing.T) {
	body := golden.Get(t, "stellar_jsonrpc_error.json")

	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	_, err := handler.ParseBlockResponse(body)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid request")
}

func TestStellarRpcHandler_FormatBlockHeight(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	tests := []struct {
		name           string
		sequenceNumber int64
		expected       string
	}{
		{
			name:           "zero",
			sequenceNumber: 0,
			expected:       "0",
		},
		{
			name:           "small_number",
			sequenceNumber: 36233,
			expected:       "36233",
		},
		{
			name:           "large_number",
			sequenceNumber: 51583040,
			expected:       "51583040",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := handler.FormatBlockHeight(tt.sequenceNumber)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStellarRpcHandler_ExtractBlockHash(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	// Parse the test data - ExtractBlockHash expects din_http.JSONRPCStellarLedgerResponse
	body := golden.Get(t, "stellar_get_ledgers_response.json")
	var response din_http.JSONRPCStellarLedgerResponse
	err := json.Unmarshal(body, &response)
	require.NoError(t, err)

	hash := handler.ExtractBlockHash(response)
	assert.Equal(t, "434de11b427aa4b6f8cda259ac2111a6aa148d2ab6b4c7affe864e94a9f4bd80", hash)
}

//
// Health Check methods
//

func TestStellarRpcHandler_GetHealthCheckMethod(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	expected := "getHealth"
	assert.Equal(t, expected, handler.GetHealthCheckMethod())
}

func TestStellarRpcHandler_GetLatestBlockNumber(t *testing.T) {
	body := golden.Get(t, "stellar_get_latest_ledger_response.json")
	statusCode := http.StatusOK

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cl := din_http.NewMockIHTTPClient(ctrl)
	cl.EXPECT().Post(
		gomock.Any(),
		gomock.Any(),
		gomock.Any(),
		nil,
	).Return(body, &statusCode, nil)

	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	result, err := handler.GetLatestBlockNumber("http://test", map[string]string{"Content-Type": "application/json"}, cl, nil, 5)
	require.NoError(t, err)
	assert.Equal(t, int64(51583040), result.BlockNumber)
	assert.Equal(t, network.Healthy, result.HealthStatus)
	assert.Equal(t, http.StatusOK, result.ResponseStatus)
}

func TestStellarRpcHandler_GetLatestBlockNumber_MissingSequence(t *testing.T) {
	// Test response missing sequence field
	body := []byte(`{"jsonrpc":"2.0","id":1,"result":{"id":"test","closeTime":"1234567890"}}`)
	statusCode := http.StatusOK

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cl := din_http.NewMockIHTTPClient(ctrl)
	cl.EXPECT().Post(
		gomock.Any(),
		gomock.Any(),
		gomock.Any(),
		nil,
	).Return(body, &statusCode, nil).Times(5)

	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	result, err := handler.GetLatestBlockNumber("http://test", map[string]string{"Content-Type": "application/json"}, cl, nil, 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing sequence")
	assert.Equal(t, int64(0), result.BlockNumber)
	assert.Equal(t, network.Unhealthy, result.HealthStatus)
}

//
// ChainID operations
//

func TestStellarRpcHandler_GetChainIDMethod(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})
	assert.Equal(t, "getNetwork", handler.GetChainIDMethod())
}

func TestStellarRpcHandler_ParseChainIDResponse(t *testing.T) {
	t.Run("passes with testnet passphrase", func(t *testing.T) {
		body := golden.Get(t, "stellar_network_testnet.json")
		handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

		chainID, err := handler.ParseChainIDResponse(body, http.StatusOK)
		require.NoError(t, err)
		assert.Equal(t, "Test SDF Network ; September 2015", chainID)
	})

	t.Run("passes with mainnet passphrase", func(t *testing.T) {
		body := golden.Get(t, "stellar_network_mainnet.json")
		handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

		chainID, err := handler.ParseChainIDResponse(body, http.StatusOK)
		require.NoError(t, err)
		assert.Equal(t, "Public Global Stellar Network ; September 2015", chainID)
	})

	t.Run("fails with non-200 status", func(t *testing.T) {
		body := golden.Get(t, "stellar_network_mainnet.json")
		handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

		_, err := handler.ParseChainIDResponse(body, http.StatusInternalServerError)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "HTTP error")
	})

	t.Run("fails with JSON-RPC error", func(t *testing.T) {
		body := golden.Get(t, "stellar_jsonrpc_error.json")
		handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

		_, err := handler.ParseChainIDResponse(body, http.StatusOK)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "JSON-RPC error")
	})
}

func TestStellarRpcHandler_GetChainID(t *testing.T) {
	body := golden.Get(t, "stellar_network_mainnet.json")
	statusCode := http.StatusOK

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cl := din_http.NewMockIHTTPClient(ctrl)
	cl.EXPECT().Post(
		gomock.Any(),
		gomock.Any(),
		gomock.Any(),
		nil,
	).Return(body, &statusCode, nil)

	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	chainID, err := handler.GetChainID("http://test", map[string]string{"Content-Type": "application/json"}, cl, nil, 5)
	require.NoError(t, err)
	assert.Equal(t, "Public Global Stellar Network ; September 2015", chainID)
}

func TestStellarRpcHandler_ValidateChainID(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	tests := []struct {
		name    string
		chainID string
		valid   bool
	}{
		{
			name:    "valid_mainnet_passphrase",
			chainID: "Public Global Stellar Network ; September 2015",
			valid:   true,
		},
		{
			name:    "valid_mainnet_passphrase_short",
			chainID: "Public Global Stellar Network",
			valid:   true,
		},
		{
			name:    "valid_testnet_passphrase",
			chainID: "Test SDF Network ; September 2015",
			valid:   true,
		},
		{
			name:    "valid_testnet_passphrase_short",
			chainID: "Test SDF Network",
			valid:   true,
		},
		{
			name:    "invalid_custom_passphrase",
			chainID: "Custom Stellar Network ; 2024",
			valid:   false,
		},
		{
			name:    "invalid_contains_colon_CAIP2",
			chainID: "stellar:Public Global Stellar Network ; September 2015",
			valid:   false,
		},
		{
			name:    "empty_chain_ID",
			chainID: "",
			valid:   false,
		},
		{
			name:    "invalid_passphrase",
			chainID: "abc123",
			valid:   false,
		},
		{
			name:    "invalid_wrong_prefix",
			chainID: "Private Stellar Network ; September 2015",
			valid:   false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := handler.ValidateChainID(tt.chainID)
			if tt.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

//
// Archive mode methods (not supported by Stellar RPC)
//

func TestStellarRpcHandler_SupportsArchiveMode(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})
	assert.False(t, handler.SupportsArchiveMode())
}

func TestStellarRpcHandler_GetArchiveMethod(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})
	assert.Empty(t, handler.GetArchiveMethod())
}

func TestStellarRpcHandler_CreateArchivePayload(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})
	_, err := handler.CreateArchivePayload("", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not require special archive mode")
}

func TestStellarRpcHandler_ParseArchiveResponse(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})
	err := handler.ParseArchiveResponse([]byte{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not require special archive mode")
}

func TestStellarRpcHandler_PerformArchiveCheck(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})
	err := handler.PerformArchiveCheck("", nil, nil, nil, 0, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not support archive mode")
}

//
// Network capabilities
//

func TestStellarRpcHandler_SupportsGetBlockByNumber(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})
	assert.True(t, handler.SupportsGetBlockByNumber())
}

func TestStellarRpcHandler_SupportsDynamicBlockLag(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})
	assert.True(t, handler.SupportsDynamicBlockLag())
}

func TestStellarRpcHandler_GetSupportedMethods(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	methods := handler.GetSupportedMethods()
	expectedMethods := []string{
		"getEvents",
		"getFeeStats",
		"getHealth",
		"getLatestLedger",
		"getLedgerEntries",
		"getLedgers",
		"getNetwork",
		"getTransaction",
		"getTransactions",
		"getVersionInfo",
		"sendTransaction",
		"simulateTransaction",
	}

	assert.Equal(t, len(expectedMethods), len(methods))

	// Check that all expected methods are present
	methodMap := make(map[string]bool)
	for _, method := range methods {
		methodMap[method] = true
	}

	for _, expected := range expectedMethods {
		assert.True(t, methodMap[expected], "Expected method '%s' not found", expected)
	}
}

func TestStellarRpcHandler_GetBlockByNumberMethod(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})
	assert.Equal(t, "getLedgers", handler.GetBlockByNumberMethod())
}

func TestStellarRpcHandler_GetNamespace(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})
	assert.Equal(t, "stellar", handler.GetNamespace())
}

func TestStellarRpcHandler_RequiresSeparateBlockInfoCall(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})
	assert.False(t, handler.RequiresSeparateBlockInfoCall())
}

func TestStellarRpcHandler_GetBlockInfoMethod(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})
	assert.Empty(t, handler.GetBlockInfoMethod())
}

//
// ParseSequenceNumber tests
//

func TestParseSequenceNumber(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    int64
		expectError bool
		errorMsg    string
	}{
		{
			name:     "valid_sequence_as_float64",
			input:    `{"sequence": 51583040}`,
			expected: 51583040,
		},
		{
			name:     "valid_sequence_as_string",
			input:    `{"sequence": "2002985"}`,
			expected: 2002985,
		},
		{
			name:     "valid_sequence_zero",
			input:    `{"sequence": 0}`,
			expected: 0,
		},
		{
			name:     "valid_sequence_large_number",
			input:    `{"sequence": 999999999}`,
			expected: 999999999,
		},
		{
			name:        "missing_sequence_field",
			input:       `{"id": "test", "closeTime": "1234567890"}`,
			expectError: true,
			errorMsg:    "missing sequence field",
		},
		{
			name:        "invalid_json",
			input:       `{invalid json}`,
			expectError: true,
			errorMsg:    "failed to unmarshal",
		},
		{
			name:        "sequence_as_invalid_string",
			input:       `{"sequence": "not-a-number"}`,
			expectError: true,
			errorMsg:    "failed to parse sequence as int64",
		},
		{
			name:        "sequence_as_boolean",
			input:       `{"sequence": true}`,
			expectError: true,
			errorMsg:    "unsupported type for sequence field",
		},
		{
			name:        "sequence_as_null",
			input:       `{"sequence": null}`,
			expectError: true,
			errorMsg:    "unsupported type for sequence field",
		},
		{
			name:        "sequence_as_array",
			input:       `{"sequence": [1, 2, 3]}`,
			expectError: true,
			errorMsg:    "unsupported type for sequence field",
		},
		{
			name:        "sequence_as_object",
			input:       `{"sequence": {"value": 123}}`,
			expectError: true,
			errorMsg:    "unsupported type for sequence field",
		},
		{
			name:        "empty_object",
			input:       `{}`,
			expectError: true,
			errorMsg:    "missing sequence field",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := network.ParseSequenceNumber(json.RawMessage(tt.input))

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestParseSequenceNumber_IntegrationWithGetLatestLedger(t *testing.T) {
	// Test that ParseSequenceNumber works correctly with actual getLatestLedger response
	body := golden.Get(t, "stellar_get_latest_ledger_response.json")

	var response din_http.JSONRPCResponse
	err := json.Unmarshal(body, &response)
	require.NoError(t, err)

	sequence, err := network.ParseSequenceNumber(response.Result)
	require.NoError(t, err)
	assert.Equal(t, int64(51583040), sequence)
}

//
// GetBlockTimestamp tests
//

func TestStellarRpcHandler_GetBlockTimestamp(t *testing.T) {
	body := golden.Get(t, "stellar_get_ledgers_response.json")
	statusCode := http.StatusOK

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cl := din_http.NewMockIHTTPClient(ctrl)
	cl.EXPECT().Post(
		gomock.Any(),
		gomock.Any(),
		gomock.Any(),
		nil,
	).Return(body, &statusCode, nil)

	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	timestamp, err := handler.GetBlockTimestamp("http://test", map[string]string{"Content-Type": "application/json"}, cl, nil, 5, 36233)
	require.NoError(t, err)
	// ledgerCloseTime from golden file is "1734032457"
	assert.Equal(t, int64(1734032457), timestamp)
}

func TestStellarRpcHandler_GetBlockTimestamp_ErrorGettingLedger(t *testing.T) {
	statusCode := http.StatusInternalServerError

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cl := din_http.NewMockIHTTPClient(ctrl)
	cl.EXPECT().Post(
		gomock.Any(),
		gomock.Any(),
		gomock.Any(),
		nil,
	).Return([]byte("error"), &statusCode, nil).Times(5)

	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	_, err := handler.GetBlockTimestamp("http://test", map[string]string{"Content-Type": "application/json"}, cl, nil, 5, 36233)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get ledger 36233")
}

func TestStellarRpcHandler_GetBlockTimestamp_InvalidResponseType(t *testing.T) {
	// Return a response that won't be of type JSONRPCStellarLedgerResponse
	body := []byte(`{"jsonrpc":"2.0","id":1,"result":"invalid"}`)
	statusCode := http.StatusOK

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cl := din_http.NewMockIHTTPClient(ctrl)
	cl.EXPECT().Post(
		gomock.Any(),
		gomock.Any(),
		gomock.Any(),
		nil,
	).Return(body, &statusCode, nil).Times(5)

	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	_, err := handler.GetBlockTimestamp("http://test", map[string]string{"Content-Type": "application/json"}, cl, nil, 5, 36233)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get ledger")
}

func TestStellarRpcHandler_GetBlockTimestamp_NoLedgersInResponse(t *testing.T) {
	// Response with empty ledgers array
	body := []byte(`{"jsonrpc":"2.0","id":1,"result":{"ledgers":[],"latestLedger":36379}}`)
	statusCode := http.StatusOK

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cl := din_http.NewMockIHTTPClient(ctrl)
	cl.EXPECT().Post(
		gomock.Any(),
		gomock.Any(),
		gomock.Any(),
		nil,
	).Return(body, &statusCode, nil)

	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	_, err := handler.GetBlockTimestamp("http://test", map[string]string{"Content-Type": "application/json"}, cl, nil, 5, 36233)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no ledger found in response for sequence 36233")
}

func TestStellarRpcHandler_GetBlockTimestamp_MissingClosedAt(t *testing.T) {
	// Response with ledger but missing ledgerCloseTime
	body := []byte(`{"jsonrpc":"2.0","id":1,"result":{"ledgers":[{"hash":"test","sequence":36233}],"latestLedger":36379}}`)
	statusCode := http.StatusOK

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cl := din_http.NewMockIHTTPClient(ctrl)
	cl.EXPECT().Post(
		gomock.Any(),
		gomock.Any(),
		gomock.Any(),
		nil,
	).Return(body, &statusCode, nil)

	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	_, err := handler.GetBlockTimestamp("http://test", map[string]string{"Content-Type": "application/json"}, cl, nil, 5, 36233)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing closedAt in ledger 36233")
}

func TestStellarRpcHandler_GetBlockTimestamp_InvalidTimestampFormat(t *testing.T) {
	// Response with invalid timestamp format
	body := []byte(`{"jsonrpc":"2.0","id":1,"result":{"ledgers":[{"hash":"test","sequence":36233,"ledgerCloseTime":"invalid-timestamp"}],"latestLedger":36379}}`)
	statusCode := http.StatusOK

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cl := din_http.NewMockIHTTPClient(ctrl)
	cl.EXPECT().Post(
		gomock.Any(),
		gomock.Any(),
		gomock.Any(),
		nil,
	).Return(body, &statusCode, nil)

	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	_, err := handler.GetBlockTimestamp("http://test", map[string]string{"Content-Type": "application/json"}, cl, nil, 5, 36233)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse closedAt timestamp")
}

//
// ParseHealthCheckResponse tests
//

func TestStellarRpcHandler_ParseHealthCheckResponse_Success(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	body := golden.Get(t, "stellar_health_response.json")

	blockInfo, err := handler.ParseHealthCheckResponse(body)
	require.NoError(t, err)
	assert.NotNil(t, blockInfo)
	assert.Equal(t, int64(51583040), blockInfo.Number)
	assert.Equal(t, "", blockInfo.Hash) // Hash is not returned by getHealth
}

func TestStellarRpcHandler_ParseHealthCheckResponse_Unhealthy(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	body := golden.Get(t, "stellar_health_response_unhealthy.json")

	// Even if status is unhealthy, we should still be able to parse the block number
	blockInfo, err := handler.ParseHealthCheckResponse(body)
	require.NoError(t, err)
	assert.NotNil(t, blockInfo)
	assert.Equal(t, int64(51583040), blockInfo.Number)
}

func TestStellarRpcHandler_ParseHealthCheckResponse_JSONRPCError(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	body := golden.Get(t, "stellar_jsonrpc_error.json")

	blockInfo, err := handler.ParseHealthCheckResponse(body)
	require.Error(t, err)
	assert.Nil(t, blockInfo)
	assert.Contains(t, err.Error(), "stellar getHealth error")
	assert.Contains(t, err.Error(), "Invalid request")
}

func TestStellarRpcHandler_ParseHealthCheckResponse_InvalidJSON(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	body := []byte(`{invalid json}`)

	blockInfo, err := handler.ParseHealthCheckResponse(body)
	require.Error(t, err)
	assert.Nil(t, blockInfo)
	assert.Contains(t, err.Error(), "failed to unmarshal JSON-RPC response")
}

func TestStellarRpcHandler_ParseHealthCheckResponse_MissingResult(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	body := []byte(`{"jsonrpc":"2.0","id":1}`)

	// Missing result field is valid JSON but results in zero values
	blockInfo, err := handler.ParseHealthCheckResponse(body)
	require.NoError(t, err)
	assert.NotNil(t, blockInfo)
	assert.Equal(t, int64(0), blockInfo.Number) // Default zero value for missing result
}

func TestStellarRpcHandler_ParseHealthCheckResponse_MissingLatestLedger(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	body := []byte(`{"jsonrpc":"2.0","id":1,"result":{"status":"healthy","oldestLedger":51565760}}`)

	blockInfo, err := handler.ParseHealthCheckResponse(body)
	require.NoError(t, err)
	assert.NotNil(t, blockInfo)
	assert.Equal(t, int64(0), blockInfo.Number) // Default zero value for missing field
}

func TestStellarRpcHandler_ParseHealthCheckResponse_InvalidResultType(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	body := []byte(`{"jsonrpc":"2.0","id":1,"result":"invalid"}`)

	blockInfo, err := handler.ParseHealthCheckResponse(body)
	require.Error(t, err)
	assert.Nil(t, blockInfo)
	assert.Contains(t, err.Error(), "failed to unmarshal Stellar health check response")
}

//
// ParseBlockNumberResponse tests
//

func TestStellarRpcHandler_ParseBlockNumberResponse_Success(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	body := golden.Get(t, "stellar_health_response.json")
	statusCode := http.StatusOK

	blockNumber, err := handler.ParseBlockNumberResponse(body, statusCode)
	require.NoError(t, err)
	assert.Equal(t, int64(51583040), blockNumber)
}

func TestStellarRpcHandler_ParseBlockNumberResponse_Unhealthy(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	body := golden.Get(t, "stellar_health_response_unhealthy.json")
	statusCode := http.StatusOK

	// Should still return block number even if unhealthy
	blockNumber, err := handler.ParseBlockNumberResponse(body, statusCode)
	require.NoError(t, err)
	assert.Equal(t, int64(51583040), blockNumber)
}

func TestStellarRpcHandler_ParseBlockNumberResponse_JSONRPCError(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	body := golden.Get(t, "stellar_jsonrpc_error.json")
	statusCode := http.StatusOK

	blockNumber, err := handler.ParseBlockNumberResponse(body, statusCode)
	require.Error(t, err)
	assert.Equal(t, int64(0), blockNumber)
	assert.Contains(t, err.Error(), "failed to parse Stellar health check response")
}

func TestStellarRpcHandler_ParseBlockNumberResponse_HTTPError(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	body := []byte(`{"error": "Internal Server Error"}`)
	statusCode := http.StatusInternalServerError

	blockNumber, err := handler.ParseBlockNumberResponse(body, statusCode)
	require.Error(t, err)
	assert.Equal(t, int64(0), blockNumber)
	assert.Contains(t, err.Error(), "error status code: 500")
}

func TestStellarRpcHandler_ParseBlockNumberResponse_RateLimitError(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	body := []byte(`{"error": "Too Many Requests"}`)
	statusCode := http.StatusTooManyRequests

	blockNumber, err := handler.ParseBlockNumberResponse(body, statusCode)
	require.Error(t, err)
	assert.Equal(t, int64(0), blockNumber)
	assert.Contains(t, err.Error(), "rate limit error")
}

func TestStellarRpcHandler_ParseBlockNumberResponse_InvalidJSON(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	body := []byte(`{invalid json}`)
	statusCode := http.StatusOK

	blockNumber, err := handler.ParseBlockNumberResponse(body, statusCode)
	require.Error(t, err)
	assert.Equal(t, int64(0), blockNumber)
	assert.Contains(t, err.Error(), "failed to parse Stellar health check response")
}

func TestStellarRpcHandler_ParseBlockNumberResponse_MissingLatestLedger(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	body := []byte(`{"jsonrpc":"2.0","id":1,"result":{"status":"healthy","oldestLedger":51565760}}`)
	statusCode := http.StatusOK

	blockNumber, err := handler.ParseBlockNumberResponse(body, statusCode)
	require.NoError(t, err)
	assert.Equal(t, int64(0), blockNumber) // Default zero value
}

func TestStellarRpcHandler_ParseBlockNumberResponse_ClientError(t *testing.T) {
	handler := network.NewStellarRpcHandler(&network.NetworkConfig{})

	body := []byte(`{"error": "Bad Request"}`)
	statusCode := http.StatusBadRequest

	blockNumber, err := handler.ParseBlockNumberResponse(body, statusCode)
	require.Error(t, err)
	assert.Equal(t, int64(0), blockNumber)
	assert.Contains(t, err.Error(), "error status code: 400")
}
