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
	if providerPath != "" {
		req.URL.RawPath = providerPath
		req.URL.Path, _ = url.PathUnescape(req.URL.RawPath)
	} else {
		// For JSON-RPC providers without configured paths, clear RawPath
		req.URL.RawPath = ""
		// Path already set, no changes needed
	}
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