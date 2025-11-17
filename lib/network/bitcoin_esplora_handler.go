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

// ExtractMethod extracts the method name from the request for logging/metrics
// Returns a normalized path to prevent cardinality explosion in metrics
func (h *BitcoinEsploraHandler) ExtractMethod(req *http.Request, body []byte) (string, error) {
	return h.NormalizeEndpoint(req.URL.Path), nil
}

// ConfigureRequestPath configures the request path for REST API requests
func (h *BitcoinEsploraHandler) ConfigureRequestPath(req *http.Request, providerPath string, networkName string) error {
	ConfigureRESTRequestPath(req, providerPath, networkName)
	return nil
}

// Helper functions for path normalization

// isHexHash checks if a string is a 64-character hexadecimal hash (txid, block hash, script hash, asset id)
func isHexHash(s string) bool {
	if len(s) != 64 {
		return false
	}
	matched, _ := regexp.MatchString("^[0-9a-fA-F]{64}$", s)
	return matched
}

// isBitcoinAddress checks if a string is a valid Bitcoin address format
func isBitcoinAddress(s string) bool {
	// Legacy P2PKH (starts with 1)
	if matched, _ := regexp.MatchString("^1[a-km-zA-HJ-NP-Z1-9]{25,34}$", s); matched {
		return true
	}
	// P2SH (starts with 3)
	if matched, _ := regexp.MatchString("^3[a-km-zA-HJ-NP-Z1-9]{25,34}$", s); matched {
		return true
	}
	// Bech32 SegWit v0 (starts with bc1q, tb1q, bcrt1q)
	if matched, _ := regexp.MatchString("^(bc1q|tb1q|bcrt1q)[a-z0-9]{38,58}$", s); matched {
		return true
	}
	// Bech32m Taproot SegWit v1 (starts with bc1p, tb1p, bcrt1p)
	if matched, _ := regexp.MatchString("^(bc1p|tb1p|bcrt1p)[a-z0-9]{58}$", s); matched {
		return true
	}
	return false
}

// isNumeric checks if a string contains only digits
func isNumeric(s string) bool {
	if len(s) == 0 {
		return false
	}
	matched, _ := regexp.MatchString("^[0-9]+$", s)
	return matched
}

func (h *BitcoinEsploraHandler) NormalizeEndpoint(path string) string {
	// Split path into segments
	segments := strings.Split(path, "/")
	normalizedSegments := make([]string, len(segments))

	for i, segment := range segments {
		// Check if previous segment provides context for normalization
		prevSegment := ""
		if i > 0 {
			prevSegment = segments[i-1]
		}

		// Context-aware normalization based on previous segment
		switch prevSegment {
		case "tx":
			if isHexHash(segment) {
				normalizedSegments[i] = "{txid}"
				continue
			}
		case "block":
			if isHexHash(segment) {
				normalizedSegments[i] = "{hash}"
				continue
			}
		case "scripthash":
			if isHexHash(segment) {
				normalizedSegments[i] = "{scripthash}"
				continue
			}
		case "address":
			if isBitcoinAddress(segment) {
				normalizedSegments[i] = "{address}"
				continue
			}
		case "asset":
			if isHexHash(segment) {
				normalizedSegments[i] = "{asset_id}"
				continue
			}
		case "block-height":
			if isNumeric(segment) {
				normalizedSegments[i] = "{height}"
				continue
			}
		case "blocks":
			if isNumeric(segment) {
				normalizedSegments[i] = "{start_height}"
				continue
			}
		case "outspend":
			if isNumeric(segment) {
				normalizedSegments[i] = "{vout}"
				continue
			}
		case "txid":
			if isNumeric(segment) {
				normalizedSegments[i] = "{index}"
				continue
			}
		case "txs":
			if isNumeric(segment) {
				normalizedSegments[i] = "{start_index}"
				continue
			}
			// Also check if it's a hex hash (for /txs/chain/{last_seen_txid})
			if isHexHash(segment) {
				normalizedSegments[i] = "{last_seen_txid}"
				continue
			}
		case "chain":
			if isHexHash(segment) {
				normalizedSegments[i] = "{last_seen_txid}"
				continue
			}
		case "address-prefix":
			// Replace any prefix with placeholder
			if len(segment) > 0 {
				normalizedSegments[i] = "{prefix}"
				continue
			}
		}

		// If no normalization applied, keep original segment
		normalizedSegments[i] = segment
	}

	// Reconstruct the normalized path
	return strings.Join(normalizedSegments, "/")
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
