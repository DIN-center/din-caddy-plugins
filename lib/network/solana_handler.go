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

type SolanaHandler struct {
	config  *NetworkConfig
	version string
}

func NewSolanaHandler(config *NetworkConfig) *SolanaHandler {
	return &SolanaHandler{
		config:  config,
		version: "1.0.0",
	}
}

// Metadata methods for registry
func (h *SolanaHandler) GetType() string {
	return "solana"
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
	return nil
}

func (h *SolanaHandler) Shutdown() error {
	return nil
}

// === EXISTING METHODS ===

func (h *SolanaHandler) ProcessRequest(req *http.Request, provider Provider) error {
	// Validate Solana JSON-RPC request
	if err := h.ValidateRequest(req); err != nil {
		return err
	}

	// For Solana, path translation is simple - just use provider.path
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

func (h *SolanaHandler) ValidateRequest(req *http.Request) error {
	// Check content type
	contentType := req.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		return fmt.Errorf("invalid content type for Solana JSON-RPC: %s", contentType)
	}

	// Check method
	if req.Method != "POST" {
		return fmt.Errorf("Solana JSON-RPC requires POST method, got %s", req.Method)
	}

	return nil
}

func (h *SolanaHandler) TranslatePath(gatewayPath string, provider Provider) (string, error) {
	// For Solana, use the provider's configured path
	if provider != nil {
		return provider.GetPath(), nil
	}
	return gatewayPath, nil
}

func (h *SolanaHandler) NormalizeEndpoint(path string) string {
	// For JSON-RPC, the "endpoint" is actually the method name
	// This will be extracted from the request body during processing
	return path
}

func (h *SolanaHandler) GetLatestBlock(provider Provider) (*BlockInfo, error) {
	// Implementation would use getBlockHeight instead of eth_blockNumber
	// For now, return placeholder implementation
	return &BlockInfo{
		Number:    0, // To be implemented with getBlockHeight call
		Hash:      "",
		Timestamp: time.Now(),
	}, nil
}

func (h *SolanaHandler) CheckHealth(provider Provider) (*HealthStatus, error) {
	// Implementation would use Solana-specific health check methods
	return &HealthStatus{
		Healthy:     true,
		BlockNumber: 0,
		Latency:     0,
		Error:       nil,
	}, nil
}

func (h *SolanaHandler) ParseResponse(body []byte, statusCode int) error {
	// Parse Solana JSON-RPC response and check for errors
	var response dinHttp.JSONRPCResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to parse Solana JSON-RPC response: %w", err)
	}

	if response.Error != nil {
		return fmt.Errorf("Solana JSON-RPC error: %s", response.Error.Message)
	}

	return nil
}

func (h *SolanaHandler) IsRetryableError(err error, statusCode int) bool {
	// Use similar logic to EVM but with Solana-specific considerations
	if statusCode >= 500 {
		return true
	}

	// Solana-specific error handling could be added here
	return false
}

// === NEW NETWORK-SPECIFIC METHODS ===

// Chain ID and Namespace methods
func (h *SolanaHandler) GetNamespace() string {
	return "solana"
}

func (h *SolanaHandler) ValidateChainID(chainID string) error {
	// Solana chain IDs are in format: solana:base58_encoded_genesis_hash
	if !strings.HasPrefix(chainID, "solana:") {
		return fmt.Errorf("invalid Solana chain ID format: %s, expected format: solana:{base58_hash}", chainID)
	}

	// Extract the genesis hash part
	parts := strings.Split(chainID, ":")
	if len(parts) != 2 {
		return fmt.Errorf("invalid Solana chain ID format: %s", chainID)
	}

	genesisHash := parts[1]
	if len(genesisHash) == 0 {
		return fmt.Errorf("empty genesis hash in Solana chain ID: %s", chainID)
	}

	// Basic validation - Solana genesis hashes are base58 encoded, typically 44 characters
	if len(genesisHash) < 32 || len(genesisHash) > 50 {
		return fmt.Errorf("invalid Solana genesis hash length: %s", genesisHash)
	}

	return nil
}

func (h *SolanaHandler) FormatChainID(networkReference string) string {
	return "solana:" + networkReference
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
	// Solana uses different parameter structure
	payload := fmt.Sprintf(`{"jsonrpc":"2.0","method":"%s","id":1,"params":[%d,{"encoding":"json","transactionDetails":"none","rewards":false}]}`,
		method, blockNum)
	return []byte(payload), nil
}

func (h *SolanaHandler) ParseBlockResponse(body []byte) (interface{}, error) {
	var response dinHttp.JSONRPCSolanaBlockResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Solana block response: %w", err)
	}
	return response, nil
}

// Archive Mode methods
func (h *SolanaHandler) SupportsArchiveMode() bool {
	return false // Solana doesn't support archive mode
}

func (h *SolanaHandler) GetArchiveMethod() string {
	return ""
}

func (h *SolanaHandler) CreateArchivePayload(method string, blockHeight string) ([]byte, error) {
	return nil, fmt.Errorf("Solana does not support archive mode")
}

func (h *SolanaHandler) ParseArchiveResponse(body []byte) error {
	return fmt.Errorf("Solana does not support archive mode")
}

// Network Capabilities methods
func (h *SolanaHandler) SupportsGetBlockByNumber() bool {
	return true
}

func (h *SolanaHandler) GetSupportedMethods() []string {
	return []string{
		"getBlockHeight",
		"getGenesisHash",
		"getBlock",
		"getTransaction",
		"getBalance",
		"getAccountInfo",
		"sendTransaction",
		"getSlot",
		"getHealth",
		"getVersion",
		"getSignaturesForAddress",
	}
}

// Data Format Conversions methods
func (h *SolanaHandler) ExtractBlockHash(blockData interface{}) string {
	if blockResponse, ok := blockData.(dinHttp.JSONRPCSolanaBlockResponse); ok {
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
	return "getBlockHeight"
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
