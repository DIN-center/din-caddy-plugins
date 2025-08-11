// lib/network/request_path_config.go
package network

import (
	"net/http"
	"net/url"
	"strings"
)

// ============================================================================
// Path Configuration Functions
// ============================================================================

// ConfigureJSONRPCRequestPath configures the request path for JSON-RPC providers
// This is shared logic for all JSON-RPC handlers (EVM, Starknet, Solana)
func ConfigureJSONRPCRequestPath(req *http.Request, providerPath string) {
	// For JSON-RPC providers, we need to handle three cases:
	// 1. Provider has a specific path (e.g., "/v1/rpc") - use it
	// 2. Provider has root path "/" or empty "" - clear the network path
	// 3. No provider path configured - keep the current path

	if providerPath == "/" || providerPath == "" {
		// Provider expects requests at root - clear any network path
		req.URL.Path = "/"
		req.URL.RawPath = ""
	} else if providerPath != "" {
		// Provider has a specific path - use it
		req.URL.Path = providerPath
		req.URL.RawPath = ""
	}
	// If providerPath is not set at all (different from empty string),
	// the original path is kept (though this shouldn't happen in practice)
}

// ConfigureRESTRequestPath configures the request path for REST API providers
// This is shared logic for all REST handlers (Beacon Chain, Bitcoin Esplora)
func ConfigureRESTRequestPath(req *http.Request, providerPath string, networkName string) {
	currentPath := req.URL.Path

	// Strip the network prefix from the path
	// e.g., "/eth-beacon-mainnet/eth/v1/beacon/genesis" -> "/eth/v1/beacon/genesis"
	pathSegments := strings.Split(strings.TrimPrefix(currentPath, "/"), "/")
	if len(pathSegments) > 1 && pathSegments[0] == networkName {
		// Remove the first segment (network name) and rebuild path
		strippedPath := "/" + strings.Join(pathSegments[1:], "/")
		currentPath = strippedPath
	}

	// Combine provider base path with processed request path
	if providerPath != "" && providerPath != "/" {
		combinedPath, err := url.JoinPath(providerPath, currentPath)
		if err != nil {
			// If JoinPath fails, fall back to current behavior
			// This ensures backward compatibility
			combinedPath = strings.TrimSuffix(providerPath, "/") + currentPath
		}
		req.URL.Path = combinedPath
	} else {
		req.URL.Path = currentPath
	}
	req.URL.RawPath = ""
}
