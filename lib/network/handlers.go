// lib/network/handlers.go
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

func init() {
	RegisterBuiltinHandlers()
}

// RegisterBuiltinHandlers registers all built-in network handlers
func RegisterBuiltinHandlers() {
	if err := DefaultRegistry.RegisterHandler("evm", NewEVMHandlerFactory()); err != nil {
		panic(fmt.Sprintf("Failed to register EVM handler: %v", err))
	}

	if err := DefaultRegistry.RegisterHandler("starknet", NewStarknetHandlerFactory()); err != nil {
		panic(fmt.Sprintf("Failed to register Starknet handler: %v", err))
	}

	if err := DefaultRegistry.RegisterHandler("solana", NewSolanaHandlerFactory()); err != nil {
		panic(fmt.Sprintf("Failed to register Solana handler: %v", err))
	}

	if err := DefaultRegistry.RegisterHandler("beacon_chain", NewBeaconChainHandlerFactory()); err != nil {
		panic(fmt.Sprintf("Failed to register Beacon Chain handler: %v", err))
	}

	if err := DefaultRegistry.RegisterHandler("bitcoin", NewBitcoinHandlerFactory()); err != nil {
		panic(fmt.Sprintf("Failed to register Bitcoin handler: %v", err))
	}
}

// NewStarknetHandlerFactory creates a factory for Starknet handlers
func NewStarknetHandlerFactory() HandlerFactory {
	return func(config *NetworkConfig) (NetworkHandler, error) {
		return NewStarknetHandler(config), nil
	}
}

// NewSolanaHandlerFactory creates a factory for Solana handlers
func NewSolanaHandlerFactory() HandlerFactory {
	return func(config *NetworkConfig) (NetworkHandler, error) {
		return NewSolanaHandler(config), nil
	}
}

// NewBitcoinHandlerFactory creates a factory for Bitcoin handlers
func NewBitcoinHandlerFactory() HandlerFactory {
	return func(config *NetworkConfig) (NetworkHandler, error) {
		return NewBitcoinHandler(config), nil
	}
}

// === BITCOIN HANDLER IMPLEMENTATION ===

// BitcoinHandler handles Bitcoin JSON-RPC networks
type BitcoinHandler struct {
	config  *NetworkConfig
	version string
}

// NewBitcoinHandler creates a new Bitcoin handler instance
func NewBitcoinHandler(config *NetworkConfig) *BitcoinHandler {
	return &BitcoinHandler{
		config:  config,
		version: "1.0.0",
	}
}

// Metadata methods for registry
func (h *BitcoinHandler) GetType() string {
	return "bitcoin"
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
	return nil
}

func (h *BitcoinHandler) Shutdown() error {
	return nil
}

// Request processing methods
func (h *BitcoinHandler) ProcessRequest(req *http.Request, provider Provider) error {
	// Validate Bitcoin JSON-RPC request
	if err := h.ValidateRequest(req); err != nil {
		return err
	}

	// For Bitcoin, path translation is simple - just use provider.path
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

func (h *BitcoinHandler) ValidateRequest(req *http.Request) error {
	// Check content type
	contentType := req.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		return fmt.Errorf("invalid content type for Bitcoin JSON-RPC: %s", contentType)
	}

	// Check method
	if req.Method != "POST" {
		return fmt.Errorf("Bitcoin JSON-RPC requires POST method, got %s", req.Method)
	}

	return nil
}

func (h *BitcoinHandler) TranslatePath(gatewayPath string, provider Provider) (string, error) {
	// For Bitcoin, use the provider's configured path
	if provider != nil {
		return provider.GetPath(), nil
	}
	return gatewayPath, nil
}

func (h *BitcoinHandler) NormalizeEndpoint(path string) string {
	// For JSON-RPC, the "endpoint" is actually the method name
	return path
}

func (h *BitcoinHandler) GetLatestBlock(provider Provider) (*BlockInfo, error) {
	// Implementation would use getblockchaininfo instead of eth_blockNumber
	return &BlockInfo{
		Number:    0, // To be implemented with getblockchaininfo call
		Hash:      "",
		Timestamp: time.Now(),
	}, nil
}

func (h *BitcoinHandler) CheckHealth(provider Provider) (*HealthStatus, error) {
	// Implementation would use Bitcoin-specific health check methods
	return &HealthStatus{
		Healthy:     true,
		BlockNumber: 0,
		Latency:     0,
		Error:       nil,
	}, nil
}

func (h *BitcoinHandler) ParseResponse(body []byte, statusCode int) error {
	// Parse Bitcoin JSON-RPC response and check for errors
	var response dinHttp.JSONRPCResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to parse Bitcoin JSON-RPC response: %w", err)
	}

	if response.Error != nil {
		return fmt.Errorf("Bitcoin JSON-RPC error: %s", response.Error.Message)
	}

	return nil
}

func (h *BitcoinHandler) IsRetryableError(err error, statusCode int) bool {
	// Use similar logic to EVM but with Bitcoin-specific considerations
	if statusCode >= 500 {
		return true
	}

	return false
}

// === NETWORK-SPECIFIC METHODS ===

// Chain ID and Namespace methods
func (h *BitcoinHandler) GetNamespace() string {
	return "bip122"
}

func (h *BitcoinHandler) ValidateChainID(chainID string) error {
	// Bitcoin chain IDs are in format: bip122:{genesis_hash}
	if !strings.HasPrefix(chainID, "bip122:") {
		return fmt.Errorf("invalid Bitcoin chain ID format: %s, expected format: bip122:{genesis_hash}", chainID)
	}

	// Extract the genesis hash part
	parts := strings.Split(chainID, ":")
	if len(parts) != 2 {
		return fmt.Errorf("invalid Bitcoin chain ID format: %s", chainID)
	}

	genesisHash := parts[1]
	if len(genesisHash) == 0 {
		return fmt.Errorf("empty genesis hash in Bitcoin chain ID: %s", chainID)
	}

	// Bitcoin genesis hashes are 64-character hex strings
	if len(genesisHash) != 64 {
		return fmt.Errorf("invalid Bitcoin genesis hash length: %s", genesisHash)
	}

	// Check if all characters are valid hex
	for _, char := range genesisHash {
		if !((char >= '0' && char <= '9') ||
			(char >= 'a' && char <= 'f') ||
			(char >= 'A' && char <= 'F')) {
			return fmt.Errorf("invalid hex character in Bitcoin genesis hash: %s", chainID)
		}
	}

	return nil
}

func (h *BitcoinHandler) FormatChainID(networkReference string) string {
	return "bip122:" + networkReference
}

func (h *BitcoinHandler) ExtractChainReference(result interface{}) (string, error) {
	// For Bitcoin, the chain ID is in a nested "chain" field in the result object
	resultMap, ok := result.(map[string]interface{})
	if !ok || resultMap["chain"] == nil {
		return "", fmt.Errorf("invalid Bitcoin chain result structure")
	}
	chainRef, ok := resultMap["chain"].(string)
	if !ok {
		return "", fmt.Errorf("invalid Bitcoin chain reference type")
	}
	return chainRef, nil
}

// Block Operations methods
func (h *BitcoinHandler) FormatBlockHeight(blockNum int64) string {
	return strconv.FormatInt(blockNum, 10) // Decimal format for Bitcoin
}

func (h *BitcoinHandler) CreateBlockRequest(method string, blockNum int64, includeTransactions bool) ([]byte, error) {
	// Bitcoin doesn't typically use block-by-number requests in the same way
	return nil, fmt.Errorf("Bitcoin does not support standard getBlockByNumber requests")
}

func (h *BitcoinHandler) ParseBlockResponse(body []byte) (interface{}, error) {
	// Bitcoin block responses have different structure
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Bitcoin block response: %w", err)
	}
	return response, nil
}

// Archive Mode methods
func (h *BitcoinHandler) SupportsArchiveMode() bool {
	return false // Bitcoin doesn't support archive mode
}

func (h *BitcoinHandler) GetArchiveMethod() string {
	return ""
}

func (h *BitcoinHandler) CreateArchivePayload(method string, blockHeight string) ([]byte, error) {
	return nil, fmt.Errorf("Bitcoin does not support archive mode")
}

func (h *BitcoinHandler) ParseArchiveResponse(body []byte) error {
	return fmt.Errorf("Bitcoin does not support archive mode")
}

// Network Capabilities methods
func (h *BitcoinHandler) SupportsGetBlockByNumber() bool {
	return false // Bitcoin skips getBlockByNumber
}

func (h *BitcoinHandler) GetSupportedMethods() []string {
	return []string{
		"getblockchaininfo",
		"getnetworkinfo",
		"getblockhash",
		"getblock",
		"getrawtransaction",
		"sendrawtransaction",
		"getbalance",
		"listunspent",
		"getnewaddress",
		"validateaddress",
	}
}

// Data Format Conversions methods
func (h *BitcoinHandler) ExtractBlockHash(blockData interface{}) string {
	// Bitcoin doesn't return block hash in the same way as other networks
	if blockMap, ok := blockData.(map[string]interface{}); ok {
		if result, ok := blockMap["result"].(map[string]interface{}); ok {
			if blockHash, ok := result["hash"].(string); ok {
				return blockHash
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

	// Bitcoin uses "blocks" field in getblockchaininfo
	if result, ok := respObject["result"].(map[string]interface{}); ok {
		if blocks, ok := result["blocks"].(float64); ok {
			return int64(blocks), nil
		}
	}

	return 0, fmt.Errorf("invalid Bitcoin block height response format")
}

// Health Check Specifics methods
func (h *BitcoinHandler) GetHealthCheckMethod() string {
	return "getblockchaininfo"
}

func (h *BitcoinHandler) GetChainIDMethod() string {
	return "getblockchaininfo" // Bitcoin uses same method for chain info
}

func (h *BitcoinHandler) CreateHealthCheckPayload(method string) ([]byte, error) {
	payload := fmt.Sprintf(`{"jsonrpc":"1.0","method":"%s","id":1}`, method)
	return []byte(payload), nil
}

func (h *BitcoinHandler) ParseHealthCheckResponse(body []byte) (*BlockInfo, error) {
	var respObject map[string]interface{}
	if err := json.Unmarshal(body, &respObject); err != nil {
		return nil, fmt.Errorf("failed to parse Bitcoin health check response: %w", err)
	}

	if result, ok := respObject["result"].(map[string]interface{}); ok {
		if blocks, ok := result["blocks"].(float64); ok {
			return &BlockInfo{
				Number:    int64(blocks),
				Hash:      "",
				Timestamp: time.Now(),
			}, nil
		}
	}

	return nil, fmt.Errorf("invalid Bitcoin health check response format")
}

// === PLACEHOLDER HANDLER (keeping existing functionality) ===

// PlaceholderHandler is a temporary handler for unsupported network types
type PlaceholderHandler struct {
	config      *NetworkConfig
	networkType string
	version     string
}

// NewPlaceholderHandler creates a placeholder handler
func NewPlaceholderHandler(config *NetworkConfig, networkType string) *PlaceholderHandler {
	return &PlaceholderHandler{
		config:      config,
		networkType: networkType,
		version:     "0.0.1",
	}
}

// Metadata methods for registry
func (h *PlaceholderHandler) GetType() string {
	return h.networkType
}

func (h *PlaceholderHandler) GetName() string {
	return h.networkType + " Placeholder Handler"
}

func (h *PlaceholderHandler) GetVersion() string {
	return h.version
}

func (h *PlaceholderHandler) GetRequestType() RequestType {
	return RequestTypeRPC // Default to RPC
}

// Lifecycle methods
func (h *PlaceholderHandler) Initialize(config *NetworkConfig) error {
	h.config = config
	return nil
}

func (h *PlaceholderHandler) Shutdown() error {
	return nil
}

// Request processing methods - all return not implemented errors
func (h *PlaceholderHandler) ProcessRequest(req *http.Request, provider Provider) error {
	return fmt.Errorf("handler for network type '%s' not implemented", h.networkType)
}

func (h *PlaceholderHandler) ValidateRequest(req *http.Request) error {
	return fmt.Errorf("handler for network type '%s' not implemented", h.networkType)
}

func (h *PlaceholderHandler) TranslatePath(gatewayPath string, provider Provider) (string, error) {
	return gatewayPath, fmt.Errorf("handler for network type '%s' not implemented", h.networkType)
}

func (h *PlaceholderHandler) NormalizeEndpoint(path string) string {
	return path
}

func (h *PlaceholderHandler) GetLatestBlock(provider Provider) (*BlockInfo, error) {
	return nil, fmt.Errorf("handler for network type '%s' not implemented", h.networkType)
}

func (h *PlaceholderHandler) CheckHealth(provider Provider) (*HealthStatus, error) {
	return nil, fmt.Errorf("handler for network type '%s' not implemented", h.networkType)
}

func (h *PlaceholderHandler) ParseResponse(body []byte, statusCode int) error {
	return fmt.Errorf("handler for network type '%s' not implemented", h.networkType)
}

func (h *PlaceholderHandler) IsRetryableError(err error, statusCode int) bool {
	return false
}

// Network-specific methods - all return not implemented errors
func (h *PlaceholderHandler) GetNamespace() string {
	return "unknown"
}

func (h *PlaceholderHandler) ValidateChainID(chainID string) error {
	return fmt.Errorf("handler for network type '%s' not implemented", h.networkType)
}

func (h *PlaceholderHandler) FormatChainID(networkReference string) string {
	return "unknown:" + networkReference
}

func (h *PlaceholderHandler) ExtractChainReference(result interface{}) (string, error) {
	return "", fmt.Errorf("handler for network type '%s' not implemented", h.networkType)
}

func (h *PlaceholderHandler) FormatBlockHeight(blockNum int64) string {
	return strconv.FormatInt(blockNum, 10)
}

func (h *PlaceholderHandler) CreateBlockRequest(method string, blockNum int64, includeTransactions bool) ([]byte, error) {
	return nil, fmt.Errorf("handler for network type '%s' not implemented", h.networkType)
}

func (h *PlaceholderHandler) ParseBlockResponse(body []byte) (interface{}, error) {
	return nil, fmt.Errorf("handler for network type '%s' not implemented", h.networkType)
}

func (h *PlaceholderHandler) SupportsArchiveMode() bool {
	return false
}

func (h *PlaceholderHandler) GetArchiveMethod() string {
	return ""
}

func (h *PlaceholderHandler) CreateArchivePayload(method string, blockHeight string) ([]byte, error) {
	return nil, fmt.Errorf("handler for network type '%s' not implemented", h.networkType)
}

func (h *PlaceholderHandler) ParseArchiveResponse(body []byte) error {
	return fmt.Errorf("handler for network type '%s' not implemented", h.networkType)
}

func (h *PlaceholderHandler) SupportsGetBlockByNumber() bool {
	return false
}

func (h *PlaceholderHandler) GetSupportedMethods() []string {
	return []string{}
}

func (h *PlaceholderHandler) ExtractBlockHash(blockData interface{}) string {
	return ""
}

func (h *PlaceholderHandler) ExtractBlockNumber(response []byte) (int64, error) {
	return 0, fmt.Errorf("handler for network type '%s' not implemented", h.networkType)
}

func (h *PlaceholderHandler) GetHealthCheckMethod() string {
	return ""
}

func (h *PlaceholderHandler) GetChainIDMethod() string {
	return ""
}

func (h *PlaceholderHandler) CreateHealthCheckPayload(method string) ([]byte, error) {
	return nil, fmt.Errorf("handler for network type '%s' not implemented", h.networkType)
}

func (h *PlaceholderHandler) ParseHealthCheckResponse(body []byte) (*BlockInfo, error) {
	return nil, fmt.Errorf("handler for network type '%s' not implemented", h.networkType)
}
