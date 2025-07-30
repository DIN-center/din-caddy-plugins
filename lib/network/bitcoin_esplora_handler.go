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

// BitcoinEsploraHandler handles Bitcoin Esplora REST API requests
type BitcoinEsploraHandler struct {
	config              *NetworkConfig
	healthCheckEndpoint string
	blockInfoEndpoint   string
	version             string
	logger              *logger.LoggerClient
	pathNormalizer      *BitcoinPathNormalizer
}

// BitcoinPathNormalizer normalizes Bitcoin Esplora API paths for metrics
type BitcoinPathNormalizer struct {
	patterns []compiledPattern
}

type compiledPattern struct {
	regex       *regexp.Regexp
	replacement string
}

// NewBitcoinEsploraHandler creates a new Bitcoin Esplora handler instance
func NewBitcoinEsploraHandler(config *NetworkConfig) *BitcoinEsploraHandler {
	return &BitcoinEsploraHandler{
		config:              config,
		healthCheckEndpoint: "/api/blocks/tip/height",
		blockInfoEndpoint:   "/api/blocks/tip/hash",
		version:             "1.0.0",
		logger:              config.Logger,
		pathNormalizer:      NewBitcoinPathNormalizer(),
	}
}

// NewBitcoinPathNormalizer creates a new path normalizer for Bitcoin Esplora
func NewBitcoinPathNormalizer() *BitcoinPathNormalizer {
	patterns := []compiledPattern{
		{
			regex:       regexp.MustCompile(`/api/tx/[a-fA-F0-9]{64}`),
			replacement: "/api/tx/{txid}",
		},
		{
			regex:       regexp.MustCompile(`/api/address/[a-zA-Z0-9]+`),
			replacement: "/api/address/{address}",
		},
		{
			regex:       regexp.MustCompile(`/api/block/[a-fA-F0-9]{64}`),
			replacement: "/api/block/{hash}",
		},
		{
			regex:       regexp.MustCompile(`/api/block-height/\d+`),
			replacement: "/api/block-height/{height}",
		},
		{
			regex:       regexp.MustCompile(`/api/scripthash/[a-fA-F0-9]+/txs`),
			replacement: "/api/scripthash/{hash}/txs",
		},
	}
	
	return &BitcoinPathNormalizer{patterns: patterns}
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
	if config.Logger != nil {
		h.logger = config.Logger
	}
	return nil
}

func (h *BitcoinEsploraHandler) Shutdown() error {
	// Cleanup resources if needed
	return nil
}

// Request processing methods
func (h *BitcoinEsploraHandler) ProcessRequest(req *http.Request) error {
	// Validate the request
	if err := h.ValidateRequest(req); err != nil {
		return err
	}

	// For REST APIs like Bitcoin Esplora, path translation is handled by DinSelect
	// This maintains consistency with the beacon chain approach
	return nil
}

func (h *BitcoinEsploraHandler) ValidateRequest(req *http.Request) error {
	// Check for valid HTTP methods - Bitcoin Esplora REST API supports GET and POST
	if req.Method != "GET" && req.Method != "POST" {
		return fmt.Errorf("unsupported HTTP method for Bitcoin Esplora REST API: %s", req.Method)
	}

	// Check for valid path format - must contain API pattern
	path := req.URL.Path
	if !strings.Contains(path, "/api/") {
		return fmt.Errorf("invalid Bitcoin Esplora API path: %s", path)
	}

	// For POST requests (transaction broadcast), validate content type
	if req.Method == "POST" {
		contentType := req.Header.Get("Content-Type")
		if path == "/api/tx" && contentType != "" && !strings.Contains(contentType, "text/plain") {
			return fmt.Errorf("invalid content type for transaction broadcast: %s", contentType)
		}
	}

	return nil
}


func (h *BitcoinEsploraHandler) NormalizeEndpoint(path string) string {
	return h.pathNormalizer.NormalizePath(path)
}

// NormalizePath applies all patterns and returns the normalized path
func (n *BitcoinPathNormalizer) NormalizePath(path string) string {
	for _, pattern := range n.patterns {
		if pattern.regex.MatchString(path) {
			return pattern.regex.ReplaceAllString(path, pattern.replacement)
		}
	}
	return path
}

// Response handling methods
func (h *BitcoinEsploraHandler) ParseResponse(body []byte, statusCode int) error {
	if statusCode >= 400 {
		return fmt.Errorf("HTTP error: %d", statusCode)
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
	// Bitcoin Esplora uses REST endpoints, not JSON-RPC
	return nil, fmt.Errorf("Bitcoin Esplora uses REST API, not JSON-RPC")
}

func (h *BitcoinEsploraHandler) ParseBlockResponse(body []byte) (interface{}, error) {
	var block map[string]interface{}
	if err := json.Unmarshal(body, &block); err != nil {
		return nil, fmt.Errorf("failed to parse block response: %w", err)
	}
	return block, nil
}

func (h *BitcoinEsploraHandler) ExtractBlockHash(blockData interface{}) string {
	if block, ok := blockData.(map[string]interface{}); ok {
		if hash, ok := block["id"].(string); ok {
			return hash
		}
	}
	return ""
}

func (h *BitcoinEsploraHandler) SupportsGetBlockByNumber() bool {
	return true // Bitcoin Esplora supports getting blocks by height
}

func (h *BitcoinEsploraHandler) GetSupportedMethods() []string {
	// These are REST endpoints, not JSON-RPC methods
	return []string{
		"/api/blocks/tip/height",
		"/api/blocks/tip/hash",
		"/api/block-height/{height}",
		"/api/block/{hash}",
		"/api/tx/{txid}",
		"/api/address/{address}",
	}
}

func (h *BitcoinEsploraHandler) GetBlockByNumberMethod() string {
	return "/api/block-height" // REST endpoint for getting block by height
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
		Extra:          make(map[string]interface{}),
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
	endpoint := fmt.Sprintf("%s/api/block-height/%d", httpUrl, blockNumber)

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
	blockEndpoint := fmt.Sprintf("%s/api/block/%s", httpUrl, blockHash)

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