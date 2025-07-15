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

	dinHttp "github.com/DIN-center/din-caddy-plugins/lib/http"
)

// EVMHandler handles EVM-compatible JSON-RPC networks
type EVMHandler struct {
	config  *NetworkConfig
	version string
}

// NewEVMHandler creates a new EVM handler instance
func NewEVMHandler(config *NetworkConfig) *EVMHandler {
	return &EVMHandler{
		config:  config,
		version: "1.0.0",
	}
}

// Metadata methods for registry
func (h *EVMHandler) GetType() string {
	return "evm"
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
	return nil
}

func (h *EVMHandler) Shutdown() error {
	// No cleanup needed for EVM handler
	return nil
}

// === EXISTING METHODS ===

// Request processing methods
func (h *EVMHandler) ProcessRequest(req *http.Request, provider Provider) error {
	// Validate JSON-RPC request
	if err := h.ValidateRequest(req); err != nil {
		return err
	}

	// For EVM, path translation is simple - just use provider's path
	if provider != nil {
		translatedPath, err := h.TranslatePath(req.URL.Path, provider)
		if err != nil {
			return err
		}
		req.URL.Path = translatedPath
		req.URL.RawPath = translatedPath
	}

	return nil
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
		var jsonRPCReq dinHttp.JSONRPCRequest
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

func (h *EVMHandler) TranslatePath(gatewayPath string, provider Provider) (string, error) {
	// For EVM, use the provider's configured path
	if provider != nil {
		return provider.GetPath(), nil
	}
	return gatewayPath, nil
}

func (h *EVMHandler) NormalizeEndpoint(path string) string {
	// For JSON-RPC, the "endpoint" is actually the method name
	// This will be extracted from the request body during processing
	// For now, return the path as-is since method extraction happens elsewhere
	return path
}

// Health check methods
func (h *EVMHandler) GetLatestBlock(provider Provider) (*BlockInfo, error) {
	// This method is used by the handler interface but the actual health check logic
	// is handled by the existing network health check system. We return a basic implementation
	// that indicates the method is available.
	return &BlockInfo{
		Number:    0,
		Hash:      "",
		Timestamp: time.Now(),
	}, fmt.Errorf("GetLatestBlock should use the existing network health check system")
}

func (h *EVMHandler) CheckHealth(provider Provider) (*HealthStatus, error) {
	// This method is used by the handler interface but the actual health check logic
	// is handled by the existing network health check system. We return a basic implementation
	// that indicates the method is available.
	return &HealthStatus{
		Healthy:     false,
		BlockNumber: 0,
		Latency:     0,
		Error:       fmt.Errorf("CheckHealth should use the existing network health check system"),
	}, fmt.Errorf("CheckHealth should use the existing network health check system")
}

// Response handling methods
func (h *EVMHandler) ParseResponse(body []byte, statusCode int) error {
	// Basic JSON-RPC response validation
	if statusCode != 200 {
		return fmt.Errorf("HTTP error: %d", statusCode)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Check for JSON-RPC error
	if errorField, exists := response["error"]; exists && errorField != nil {
		return fmt.Errorf("JSON-RPC error: %v", errorField)
	}

	return nil
}

func (h *EVMHandler) IsRetryableError(err error, statusCode int) bool {
	// HTTP server errors are retryable
	if statusCode >= 500 {
		return true
	}

	// If no error provided, check only status code
	if err == nil {
		return false
	}

	// Connection errors are retryable
	errMsg := strings.ToLower(err.Error())
	retryablePatterns := []string{
		"timeout",
		"connection",
		"network",
		"rate limit",
		"server error",
		"internal error",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}

	return false
}

// === NEW NETWORK-SPECIFIC METHODS ===

// Chain ID and Namespace methods
func (h *EVMHandler) GetNamespace() string {
	return "eip155"
}

func (h *EVMHandler) ValidateChainID(chainID string) error {
	if !strings.HasPrefix(chainID, "eip155:") {
		return fmt.Errorf("invalid EVM chain ID format: %s, expected format: eip155:{chainId}", chainID)
	}

	// Extract chain ID number and validate it's numeric
	parts := strings.Split(chainID, ":")
	if len(parts) != 2 {
		return fmt.Errorf("invalid EVM chain ID format: %s", chainID)
	}

	chainIDNum := parts[1]
	if chainIDNum == "" {
		return fmt.Errorf("empty chain ID number in: %s", chainID)
	}

	// Remove 0x prefix if present and validate hex
	if strings.HasPrefix(chainIDNum, "0x") {
		chainIDNum = strings.TrimPrefix(chainIDNum, "0x")
	}

	// Convert to ensure it's a valid number
	if _, err := strconv.ParseInt(chainIDNum, 16, 64); err != nil {
		return fmt.Errorf("invalid chain ID number in %s: %w", chainID, err)
	}

	return nil
}

func (h *EVMHandler) FormatChainID(networkReference string) string {
	return "eip155:" + networkReference
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
	return fmt.Sprintf("0x%x", blockNum) // Hex format for EVM
}

func (h *EVMHandler) CreateBlockRequest(method string, blockNum int64, includeTransactions bool) ([]byte, error) {
	blockHex := h.FormatBlockHeight(blockNum)
	payload := fmt.Sprintf(`{"jsonrpc":"2.0","method":"%s","id":1,"params":["%s",%t]}`,
		method, blockHex, includeTransactions)
	return []byte(payload), nil
}

func (h *EVMHandler) ParseBlockResponse(body []byte) (interface{}, error) {
	var response dinHttp.JSONRPCEVMBlockResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal EVM block response: %w", err)
	}
	return response, nil
}

// Archive Mode methods
func (h *EVMHandler) SupportsArchiveMode() bool {
	return true
}

func (h *EVMHandler) GetArchiveMethod() string {
	return "eth_call"
}

func (h *EVMHandler) CreateArchivePayload(method string, blockHeight string) ([]byte, error) {
	payload := fmt.Sprintf(`{"jsonrpc":"2.0","method":"%s","id":1,"params":[{"input":"0x436000526004601cf3"},"%s"]}`,
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

// Data Format Conversions methods
func (h *EVMHandler) ExtractBlockHash(blockData interface{}) string {
	if blockResponse, ok := blockData.(dinHttp.JSONRPCEVMBlockResponse); ok {
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

// Factory function for EVM handler
func NewEVMHandlerFactory() HandlerFactory {
	return func(config *NetworkConfig) (NetworkHandler, error) {
		return NewEVMHandler(config), nil
	}
}
