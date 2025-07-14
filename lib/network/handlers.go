// lib/network/handlers.go
package network

import (
	"fmt"
	"log"
	"net/http"
)

// RegisterBuiltinHandlers registers all built-in network handlers
func RegisterBuiltinHandlers() {
	// Register EVM/RPC handler
	if err := DefaultRegistry.RegisterHandler("evm", NewEVMHandlerFactory()); err != nil {
		log.Printf("Failed to register EVM handler: %v", err)
	}

	// Register Beacon Chain handler
	if err := DefaultRegistry.RegisterHandler("beacon_chain", NewBeaconChainHandlerFactory()); err != nil {
		log.Printf("Failed to register Beacon Chain handler: %v", err)
	}

	// Register Starknet handler
	if err := DefaultRegistry.RegisterHandler("starknet", NewStarknetHandlerFactory()); err != nil {
		log.Printf("Failed to register Starknet handler: %v", err)
	}

	// Register Solana handler (placeholder for future implementation)
	if err := DefaultRegistry.RegisterHandler("solana", NewSolanaHandlerFactory()); err != nil {
		log.Printf("Failed to register Solana handler: %v", err)
	}

	// Register Bitcoin handler (placeholder for future implementation)
	if err := DefaultRegistry.RegisterHandler("bitcoin", NewBitcoinHandlerFactory()); err != nil {
		log.Printf("Failed to register Bitcoin handler: %v", err)
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

// NewBitcoinHandlerFactory creates a factory for Bitcoin handlers (placeholder)
func NewBitcoinHandlerFactory() HandlerFactory {
	return func(config *NetworkConfig) (NetworkHandler, error) {
		// This would return a Bitcoin handler implementation
		// For now, return a placeholder that would fail gracefully
		return NewPlaceholderHandler(config, "bitcoin"), nil
	}
}

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

// Health check methods
func (h *PlaceholderHandler) GetLatestBlock(provider Provider) (*BlockInfo, error) {
	return nil, fmt.Errorf("handler for network type '%s' not implemented", h.networkType)
}

func (h *PlaceholderHandler) CheckHealth(provider Provider) (*HealthStatus, error) {
	return nil, fmt.Errorf("handler for network type '%s' not implemented", h.networkType)
}

// Response handling methods
func (h *PlaceholderHandler) ParseResponse(body []byte, statusCode int) error {
	return fmt.Errorf("handler for network type '%s' not implemented", h.networkType)
}

func (h *PlaceholderHandler) IsRetryableError(err error, statusCode int) bool {
	return false
}

// init function to register built-in handlers at startup
func init() {
	RegisterBuiltinHandlers()
}
