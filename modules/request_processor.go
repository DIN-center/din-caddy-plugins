package modules

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	networklib "github.com/DIN-center/din-caddy-plugins/lib/network"
)

// RequestProcessor contains information about the request type and appropriate handler
type RequestProcessor struct {
	Type          networklib.RequestType    `json:"type"`
	Method        string                    `json:"method"`       // RPC method or REST endpoint
	NetworkType   string                    `json:"network_type"` // "evm", "eth_beacon_chain", etc.
	OriginalPath  string                    `json:"original_path"`
	Parameters    map[string]string         `json:"parameters"`
	Handler       networklib.NetworkHandler `json:"-"` // Don't serialize handler
	IsHealthCheck bool                      `json:"is_health_check"`
}

// DetectRequestType analyzes the incoming request and determines the appropriate handler
func DetectRequestType(req *http.Request, networkObj *network, registry *networklib.HandlerRegistry) (*RequestProcessor, error) {
	ctx := &RequestProcessor{
		OriginalPath: req.URL.Path,
		Parameters:   make(map[string]string),
	}

	// Create network config from network object
	config := &networklib.NetworkConfig{
		Name:           networkObj.Name,
		Type:           networkObj.Type,
		ChainID:        networkObj.ChainId,
		MaxPayloadSize: networkObj.MaxRequestPayloadSizeKB,
		RequestTimeout: time.Duration(networkObj.HCTimeout) * time.Second,
		Custom:         make(map[string]interface{}),
	}

	// Check if network has explicit type declaration
	if networkObj.Type != "" {
		handler, err := registry.GetHandler(networkObj.Type, config)
		if err != nil {
			return nil, fmt.Errorf("failed to get handler for network type '%s': %w", networkObj.Type, err)
		}

		ctx.Handler = handler
		ctx.NetworkType = networkObj.Type
		ctx.Type = handler.GetRequestType()

		return ctx, nil
	}

	// Auto-detect based on request characteristics
	return autoDetectRequestType(req, networkObj, registry, config)
}

// autoDetectRequestType attempts to determine request type based on request characteristics
func autoDetectRequestType(req *http.Request, networkObj *network, registry *networklib.HandlerRegistry, config *networklib.NetworkConfig) (*RequestProcessor, error) {
	contentType := req.Header.Get("Content-Type")

	// Check for JSON-RPC characteristics
	if strings.Contains(contentType, "application/json") && req.Method == "POST" {
		// Try to parse as JSON-RPC
		if isJSONRPC(req) {
			handler, err := registry.GetHandler("evm", config)
			if err != nil {
				return nil, fmt.Errorf("failed to get EVM handler: %w", err)
			}

			ctx := &RequestProcessor{
				Type:         networklib.RequestTypeRPC,
				NetworkType:  "evm",
				Handler:      handler,
				OriginalPath: req.URL.Path,
				Parameters:   make(map[string]string),
			}

			return ctx, nil
		}
	}

	// Check for REST characteristics
	if isRESTPattern(req.URL.Path) {
		handler, err := registry.GetHandler("eth_beacon_chain", config)
		if err != nil {
			return nil, fmt.Errorf("failed to get Beacon Chain handler: %w", err)
		}

		ctx := &RequestProcessor{
			Type:         networklib.RequestTypeREST,
			NetworkType:  "eth_beacon_chain",
			Handler:      handler,
			OriginalPath: req.URL.Path,
			Parameters:   make(map[string]string),
		}

		return ctx, nil
	}

	// Default to RPC for backward compatibility
	handler, err := registry.GetHandler("evm", config)
	if err != nil {
		return nil, fmt.Errorf("failed to get default EVM handler: %w", err)
	}

	ctx := &RequestProcessor{
		Type:         networklib.RequestTypeRPC,
		NetworkType:  "evm",
		Handler:      handler,
		OriginalPath: req.URL.Path,
		Parameters:   make(map[string]string),
	}

	return ctx, nil
}

// isJSONRPC checks if the request appears to be a JSON-RPC request
func isJSONRPC(req *http.Request) bool {
	if req.Method != "POST" {
		return false
	}

	contentType := req.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		return false
	}

	// Try to peek at the body to see if it has JSON-RPC structure
	// Note: This is a heuristic check, the actual parsing happens later
	return true
}

// isRESTPattern checks if the path matches common REST API patterns
func isRESTPattern(path string) bool {
	// Beacon Chain REST API patterns
	beaconPatterns := []string{
		"/eth/v1/",
		"/eth/v2/",
		"/api/v1/",
		"/api/v2/",
	}

	for _, pattern := range beaconPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}

	// Check for typical REST path patterns (resource-based URLs)
	pathSegments := strings.Split(strings.Trim(path, "/"), "/")
	if len(pathSegments) >= 2 {
		// Look for patterns like /resource/identifier or /api/version/resource
		for _, segment := range pathSegments {
			if strings.HasPrefix(segment, "v") && len(segment) >= 2 {
				// Version pattern like v1, v2, etc.
				if segment[1] >= '0' && segment[1] <= '9' {
					return true
				}
			}
		}
	}

	return false
}

// isHealthCheckEndpoint checks if the path matches the configured health check endpoint
func isHealthCheckEndpoint(path, healthEndpoint string) bool {
	if healthEndpoint == "" {
		return false
	}

	// Remove network prefix from path for comparison
	cleanPath := path
	if idx := strings.Index(path[1:], "/"); idx != -1 {
		cleanPath = path[idx+1:]
	}

	return strings.HasSuffix(cleanPath, healthEndpoint)
}

// GetRequestTypeString returns a human-readable string for the request type
func (ctx *RequestProcessor) GetRequestTypeString() string {
	switch ctx.Type {
	case networklib.RequestTypeRPC:
		return "JSON-RPC"
	case networklib.RequestTypeREST:
		return "REST"
	case networklib.RequestTypeGraphQL:
		return "GraphQL"
	default:
		return "Unknown"
	}
}

// IsRetryableError determines if an error should trigger a retry based on the request type
func (ctx *RequestProcessor) IsRetryableError(err error, statusCode int, responseBody []byte) bool {
	if ctx.Handler == nil {
		// Fallback to generic retry logic
		return statusCode >= 500
	}

	// Use handler-specific retry logic
	return ctx.Handler.IsRetryableError(err, statusCode)
}

// ProcessResponse processes the response using the appropriate handler
func (ctx *RequestProcessor) ProcessResponse(body []byte, statusCode int) error {
	if ctx.Handler == nil {
		return fmt.Errorf("no handler available for response processing")
	}

	return ctx.Handler.ParseResponse(body, statusCode)
}
