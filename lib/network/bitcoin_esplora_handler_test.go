package network

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBitcoinEsploraHandler_GetType(t *testing.T) {
	handler := NewBitcoinEsploraHandler(&NetworkConfig{})
	assert.Equal(t, "bitcoin-esplora", handler.GetType())
}

func TestBitcoinEsploraHandler_GetName(t *testing.T) {
	handler := NewBitcoinEsploraHandler(&NetworkConfig{})
	assert.Equal(t, "Bitcoin Esplora Handler", handler.GetName())
}

func TestBitcoinEsploraHandler_GetRequestType(t *testing.T) {
	handler := NewBitcoinEsploraHandler(&NetworkConfig{})
	assert.Equal(t, RequestTypeREST, handler.GetRequestType())
}

func TestBitcoinEsploraHandler_Initialize(t *testing.T) {
	config := &NetworkConfig{
		Name:    "bitcoin-test",
		Type:    "bitcoin-esplora",
		ChainID: "mainnet",
	}

	handler := NewBitcoinEsploraHandler(config)
	err := handler.Initialize(config)
	assert.NoError(t, err)
	assert.Equal(t, config, handler.config)
}

func TestBitcoinEsploraHandler_ValidateRequest(t *testing.T) {
	handler := NewBitcoinEsploraHandler(&NetworkConfig{})

	tests := []struct {
		name      string
		method    string
		path      string
		headers   map[string]string
		expectErr bool
	}{
		{
			name:      "valid GET request",
			method:    "GET",
			path:      "/blocks/tip/height",
			expectErr: false,
		},
		{
			name:      "blocked POST request for tx broadcast",
			method:    "POST",
			path:      "/tx",
			headers:   map[string]string{"Content-Type": "text/plain"},
			expectErr: true,
		},
		{
			name:      "invalid method PUT",
			method:    "PUT",
			path:      "/blocks/tip/height",
			expectErr: true,
		},
		{
			name:      "blocked POST request with invalid content type",
			method:    "POST",
			path:      "/tx",
			headers:   map[string]string{"Content-Type": "application/json"},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, "https://example.com"+tt.path, nil)
			require.NoError(t, err)

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			err = handler.ValidateRequest(req)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBitcoinEsploraHandler_ValidateRequest_HTTPError(t *testing.T) {
	handler := NewBitcoinEsploraHandler(&NetworkConfig{})

	// Test that POST requests return HTTPError with 405 status code
	req, err := http.NewRequest("POST", "https://example.com/tx", nil)
	require.NoError(t, err)

	err = handler.ValidateRequest(req)
	require.Error(t, err)

	// Check that it's an HTTPError with the correct status code
	httpErr, ok := err.(*HTTPError)
	require.True(t, ok, "Expected HTTPError type")
	assert.Equal(t, http.StatusMethodNotAllowed, httpErr.StatusCode)
	assert.Equal(t, "POST method not allowed for Bitcoin Esplora API", httpErr.Message)
}

func TestBitcoinEsploraHandler_NormalizeEndpoint(t *testing.T) {
	handler := NewBitcoinEsploraHandler(&NetworkConfig{})

	tests := []struct {
		path     string
		expected string
	}{
		{
			path:     "/tx/1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			expected: "/tx/1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		},
		{
			path:     "/address/1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			expected: "/address/1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		},
		{
			path:     "/block/00000000000000000007878ec04bb2b2e12317804810f4c26033585b3f81ffaa",
			expected: "/block/00000000000000000007878ec04bb2b2e12317804810f4c26033585b3f81ffaa",
		},
		{
			path:     "/block-height/123456",
			expected: "/block-height/123456",
		},
		{
			path:     "/blocks/tip/height",
			expected: "/blocks/tip/height",
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := handler.NormalizeEndpoint(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBitcoinEsploraHandler_IsRetryableError(t *testing.T) {
	handler := NewBitcoinEsploraHandler(&NetworkConfig{})

	tests := []struct {
		name       string
		err        error
		statusCode int
		expected   bool
	}{
		{
			name:       "server error 500",
			err:        nil,
			statusCode: 500,
			expected:   true,
		},
		{
			name:       "rate limit 429",
			err:        nil,
			statusCode: 429,
			expected:   true,
		},
		{
			name:       "client error 400",
			err:        nil,
			statusCode: 400,
			expected:   false,
		},
		{
			name:       "not found 404",
			err:        nil,
			statusCode: 404,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.IsRetryableError(tt.err, tt.statusCode)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBitcoinEsploraHandler_HealthCheckMethods(t *testing.T) {
	handler := NewBitcoinEsploraHandler(&NetworkConfig{})

	assert.Equal(t, "/blocks/tip/height", handler.GetHealthCheckMethod())
	assert.Equal(t, "GET", handler.GetHealthCheckHTTPMethod())
	assert.True(t, handler.RequiresSeparateBlockInfoCall())
	assert.Equal(t, "/blocks/tip/hash", handler.GetBlockInfoMethod())
}

func TestBitcoinEsploraHandler_ParseHealthCheckResponse(t *testing.T) {
	handler := NewBitcoinEsploraHandler(&NetworkConfig{})

	// Test valid response with the actual format from enterprise.blockstream.info
	body := []byte("908752")
	blockInfo, err := handler.ParseHealthCheckResponse(body)
	require.NoError(t, err)
	assert.Equal(t, int64(908752), blockInfo.Number)
	assert.NotZero(t, blockInfo.Timestamp)

	// Test another valid response
	body = []byte("123456")
	blockInfo, err = handler.ParseHealthCheckResponse(body)
	require.NoError(t, err)
	assert.Equal(t, int64(123456), blockInfo.Number)

	// Test invalid response
	body = []byte("invalid")
	_, err = handler.ParseHealthCheckResponse(body)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse block height")

	// Test empty response
	body = []byte("")
	_, err = handler.ParseHealthCheckResponse(body)
	assert.Error(t, err)
}

func TestBitcoinEsploraHandler_ParseBlockResponse(t *testing.T) {
	handler := NewBitcoinEsploraHandler(&NetworkConfig{})

	// Test with full block response matching the actual Esplora format
	body := []byte(`{
		"id": "000000000000000000015bab48eb378f6327ead7d07615d11d5f14a701ce9f14",
		"height": 908747,
		"version": 723279872,
		"timestamp": 1754411308,
		"tx_count": 3559,
		"size": 1757871,
		"weight": 3993702,
		"merkle_root": "2f6dd2d049cf681a55c342a7fdab4eab38e0c7bafa41c65c510c93c5e645ad4a",
		"previousblockhash": "000000000000000000020b810fd6ad7348f0cf28e5913a4ac3e2042716d1a63b",
		"mediantime": 1754408985,
		"nonce": 1659613267,
		"bits": 386020510,
		"difficulty": 127620086886391.78
	}`)

	block, err := handler.ParseBlockResponse(body)
	require.NoError(t, err)

	// Type assert to our struct
	bitcoinBlock, ok := block.(BitcoinEsploraBlock)
	require.True(t, ok, "Expected BitcoinEsploraBlock type")

	// Verify parsed fields
	assert.Equal(t, "000000000000000000015bab48eb378f6327ead7d07615d11d5f14a701ce9f14", bitcoinBlock.ID)
	assert.Equal(t, int64(908747), bitcoinBlock.Height)
	assert.Equal(t, int64(723279872), bitcoinBlock.Version)
	assert.Equal(t, int64(1754411308), bitcoinBlock.Timestamp)
	assert.Equal(t, 3559, bitcoinBlock.TxCount)
	assert.Equal(t, 1757871, bitcoinBlock.Size)
	assert.Equal(t, 3993702, bitcoinBlock.Weight)
	assert.Equal(t, "2f6dd2d049cf681a55c342a7fdab4eab38e0c7bafa41c65c510c93c5e645ad4a", bitcoinBlock.MerkleRoot)
	assert.Equal(t, "000000000000000000020b810fd6ad7348f0cf28e5913a4ac3e2042716d1a63b", bitcoinBlock.PreviousBlockHash)
	assert.Equal(t, int64(1754408985), bitcoinBlock.MedianTime)
	assert.Equal(t, int64(1659613267), bitcoinBlock.Nonce)
	assert.Equal(t, int64(386020510), bitcoinBlock.Bits)
	assert.Equal(t, 127620086886391.78, bitcoinBlock.Difficulty)

	// Test block hash extraction
	hash := handler.ExtractBlockHash(block)
	assert.Equal(t, "000000000000000000015bab48eb378f6327ead7d07615d11d5f14a701ce9f14", hash)

	// Test invalid JSON
	invalidBody := []byte(`{"invalid": json}`)
	_, err = handler.ParseBlockResponse(invalidBody)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse Bitcoin block response")
}

func TestBitcoinEsploraHandler_UnsupportedOperations(t *testing.T) {
	handler := NewBitcoinEsploraHandler(&NetworkConfig{})

	// Test unsupported archive mode
	assert.False(t, handler.SupportsArchiveMode())
	assert.Equal(t, "", handler.GetArchiveMethod())

	_, err := handler.CreateArchivePayload("method", "12345")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "archive mode not supported")

	err = handler.ParseArchiveResponse([]byte{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "archive mode not supported")

	// Test that CreateBlockRequest returns error (REST API doesn't use JSON-RPC)
	_, err = handler.CreateBlockRequest("method", 12345, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "REST API, not JSON-RPC")
}

func TestBitcoinEsploraHandler_ChainID(t *testing.T) {
	config := &NetworkConfig{
		ChainID: "bitcoin:mainnet",
	}
	handler := NewBitcoinEsploraHandler(config)

	// Bitcoin doesn't have dynamic chain ID
	assert.Equal(t, "", handler.GetChainIDMethod())

	// ValidateChainID accepts any chain ID
	err := handler.ValidateChainID("bitcoin:testnet")
	assert.NoError(t, err)

	// ParseChainIDResponse returns configured chain ID
	chainID, err := handler.ParseChainIDResponse([]byte{}, 200)
	assert.NoError(t, err)
	assert.Equal(t, "bitcoin:mainnet", chainID)
}
