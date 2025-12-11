// lib/network/evm_handler.go
package network

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"

	"go.uber.org/zap"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
)

var _ NetworkHandler = (*EVMHandler)(nil)

// EVMHandler handles EVM-compatible JSON-RPC networks
type EVMHandler struct {
	config  *NetworkConfig
	version string
	logger  *logger.LoggerClient
}

// NewEVMHandler creates a new EVM handler instance
func NewEVMHandler(config *NetworkConfig) *EVMHandler {
	return &EVMHandler{
		config:  config,
		version: "1.0.0",
		logger:  config.Logger,
	}
}

// Metadata methods for registry
func (h *EVMHandler) GetType() string {
	return "evm" // Must match modules.EVMHandler constant value
}

func (h *EVMHandler) GetName() string {
	return "EVM JSON-RPC Handler"
}

func (h *EVMHandler) GetVersion() string {
	return h.version
}

func (h *EVMHandler) GetRequestType() RequestType {
	return RequestTypeRPC
}

// Lifecycle methods
func (h *EVMHandler) Initialize(config *NetworkConfig) error {
	h.config = config

	if config.Logger != nil {
		h.logger = config.Logger
	}

	return nil
}

func (h *EVMHandler) Shutdown() error {
	// No cleanup needed for EVM handler
	return nil
}

// === EXISTING METHODS ===

// Request processing methods
func (h *EVMHandler) ProcessRequest(req *http.Request) error {
	// Validate the request using our validation logic
	if err := h.ValidateRequest(req); err != nil {
		return err
	}

	// Path translation is handled in DinSelect module
	return nil
}

// ExtractMethod extracts the JSON-RPC method from the request body
func (h *EVMHandler) ExtractMethod(req *http.Request, body []byte) (string, error) {
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

func (h *EVMHandler) ValidateRequest(req *http.Request) error {
	// Check content type
	contentType := req.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		return fmt.Errorf("invalid content type for JSON-RPC: %s", contentType)
	}

	// Check method
	if req.Method != "POST" {
		return fmt.Errorf("JSON-RPC requires POST method, got %s", req.Method)
	}

	// Parse and validate JSON-RPC payload structure
	if req.Body != nil {
		// Read the body
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return fmt.Errorf("failed to read request body: %w", err)
		}

		// Reset the body so downstream handlers can read it again
		req.Body = io.NopCloser(strings.NewReader(string(body)))

		// Parse JSON-RPC request
		var jsonRPCReq din_http.JSONRPCRequest
		if err := json.Unmarshal(body, &jsonRPCReq); err != nil {
			return fmt.Errorf("invalid JSON payload: %w", err)
		}

		// Validate required JSON-RPC fields
		if jsonRPCReq.JSONRPC == "" {
			return fmt.Errorf("missing required field: jsonrpc")
		}
		if jsonRPCReq.JSONRPC != "2.0" {
			return fmt.Errorf("invalid jsonrpc version, expected '2.0', got '%s'", jsonRPCReq.JSONRPC)
		}
		if jsonRPCReq.Method == "" {
			return fmt.Errorf("missing required field: method")
		}
		// ID field is technically optional in notifications, but params should be present (can be null/empty)
		// We don't need to validate the actual method name here as that's provider-specific
	}

	return nil
}

// ConfigureRequestPath configures the request path for JSON-RPC requests
func (h *EVMHandler) ConfigureRequestPath(req *http.Request, providerPath string, providerQuery string, networkName string) error {
	ConfigureJSONRPCRequestPath(req, providerPath)

	// Merge provider query params with request query params
	// Provider query params take precedence (placed first)
	if providerQuery != "" {
		if req.URL.RawQuery == "" {
			req.URL.RawQuery = providerQuery
		} else {
			req.URL.RawQuery = providerQuery + "&" + req.URL.RawQuery
		}
	}

	return nil
}

// Response handling methods
func (h *EVMHandler) ParseResponse(body []byte, statusCode int) error {
	// Use shared JSON-RPC response parsing logic
	return ParseJSONRPCResponse(body, statusCode)
}

func (h *EVMHandler) IsRetryableError(err error, statusCode int) bool {
	// Use shared JSON-RPC retry logic
	return IsRetryableJSONRPCError(err, statusCode)
}

// === NEW NETWORK-SPECIFIC METHODS ===

// Chain ID and Namespace methods
func (h *EVMHandler) GetNamespace() string {
	return "eip155"
}

func (h *EVMHandler) ValidateChainID(chainID string) error {
	if chainID == "" {
		return fmt.Errorf("empty chain ID")
	}

	// Support both formats for backwards compatibility
	actualChainID := chainID

	// If it contains a colon, it might be CAIP-2 format
	if strings.Contains(chainID, ":") {
		parts := strings.Split(chainID, ":")
		if len(parts) == 2 && parts[0] == "eip155" {
			// Valid CAIP-2 format for EVM, extract the actual chain ID
			actualChainID = parts[1]
			// Log that we're using backwards compatibility
			if h.logger != nil {
				h.logger.Debug("Using CAIP-2 format for backwards compatibility",
					zap.String("original", chainID),
					zap.String("extracted", actualChainID))
			}
		} else {
			// Invalid format
			return fmt.Errorf("invalid EVM chain ID format: %s, expected format 'eip155:chainID' or just 'chainID'", chainID)
		}
	}

	// Validate the actual chain ID (with or without 0x prefix)
	chainIDNum := strings.TrimPrefix(actualChainID, "0x")

	// Convert to ensure it's a valid number
	if _, err := strconv.ParseInt(chainIDNum, 16, 64); err != nil {
		// Try decimal format as well
		if _, err := strconv.ParseInt(actualChainID, 10, 64); err != nil {
			return fmt.Errorf("invalid chain ID number %s: must be hex (0x...) or decimal", actualChainID)
		}
	}

	return nil
}

func (h *EVMHandler) ExtractChainReference(result interface{}) (string, error) {
	chainRef, ok := result.(string)
	if !ok {
		return "", fmt.Errorf("invalid chain reference type: %T", result)
	}
	return chainRef, nil
}

// Block Operations methods
func (h *EVMHandler) FormatBlockHeight(blockNum int64) string {
	return fmt.Sprintf("%#x", blockNum) // Hex format for EVM
}

func (h *EVMHandler) CreateBlockRequest(method string, blockNum int64, includeTransactions bool) ([]byte, error) {
	payload := fmt.Sprintf(`{"jsonrpc":"2.0","method":"%s","id":1,"params":["%#x",%t]}`,
		method, blockNum, includeTransactions)
	return []byte(payload), nil
}

func (h *EVMHandler) ParseBlockResponse(body []byte) (interface{}, error) {
	var response din_http.JSONRPCEVMBlockResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal EVM block response: %w", err)
	}
	return response, nil
}

// ParseBlockNumberResponse parses the block number from a raw response
func (h *EVMHandler) ParseBlockNumberResponse(body []byte, statusCode int) (int64, error) {
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

	// EVM chains always return block numbers as hex strings
	return ParseHexBlockNumber(response.Result)
}

// RequiresSeparateBlockInfoCall returns false for EVM as health check includes block info
func (h *EVMHandler) RequiresSeparateBlockInfoCall() bool {
	return false
}

// GetBlockInfoMethod returns empty for EVM as it uses the same endpoint for health and block info
func (h *EVMHandler) GetBlockInfoMethod() string {
	return "" // Not used for JSON-RPC chains
}

// GetChainID retrieves the chain ID from the EVM provider
func (h *EVMHandler) GetChainID(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int) (string, error) {
	// Use the shared JSON-RPC logic with EVM-specific parsing
	return GetChainIDViaJSONRPC(httpUrl, headers, httpClient, authClient, requestAttempts, h.GetChainIDMethod(), h.ParseChainIDResponse)
}

// Archive Mode methods
func (h *EVMHandler) SupportsArchiveMode() bool {
	return true
}

func (h *EVMHandler) GetArchiveMethod() string {
	return "eth_getBalance"
}

func (h *EVMHandler) CreateArchivePayload(method string, blockHeight string) ([]byte, error) {
	payload := fmt.Sprintf(`{"jsonrpc":"2.0","method":"%s","id":1,"params":["0x0000000000000000000000000000000000000000","%s"]}`,
		method, blockHeight)
	return []byte(payload), nil
}

func (h *EVMHandler) ParseArchiveResponse(body []byte) error {
	var respObject map[string]interface{}
	if err := json.Unmarshal(body, &respObject); err != nil {
		return fmt.Errorf("failed to unmarshal archive response: %w", err)
	}

	if _, ok := respObject["error"]; ok {
		return fmt.Errorf("archive mode not supported")
	}

	return nil
}

// Network Capabilities methods
func (h *EVMHandler) SupportsGetBlockByNumber() bool {
	return true
}

func (h *EVMHandler) GetSupportedMethods() []string {
	return []string{
		"eth_blockNumber",
		"eth_chainId",
		"eth_call",
		"eth_getBlockByNumber",
		"eth_getBlockByHash",
		"eth_getBalance",
		"eth_getTransactionByHash",
		"eth_getTransactionReceipt",
		"eth_sendRawTransaction",
		"eth_gasPrice",
		"eth_estimateGas",
	}
}

func (h *EVMHandler) GetBlockByNumberMethod() string {
	return "eth_getBlockByNumber"
}

// Data Format Conversions methods
func (h *EVMHandler) ExtractBlockHash(blockData interface{}) string {
	if blockResponse, ok := blockData.(din_http.JSONRPCEVMBlockResponse); ok {
		return blockResponse.Result.Hash
	}
	return ""
}

func (h *EVMHandler) ExtractBlockNumber(response []byte) (int64, error) {
	var respObject map[string]interface{}
	if err := json.Unmarshal(response, &respObject); err != nil {
		return 0, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	result, ok := respObject["result"].(string)
	if !ok {
		return 0, fmt.Errorf("invalid block number response format")
	}

	if !strings.HasPrefix(result, "0x") {
		return 0, fmt.Errorf("invalid block number format: %s", result)
	}

	blockNumber, err := strconv.ParseInt(result[2:], 16, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse block number: %w", err)
	}

	return blockNumber, nil
}

// Health Check Specifics methods
func (h *EVMHandler) GetHealthCheckMethod() string {
	return "eth_blockNumber"
}

func (h *EVMHandler) GetHealthCheckHTTPMethod() string {
	return "POST" // EVM uses JSON-RPC POST requests
}

func (h *EVMHandler) GetChainIDMethod() string {
	return "eth_chainId"
}

func (h *EVMHandler) CreateHealthCheckPayload(method string) ([]byte, error) {
	payload := fmt.Sprintf(`{"jsonrpc":"2.0","method":"%s","id":1}`, method)
	return []byte(payload), nil
}

func (h *EVMHandler) ParseHealthCheckResponse(body []byte) (*BlockInfo, error) {
	var respObject map[string]interface{}
	if err := json.Unmarshal(body, &respObject); err != nil {
		return nil, fmt.Errorf("failed to parse health check response: %w", err)
	}

	result, ok := respObject["result"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid health check response format")
	}

	if !strings.HasPrefix(result, "0x") {
		return nil, fmt.Errorf("invalid block number format: %s", result)
	}

	blockNumber, err := strconv.ParseInt(result[2:], 16, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse block number: %w", err)
	}

	return &BlockInfo{
		Number:    blockNumber,
		Hash:      "",
		Timestamp: time.Now(),
	}, nil
}

func (h *EVMHandler) ParseChainIDResponse(body []byte, statusCode int) (string, error) {
	if statusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP error: %d", statusCode)
	}

	var respObject map[string]interface{}
	if err := json.Unmarshal(body, &respObject); err != nil {
		return "", fmt.Errorf("failed to parse EVM chain ID response: %w", err)
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

// GetLatestBlockNumber retrieves the latest block number for EVM chains
// Uses the JSON-RPC method eth_blockNumber to get the current block height
func (h *EVMHandler) GetLatestBlockNumber(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int) (*LatestBlockResult, error) {
	// Use the shared JSON-RPC helper with EVM-specific hex parsing
	return GetLatestBlockNumberViaJSONRPC(
		httpUrl,
		headers,
		httpClient,
		authClient,
		requestAttempts,
		h.GetHealthCheckMethod(), // "eth_blockNumber"
		ParseHexBlockNumber,      // EVM uses hex-encoded block numbers
	)
}

// PerformArchiveCheck performs archive mode check for EVM chains using JSON-RPC
func (h *EVMHandler) PerformArchiveCheck(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int, blockHeight string) error {
	// Use the shared JSON-RPC helper for archive checks
	return PerformArchiveCheckViaJSONRPC(
		httpUrl,
		headers,
		httpClient,
		authClient,
		requestAttempts,
		h.GetArchiveMethod(), // "eth_getBlockByNumber"
		blockHeight,
		h.CreateArchivePayload, // EVM-specific payload creation
		h.ParseArchiveResponse, // EVM-specific response parsing
	)
}

// PerformTraceBlockByNumberCheck performs debug_traceBlockByNumber check for EVM chains
// This is an additional archive mode check that verifies trace capabilities (MetaMask requirement)
func (h *EVMHandler) PerformTraceBlockByNumberCheck(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int, blockHeight string) error {
	return PerformTraceBlockByNumberCheckViaJSONRPC(
		httpUrl,
		headers,
		httpClient,
		authClient,
		requestAttempts,
		blockHeight,
		h.CreateTraceBlockByNumberPayload,
		h.ParseTraceBlockByNumberResponse,
	)
}

// CreateTraceBlockByNumberPayload creates the debug_traceBlockByNumber request payload
func (h *EVMHandler) CreateTraceBlockByNumberPayload(blockHeight string) ([]byte, error) {
	payload := fmt.Sprintf(`{"jsonrpc":"2.0","method":"debug_traceBlockByNumber","id":1,"params":["%s",{"tracer":"callTracer","timeout":"30s","onlyTopCall":true}]}`, blockHeight)
	return []byte(payload), nil
}

// ParseTraceBlockByNumberResponse validates the debug_traceBlockByNumber response
func (h *EVMHandler) ParseTraceBlockByNumberResponse(body []byte) error {
	var respObject map[string]interface{}
	if err := json.Unmarshal(body, &respObject); err != nil {
		return fmt.Errorf("failed to unmarshal trace response: %w", err)
	}

	// Check for JSON-RPC error
	if errField, ok := respObject["error"]; ok && errField != nil {
		return fmt.Errorf("debug_traceBlockByNumber not supported: %v", errField)
	}

	return nil
}

// PerformGetBlockByNumber performs get block by number operation for EVM chains using JSON-RPC
func (h *EVMHandler) PerformGetBlockByNumber(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int, blockNumber int64) (interface{}, error) {
	// Use the shared JSON-RPC helper for get block by number operations
	return PerformGetBlockByNumberViaJSONRPC(
		httpUrl,
		headers,
		httpClient,
		authClient,
		requestAttempts,
		blockNumber,
		h.GetSupportedMethods, // EVM-specific supported methods
		h.CreateBlockRequest,  // EVM-specific block request creation
		h.ParseBlockResponse,  // EVM-specific block response parsing
	)
}
