package network

import (
	"encoding/json"
	"fmt"
	"net/http"
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

// Solana-specific method to validate chain ID format
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
