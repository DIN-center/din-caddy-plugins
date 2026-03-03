// lib/network/bitcoin_esplora_handler.go
package network

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
)

// BitcoinEsploraBlock represents a Bitcoin block from Esplora API
type BitcoinEsploraBlock struct {
	ID                string  `json:"id"`
	Height            int64   `json:"height"`
	Version           int64   `json:"version"`
	Timestamp         int64   `json:"timestamp"`
	TxCount           int     `json:"tx_count"`
	Size              int     `json:"size"`
	Weight            int     `json:"weight"`
	MerkleRoot        string  `json:"merkle_root"`
	PreviousBlockHash string  `json:"previousblockhash"`
	MedianTime        int64   `json:"mediantime"`
	Nonce             int64   `json:"nonce"`
	Bits              int64   `json:"bits"`
	Difficulty        float64 `json:"difficulty"`
}

var _ NetworkHandler = (*BitcoinEsploraHandler)(nil)

// BitcoinEsploraHandler handles Bitcoin Esplora REST API requests
type BitcoinEsploraHandler struct {
	config              *NetworkConfig
	healthCheckEndpoint string
	blockInfoEndpoint   string
	version             string
	logger              *logger.LoggerClient
}

// NewBitcoinEsploraHandler creates a new Bitcoin Esplora handler instance
func NewBitcoinEsploraHandler(config *NetworkConfig) *BitcoinEsploraHandler {
	return &BitcoinEsploraHandler{
		config:              config,
		healthCheckEndpoint: "/blocks/tip/height",
		blockInfoEndpoint:   "/blocks/tip/hash",
		version:             "1.0.0",
		logger:              config.Logger,
	}
}

// Metadata methods for registry
func (h *BitcoinEsploraHandler) GetType() string {
	return "bitcoin-esplora"
}

func (h *BitcoinEsploraHandler) GetName() string {
	return "Bitcoin Esplora Handler"
}

func (h *BitcoinEsploraHandler) GetVersion() string {
	return h.version
}

func (h *BitcoinEsploraHandler) GetRequestType() RequestType {
	return RequestTypeREST
}

// Lifecycle methods
func (h *BitcoinEsploraHandler) Initialize(config *NetworkConfig) error {
	h.config = config

	// Update logger from config if available
	if config.Logger != nil {
		h.logger = config.Logger
	}
	return nil
}

func (h *BitcoinEsploraHandler) Shutdown() error {
	// Cleanup resources if needed
	return nil
}

// === EXISTING METHODS ===

// Request processing methods
func (h *BitcoinEsploraHandler) ProcessRequest(req *http.Request) error {
	// Validate the request, reject POST requests
	if err := h.ValidateRequest(req); err != nil {
		return err
	}

	return nil
}

var (
	reTx                = regexp.MustCompile(`/tx/[^/]+$`)
	reTxStatus          = regexp.MustCompile(`/tx/[^/]+/status$`)
	reTxHex             = regexp.MustCompile(`/tx/[^/]+/hex$`)
	reTxRaw             = regexp.MustCompile(`/tx/[^/]+/raw$`)
	reTxMerkleblock     = regexp.MustCompile(`/tx/[^/]+/merkleblock-proof$`)
	reTxMerkle          = regexp.MustCompile(`/tx/[^/]+/merkle-proof$`)
	reTxOutspendVout    = regexp.MustCompile(`/tx/[^/]+/outspend/\d+$`)
	reTxOutspends       = regexp.MustCompile(`/tx/[^/]+/outspends$`)
	rePostTxBroadcast   = regexp.MustCompile(`/tx$`)
	rePostTxsPackage    = regexp.MustCompile(`/txs/package$`)

	// ---- Addresses / Scripthash ----
	reAddressInfo       = regexp.MustCompile(`/address/[^/]+$`)
	reScripthashInfo    = regexp.MustCompile(`/scripthash/[^/]+$`)
	reAddressTxs        = regexp.MustCompile(`/address/[^/]+/txs$`)
	reScripthashTxs     = regexp.MustCompile(`/scripthash/[^/]+/txs$`)
	reAddressTxsChain   = regexp.MustCompile(`/address/[^/]+/txs/chain(?:/[^/]+)?$`)
	reScripthashTxsChain= regexp.MustCompile(`/scripthash/[^/]+/txs/chain(?:/[^/]+)?$`)
	reAddressTxsMempool = regexp.MustCompile(`/address/[^/]+/txs/mempool$`)
	reScripthashTxsMem  = regexp.MustCompile(`/scripthash/[^/]+/txs/mempool$`)
	reAddressUtxo       = regexp.MustCompile(`/address/[^/]+/utxo$`)
	reScripthashUtxo    = regexp.MustCompile(`/scripthash/[^/]+/utxo$`)
	reAddressPrefix     = regexp.MustCompile(`/address-prefix/[^/]+$`)

	// ---- Blocks ----
	reBlock             = regexp.MustCompile(`/block/[^/]+$`)
	reBlockHeader       = regexp.MustCompile(`/block/[^/]+/header$`)
	reBlockStatus       = regexp.MustCompile(`/block/[^/]+/status$`)
	reBlockTxs          = regexp.MustCompile(`/block/[^/]+/txs(?:/\d+)?$`)
	reBlockTxids        = regexp.MustCompile(`/block/[^/]+/txids$`)
	reBlockTxidIndex    = regexp.MustCompile(`/block/[^/]+/txid/\d+$`)
	reBlockRaw          = regexp.MustCompile(`/block/[^/]+/raw$`)
	reBlockHeight       = regexp.MustCompile(`/block-height/\d+$`)
	reBlocks            = regexp.MustCompile(`/blocks(?:/\d+)?$`)
	reBlocksTipHeight   = regexp.MustCompile(`/blocks/tip/height$`)
	reBlocksTipHash     = regexp.MustCompile(`/blocks/tip/hash$`)

	// ---- Mempool / Fees ----
	reMempool           = regexp.MustCompile(`/mempool$`)
	reMempoolTxids      = regexp.MustCompile(`/mempool/txids$`)
	reMempoolRecent     = regexp.MustCompile(`/mempool/recent$`)
	reFeeEstimates      = regexp.MustCompile(`/fee-estimates$`)

	// ---- Assets (Elements/Liquid only) ----
	reAsset             = regexp.MustCompile(`/asset/[^/]+$`)
	reAssetTxs          = regexp.MustCompile(`/asset/[^/]+/txs$`)
	reAssetTxsMempool   = regexp.MustCompile(`/asset/[^/]+/txs/mempool$`)
	reAssetTxsChain     = regexp.MustCompile(`/asset/[^/]+/txs/chain(?:/[^/]+)?$`)
	reAssetSupply       = regexp.MustCompile(`/asset/[^/]+/supply$`)
	reAssetSupplyDec    = regexp.MustCompile(`/asset/[^/]+/supply/decimal$`)
	reAssetsRegistry    = regexp.MustCompile(`/assets/registry$`)
)

// EndpointID maps an HTTP method + request path to a stable, unique endpoint identifier.
//
// Notes:
//   - Paths below are derived from Blockstream Esplora API docs.
//   - `path` should be the URL path only (no scheme/host/querystring), e.g. "/tx/<txid>/status".
func EndpointID(method, path string) string {
	switch {
	// ---- Transactions ----
	case method == "GET" && reTx.MatchString(path):
		return "GET_TX"
	case method == "GET" && reTxStatus.MatchString(path):
		return "GET_TX_STATUS"
	case method == "GET" && reTxHex.MatchString(path):
		return "GET_TX_HEX"
	case method == "GET" && reTxRaw.MatchString(path):
		return "GET_TX_RAW"
	case method == "GET" && reTxMerkleblock.MatchString(path):
		return "GET_TX_MERKLEBLOCK_PROOF"
	case method == "GET" && reTxMerkle.MatchString(path):
		return "GET_TX_MERKLE_PROOF"
	case method == "GET" && reTxOutspendVout.MatchString(path):
		return "GET_TX_OUTSPEND_VOUT"
	case method == "GET" && reTxOutspends.MatchString(path):
		return "GET_TX_OUTSPENDS"
	case method == "POST" && rePostTxBroadcast.MatchString(path):
		return "POST_TX_BROADCAST"
	case method == "POST" && rePostTxsPackage.MatchString(path):
		return "POST_TXS_PACKAGE"

	// ---- Addresses / Scripthash ----
	case method == "GET" && reAddressInfo.MatchString(path):
		return "GET_ADDRESS"
	case method == "GET" && reScripthashInfo.MatchString(path):
		return "GET_SCRIPTHASH"
	case method == "GET" && reAddressTxs.MatchString(path):
		return "GET_ADDRESS_TXS"
	case method == "GET" && reScripthashTxs.MatchString(path):
		return "GET_SCRIPTHASH_TXS"
	case method == "GET" && reAddressTxsChain.MatchString(path):
		return "GET_ADDRESS_TXS_CHAIN"
	case method == "GET" && reScripthashTxsChain.MatchString(path):
		return "GET_SCRIPTHASH_TXS_CHAIN"
	case method == "GET" && reAddressTxsMempool.MatchString(path):
		return "GET_ADDRESS_TXS_MEMPOOL"
	case method == "GET" && reScripthashTxsMem.MatchString(path):
		return "GET_SCRIPTHASH_TXS_MEMPOOL"
	case method == "GET" && reAddressUtxo.MatchString(path):
		return "GET_ADDRESS_UTXO"
	case method == "GET" && reScripthashUtxo.MatchString(path):
		return "GET_SCRIPTHASH_UTXO"
	case method == "GET" && reAddressPrefix.MatchString(path):
		return "GET_ADDRESS_PREFIX"

	// ---- Blocks ----
	case method == "GET" && reBlock.MatchString(path):
		return "GET_BLOCK"
	case method == "GET" && reBlockHeader.MatchString(path):
		return "GET_BLOCK_HEADER"
	case method == "GET" && reBlockStatus.MatchString(path):
		return "GET_BLOCK_STATUS"
	case method == "GET" && reBlockTxs.MatchString(path):
		return "GET_BLOCK_TXS"
	case method == "GET" && reBlockTxids.MatchString(path):
		return "GET_BLOCK_TXIDS"
	case method == "GET" && reBlockTxidIndex.MatchString(path):
		return "GET_BLOCK_TXID_INDEX"
	case method == "GET" && reBlockRaw.MatchString(path):
		return "GET_BLOCK_RAW"
	case method == "GET" && reBlockHeight.MatchString(path):
		return "GET_BLOCK_HEIGHT"
	case method == "GET" && reBlocks.MatchString(path):
		return "GET_BLOCKS"
	case method == "GET" && reBlocksTipHeight.MatchString(path):
		return "GET_BLOCKS_TIP_HEIGHT"
	case method == "GET" && reBlocksTipHash.MatchString(path):
		return "GET_BLOCKS_TIP_HASH"

	// ---- Assets (Elements/Liquid only) ----
	case method == "GET" && reAsset.MatchString(path):
		return "GET_ASSET"
	case method == "GET" && reAssetTxs.MatchString(path):
		return "GET_ASSET_TXS"
	case method == "GET" && reAssetTxsMempool.MatchString(path):
		return "GET_ASSET_TXS_MEMPOOL"
	case method == "GET" && reAssetTxsChain.MatchString(path):
		return "GET_ASSET_TXS_CHAIN"
	case method == "GET" && reAssetSupply.MatchString(path):
		return "GET_ASSET_SUPPLY"
	case method == "GET" && reAssetSupplyDec.MatchString(path):
		return "GET_ASSET_SUPPLY_DECIMAL"
	case method == "GET" && reAssetsRegistry.MatchString(path):
		return "GET_ASSETS_REGISTRY"
	
	// ---- Mempool / Fees ----
	case method == "GET" && reMempool.MatchString(path):
		return "GET_MEMPOOL"
	case method == "GET" && reMempoolTxids.MatchString(path):
		return "GET_MEMPOOL_TXIDS"
	case method == "GET" && reMempoolRecent.MatchString(path):
		return "GET_MEMPOOL_RECENT"
	case method == "GET" && reFeeEstimates.MatchString(path):
		return "GET_FEE_ESTIMATES"
	}

	return "UNKNOWN"
}


// ExtractMethod extracts the method name from the request for logging/metrics
func (h *BitcoinEsploraHandler) ExtractMethod(req *http.Request, body []byte) (string, error) {
	return EndpointID(req.Method, req.URL.Path), nil
}

// ConfigureRequestPath configures the request path for REST API requests
func (h *BitcoinEsploraHandler) ConfigureRequestPath(req *http.Request, providerPath string, providerQuery string, networkName string) error {
	ConfigureRESTRequestPath(req, providerPath, networkName)
	// Note: providerQuery is not used for Bitcoin Esplora currently. Can be implemented if needed.
	return nil
}

func (h *BitcoinEsploraHandler) NormalizeEndpoint(path string) string {
	// For now, return path as-is since we don't have path normalizer
	// This can be enhanced later if needed
	return path
}

func (h *BitcoinEsploraHandler) ValidateRequest(req *http.Request) error {
	// For now we let everything through
	return nil
}

// Response handling methods
func (h *BitcoinEsploraHandler) ParseResponse(body []byte, statusCode int) error {
	if statusCode >= 400 {
		return fmt.Errorf("HTTP error: %d, body: %s", statusCode, string(body))
	}
	return nil
}

func (h *BitcoinEsploraHandler) IsRetryableOnDifferentProvider(err error, statusCode int) bool {
	// REST APIs don't have JSON-RPC -32601 semantics
	return false
}

func (h *BitcoinEsploraHandler) IsRetryableError(err error, statusCode int) bool {
	// Server errors and rate limits are retryable
	if statusCode >= 500 || statusCode == 429 {
		return true
	}

	// Network errors are retryable
	if err != nil {
		message := strings.ToLower(err.Error())
		retryablePatterns := []string{
			"timeout",
			"connection",
			"temporary",
			"rate limit",
		}

		for _, pattern := range retryablePatterns {
			if strings.Contains(message, pattern) {
				return true
			}
		}
	}

	return false
}

// Block operations - Bitcoin Esplora doesn't use JSON-RPC
func (h *BitcoinEsploraHandler) FormatBlockHeight(blockNum int64) string {
	return strconv.FormatInt(blockNum, 10)
}

func (h *BitcoinEsploraHandler) CreateBlockRequest(method string, blockNum int64, includeTransactions bool) ([]byte, error) {
	return nil, fmt.Errorf("the Bitcoin Esplora uses REST API, not JSON-RPC")
}

func (h *BitcoinEsploraHandler) ParseBlockResponse(body []byte) (interface{}, error) {
	var block BitcoinEsploraBlock
	if err := json.Unmarshal(body, &block); err != nil {
		return nil, fmt.Errorf("failed to parse Bitcoin block response: %w", err)
	}
	return block, nil
}

func (h *BitcoinEsploraHandler) ExtractBlockHash(blockData interface{}) string {
	if block, ok := blockData.(BitcoinEsploraBlock); ok {
		return block.ID
	}
	return ""
}

func (h *BitcoinEsploraHandler) SupportsGetBlockByNumber() bool {
	return true // Bitcoin Esplora supports getting blocks by height
}

func (h *BitcoinEsploraHandler) SupportsDynamicBlockLag() bool {
	return false // Bitcoin has 10 min blocks, use configured limit
}

func (h *BitcoinEsploraHandler) GetSupportedMethods() []string {
	// These are REST endpoints from Esplora API documentation, not JSON-RPC methods
	return []string{
		// Transaction endpoints
		"/tx/{txid}",
		"/tx/{txid}/status",
		"/tx/{txid}/hex",
		"/tx/{txid}/raw",
		"/tx/{txid}/merkleblock-proof",
		"/tx/{txid}/merkle-proof",
		"/tx/{txid}/outspend/{vout}",
		"/tx/{txid}/outspends",

		// Address endpoints
		"/address/{address}",
		"/scripthash/{hash}",
		"/address/{address}/txs",
		"/scripthash/{hash}/txs",
		"/address/{address}/txs/chain",
		"/address/{address}/txs/chain/{last_seen_txid}",
		"/scripthash/{hash}/txs/chain",
		"/scripthash/{hash}/txs/chain/{last_seen_txid}",
		"/address/{address}/txs/mempool",
		"/scripthash/{hash}/txs/mempool",
		"/address/{address}/utxo",
		"/scripthash/{hash}/utxo",
		"/address-prefix/{prefix}",

		// Block endpoints
		"/block/{hash}",
		"/block/{hash}/header",
		"/block/{hash}/status",
		"/block/{hash}/txs",
		"/block/{hash}/txs/{start_index}",
		"/block/{hash}/txids",
		"/block/{hash}/txid/{index}",
		"/block/{hash}/raw",
		"/block-height/{height}",
		"/blocks",
		"/blocks/{start_height}",
		"/blocks/tip/height",
		"/blocks/tip/hash",

		// Mempool endpoints
		"/mempool",
		"/mempool/txids",
		"/mempool/recent",

		// Fee estimate endpoints
		"/fee-estimates",
	}
}

func (h *BitcoinEsploraHandler) GetBlockByNumberMethod() string {
	return "/blocks/tip/height" // REST endpoint for getting block by height
}

// Health check methods
func (h *BitcoinEsploraHandler) GetHealthCheckMethod() string {
	return h.healthCheckEndpoint
}

func (h *BitcoinEsploraHandler) GetHealthCheckHTTPMethod() string {
	return "GET"
}

func (h *BitcoinEsploraHandler) CreateHealthCheckPayload(method string) ([]byte, error) {
	// REST API doesn't need request payload for GET requests
	return nil, nil
}

func (h *BitcoinEsploraHandler) ParseHealthCheckResponse(body []byte) (*BlockInfo, error) {
	// Parse the block height response
	var height int64
	if err := json.Unmarshal(body, &height); err != nil {
		return nil, fmt.Errorf("failed to parse block height: %w", err)
	}

	return &BlockInfo{
		Number:    height,
		Timestamp: time.Now(),
	}, nil
}

// GetLatestBlockNumber retrieves the latest block number from the provider
func (h *BitcoinEsploraHandler) GetLatestBlockNumber(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int) (*LatestBlockResult, error) {
	endpoint := httpUrl + h.healthCheckEndpoint

	// Execute request with retries
	var resp []byte
	var lastErr error
	var statusCode int

	for attempt := 0; attempt < requestAttempts; attempt++ {
		respBytes, status, err := httpClient.Get(endpoint, headers, authClient)
		if err != nil {
			lastErr = err
			if status != nil {
				statusCode = *status
			}
			continue
		}
		if status != nil {
			statusCode = *status
		}
		if statusCode >= 200 && statusCode < 300 {
			resp = respBytes
			break
		}
		lastErr = fmt.Errorf("HTTP error %d", statusCode)
	}

	if resp == nil {
		return nil, lastErr
	}

	// Parse response
	blockInfo, err := h.ParseHealthCheckResponse(resp)
	if err != nil {
		return nil, err
	}

	return &LatestBlockResult{
		BlockNumber:    blockInfo.Number,
		HealthStatus:   Healthy,
		ResponseStatus: 200,
		Metadata:       make(map[string]interface{}),
	}, nil
}

// RequiresSeparateBlockInfoCall returns true since we need separate calls for height and hash
func (h *BitcoinEsploraHandler) RequiresSeparateBlockInfoCall() bool {
	return true
}

// GetBlockInfoMethod returns the endpoint to get block hash
func (h *BitcoinEsploraHandler) GetBlockInfoMethod() string {
	return h.blockInfoEndpoint
}

// ParseBlockNumberResponse parses the raw response from a block number query
func (h *BitcoinEsploraHandler) ParseBlockNumberResponse(body []byte, statusCode int) (int64, error) {
	if statusCode != 200 {
		return 0, fmt.Errorf("unexpected status code: %d", statusCode)
	}

	var height int64
	if err := json.Unmarshal(body, &height); err != nil {
		return 0, fmt.Errorf("failed to parse block height: %w", err)
	}

	return height, nil
}

// Chain ID operations - Bitcoin doesn't have chain ID like EVM
func (h *BitcoinEsploraHandler) GetChainIDMethod() string {
	return "" // Bitcoin doesn't have chain ID concept
}

func (h *BitcoinEsploraHandler) ParseChainIDResponse(body []byte, statusCode int) (string, error) {
	// Bitcoin doesn't have chain ID, return the configured chain ID
	return h.config.ChainID, nil
}

func (h *BitcoinEsploraHandler) ValidateChainID(chainID string) error {
	// Accept any chain ID for Bitcoin (mainnet, testnet, regtest, etc.)
	return nil
}

func (h *BitcoinEsploraHandler) GetChainID(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int) (string, error) {
	// Bitcoin doesn't have dynamic chain ID, return configured value
	return h.config.ChainID, nil
}

// Archive mode - not applicable for REST API
func (h *BitcoinEsploraHandler) SupportsArchiveMode() bool {
	return false
}

func (h *BitcoinEsploraHandler) GetArchiveMethod() string {
	return ""
}

func (h *BitcoinEsploraHandler) CreateArchivePayload(method string, blockHeight string) ([]byte, error) {
	return nil, fmt.Errorf("archive mode not supported for Bitcoin Esplora")
}

func (h *BitcoinEsploraHandler) ParseArchiveResponse(body []byte) error {
	return fmt.Errorf("archive mode not supported for Bitcoin Esplora")
}

func (h *BitcoinEsploraHandler) PerformArchiveCheck(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int, blockHeight string) error {
	return fmt.Errorf("archive mode not supported for Bitcoin Esplora")
}

// PerformGetBlockByNumber performs the complete get block by number operation
func (h *BitcoinEsploraHandler) PerformGetBlockByNumber(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int, blockNumber int64) (interface{}, error) {
	// First get block hash by height
	endpoint := fmt.Sprintf("%s/block-height/%d", httpUrl, blockNumber)

	// Execute request to get block hash
	var resp []byte
	var lastErr error
	var statusCode int

	for attempt := 0; attempt < requestAttempts; attempt++ {
		respBytes, status, err := httpClient.Get(endpoint, headers, authClient)
		if err != nil {
			lastErr = err
			if status != nil {
				statusCode = *status
			}
			continue
		}
		if status != nil {
			statusCode = *status
		}
		if statusCode >= 200 && statusCode < 300 {
			resp = respBytes
			break
		}
		lastErr = fmt.Errorf("HTTP error %d", statusCode)
	}

	if resp == nil {
		return nil, fmt.Errorf("failed to get block hash: %w", lastErr)
	}

	var blockHash string
	if err := json.Unmarshal(resp, &blockHash); err != nil {
		return nil, fmt.Errorf("failed to parse block hash: %w", err)
	}

	// Now get full block data using the hash
	blockEndpoint := fmt.Sprintf("%s/block/%s", httpUrl, blockHash)

	// Execute request to get block data
	var blockResp []byte
	lastErr = nil

	for attempt := 0; attempt < requestAttempts; attempt++ {
		respBytes, status, err := httpClient.Get(blockEndpoint, headers, authClient)
		if err != nil {
			lastErr = err
			if status != nil {
				statusCode = *status
			}
			continue
		}
		if status != nil {
			statusCode = *status
		}
		if statusCode >= 200 && statusCode < 300 {
			blockResp = respBytes
			break
		}
		lastErr = fmt.Errorf("HTTP error %d", statusCode)
	}

	if blockResp == nil {
		return nil, fmt.Errorf("failed to get block data: %w", lastErr)
	}

	return h.ParseBlockResponse(blockResp)
}

// GetBlockTimestamp returns an error as Bitcoin Esplora doesn't support dynamic block lag
func (h *BitcoinEsploraHandler) GetBlockTimestamp(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int, blockNumber int64) (int64, error) {
	return 0, fmt.Errorf("bitcoin esplora does not support dynamic block lag calculation")
}
