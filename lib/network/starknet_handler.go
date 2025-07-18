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

type StarknetHandler struct {
	config  *NetworkConfig
	version string
	logger  *logger.LoggerClient
}

func NewStarknetHandler(config *NetworkConfig) *StarknetHandler {
	return &StarknetHandler{
		config:  config,
		version: "1.0.0",
		logger:  config.Logger,
	}
}

// Metadata methods for registry
func (h *StarknetHandler) GetType() string {
	return "starknet"
}

func (h *StarknetHandler) GetName() string {
	return "Starknet JSON-RPC Handler"
}

func (h *StarknetHandler) GetVersion() string {
	return h.version
}

func (h *StarknetHandler) GetRequestType() RequestType {
	return RequestTypeRPC
}

// Lifecycle methods
func (h *StarknetHandler) Initialize(config *NetworkConfig) error {
	h.config = config
	return nil
}

func (h *StarknetHandler) Shutdown() error {
	return nil
}

// === EXISTING METHODS ===

func (h *StarknetHandler) ProcessRequest(req *http.Request) error {
	// Path translation is handled in DinSelect module
	// This method is mainly for request validation

	return nil
}

func (h *StarknetHandler) ValidateRequest(req *http.Request) error {
	// Check content type
	contentType := req.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		return fmt.Errorf("invalid content type for Starknet JSON-RPC: %s", contentType)
	}

	// Check method
	if req.Method != "POST" {
		return fmt.Errorf("Starknet JSON-RPC requires POST method, got %s", req.Method)
	}

	return nil
}

func (h *StarknetHandler) ParseResponse(body []byte, statusCode int) error {
	// Use the shared JSON-RPC response parser
	return ParseJSONRPCResponse(body, statusCode)
}

func (h *StarknetHandler) IsRetryableError(err error, statusCode int) bool {
	// Use the shared JSON-RPC error retry logic
	return IsRetryableJSONRPCError(err, statusCode)
}

// === NEW NETWORK-SPECIFIC METHODS ===

// Chain ID and Namespace methods
func (h *StarknetHandler) GetNamespace() string {
	return "starknet"
}

func (h *StarknetHandler) ValidateChainID(chainID string) error {
	// Starknet chain IDs have format: starknet:0x{hex}
	// Examples:
	// - starknet:0x534e5f4d41494e (mainnet)
	// - starknet:0x534e5f5345504f4c4941 (sepolia)

	if !strings.HasPrefix(chainID, "starknet:") {
		return fmt.Errorf("invalid Starknet chain ID format: %s, expected format: starknet:0x{hex}", chainID)
	}

	hexPart := strings.TrimPrefix(chainID, "starknet:")
	if !strings.HasPrefix(hexPart, "0x") {
		return fmt.Errorf("invalid Starknet chain ID hex format: %s", chainID)
	}

	// Validate hex format
	hexDigits := strings.TrimPrefix(hexPart, "0x")
	if len(hexDigits) == 0 {
		return fmt.Errorf("empty hex part in Starknet chain ID: %s", chainID)
	}

	// Check if all characters are valid hex
	for _, char := range hexDigits {
		if !((char >= '0' && char <= '9') ||
			(char >= 'a' && char <= 'f') ||
			(char >= 'A' && char <= 'F')) {
			return fmt.Errorf("invalid hex character in Starknet chain ID: %s", chainID)
		}
	}

	return nil
}

func (h *StarknetHandler) FormatChainID(networkReference string) string {
	return "starknet:" + networkReference
}

func (h *StarknetHandler) ExtractChainReference(result interface{}) (string, error) {
	chainRef, ok := result.(string)
	if !ok {
		return "", fmt.Errorf("invalid chain reference type: %T", result)
	}
	return chainRef, nil
}

// Block Operations methods
func (h *StarknetHandler) FormatBlockHeight(blockNum int64) string {
	return strconv.FormatInt(blockNum, 10) // Decimal format for Starknet
}

func (h *StarknetHandler) CreateBlockRequest(method string, blockNum int64, includeTransactions bool) ([]byte, error) {
	blockDecimal := h.FormatBlockHeight(blockNum)
	payload := fmt.Sprintf(`{"jsonrpc":"2.0","method":"%s","id":1,"params":["%s",%t]}`,
		method, blockDecimal, includeTransactions)
	return []byte(payload), nil
}

func (h *StarknetHandler) ParseBlockResponse(body []byte) (interface{}, error) {
	// Starknet block responses have different structure than EVM
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Starknet block response: %w", err)
	}
	return response, nil
}

// ParseBlockNumberResponse parses the block number from a raw response
func (h *StarknetHandler) ParseBlockNumberResponse(body []byte, statusCode int) (int64, error) {
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

	// Starknet can return block numbers as either numeric or hex values
	// First try to parse as a number
	if num, err := ParseNumericBlockNumber(response.Result); err == nil {
		return num, nil
	}

	// If that fails, try hex format
	return ParseHexBlockNumber(response.Result)
}

// RequiresSeparateBlockInfoCall returns false for Starknet as health check includes block info
func (h *StarknetHandler) RequiresSeparateBlockInfoCall() bool {
	return false
}

// GetBlockInfoMethod returns empty for Starknet as it uses the same endpoint for health and block info
func (h *StarknetHandler) GetBlockInfoMethod() string {
	return "" // Not used for JSON-RPC chains
}

// GetChainID retrieves the chain ID from the Starknet provider
func (h *StarknetHandler) GetChainID(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int) (string, error) {
	// Use the shared JSON-RPC logic with Starknet-specific parsing
	return GetChainIDViaJSONRPC(httpUrl, headers, httpClient, authClient, requestAttempts, h.GetChainIDMethod(), h.ParseChainIDResponse)
}

// Archive Mode methods
func (h *StarknetHandler) SupportsArchiveMode() bool {
	return true
}

func (h *StarknetHandler) GetArchiveMethod() string {
	return "starknet_getBlockWithTxs"
}

func (h *StarknetHandler) CreateArchivePayload(method string, blockHeight string) ([]byte, error) {
	blockNum, err := strconv.ParseInt(blockHeight, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid block height for Starknet: %s", blockHeight)
	}
	payload := fmt.Sprintf(`{"jsonrpc":"2.0","method":"%s","id":1,"params":[{"block_number":%d}]}`,
		method, blockNum)
	return []byte(payload), nil
}

func (h *StarknetHandler) ParseArchiveResponse(body []byte) error {
	var respObject map[string]interface{}
	if err := json.Unmarshal(body, &respObject); err != nil {
		return fmt.Errorf("failed to unmarshal Starknet archive response: %w", err)
	}

	if _, ok := respObject["error"]; ok {
		return fmt.Errorf("archive mode not supported")
	}

	// Starknet-specific response validation
	result, ok := respObject["result"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid Starknet archive response structure")
	}

	blockHash, ok := result["block_hash"].(string)
	if !ok || blockHash == "" {
		return fmt.Errorf("missing or invalid block_hash in Starknet archive response")
	}

	return nil
}

// Network Capabilities methods
func (h *StarknetHandler) SupportsGetBlockByNumber() bool {
	return false // Starknet skips getBlockByNumber
}

func (h *StarknetHandler) GetSupportedMethods() []string {
	return []string{
		"starknet_blockNumber",
		"starknet_chainId",
		"starknet_call",
		"starknet_getBlockByNumber",
		"starknet_getBlockByHash",
		"starknet_getTransactionByHash",
		"starknet_getTransactionReceipt",
		"starknet_getBalance",
		"starknet_syncing",
		"starknet_sendTransaction",
		"starknet_getBlockWithTxs",
	}
}

// Data Format Conversions methods
func (h *StarknetHandler) ExtractBlockHash(blockData interface{}) string {
	if blockMap, ok := blockData.(map[string]interface{}); ok {
		if result, ok := blockMap["result"].(map[string]interface{}); ok {
			if blockHash, ok := result["block_hash"].(string); ok {
				return blockHash
			}
		}
	}
	return ""
}

func (h *StarknetHandler) ExtractBlockNumber(response []byte) (int64, error) {
	var respObject map[string]interface{}
	if err := json.Unmarshal(response, &respObject); err != nil {
		return 0, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	var blockNumber int64
	switch result := respObject["result"].(type) {
	case string:
		var err error
		blockNumber, err = strconv.ParseInt(result, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse Starknet block number: %w", err)
		}
	case float64:
		blockNumber = int64(result)
	default:
		return 0, fmt.Errorf("unsupported Starknet block number type: %T", result)
	}

	return blockNumber, nil
}

// Health Check Specifics methods
func (h *StarknetHandler) GetHealthCheckMethod() string {
	return "starknet_blockNumber"
}

func (h *StarknetHandler) GetHealthCheckHTTPMethod() string {
	return "POST" // Starknet uses JSON-RPC POST requests
}

func (h *StarknetHandler) GetChainIDMethod() string {
	return "starknet_chainId"
}

func (h *StarknetHandler) CreateHealthCheckPayload(method string) ([]byte, error) {
	payload := fmt.Sprintf(`{"jsonrpc":"2.0","method":"%s","id":1}`, method)
	return []byte(payload), nil
}

func (h *StarknetHandler) ParseHealthCheckResponse(body []byte) (*BlockInfo, error) {
	var respObject map[string]interface{}
	if err := json.Unmarshal(body, &respObject); err != nil {
		return nil, fmt.Errorf("failed to parse Starknet health check response: %w", err)
	}

	var blockNumber int64
	switch result := respObject["result"].(type) {
	case string:
		var err error
		blockNumber, err = strconv.ParseInt(result, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse Starknet block number: %w", err)
		}
	case float64:
		blockNumber = int64(result)
	default:
		return nil, fmt.Errorf("unsupported Starknet block number type: %T", result)
	}

	return &BlockInfo{
		Number:    blockNumber,
		Hash:      "",
		Timestamp: time.Now(),
	}, nil
}

func (h *StarknetHandler) ParseChainIDResponse(body []byte, statusCode int) (string, error) {
	if statusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP error: %d", statusCode)
	}

	var respObject map[string]interface{}
	if err := json.Unmarshal(body, &respObject); err != nil {
		return "", fmt.Errorf("failed to parse Starknet chain ID response: %w", err)
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

	// Extract chain reference and format full chain ID
	chainReference, err := h.ExtractChainReference(result)
	if err != nil {
		return "", fmt.Errorf("failed to extract chain reference: %w", err)
	}

	return h.FormatChainID(chainReference), nil
}

// GetLatestBlockNumber retrieves the latest block number for Starknet chains
// Uses the JSON-RPC method starknet_blockNumber to get the current block height
func (h *StarknetHandler) GetLatestBlockNumber(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int) (*LatestBlockResult, error) {
	return GetLatestBlockNumberViaJSONRPC(
		httpUrl,
		headers,
		httpClient,
		authClient,
		requestAttempts,
		h.GetHealthCheckMethod(), // "starknet_blockNumber"
		ParseNumericBlockNumber,  // Starknet returns numeric block numbers
	)
}

// PerformArchiveCheck performs archive mode check for Starknet chains using JSON-RPC
func (h *StarknetHandler) PerformArchiveCheck(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int, blockHeight string) error {
	// Use the shared JSON-RPC helper for archive checks
	return PerformArchiveCheckViaJSONRPC(
		httpUrl,
		headers,
		httpClient,
		authClient,
		requestAttempts,
		h.GetArchiveMethod(), // "starknet_getBlockWithTxHashes"
		blockHeight,
		h.CreateArchivePayload, // Starknet-specific payload creation
		h.ParseArchiveResponse, // Starknet-specific response parsing
	)
}

// PerformGetBlockByNumber performs get block by number operation for Starknet chains using JSON-RPC
func (h *StarknetHandler) PerformGetBlockByNumber(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int, blockNumber int64) (interface{}, error) {
	// Use the shared JSON-RPC helper for get block by number operations
	return PerformGetBlockByNumberViaJSONRPC(
		httpUrl,
		headers,
		httpClient,
		authClient,
		requestAttempts,
		blockNumber,
		h.GetSupportedMethods, // Starknet-specific supported methods
		h.CreateBlockRequest,  // Starknet-specific block request creation
		h.ParseBlockResponse,  // Starknet-specific block response parsing
	)
}

// === COMPATIBILITY METHODS (keeping existing methods) ===

// ValidateChainID validates Starknet-specific chain ID format (legacy method)
func (h *StarknetHandler) ValidateChainIDLegacy(chainID string) bool {
	// Use the new ValidateChainID method and return boolean
	return h.ValidateChainID(chainID) == nil
}

// GetDefaultMethods returns the default Starknet methods (legacy method)
func (h *StarknetHandler) GetDefaultMethods() []string {
	return h.GetSupportedMethods()
}

// GetCallContractMethod returns the Starknet call contract method (legacy method)
func (h *StarknetHandler) GetCallContractMethod() string {
	return "starknet_call"
}

// GetBlockByNumberMethod returns the Starknet get block by number method (legacy method)
func (h *StarknetHandler) GetBlockByNumberMethod() string {
	return "starknet_getBlockWithTxHashes"
}
