package network

import (
	"encoding/json"
	"fmt"
	"net/http"
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

// ValidateChainID validates Starknet-specific chain ID format
func (h *StarknetHandler) ValidateChainID(chainID string) bool {
	// Starknet chain IDs have format: starknet:0x{hex}
	// Examples:
	// - starknet:0x534e5f4d41494e (mainnet)
	// - starknet:0x534e5f5345504f4c4941 (sepolia)

	if !strings.HasPrefix(chainID, "starknet:") {
		return false
	}

	hexPart := strings.TrimPrefix(chainID, "starknet:")
	if !strings.HasPrefix(hexPart, "0x") {
		return false
	}

	// Validate hex format
	hexDigits := strings.TrimPrefix(hexPart, "0x")
	if len(hexDigits) == 0 {
		return false
	}

	// Check if all characters are valid hex
	for _, char := range hexDigits {
		if !((char >= '0' && char <= '9') ||
			(char >= 'a' && char <= 'f') ||
			(char >= 'A' && char <= 'F')) {
			return false
		}
	}

	return true
}

// GetDefaultMethods returns the default Starknet methods
func (h *StarknetHandler) GetDefaultMethods() []string {
	return []string{
		"starknet_blockNumber",
		"starknet_chainId",
		"starknet_call",
		"starknet_getBlockByNumber",
		"starknet_getBlockByHash",
		"starknet_getTransactionByHash",
		"starknet_getTransactionReceipt",
		"starknet_getBalance", // Note: Different from eth_getBalance
		"starknet_syncing",
		"starknet_sendTransaction", // Note: Different from eth_sendRawTransaction
	}
}

// GetHealthCheckMethod returns the Starknet health check method
func (h *StarknetHandler) GetHealthCheckMethod() string {
	return "starknet_blockNumber"
}

// GetChainIDMethod returns the Starknet chain ID method
func (h *StarknetHandler) GetChainIDMethod() string {
	return "starknet_chainId"
}

// GetCallContractMethod returns the Starknet call contract method
func (h *StarknetHandler) GetCallContractMethod() string {
	return "starknet_call"
}

// GetBlockByNumberMethod returns the Starknet get block by number method
func (h *StarknetHandler) GetBlockByNumberMethod() string {
	return "starknet_getBlockByNumber"
}
