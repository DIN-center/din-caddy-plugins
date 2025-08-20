package network

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
)

type BitcoinHandler struct {
	config  *NetworkConfig
	version string
	logger  *logger.LoggerClient
}

func NewBitcoinHandler(config *NetworkConfig) *BitcoinHandler {
	return &BitcoinHandler{
		config:  config,
		version: "1.0.0",
		logger:  config.Logger,
	}
}

// Metadata methods for registry
func (h *BitcoinHandler) GetType() string {
	return "bitcoin" // Must match modules.BitcoinHandler constant value
}

func (h *BitcoinHandler) GetName() string {
	return "Bitcoin JSON-RPC Handler"
}

func (h *BitcoinHandler) GetVersion() string {
	return h.version
}

func (h *BitcoinHandler) GetRequestType() RequestType {
	return RequestTypeRPC
}

// Lifecycle methods
func (h *BitcoinHandler) Initialize(config *NetworkConfig) error {
	h.config = config

	if config.Logger != nil {
		h.logger = config.Logger
	}

	return nil
}

func (h *BitcoinHandler) Shutdown() error {
	return nil
}

// === EXISTING METHODS ===

func (h *BitcoinHandler) ProcessRequest(req *http.Request) error {
	// Validate the request using our validation logic
	if err := h.ValidateRequest(req); err != nil {
		return err
	}

	// Path translation is handled in DinSelect module
	return nil
}

// ExtractMethod extracts the JSON-RPC method from the request body
func (h *BitcoinHandler) ExtractMethod(req *http.Request, body []byte) (string, error) {
	if len(body) == 0 {
		return "", fmt.Errorf("empty request body")
	}

	var rpcRequest din_http.JSONRPCRequest
	if err := json.Unmarshal(body, &rpcRequest); err != nil {
		return "", fmt.Errorf("failed to parse JSON-RPC request: %w", err)
	}

	if rpcRequest.Method == "" {
		return "", fmt.Errorf("missing method in JSON-RPC request")
	}

	return rpcRequest.Method, nil
}

// ConfigureRequestPath configures the request path for JSON-RPC requests
func (h *BitcoinHandler) ConfigureRequestPath(req *http.Request, providerPath string, networkName string) error {
	ConfigureJSONRPCRequestPath(req, providerPath)
	return nil
}

func (h *BitcoinHandler) ValidateRequest(req *http.Request) error {
	// Check content type
	contentType := req.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		return fmt.Errorf("invalid content type for Bitcoin JSON-RPC: %s", contentType)
	}

	// Check method
	if req.Method != "POST" {
		return fmt.Errorf("bitcoin JSON-RPC requires POST method, got %s", req.Method)
	}

	return nil
}

func (h *BitcoinHandler) ParseResponse(body []byte, statusCode int) error {
	// Use the shared JSON-RPC response parser
	return ParseJSONRPCResponse(body, statusCode)
}

func (h *BitcoinHandler) IsRetryableError(err error, statusCode int) bool {
	// Use the shared JSON-RPC error retry logic
	return IsRetryableJSONRPCError(err, statusCode)
}

// === NEW NETWORK-SPECIFIC METHODS ===

// Chain ID and Namespace methods
func (h *BitcoinHandler) GetNamespace() string {
	return "bitcoin"
}

func (h *BitcoinHandler) ValidateChainID(chainID string) error {
	// Fail if there's a colon (old CAIP-2 format)
	if strings.Contains(chainID, ":") {
		return fmt.Errorf("invalid Bitcoin chain ID format: %s, chain ID should not contain ':' (CAIP-2 prefix no longer required)", chainID)
	}

	if len(chainID) == 0 {
		return fmt.Errorf("empty chain ID")
	}

	// Valid chain IDs for Bitcoin
	validChainIDs := map[string]bool{
		"main":    true,
		"test":    true,
		"regtest": true,
		"signet":  true,
	}

	if !validChainIDs[chainID] {
		return fmt.Errorf("invalid Bitcoin chain ID: %s, expected one of: main, test, regtest, signet", chainID)
	}

	return nil
}

func (h *BitcoinHandler) ExtractChainReference(result interface{}) (string, error) {
	// Bitcoin's getblockchaininfo returns an object with a "chain" field
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid chain info response type: %T", result)
	}

	chain, ok := resultMap["chain"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid chain field in response")
	}

	return chain, nil
}

// Block Operations methods
func (h *BitcoinHandler) FormatBlockHeight(blockNum int64) string {
	return strconv.FormatInt(blockNum, 10) // Decimal format for Bitcoin
}

func (h *BitcoinHandler) CreateBlockRequest(method string, blockNum int64, includeTransactions bool) ([]byte, error) {
	// Bitcoin requires two-step process: getblockhash then getblock
	// This method will be called twice in the process

	switch method {
	case "getblockhash":
		// First step: get block hash from block number
		payload := fmt.Sprintf(`{"jsonrpc":"2.0","method":"getblockhash","params":[%d],"id":1}`, blockNum)
		return []byte(payload), nil
	case "getblock":
		// This will be handled by a separate method since we need the hash
		return nil, fmt.Errorf("getblock requires block hash, use CreateBlockRequestWithHash")
	default:
		return nil, fmt.Errorf("unsupported method for block request: %s", method)
	}
}

// CreateBlockRequestWithHash creates a getblock request with a block hash
func (h *BitcoinHandler) CreateBlockRequestWithHash(blockHash string, includeTransactions bool) ([]byte, error) {
	// verbosity: 0 = hex, 1 = JSON without tx details, 2 = JSON with tx details
	verbosity := 1
	if includeTransactions {
		verbosity = 2
	}

	payload := fmt.Sprintf(`{"jsonrpc":"2.0","method":"getblock","params":["%s",%d],"id":1}`, blockHash, verbosity)
	return []byte(payload), nil
}

func (h *BitcoinHandler) ParseBlockResponse(body []byte) (interface{}, error) {
	// First check for JSON-RPC errors using generic response
	var genericResponse din_http.JSONRPCResponse
	if err := json.Unmarshal(body, &genericResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON-RPC response: %w", err)
	}

	// Check for JSON-RPC errors in getBlock response
	if genericResponse.Error != nil {
		return nil, fmt.Errorf("bitcoin getBlock error: %s", genericResponse.Error.Message)
	}

	// Return the raw result for Bitcoin block
	// The result structure will vary based on verbosity parameter
	return genericResponse.Result, nil
}

// ParseBlockNumberResponse parses the block number from a raw response
func (h *BitcoinHandler) ParseBlockNumberResponse(body []byte, statusCode int) (int64, error) {
	// Check HTTP status first
	if statusCode >= 400 {
		if statusCode == 429 {
			return 0, fmt.Errorf("rate limit error (status code: %d)", statusCode)
		}
		return 0, fmt.Errorf("error status code: %d", statusCode)
	}

	// Parse JSON-RPC response
	var response JSONRPCResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return 0, fmt.Errorf("failed to parse JSON-RPC response: %w", err)
	}

	// Check for JSON-RPC error
	if response.Error != nil {
		return 0, fmt.Errorf("JSON-RPC error %d: %s", response.Error.Code, response.Error.Message)
	}

	// Bitcoin returns block numbers as numeric values
	return ParseNumericBlockNumber(response.Result)
}

// RequiresSeparateBlockInfoCall returns false for Bitcoin as health check includes block info
func (h *BitcoinHandler) RequiresSeparateBlockInfoCall() bool {
	return false
}

// GetBlockInfoMethod returns empty for Bitcoin as it uses the same endpoint for health and block info
func (h *BitcoinHandler) GetBlockInfoMethod() string {
	return "" // Not used for JSON-RPC chains
}

// GetChainID retrieves the chain ID from the Bitcoin provider
func (h *BitcoinHandler) GetChainID(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int) (string, error) {
	// Use the shared JSON-RPC logic with Bitcoin-specific parsing
	return GetChainIDViaJSONRPC(httpUrl, headers, httpClient, authClient, requestAttempts, h.GetChainIDMethod(), h.ParseChainIDResponse)
}

// Archive Mode methods
func (h *BitcoinHandler) SupportsArchiveMode() bool {
	return false // Bitcoin doesn't need special archive mode handling
}

func (h *BitcoinHandler) GetArchiveMethod() string {
	return ""
}

func (h *BitcoinHandler) CreateArchivePayload(method string, blockHeight string) ([]byte, error) {
	return nil, fmt.Errorf("bitcoin does not support archive mode")
}

func (h *BitcoinHandler) ParseArchiveResponse(body []byte) error {
	return fmt.Errorf("bitcoin does not support archive mode")
}

// Network Capabilities methods
func (h *BitcoinHandler) SupportsGetBlockByNumber() bool {
	return true // Bitcoin supports it via two-step process
}

func (h *BitcoinHandler) GetSupportedMethods() []string {
	return []string{
		"getblockcount",
		"getblockhash",
		"getblock",
		"getblockchaininfo",
		"getbestblockhash",
		"getdifficulty",
		"getmempoolinfo",
		"getrawmempool",
		"getrawtransaction",
		"sendrawtransaction",
		"getnetworkinfo",
		"getpeerinfo",
		"getmininginfo",
	}
}

func (h *BitcoinHandler) GetBlockByNumberMethod() string {
	return "getblockhash" // First step in the two-step process
}

// Data Format Conversions methods
func (h *BitcoinHandler) ExtractBlockHash(blockData interface{}) string {
	// For Bitcoin, the block hash can be in the result directly (from getblockhash)
	// or in the block object (from getblock)

	// Check if it's a string (from getblockhash)
	if hash, ok := blockData.(string); ok {
		return hash
	}

	// Check if it's a block object (from getblock)
	if blockMap, ok := blockData.(map[string]interface{}); ok {
		if hash, ok := blockMap["hash"].(string); ok {
			return hash
		}
	}

	// Check if it's a RawMessage that needs parsing
	if rawMsg, ok := blockData.(json.RawMessage); ok {
		var hash string
		if err := json.Unmarshal(rawMsg, &hash); err == nil {
			return hash
		}

		var blockMap map[string]interface{}
		if err := json.Unmarshal(rawMsg, &blockMap); err == nil {
			if hash, ok := blockMap["hash"].(string); ok {
				return hash
			}
		}
	}

	return ""
}

func (h *BitcoinHandler) ExtractBlockNumber(response []byte) (int64, error) {
	var respObject map[string]interface{}
	if err := json.Unmarshal(response, &respObject); err != nil {
		return 0, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	result, ok := respObject["result"].(float64)
	if !ok {
		return 0, fmt.Errorf("invalid Bitcoin block count response format")
	}

	return int64(result), nil
}

// Health Check Specifics methods
func (h *BitcoinHandler) GetHealthCheckMethod() string {
	return "getblockcount" // Returns the current block height
}

func (h *BitcoinHandler) GetHealthCheckHTTPMethod() string {
	return "POST" // Bitcoin uses JSON-RPC POST requests
}

func (h *BitcoinHandler) GetChainIDMethod() string {
	return "getblockchaininfo" // Returns chain info including chain name
}

func (h *BitcoinHandler) CreateHealthCheckPayload(method string) ([]byte, error) {
	payload := fmt.Sprintf(`{"jsonrpc":"2.0","method":"%s","id":1}`, method)
	return []byte(payload), nil
}

func (h *BitcoinHandler) ParseHealthCheckResponse(body []byte) (*BlockInfo, error) {
	var respObject map[string]interface{}
	if err := json.Unmarshal(body, &respObject); err != nil {
		return nil, fmt.Errorf("failed to parse Bitcoin health check response: %w", err)
	}

	result, ok := respObject["result"].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid Bitcoin health check response format")
	}

	return &BlockInfo{
		Number:    int64(result),
		Hash:      "",
		Timestamp: time.Now(),
	}, nil
}

func (h *BitcoinHandler) ParseChainIDResponse(body []byte, statusCode int) (string, error) {
	if statusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP error: %d", statusCode)
	}

	var respObject map[string]interface{}
	if err := json.Unmarshal(body, &respObject); err != nil {
		return "", fmt.Errorf("failed to parse Bitcoin chain ID response: %w", err)
	}

	// Check for JSON-RPC error
	if errorField, exists := respObject["error"]; exists && errorField != nil {
		return "", fmt.Errorf("JSON-RPC error: %v", errorField)
	}

	// Extract result field
	result, ok := respObject["result"]
	if !ok {
		return "", fmt.Errorf("missing result field in chain ID response")
	}

	// Extract chain reference
	chainReference, err := h.ExtractChainReference(result)
	if err != nil {
		return "", fmt.Errorf("failed to extract chain reference: %w", err)
	}

	return chainReference, nil
}

// GetLatestBlockNumber retrieves the latest block number for Bitcoin chains
// Uses the JSON-RPC method getblockcount to get the current block height
func (h *BitcoinHandler) GetLatestBlockNumber(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int) (*LatestBlockResult, error) {
	// Use the shared JSON-RPC helper with Bitcoin-specific numeric parsing
	return GetLatestBlockNumberViaJSONRPC(
		httpUrl,
		headers,
		httpClient,
		authClient,
		requestAttempts,
		h.GetHealthCheckMethod(), // "getblockcount"
		ParseNumericBlockNumber,  // Bitcoin returns numeric block heights
	)
}

// PerformArchiveCheck performs archive mode check for Bitcoin chains using JSON-RPC
func (h *BitcoinHandler) PerformArchiveCheck(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int, blockHeight string) error {
	// Bitcoin doesn't need archive mode check
	return fmt.Errorf("bitcoin does not support archive mode")
}

// PerformGetBlockByNumber performs get block by number operation for Bitcoin chains using JSON-RPC
// This implements the two-step process: getblockhash -> getblock
func (h *BitcoinHandler) PerformGetBlockByNumber(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int, blockNumber int64) (interface{}, error) {
	var lastErr error

	// Step 1: Get block hash from block number
	for attempt := 0; attempt < requestAttempts; attempt++ {
		// Create getblockhash request
		hashPayload, err := h.CreateBlockRequest("getblockhash", blockNumber, false)
		if err != nil {
			lastErr = fmt.Errorf("failed to create getblockhash request: %w", err)
			continue
		}

		// Make POST request for block hash
		hashResBytes, statusCode, err := httpClient.Post(httpUrl, headers, hashPayload, authClient)
		if err != nil {
			lastErr = fmt.Errorf("error sending getblockhash request: %w", err)
			continue
		}

		// Check HTTP status
		if statusCode != nil && *statusCode >= 400 {
			lastErr = fmt.Errorf("HTTP error for getblockhash: %d", *statusCode)
			continue
		}

		// Parse response to get block hash
		var hashResponse JSONRPCResponse
		if err := json.Unmarshal(hashResBytes, &hashResponse); err != nil {
			lastErr = fmt.Errorf("failed to parse getblockhash response: %w", err)
			continue
		}

		// Check for JSON-RPC error
		if hashResponse.Error != nil {
			lastErr = fmt.Errorf("getblockhash error %d: %s", hashResponse.Error.Code, hashResponse.Error.Message)
			continue
		}

		// Extract block hash from result
		var blockHash string
		if err := json.Unmarshal(hashResponse.Result, &blockHash); err != nil {
			lastErr = fmt.Errorf("failed to extract block hash: %w", err)
			continue
		}

		// Step 2: Get block details using the hash
		blockPayload, err := h.CreateBlockRequestWithHash(blockHash, false)
		if err != nil {
			lastErr = fmt.Errorf("failed to create getblock request: %w", err)
			continue
		}

		// Make POST request for block details
		blockResBytes, statusCode, err := httpClient.Post(httpUrl, headers, blockPayload, authClient)
		if err != nil {
			lastErr = fmt.Errorf("error sending getblock request: %w", err)
			continue
		}

		// Check HTTP status
		if statusCode != nil && *statusCode >= 400 {
			lastErr = fmt.Errorf("HTTP error for getblock: %d", *statusCode)
			continue
		}

		// Parse block response
		blockData, err := h.ParseBlockResponse(blockResBytes)
		if err != nil {
			lastErr = err
			continue
		}

		// Success!
		return blockData, nil
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", requestAttempts, lastErr)
}
