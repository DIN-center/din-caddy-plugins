package network

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
	dinHttp "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	"go.uber.org/zap"
)

// BitcoinEsploraHandler handles Bitcoin Esplora REST API requests
type BitcoinEsploraHandler struct {
	config              *NetworkConfig
	version             string
	pathNormalizer      *BitcoinPathNormalizer
	logger              *logger.LoggerClient
	healthCheckEndpoint string
}

// BitcoinPathNormalizer handles normalization of Bitcoin-specific paths
type BitcoinPathNormalizer struct {
	patterns []compiledPattern
}

type compiledPattern struct {
	regex       *regexp.Regexp
	replacement string
}

// NewBitcoinEsploraHandler creates a new Bitcoin Esplora handler
func NewBitcoinEsploraHandler(config *NetworkConfig) *BitcoinEsploraHandler {
	handler := &BitcoinEsploraHandler{
		config:              config,
		version:             "1.0.0",
		pathNormalizer:      NewBitcoinPathNormalizer(),
		healthCheckEndpoint: "/blocks/tip/height",
	}

	if config != nil {
		handler.logger = config.Logger
	}

	return handler
}

// NewBitcoinPathNormalizer creates a new path normalizer for Bitcoin endpoints
func NewBitcoinPathNormalizer() *BitcoinPathNormalizer {
	patterns := []compiledPattern{
		// Transaction patterns
		{
			regex:       regexp.MustCompile(`/tx/[a-fA-F0-9]{64}`),
			replacement: "/tx/{txid}",
		},
		{
			regex:       regexp.MustCompile(`/tx/[a-fA-F0-9]{64}/[a-z]+`),
			replacement: "/tx/{txid}/{action}",
		},
		// Address patterns
		{
			regex:       regexp.MustCompile(`/address/[a-zA-Z0-9]{25,}`),
			replacement: "/address/{address}",
		},
		{
			regex:       regexp.MustCompile(`/address/[a-zA-Z0-9]{25,}/[a-z]+`),
			replacement: "/address/{address}/{action}",
		},
		// Block patterns
		{
			regex:       regexp.MustCompile(`/block/[a-fA-F0-9]{64}`),
			replacement: "/block/{hash}",
		},
		{
			regex:       regexp.MustCompile(`/block-height/\d+`),
			replacement: "/block-height/{height}",
		},
		// Mempool patterns
		{
			regex:       regexp.MustCompile(`/mempool/[a-zA-Z0-9]+`),
			replacement: "/mempool/{txid}",
		},
	}

	return &BitcoinPathNormalizer{patterns: patterns}
}

// NormalizePath normalizes Bitcoin-specific endpoints for metrics
func (n *BitcoinPathNormalizer) NormalizePath(path string) string {
	for _, pattern := range n.patterns {
		if pattern.regex.MatchString(path) {
			return pattern.regex.ReplaceAllString(path, pattern.replacement)
		}
	}
	return path
}

// === Core Identification ===

func (h *BitcoinEsploraHandler) GetType() string {
	return "bitcoin-esplora" // Must match modules.BitcoinEsploraHandler constant value
}

func (h *BitcoinEsploraHandler) GetName() string {
	return "Bitcoin Esplora REST API Handler"
}

func (h *BitcoinEsploraHandler) GetRequestType() RequestType {
	return RequestTypeREST
}

// === Request Processing ===

func (h *BitcoinEsploraHandler) ProcessRequest(req *http.Request) error {
	// For REST APIs, the path is already correct
	// We might need to add any Bitcoin-specific headers here
	return nil
}

// ExtractMethod extracts the method (URL path) for REST APIs
func (h *BitcoinEsploraHandler) ExtractMethod(req *http.Request, body []byte) (string, error) {
	// For REST APIs, use the normalized path for better metrics
	normalizedPath := h.pathNormalizer.NormalizePath(req.URL.Path)
	return normalizedPath, nil
}

// ConfigureRequestPath configures the request path for REST API requests
func (h *BitcoinEsploraHandler) ConfigureRequestPath(req *http.Request, providerPath string, networkName string) error {
	ConfigureRESTRequestPath(req, providerPath, networkName)
	return nil
}

// === Response Handling ===

func (h *BitcoinEsploraHandler) ParseResponse(body []byte, statusCode int) error {
	if statusCode >= 400 {
		// Try to parse error response
		var errorResp struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(body, &errorResp); err == nil && errorResp.Error != "" {
			return fmt.Errorf("Bitcoin Esplora API error: %s", errorResp.Error)
		}
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
		errMsg := strings.ToLower(err.Error())
		retryablePatterns := []string{
			"timeout",
			"connection",
			"temporary",
			"rate limit",
			"service unavailable",
		}
		for _, pattern := range retryablePatterns {
			if strings.Contains(errMsg, pattern) {
				return true
			}
		}
	}

	return false
}

// === Block Operations ===

func (h *BitcoinEsploraHandler) FormatBlockHeight(blockNum int64) string {
	return strconv.FormatInt(blockNum, 10)
}

func (h *BitcoinEsploraHandler) CreateBlockRequest(method string, blockNum int64, includeTransactions bool) ([]byte, error) {
	// For REST APIs, we don't create request payloads
	return nil, fmt.Errorf("CreateBlockRequest not supported for REST API")
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
	return true
}

func (h *BitcoinEsploraHandler) GetSupportedMethods() []string {
	// REST endpoints, not RPC methods
	return []string{
		"/blocks/tip",
		"/blocks/tip/height",
		"/block/{hash}",
		"/block-height/{height}",
		"/tx/{txid}",
		"/address/{address}",
		"/mempool",
		"/fee-estimates",
	}
}

func (h *BitcoinEsploraHandler) GetBlockByNumberMethod() string {
	return "/block-height/{height}"
}

// === Health Check ===

func (h *BitcoinEsploraHandler) GetHealthCheckMethod() string {
	return h.healthCheckEndpoint
}

func (h *BitcoinEsploraHandler) GetHealthCheckHTTPMethod() string {
	return "GET"
}

func (h *BitcoinEsploraHandler) CreateHealthCheckPayload(method string) ([]byte, error) {
	// REST API doesn't need payloads for GET requests
	return nil, nil
}

func (h *BitcoinEsploraHandler) ParseHealthCheckResponse(body []byte) (*BlockInfo, error) {
	// The /blocks/tip/height endpoint returns just the height as a number
	var height int64
	if err := json.Unmarshal(body, &height); err != nil {
		return nil, fmt.Errorf("failed to parse block height: %w", err)
	}

	return &BlockInfo{
		Number:    height,
		Timestamp: time.Now(),
	}, nil
}

func (h *BitcoinEsploraHandler) GetLatestBlockNumber(httpUrl string, headers map[string]string, httpClient dinHttp.IHTTPClient, authClient auth.IAuthClient, requestAttempts int) (*LatestBlockResult, error) {
	endpoint := httpUrl + h.healthCheckEndpoint

	for attempt := 0; attempt < requestAttempts; attempt++ {
		respBytes, status, err := httpClient.Get(endpoint, headers, authClient)

		if err != nil {
			h.logger.Warn("Bitcoin Esplora health check request failed",
				zap.String("endpoint", endpoint),
				zap.Int("attempt", attempt+1),
				zap.Int("max_attempts", requestAttempts),
				zap.Error(err))

			if attempt < requestAttempts-1 && h.IsRetryableError(err, 0) {
				time.Sleep(time.Duration(attempt+1) * time.Second)
				continue
			}

			return &LatestBlockResult{
				BlockNumber:    0,
				HealthStatus:   Unhealthy,
				ResponseStatus: 0,
			}, err
		}

		if status != nil && *status >= 200 && *status < 300 {
			blockInfo, err := h.ParseHealthCheckResponse(respBytes)
			if err != nil {
				return &LatestBlockResult{
					BlockNumber:    0,
					HealthStatus:   Unhealthy,
					ResponseStatus: *status,
				}, err
			}

			return &LatestBlockResult{
				BlockNumber:    blockInfo.Number,
				HealthStatus:   Healthy,
				ResponseStatus: *status,
			}, nil
		}

		// Handle non-200 status codes
		healthStatus := Unhealthy
		if status != nil && *status == 429 {
			healthStatus = Warning
		}

		if attempt < requestAttempts-1 && h.IsRetryableError(nil, *status) {
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}

		return &LatestBlockResult{
			BlockNumber:    0,
			HealthStatus:   healthStatus,
			ResponseStatus: *status,
		}, fmt.Errorf("health check failed with status %d", *status)
	}

	return &LatestBlockResult{
		BlockNumber:    0,
		HealthStatus:   Unhealthy,
		ResponseStatus: 0,
	}, fmt.Errorf("health check failed after %d attempts", requestAttempts)
}

// === Separate Block Info Call ===

func (h *BitcoinEsploraHandler) RequiresSeparateBlockInfoCall() bool {
	return false // /blocks/tip/height provides the info we need
}

func (h *BitcoinEsploraHandler) GetBlockInfoMethod() string {
	return "/blocks/tip"
}

// === Block Number Parsing ===

func (h *BitcoinEsploraHandler) ParseBlockNumberResponse(body []byte, statusCode int) (int64, error) {
	if statusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code: %d", statusCode)
	}

	var height int64
	if err := json.Unmarshal(body, &height); err != nil {
		return 0, fmt.Errorf("failed to parse block height: %w", err)
	}

	return height, nil
}

// === Chain ID Operations ===

func (h *BitcoinEsploraHandler) GetChainIDMethod() string {
	// Bitcoin doesn't have a chain ID method like Ethereum
	// We use the network identifier from config
	return ""
}

func (h *BitcoinEsploraHandler) ParseChainIDResponse(body []byte, statusCode int) (string, error) {
	// Bitcoin doesn't have dynamic chain IDs
	return h.config.ChainID, nil
}

func (h *BitcoinEsploraHandler) ValidateChainID(chainID string) error {
	if h.config.ChainID == "" {
		return nil // No validation if not configured
	}

	if chainID != h.config.ChainID {
		return fmt.Errorf("chain ID mismatch: expected %s, got %s", h.config.ChainID, chainID)
	}

	return nil
}

func (h *BitcoinEsploraHandler) GetChainID(httpUrl string, headers map[string]string, httpClient dinHttp.IHTTPClient, authClient auth.IAuthClient, requestAttempts int) (string, error) {
	// Bitcoin doesn't have a chain ID endpoint, return configured value
	return h.config.ChainID, nil
}

// === Archive Mode ===

func (h *BitcoinEsploraHandler) SupportsArchiveMode() bool {
	return true // Esplora typically provides full blockchain history
}

func (h *BitcoinEsploraHandler) GetArchiveMethod() string {
	return "/block-height/{height}"
}

func (h *BitcoinEsploraHandler) CreateArchivePayload(method string, blockHeight string) ([]byte, error) {
	// REST API doesn't need payloads
	return nil, nil
}

func (h *BitcoinEsploraHandler) ParseArchiveResponse(body []byte) error {
	// If we can parse the block, archive mode is working
	var block map[string]interface{}
	return json.Unmarshal(body, &block)
}

func (h *BitcoinEsploraHandler) PerformArchiveCheck(httpUrl string, headers map[string]string, httpClient dinHttp.IHTTPClient, authClient auth.IAuthClient, requestAttempts int, blockHeight string) error {
	endpoint, err := url.JoinPath(httpUrl, "block-height", blockHeight)
	if err != nil {
		return fmt.Errorf("failed to construct archive check URL: %w", err)
	}

	respBytes, status, err := httpClient.Get(endpoint, headers, authClient)
	if err != nil {
		return fmt.Errorf("archive check request failed: %w", err)
	}

	if status == nil || *status != http.StatusOK {
		return fmt.Errorf("archive check failed with status %d", *status)
	}

	return h.ParseArchiveResponse(respBytes)
}

// === Get Block By Number ===

func (h *BitcoinEsploraHandler) PerformGetBlockByNumber(httpUrl string, headers map[string]string, httpClient dinHttp.IHTTPClient, authClient auth.IAuthClient, requestAttempts int, blockNumber int64) (interface{}, error) {
	endpoint, err := url.JoinPath(httpUrl, "block-height", strconv.FormatInt(blockNumber, 10))
	if err != nil {
		return nil, fmt.Errorf("failed to construct block by number URL: %w", err)
	}

	for attempt := 0; attempt < requestAttempts; attempt++ {
		respBytes, status, err := httpClient.Get(endpoint, headers, authClient)

		if err != nil {
			if attempt < requestAttempts-1 && h.IsRetryableError(err, 0) {
				time.Sleep(time.Duration(attempt+1) * time.Second)
				continue
			}
			return nil, err
		}

		if status != nil && *status == http.StatusOK {
			return h.ParseBlockResponse(respBytes)
		}

		if attempt < requestAttempts-1 && h.IsRetryableError(nil, *status) {
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}

		return nil, fmt.Errorf("failed to get block with status %d", *status)
	}

	return nil, fmt.Errorf("failed to get block after %d attempts", requestAttempts)
}

// === Lifecycle ===

func (h *BitcoinEsploraHandler) Initialize(config *NetworkConfig) error {
	h.config = config

	if config.Logger != nil {
		h.logger = config.Logger
	}

	// Parse custom configuration if provided
	if config.Custom != nil {
		if endpoint, ok := config.Custom["health_endpoint"].(string); ok {
			h.healthCheckEndpoint = endpoint
		}
	}

	return nil
}
