package network

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	dinHttp "github.com/DIN-center/din-caddy-plugins/lib/http"
)

type StarknetHandler struct {
	config  *NetworkConfig
	version string
}

func NewStarknetHandler(config *NetworkConfig) *StarknetHandler {
	return &StarknetHandler{
		config:  config,
		version: "1.0.0",
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

func (h *StarknetHandler) ProcessRequest(req *http.Request, provider Provider) error {
	// Validate Starknet JSON-RPC request
	if err := h.ValidateRequest(req); err != nil {
		return err
	}

	// For Starknet, path translation is simple - just use provider.path
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

func (h *StarknetHandler) TranslatePath(gatewayPath string, provider Provider) (string, error) {
	// For Starknet, use the provider's configured path
	if provider != nil {
		return provider.GetPath(), nil
	}
	return gatewayPath, nil
}

func (h *StarknetHandler) NormalizeEndpoint(path string) string {
	// For JSON-RPC, the "endpoint" is actually the method name
	// This will be extracted from the request body during processing
	return path
}

func (h *StarknetHandler) GetLatestBlock(provider Provider) (*BlockInfo, error) {
	// Implementation would use starknet_blockNumber instead of eth_blockNumber
	// For now, return placeholder implementation
	return &BlockInfo{
		Number:    0, // To be implemented with starknet_blockNumber call
		Hash:      "",
		Timestamp: time.Now(),
	}, nil
}

func (h *StarknetHandler) CheckHealth(provider Provider) (*HealthStatus, error) {
	// Implementation would use starknet-specific health check logic
	return &HealthStatus{
		Healthy:     true,
		BlockNumber: 0,
		Latency:     0,
		Error:       nil,
	}, nil
}

func (h *StarknetHandler) ParseResponse(body []byte, statusCode int) error {
	// Parse Starknet JSON-RPC response and check for errors
	var response dinHttp.JSONRPCResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to parse Starknet JSON-RPC response: %w", err)
	}

	if response.Error != nil {
		return fmt.Errorf("Starknet JSON-RPC error: %s", response.Error.Message)
	}

	return nil
}

func (h *StarknetHandler) IsRetryableError(err error, statusCode int) bool {
	// Use similar logic to EVM but could be customized for Starknet-specific errors
	if statusCode >= 500 {
		return true
	}

	// Check for specific Starknet JSON-RPC error codes
	// This would integrate with existing isJSONRPCErrorRetryable logic
	// but could be customized for Starknet-specific error patterns
	return false
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
	return "starknet_getBlockByNumber"
}
