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

	// Test that POST requests no longer return HTTPError with 405 status code
	req, err := http.NewRequest("POST", "https://example.com/tx", nil)
	require.NoError(t, err)

	err = handler.ValidateRequest(req)
	require.NoError(t, err)
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


func TestEndpointID_CoversAllRegexRoutes(t *testing.T) {
	// All cases use a non-empty prefix to assert we match by suffix (…$) not exact-path.
	prefix := "/api/v1/esplora"

	tests := []struct {
		name   string
		method string
		path   string
		want   string
	}{
		// ---- Transactions ----
		{"GET_TX", "GET", prefix + "/tx/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "GET_TX"},
		{"GET_TX_STATUS", "GET", prefix + "/tx/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb/status", "GET_TX_STATUS"},
		{"GET_TX_HEX", "GET", prefix + "/tx/cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc/hex", "GET_TX_HEX"},
		{"GET_TX_RAW", "GET", prefix + "/tx/dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd/raw", "GET_TX_RAW"},
		{"GET_TX_MERKLEBLOCK_PROOF", "GET", prefix + "/tx/eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee/merkleblock-proof", "GET_TX_MERKLEBLOCK_PROOF"},
		{"GET_TX_MERKLE_PROOF", "GET", prefix + "/tx/ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff/merkle-proof", "GET_TX_MERKLE_PROOF"},
		{"GET_TX_OUTSPEND_VOUT", "GET", prefix + "/tx/1111111111111111111111111111111111111111111111111111111111111111/outspend/0", "GET_TX_OUTSPEND_VOUT"},
		{"GET_TX_OUTSPENDS", "GET", prefix + "/tx/2222222222222222222222222222222222222222222222222222222222222222/outspends", "GET_TX_OUTSPENDS"},
		{"POST_TX_BROADCAST", "POST", prefix + "/tx", "POST_TX_BROADCAST"},
		{"POST_TXS_PACKAGE", "POST", prefix + "/txs/package", "POST_TXS_PACKAGE"},

		// ---- Addresses / Scripthash ----
		{"GET_ADDRESS", "GET", prefix + "/address/bc1qexampleaddress0000000000000000000000000000000000", "GET_ADDRESS"},
		{"GET_SCRIPTHASH", "GET", prefix + "/scripthash/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", "GET_SCRIPTHASH"},
		{"GET_ADDRESS_TXS", "GET", prefix + "/address/bc1qexampleaddress0000000000000000000000000000000000/txs", "GET_ADDRESS_TXS"},
		{"GET_SCRIPTHASH_TXS", "GET", prefix + "/scripthash/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef/txs", "GET_SCRIPTHASH_TXS"},
		{"GET_ADDRESS_TXS_CHAIN", "GET", prefix + "/address/bc1qexampleaddress0000000000000000000000000000000000/txs/chain", "GET_ADDRESS_TXS_CHAIN"},
		{"GET_ADDRESS_TXS_CHAIN_PAGINATED", "GET", prefix + "/address/bc1qexampleaddress0000000000000000000000000000000000/txs/chain/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "GET_ADDRESS_TXS_CHAIN"},
		{"GET_SCRIPTHASH_TXS_CHAIN", "GET", prefix + "/scripthash/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef/txs/chain", "GET_SCRIPTHASH_TXS_CHAIN"},
		{"GET_SCRIPTHASH_TXS_CHAIN_PAGINATED", "GET", prefix + "/scripthash/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef/txs/chain/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "GET_SCRIPTHASH_TXS_CHAIN"},
		{"GET_ADDRESS_TXS_MEMPOOL", "GET", prefix + "/address/bc1qexampleaddress0000000000000000000000000000000000/txs/mempool", "GET_ADDRESS_TXS_MEMPOOL"},
		{"GET_SCRIPTHASH_TXS_MEMPOOL", "GET", prefix + "/scripthash/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef/txs/mempool", "GET_SCRIPTHASH_TXS_MEMPOOL"},
		{"GET_ADDRESS_UTXO", "GET", prefix + "/address/bc1qexampleaddress0000000000000000000000000000000000/utxo", "GET_ADDRESS_UTXO"},
		{"GET_SCRIPTHASH_UTXO", "GET", prefix + "/scripthash/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef/utxo", "GET_SCRIPTHASH_UTXO"},
		{"GET_ADDRESS_PREFIX", "GET", prefix + "/address-prefix/bc1q", "GET_ADDRESS_PREFIX"},

		// ---- Blocks ----
		{"GET_BLOCK", "GET", prefix + "/block/0000000000000000000000000000000000000000000000000000000000000000", "GET_BLOCK"},
		{"GET_BLOCK_HEADER", "GET", prefix + "/block/0000000000000000000000000000000000000000000000000000000000000000/header", "GET_BLOCK_HEADER"},
		{"GET_BLOCK_STATUS", "GET", prefix + "/block/0000000000000000000000000000000000000000000000000000000000000000/status", "GET_BLOCK_STATUS"},
		{"GET_BLOCK_TXS", "GET", prefix + "/block/0000000000000000000000000000000000000000000000000000000000000000/txs", "GET_BLOCK_TXS"},
		{"GET_BLOCK_TXS_PAGINATED", "GET", prefix + "/block/0000000000000000000000000000000000000000000000000000000000000000/txs/25", "GET_BLOCK_TXS"},
		{"GET_BLOCK_TXIDS", "GET", prefix + "/block/0000000000000000000000000000000000000000000000000000000000000000/txids", "GET_BLOCK_TXIDS"},
		{"GET_BLOCK_TXID_INDEX", "GET", prefix + "/block/0000000000000000000000000000000000000000000000000000000000000000/txid/7", "GET_BLOCK_TXID_INDEX"},
		{"GET_BLOCK_RAW", "GET", prefix + "/block/0000000000000000000000000000000000000000000000000000000000000000/raw", "GET_BLOCK_RAW"},
		{"GET_BLOCK_HEIGHT", "GET", prefix + "/block-height/840000", "GET_BLOCK_HEIGHT"},
		{"GET_BLOCKS", "GET", prefix + "/blocks", "GET_BLOCKS"},
		{"GET_BLOCKS_START", "GET", prefix + "/blocks/839000", "GET_BLOCKS"},
		{"GET_BLOCKS_TIP_HEIGHT", "GET", prefix + "/blocks/tip/height", "GET_BLOCKS_TIP_HEIGHT"},
		{"GET_BLOCKS_TIP_HASH", "GET", prefix + "/blocks/tip/hash", "GET_BLOCKS_TIP_HASH"},

		// ---- Mempool / Fees ----
		{"GET_MEMPOOL", "GET", prefix + "/mempool", "GET_MEMPOOL"},
		{"GET_MEMPOOL_TXIDS", "GET", prefix + "/mempool/txids", "GET_MEMPOOL_TXIDS"},
		{"GET_MEMPOOL_RECENT", "GET", prefix + "/mempool/recent", "GET_MEMPOOL_RECENT"},
		{"GET_FEE_ESTIMATES", "GET", prefix + "/fee-estimates", "GET_FEE_ESTIMATES"},

		// ---- Assets (Elements/Liquid only) ----
		{"GET_ASSET", "GET", prefix + "/asset/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "GET_ASSET"},
		{"GET_ASSET_TXS", "GET", prefix + "/asset/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/txs", "GET_ASSET_TXS"},
		{"GET_ASSET_TXS_MEMPOOL", "GET", prefix + "/asset/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/txs/mempool", "GET_ASSET_TXS_MEMPOOL"},
		{"GET_ASSET_TXS_CHAIN", "GET", prefix + "/asset/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/txs/chain", "GET_ASSET_TXS_CHAIN"},
		{"GET_ASSET_TXS_CHAIN_PAGINATED", "GET", prefix + "/asset/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/txs/chain/ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", "GET_ASSET_TXS_CHAIN"},
		{"GET_ASSET_SUPPLY", "GET", prefix + "/asset/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/supply", "GET_ASSET_SUPPLY"},
		{"GET_ASSET_SUPPLY_DECIMAL", "GET", prefix + "/asset/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/supply/decimal", "GET_ASSET_SUPPLY_DECIMAL"},
		{"GET_ASSETS_REGISTRY", "GET", prefix + "/assets/registry", "GET_ASSETS_REGISTRY"},

		// ---- Unknown ----
		{"UNKNOWN", "GET", prefix + "/nope/not-a-route", "UNKNOWN"},
		{"UNKNOWN_wrong_method", "PUT", prefix + "/tx", "UNKNOWN"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if got := EndpointID(tt.method, tt.path); got != tt.want {
				t.Fatalf("EndpointID(%q, %q) = %q, want %q", tt.method, tt.path, got, tt.want)
			}
		})
	}
}
