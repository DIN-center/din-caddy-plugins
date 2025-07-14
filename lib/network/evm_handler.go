// lib/network/evm_handler.go
package network

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	// This would integrate with existing health check logic
	// For now, return a placeholder - this will be implemented in integration
	return &BlockInfo{
		Number:    0,
		Hash:      "",
		Timestamp: time.Now(),
	}, nil
}

func (h *EVMHandler) CheckHealth(provider Provider) (*HealthStatus, error) {
	// This would integrate with existing health check logic
	// For now, return a placeholder - this will be implemented in integration
	return &HealthStatus{
		Healthy:     true,
		BlockNumber: 0,
		Latency:     0,
		Error:       nil,
	}, nil
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

// Factory function for EVM handler
func NewEVMHandlerFactory() HandlerFactory {
	return func(config *NetworkConfig) (NetworkHandler, error) {
		return NewEVMHandler(config), nil
	}
}
