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

// Test helper functions
func Test_isHexHash(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid lowercase hash", "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", true},
		{"valid uppercase hash", "1234567890ABCDEF1234567890ABCDEF1234567890ABCDEF1234567890ABCDEF", true},
		{"valid mixed case hash", "1234567890AbCdEf1234567890AbCdEf1234567890AbCdEf1234567890AbCdEf", true},
		{"too short", "1234567890abcdef", false},
		{"too long", "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef00", false},
		{"non-hex characters", "123456789ghijklm1234567890abcdef1234567890abcdef1234567890abcdef", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isHexHash(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_isBitcoinAddress(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Legacy P2PKH (starts with 1)
		{"valid P2PKH", "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", true},
		{"valid P2PKH short", "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2", true},

		// P2SH (starts with 3)
		{"valid P2SH", "3J98t1WpEZ73CNmYviecrnyiWrnqRhWNLy", true},
		{"valid P2SH alt", "3QJmV3qfvL9SuYo34YihAf3sRCW3qSinyC", true},

		// Bech32 SegWit v0 (starts with bc1q)
		{"valid Bech32 mainnet", "bc1qar0srrr7xfkvy5l643lydnw9re59gtzzwf5mdq", true},
		{"valid Bech32 testnet", "tb1q5cymx5zvdh3cpnqpj4vkp4m5j3d6x5zt8mqdj2", true},
		{"valid Bech32 regtest", "bcrt1qrp33g0q5c5txsp9arysrx4k6zdkfs4nce4xj0g", true},

		// Bech32m Taproot (starts with bc1p)
		{"valid Bech32m mainnet", "bc1p5d7rjq7g6rdk2yhzks9smlaqtedr4dekq08ge8ztwac72sfr9rusxg3297", true},
		{"valid Bech32m testnet", "tb1p5d7rjq7g6rdk2yhzks9smlaqtedr4dekq08ge8ztwac72sfr9ruso49ar4", true},
		{"valid Bech32m regtest", "bcrt1p5d7rjq7g6rdk2yhzks9smlaqtedr4dekq08ge8ztwac72sfr9rusmze9qd", true},

		// Invalid addresses
		{"invalid starts with 2", "2MzQwSSnBHWHqSAqtTVQ6v47XtaisrJa1Vc", false},
		{"invalid starts with 0", "0BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2", false},
		{"invalid Bech32 uppercase", "BC1QAR0SRRR7XFKVY5L643LYDNW9RE59GTZZWF5MDQ", false},
		{"invalid too short", "1A1z", false},
		{"invalid hex hash", "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isBitcoinAddress(tt.input)
			assert.Equal(t, tt.expected, result, "Address: %s", tt.input)
		})
	}
}

func Test_isNumeric(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid single digit", "0", true},
		{"valid multi digit", "123456", true},
		{"valid large number", "999999999", true},
		{"invalid with letters", "123abc", false},
		{"invalid with spaces", "123 456", false},
		{"invalid negative", "-123", false},
		{"invalid decimal", "123.456", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNumeric(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBitcoinEsploraHandler_NormalizeEndpoint(t *testing.T) {
	handler := NewBitcoinEsploraHandler(&NetworkConfig{})

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		// Transaction endpoints
		{
			name:     "transaction by txid",
			path:     "/tx/1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			expected: "/tx/{txid}",
		},
		{
			name:     "transaction status",
			path:     "/tx/abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890/status",
			expected: "/tx/{txid}/status",
		},
		{
			name:     "transaction hex",
			path:     "/tx/a914748284390f9e263a4b766a75d0633c50426eb87587a914748284390f9e26/hex",
			expected: "/tx/{txid}/hex",
		},
		{
			name:     "transaction outspend",
			path:     "/tx/1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef/outspend/2",
			expected: "/tx/{txid}/outspend/{vout}",
		},

		// Address endpoints - Legacy P2PKH
		{
			name:     "P2PKH address",
			path:     "/address/1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			expected: "/address/{address}",
		},
		{
			name:     "P2PKH address txs",
			path:     "/address/1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2/txs",
			expected: "/address/{address}/txs",
		},
		{
			name:     "P2PKH address txs chain",
			path:     "/address/1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa/txs/chain",
			expected: "/address/{address}/txs/chain",
		},
		{
			name:     "P2PKH address txs chain with last_seen",
			path:     "/address/1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa/txs/chain/abc123def456abc123def456abc123def456abc123def456abc123def456abc1",
			expected: "/address/{address}/txs/chain/{last_seen_txid}",
		},

		// Address endpoints - P2SH
		{
			name:     "P2SH address",
			path:     "/address/3J98t1WpEZ73CNmYviecrnyiWrnqRhWNLy",
			expected: "/address/{address}",
		},
		{
			name:     "P2SH address utxo",
			path:     "/address/3J98t1WpEZ73CNmYviecrnyiWrnqRhWNLy/utxo",
			expected: "/address/{address}/utxo",
		},

		// Address endpoints - Bech32
		{
			name:     "Bech32 address",
			path:     "/address/bc1qar0srrr7xfkvy5l643lydnw9re59gtzzwf5mdq",
			expected: "/address/{address}",
		},
		{
			name:     "Bech32 testnet address",
			path:     "/address/tb1q5cymx5zvdh3cpnqpj4vkp4m5j3d6x5zt8mqdj2/txs/mempool",
			expected: "/address/{address}/txs/mempool",
		},

		// Address endpoints - Bech32m Taproot
		{
			name:     "Bech32m address",
			path:     "/address/bc1p5d7rjq7g6rdk2yhzks9smlaqtedr4dekq08ge8ztwac72sfr9rusxg3297",
			expected: "/address/{address}",
		},

		// Script hash endpoints
		{
			name:     "scripthash",
			path:     "/scripthash/20fd0a38027a2eeb14fd50fcbd94934f832bef4cc279958c30c72704338eb065",
			expected: "/scripthash/{scripthash}",
		},
		{
			name:     "scripthash txs",
			path:     "/scripthash/20fd0a38027a2eeb14fd50fcbd94934f832bef4cc279958c30c72704338eb065/txs",
			expected: "/scripthash/{scripthash}/txs",
		},
		{
			name:     "scripthash utxo",
			path:     "/scripthash/a914748284390f9e263a4b766a75d0633c50426eb87587a914748284390f9e26/utxo",
			expected: "/scripthash/{scripthash}/utxo",
		},

		// Block endpoints
		{
			name:     "block by hash",
			path:     "/block/00000000000000000007878ec04bb2b2e12317804810f4c26033585b3f81ffaa",
			expected: "/block/{hash}",
		},
		{
			name:     "block header",
			path:     "/block/00000000000000000007878ec04bb2b2e12317804810f4c26033585b3f81ffaa/header",
			expected: "/block/{hash}/header",
		},
		{
			name:     "block txs",
			path:     "/block/00000000000000000007878ec04bb2b2e12317804810f4c26033585b3f81ffaa/txs",
			expected: "/block/{hash}/txs",
		},
		{
			name:     "block txs with index",
			path:     "/block/00000000000000000007878ec04bb2b2e12317804810f4c26033585b3f81ffaa/txs/25",
			expected: "/block/{hash}/txs/{start_index}",
		},
		{
			name:     "block txid by index",
			path:     "/block/00000000000000000007878ec04bb2b2e12317804810f4c26033585b3f81ffaa/txid/15",
			expected: "/block/{hash}/txid/{index}",
		},
		{
			name:     "block by height",
			path:     "/block-height/123456",
			expected: "/block-height/{height}",
		},
		{
			name:     "blocks with start height",
			path:     "/blocks/750000",
			expected: "/blocks/{start_height}",
		},

		// Static endpoints (should not be normalized)
		{
			name:     "blocks tip height",
			path:     "/blocks/tip/height",
			expected: "/blocks/tip/height",
		},
		{
			name:     "blocks tip hash",
			path:     "/blocks/tip/hash",
			expected: "/blocks/tip/hash",
		},
		{
			name:     "mempool",
			path:     "/mempool",
			expected: "/mempool",
		},
		{
			name:     "mempool txids",
			path:     "/mempool/txids",
			expected: "/mempool/txids",
		},
		{
			name:     "fee estimates",
			path:     "/fee-estimates",
			expected: "/fee-estimates",
		},

		// Address prefix search
		{
			name:     "address prefix search",
			path:     "/address-prefix/1A1z",
			expected: "/address-prefix/{prefix}",
		},

		// Edge cases
		{
			name:     "empty path",
			path:     "",
			expected: "",
		},
		{
			name:     "root path",
			path:     "/",
			expected: "/",
		},
		{
			name:     "network prefix",
			path:     "/bitcoin-mainnet-esplora/scripthash/20fd0a38027a2eeb14fd50fcbd94934f832bef4cc279958c30c72704338eb065/txs",
			expected: "/bitcoin-mainnet-esplora/scripthash/{scripthash}/txs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.NormalizeEndpoint(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBitcoinEsploraHandler_ExtractMethod(t *testing.T) {
	handler := NewBitcoinEsploraHandler(&NetworkConfig{})

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "transaction endpoint",
			path:     "/tx/1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			expected: "/tx/{txid}",
		},
		{
			name:     "address endpoint",
			path:     "/address/1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa/txs",
			expected: "/address/{address}/txs",
		},
		{
			name:     "block endpoint",
			path:     "/block/00000000000000000007878ec04bb2b2e12317804810f4c26033585b3f81ffaa/txs/25",
			expected: "/block/{hash}/txs/{start_index}",
		},
		{
			name:     "static endpoint",
			path:     "/blocks/tip/height",
			expected: "/blocks/tip/height",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", "https://example.com"+tt.path, nil)
			require.NoError(t, err)

			result, err := handler.ExtractMethod(req, nil)
			require.NoError(t, err)
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
