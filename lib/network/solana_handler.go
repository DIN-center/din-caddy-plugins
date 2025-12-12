package network

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
	dinhttp "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
)

var _ NetworkHandler = (*SolanaHandler)(nil)

type SolanaHandler struct {
	config  *NetworkConfig
	version string
	logger  *logger.LoggerClient
}

func NewSolanaHandler(config *NetworkConfig) *SolanaHandler {
	return &SolanaHandler{
		config:  config,
		version: "1.0.0",
		logger:  config.Logger,
	}
}

// Metadata methods for registry
func (h *SolanaHandler) GetType() string {
	return "solana" // Must match modules.SolanaHandler constant value
}

func (h *SolanaHandler) GetName() string {
	return "Solana JSON-RPC Handler"
}

func (h *SolanaHandler) GetVersion() string {
	return h.version
}

func (h *SolanaHandler) GetRequestType() RequestType {
	return RequestTypeRPC
}

// Lifecycle methods
func (h *SolanaHandler) Initialize(config *NetworkConfig) error {
	h.config = config

	if config.Logger != nil {
		h.logger = config.Logger
	}

	return nil
}

func (h *SolanaHandler) Shutdown() error {
	return nil
}

// === EXISTING METHODS ===

func (h *SolanaHandler) ProcessRequest(req *http.Request) error {
	// Validate the request using our validation logic
	if err := h.ValidateRequest(req); err != nil {
		return err
	}

	// Path translation is handled in DinSelect module
	return nil
}

// ExtractMethod extracts the JSON-RPC method from the request body
func (h *SolanaHandler) ExtractMethod(req *http.Request, body []byte) (string, error) {
	if len(body) == 0 {
		return "", fmt.Errorf("empty request body")
	}

	var rpcRequest dinhttp.JSONRPCRequest
	if err := json.Unmarshal(body, &rpcRequest); err != nil {
		return "", fmt.Errorf("failed to parse JSON-RPC request: %w", err)
	}

	if rpcRequest.Method == "" {
		return "", fmt.Errorf("missing method in JSON-RPC request")
	}

	return rpcRequest.Method, nil
}

// ConfigureRequestPath configures the request path for JSON-RPC requests
func (h *SolanaHandler) ConfigureRequestPath(req *http.Request, providerPath string, providerQuery string, networkName string) error {
	ConfigureJSONRPCRequestPath(req, providerPath)
	// Note: providerQuery is not used for Solana currently. Can be implemented if needed.
	return nil
}

func (h *SolanaHandler) ValidateRequest(req *http.Request) error {
	// Check content type
	contentType := req.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		return fmt.Errorf("invalid content type for Solana JSON-RPC: %s", contentType)
	}

	// Check method
	if req.Method != "POST" {
		return fmt.Errorf("the Solana JSON-RPC requires POST method, got %s", req.Method)
	}

	return nil
}

func (h *SolanaHandler) ParseResponse(body []byte, statusCode int) error {
	// Use the shared JSON-RPC response parser
	return ParseJSONRPCResponse(body, statusCode)
}

func (h *SolanaHandler) IsRetryableError(err error, statusCode int) bool {
	// Use the shared JSON-RPC error retry logic
	return IsRetryableJSONRPCError(err, statusCode)
}

// === NEW NETWORK-SPECIFIC METHODS ===

// Chain ID and Namespace methods
func (h *SolanaHandler) GetNamespace() string {
	return "solana"
}

func (h *SolanaHandler) ValidateChainID(chainID string) error {
	// Fail if there's a colon (old CAIP-2 format)
	if strings.Contains(chainID, ":") {
		return fmt.Errorf("invalid Solana chain ID format: %s, chain ID should not contain ':' (CAIP-2 prefix no longer required)", chainID)
	}

	if len(chainID) == 0 {
		return fmt.Errorf("empty chain ID")
	}

	// Basic validation - Solana genesis hashes are base58 encoded, typically 44 characters
	if len(chainID) < 32 || len(chainID) > 50 {
		return fmt.Errorf("invalid Solana genesis hash length: %s", chainID)
	}

	return nil
}

func (h *SolanaHandler) ExtractChainReference(result interface{}) (string, error) {
	genesisHash, ok := result.(string)
	if !ok {
		return "", fmt.Errorf("invalid genesis hash type: %T", result)
	}
	return genesisHash, nil
}

// Block Operations methods
func (h *SolanaHandler) FormatBlockHeight(blockNum int64) string {
	return strconv.FormatInt(blockNum, 10) // Decimal format for Solana
}

func (h *SolanaHandler) CreateBlockRequest(method string, blockNum int64, includeTransactions bool) ([]byte, error) {
	// CRITICAL FIX: Use getBlock for specific block data, not getBlockHeight
	// getBlockHeight returns current height and doesn't accept block numbers
	// getBlock is used to retrieve specific block data

	blockMethod := "getBlock"
	if includeTransactions {
		payload := fmt.Sprintf(`{"jsonrpc":"2.0","method":"%s","id":1,"params":[%d,{"encoding":"json","transactionDetails":"full","rewards":false}]}`,
			blockMethod, blockNum)
		return []byte(payload), nil
	} else {
		payload := fmt.Sprintf(`{"jsonrpc":"2.0","method":"%s","id":1,"params":[%d,{"encoding":"json","transactionDetails":"none","rewards":false}]}`,
			blockMethod, blockNum)
		return []byte(payload), nil
	}
}

func (h *SolanaHandler) ParseBlockResponse(body []byte) (interface{}, error) {
	// First check for JSON-RPC errors using generic response
	var genericResponse dinhttp.JSONRPCResponse
	if err := json.Unmarshal(body, &genericResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON-RPC response: %w", err)
	}

	// Check for JSON-RPC errors in getBlock response
	if genericResponse.Error != nil {
		return nil, fmt.Errorf("solana getBlock error: %s", genericResponse.Error.Message)
	}

	// Parse as Solana-specific response if no errors
	var response dinhttp.JSONRPCSolanaBlockResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Solana block response: %w", err)
	}

	return response, nil
}

// ParseBlockNumberResponse parses the block number from a raw response
func (h *SolanaHandler) ParseBlockNumberResponse(body []byte, statusCode int) (int64, error) {
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

	// Solana returns block numbers as numeric values
	return ParseNumericBlockNumber(response.Result)
}

// RequiresSeparateBlockInfoCall returns false for Solana as health check includes block info
func (h *SolanaHandler) RequiresSeparateBlockInfoCall() bool {
	return false
}

// GetBlockInfoMethod returns empty for Solana as it uses the same endpoint for health and block info
func (h *SolanaHandler) GetBlockInfoMethod() string {
	return "" // Not used for JSON-RPC chains
}

// GetChainID retrieves the chain ID from the Solana provider
func (h *SolanaHandler) GetChainID(httpUrl string, headers map[string]string, httpClient dinhttp.IHTTPClient, authClient auth.IAuthClient, requestAttempts int) (string, error) {
	// Use the shared JSON-RPC logic with Solana-specific parsing
	return GetChainIDViaJSONRPC(httpUrl, headers, httpClient, authClient, requestAttempts, h.GetChainIDMethod(), h.ParseChainIDResponse)
}

// Archive Mode methods
func (h *SolanaHandler) SupportsArchiveMode() bool {
	return false // Solana doesn't support archive mode
}

func (h *SolanaHandler) GetArchiveMethod() string {
	return ""
}

func (h *SolanaHandler) CreateArchivePayload(method string, blockHeight string) ([]byte, error) {
	return nil, fmt.Errorf("the Solana does not support archive mode")
}

func (h *SolanaHandler) ParseArchiveResponse(body []byte) error {
	return fmt.Errorf("the Solana does not support archive mode")
}

// Network Capabilities methods
func (h *SolanaHandler) SupportsGetBlockByNumber() bool {
	return true
}

func (h *SolanaHandler) GetSupportedMethods() []string {
	return []string{
		"getBlockHeight",
		"getBlock",
		"getAccountInfo",
		"getBalance",
		"sendTransaction",
		"getTransaction",
		"getRecentBlockhash",
		"getSlot",
	}
}

func (h *SolanaHandler) GetBlockByNumberMethod() string {
	return "getBlock"
}

// Data Format Conversions methods
func (h *SolanaHandler) ExtractBlockHash(blockData interface{}) string {
	if blockResponse, ok := blockData.(dinhttp.JSONRPCSolanaBlockResponse); ok {
		return blockResponse.Result.Blockhash
	}
	return ""
}

func (h *SolanaHandler) ExtractBlockNumber(response []byte) (int64, error) {
	var respObject map[string]interface{}
	if err := json.Unmarshal(response, &respObject); err != nil {
		return 0, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	result, ok := respObject["result"].(float64)
	if !ok {
		return 0, fmt.Errorf("invalid Solana block height response format")
	}

	return int64(result), nil
}

// Health Check Specifics methods
func (h *SolanaHandler) GetHealthCheckMethod() string {
	// getBlockHeight is used for health checks only (no parameters required)
	// For specific block data retrieval, use getBlock method via CreateBlockRequest
	return "getBlockHeight"
}

func (h *SolanaHandler) GetHealthCheckHTTPMethod() string {
	return "POST" // Solana uses JSON-RPC POST requests
}

func (h *SolanaHandler) GetChainIDMethod() string {
	return "getGenesisHash"
}

func (h *SolanaHandler) CreateHealthCheckPayload(method string) ([]byte, error) {
	payload := fmt.Sprintf(`{"jsonrpc":"2.0","method":"%s","id":1}`, method)
	return []byte(payload), nil
}

func (h *SolanaHandler) ParseHealthCheckResponse(body []byte) (*BlockInfo, error) {
	var respObject map[string]interface{}
	if err := json.Unmarshal(body, &respObject); err != nil {
		return nil, fmt.Errorf("failed to parse Solana health check response: %w", err)
	}

	result, ok := respObject["result"].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid Solana health check response format")
	}

	return &BlockInfo{
		Number:    int64(result),
		Hash:      "",
		Timestamp: time.Now(),
	}, nil
}

func (h *SolanaHandler) ParseChainIDResponse(body []byte, statusCode int) (string, error) {
	if statusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP error: %d", statusCode)
	}

	var respObject map[string]interface{}
	if err := json.Unmarshal(body, &respObject); err != nil {
		return "", fmt.Errorf("failed to parse Solana chain ID response: %w", err)
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

	// Extract chain reference (already in correct format without prefix)
	chainReference, err := h.ExtractChainReference(result)
	if err != nil {
		return "", fmt.Errorf("failed to extract chain reference: %w", err)
	}

	return chainReference, nil
}

// GetLatestBlockNumber retrieves the latest block number for Solana chains
// Uses the JSON-RPC method getBlockHeight to get the current block height
func (h *SolanaHandler) GetLatestBlockNumber(httpUrl string, headers map[string]string, httpClient dinhttp.IHTTPClient, authClient auth.IAuthClient, requestAttempts int) (*LatestBlockResult, error) {
	// Use the shared JSON-RPC helper with Solana-specific numeric parsing
	return GetLatestBlockNumberViaJSONRPC(
		httpUrl,
		headers,
		httpClient,
		authClient,
		requestAttempts,
		h.GetHealthCheckMethod(), // "getBlockHeight"
		ParseNumericBlockNumber,  // Solana returns numeric block heights
	)
}

// PerformArchiveCheck performs archive mode check for Solana chains using JSON-RPC
func (h *SolanaHandler) PerformArchiveCheck(httpUrl string, headers map[string]string, httpClient dinhttp.IHTTPClient, authClient auth.IAuthClient, requestAttempts int, blockHeight string) error {
	// Use the shared JSON-RPC helper for archive checks
	return PerformArchiveCheckViaJSONRPC(
		httpUrl,
		headers,
		httpClient,
		authClient,
		requestAttempts,
		h.GetArchiveMethod(), // "getBlock"
		blockHeight,
		h.CreateArchivePayload, // Solana-specific payload creation
		h.ParseArchiveResponse, // Solana-specific response parsing
	)
}

// PerformGetBlockByNumber performs get block by number operation for Solana chains using JSON-RPC
func (h *SolanaHandler) PerformGetBlockByNumber(httpUrl string, headers map[string]string, httpClient dinhttp.IHTTPClient, authClient auth.IAuthClient, requestAttempts int, blockNumber int64) (interface{}, error) {
	// Use the shared JSON-RPC helper for get block by number operations
	return PerformGetBlockByNumberViaJSONRPC(
		httpUrl,
		headers,
		httpClient,
		authClient,
		requestAttempts,
		blockNumber,
		h.GetSupportedMethods, // Solana-specific supported methods
		h.CreateBlockRequest,  // Solana-specific block request creation
		h.ParseBlockResponse,  // Solana-specific block response parsing
	)
}
