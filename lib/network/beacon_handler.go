// lib/network/beacon_handler.go
package network

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// BeaconChainHandler handles Ethereum Beacon Chain REST API requests
type BeaconChainHandler struct {
	config              *NetworkConfig
	pathNormalizer      *BeaconPathNormalizer
	healthCheckEndpoint string
	version             string
}

// NewBeaconChainHandler creates a new Beacon Chain handler instance
func NewBeaconChainHandler(config *NetworkConfig) *BeaconChainHandler {
	return &BeaconChainHandler{
		config:              config,
		pathNormalizer:      NewBeaconPathNormalizer(),
		healthCheckEndpoint: "/eth/v1/beacon/headers/head",
		version:             "1.0.0",
	}
}

// Metadata methods for registry
func (h *BeaconChainHandler) GetType() string {
	return "beacon_chain"
}

func (h *BeaconChainHandler) GetName() string {
	return "Ethereum Beacon Chain Handler"
}

func (h *BeaconChainHandler) GetVersion() string {
	return h.version
}

func (h *BeaconChainHandler) GetRequestType() RequestType {
	return RequestTypeREST
}

// Lifecycle methods
func (h *BeaconChainHandler) Initialize(config *NetworkConfig) error {
	h.config = config

	// Use custom health endpoint if provided
	if config.HealthEndpoint != "" {
		h.healthCheckEndpoint = config.HealthEndpoint
	}

	// Initialize path normalizer with custom patterns if provided
	if patterns, ok := config.Custom["path_patterns"]; ok {
		if patternSlice, ok := patterns.([]PathPattern); ok {
			h.pathNormalizer = NewBeaconPathNormalizerWithPatterns(patternSlice)
		}
	}

	return nil
}

func (h *BeaconChainHandler) Shutdown() error {
	// Cleanup resources if needed
	return nil
}

// Request processing methods
func (h *BeaconChainHandler) ProcessRequest(req *http.Request, provider Provider) error {
	// Validate the request
	if err := h.ValidateRequest(req); err != nil {
		return err
	}

	// Translate the path for the specific provider
	if provider != nil {
		translatedPath, err := h.TranslatePath(req.URL.Path, provider)
		if err != nil {
			return err
		}

		// Update the request
		req.URL.Path = translatedPath
		req.URL.RawPath = translatedPath
	}

	return nil
}

func (h *BeaconChainHandler) ValidateRequest(req *http.Request) error {
	// Check for valid HTTP methods
	if req.Method != "GET" && req.Method != "POST" {
		return fmt.Errorf("unsupported HTTP method for Beacon Chain REST API: %s", req.Method)
	}

	// Check for valid path format
	if !strings.HasPrefix(req.URL.Path, "/eth/v") {
		return fmt.Errorf("invalid Beacon Chain API path: %s", req.URL.Path)
	}

	return nil
}

func (h *BeaconChainHandler) TranslatePath(gatewayPath string, provider Provider) (string, error) {
	// Remove gateway prefix if present (e.g., /ethereum-beacon)
	relativePath := gatewayPath
	if h.config != nil && h.config.Name != "" {
		relativePath = strings.TrimPrefix(gatewayPath, "/"+h.config.Name)
	}

	// Apply provider-specific transformations
	if provider != nil {
		// Check if provider has path prefix method
		if pathPrefix := getProviderPathPrefix(provider); pathPrefix != "" {
			return pathPrefix + relativePath, nil
		}

		// Check if provider has path template method
		if pathTemplate := getProviderPathTemplate(provider); pathTemplate != "" {
			return strings.Replace(pathTemplate, "{path}", relativePath, 1), nil
		}

		// Check if provider has strip prefix method
		if stripPrefix := getProviderStripPrefix(provider); stripPrefix != "" {
			relativePath = strings.TrimPrefix(relativePath, stripPrefix)
		}
	}

	return relativePath, nil
}

// Helper functions to extract provider-specific path configuration
// These would be implemented when provider struct is updated in Phase 3
func getProviderPathPrefix(provider Provider) string {
	// This is a placeholder - will be implemented when provider is extended
	return ""
}

func getProviderPathTemplate(provider Provider) string {
	// This is a placeholder - will be implemented when provider is extended
	return ""
}

func getProviderStripPrefix(provider Provider) string {
	// This is a placeholder - will be implemented when provider is extended
	return ""
}

func (h *BeaconChainHandler) NormalizeEndpoint(path string) string {
	return h.pathNormalizer.NormalizePath(path)
}

// Health check methods
func (h *BeaconChainHandler) GetLatestBlock(provider Provider) (*BlockInfo, error) {
	// This would make a request to /eth/v1/beacon/headers/head
	// For now, return a placeholder - this will be implemented in integration
	return &BlockInfo{
		Number:    0,
		Hash:      "",
		Timestamp: time.Now(),
		Slot:      0,
		Epoch:     0,
	}, nil
}

func (h *BeaconChainHandler) CheckHealth(provider Provider) (*HealthStatus, error) {
	// This would implement health check logic using the beacon chain endpoint
	// For now, return a placeholder - this will be implemented in integration
	return &HealthStatus{
		Healthy:     true,
		BlockNumber: 0,
		Latency:     0,
		Error:       nil,
	}, nil
}

// Response handling methods
func (h *BeaconChainHandler) ParseResponse(body []byte, statusCode int) error {
	// Check HTTP status code
	if statusCode < 200 || statusCode >= 300 {
		return fmt.Errorf("HTTP error: %d", statusCode)
	}

	// Parse JSON response
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Check for error field in response
	if errorField, exists := response["error"]; exists && errorField != nil {
		return fmt.Errorf("Beacon Chain API error: %v", errorField)
	}

	return nil
}

func (h *BeaconChainHandler) IsRetryableError(err error, statusCode int) bool {
	// HTTP server errors are retryable
	if statusCode >= 500 {
		return true
	}

	// Rate limiting is retryable
	if statusCode == 429 {
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
		"unavailable",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}

	return false
}

// Helper function to parse beacon chain responses
func (h *BeaconChainHandler) parseBeaconResponse(body []byte) (*BeaconHeadResponse, error) {
	var response BeaconHeadResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse beacon head response: %w", err)
	}
	return &response, nil
}

// Response structures for beacon chain
type BeaconHeadResponse struct {
	Data struct {
		Root   string `json:"root"`
		Header struct {
			Message struct {
				Slot          string `json:"slot"`
				ProposerIndex string `json:"proposer_index"`
				ParentRoot    string `json:"parent_root"`
				StateRoot     string `json:"state_root"`
			} `json:"message"`
		} `json:"header"`
	} `json:"data"`
}

// Helper function to convert slot string to int64
func (h *BeaconChainHandler) parseSlot(slotStr string) (int64, error) {
	slot, err := strconv.ParseInt(slotStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse slot: %w", err)
	}
	return slot, nil
}

// Factory function for Beacon Chain handler
func NewBeaconChainHandlerFactory() HandlerFactory {
	return func(config *NetworkConfig) (NetworkHandler, error) {
		return NewBeaconChainHandler(config), nil
	}
}
