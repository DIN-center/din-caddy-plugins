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
		ChainID: "bitcoin:mainnet",
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
			path:      "/api/blocks/tip/height",
			expectErr: false,
		},
		{
			name:      "valid POST request for tx broadcast",
			method:    "POST",
			path:      "/api/tx",
			headers:   map[string]string{"Content-Type": "text/plain"},
			expectErr: false,
		},
		{
			name:      "invalid method PUT",
			method:    "PUT",
			path:      "/api/blocks/tip/height",
			expectErr: true,
		},
		{
			name:      "invalid path without /api/",
			method:    "GET",
			path:      "/blocks/tip/height",
			expectErr: true,
		},
		{
			name:      "invalid content type for tx broadcast",
			method:    "POST",
			path:      "/api/tx",
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

func TestBitcoinEsploraHandler_NormalizeEndpoint(t *testing.T) {
	handler := NewBitcoinEsploraHandler(&NetworkConfig{})

	tests := []struct {
		path     string
		expected string
	}{
		{
			path:     "/api/tx/1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			expected: "/api/tx/{txid}",
		},
		{
			path:     "/api/address/1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			expected: "/api/address/{address}",
		},
		{
			path:     "/api/block/00000000000000000007878ec04bb2b2e12317804810f4c26033585b3f81ffaa",
			expected: "/api/block/{hash}",
		},
		{
			path:     "/api/block-height/123456",
			expected: "/api/block-height/{height}",
		},
		{
			path:     "/api/blocks/tip/height",
			expected: "/api/blocks/tip/height",
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

	assert.Equal(t, "/api/blocks/tip/height", handler.GetHealthCheckMethod())
	assert.Equal(t, "GET", handler.GetHealthCheckHTTPMethod())
	assert.True(t, handler.RequiresSeparateBlockInfoCall())
	assert.Equal(t, "/api/blocks/tip/hash", handler.GetBlockInfoMethod())
}

func TestBitcoinEsploraHandler_ParseHealthCheckResponse(t *testing.T) {
	handler := NewBitcoinEsploraHandler(&NetworkConfig{})

	// Test valid response
	body := []byte("123456")
	blockInfo, err := handler.ParseHealthCheckResponse(body)
	require.NoError(t, err)
	assert.Equal(t, int64(123456), blockInfo.Number)

	// Test invalid response
	body = []byte("invalid")
	_, err = handler.ParseHealthCheckResponse(body)
	assert.Error(t, err)
}

func TestBitcoinEsploraHandler_ParseBlockResponse(t *testing.T) {
	handler := NewBitcoinEsploraHandler(&NetworkConfig{})

	// Test valid block response
	body := []byte(`{"id": "00000000000000000007878ec04bb2b2e12317804810f4c26033585b3f81ffaa", "height": 123456}`)
	block, err := handler.ParseBlockResponse(body)
	require.NoError(t, err)

	blockMap, ok := block.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "00000000000000000007878ec04bb2b2e12317804810f4c26033585b3f81ffaa", blockMap["id"])

	// Test block hash extraction
	hash := handler.ExtractBlockHash(block)
	assert.Equal(t, "00000000000000000007878ec04bb2b2e12317804810f4c26033585b3f81ffaa", hash)
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