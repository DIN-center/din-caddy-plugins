package network

import (
	"net/http"
	"net/url"
	"testing"
)

func TestConfigureRESTRequestPath_URLJoinPath(t *testing.T) {
	tests := []struct {
		name         string
		originalPath string
		providerPath string
		networkName  string
		expectedPath string
		description  string
	}{
		{
			name:         "provider path with trailing slash and request path with leading slash",
			originalPath: "/beacon-mainnet/eth/v1/beacon/genesis",
			providerPath: "https://example.com/ethbeacon/",
			networkName:  "beacon-mainnet",
			expectedPath: "https://example.com/ethbeacon/eth/v1/beacon/genesis",
			description:  "Should handle double slashes correctly",
		},
		{
			name:         "provider path without trailing slash and request path with leading slash",
			originalPath: "/beacon-mainnet/eth/v1/beacon/genesis",
			providerPath: "https://example.com/ethbeacon",
			networkName:  "beacon-mainnet",
			expectedPath: "https://example.com/ethbeacon/eth/v1/beacon/genesis",
			description:  "Should add single slash between parts",
		},
		{
			name:         "provider path with multiple trailing slashes",
			originalPath: "/beacon-mainnet/eth/v1/config/spec",
			providerPath: "https://example.com/ethbeacon///",
			networkName:  "beacon-mainnet",
			expectedPath: "https://example.com/ethbeacon/eth/v1/config/spec",
			description:  "Should normalize multiple slashes",
		},
		{
			name:         "provider path with path segments",
			originalPath: "/beacon-mainnet/eth/v2/beacon/blocks/head",
			providerPath: "/api/v1/beacon",
			networkName:  "beacon-mainnet",
			expectedPath: "/api/v1/beacon/eth/v2/beacon/blocks/head",
			description:  "Should properly join path segments",
		},
		{
			name:         "empty provider path",
			originalPath: "/beacon-mainnet/eth/v1/node/health",
			providerPath: "",
			networkName:  "beacon-mainnet",
			expectedPath: "/eth/v1/node/health",
			description:  "Should use stripped path when provider path is empty",
		},
		{
			name:         "root provider path",
			originalPath: "/beacon-mainnet/eth/v1/node/version",
			providerPath: "/",
			networkName:  "beacon-mainnet",
			expectedPath: "/eth/v1/node/version",
			description:  "Should use stripped path when provider path is root",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				URL: &url.URL{
					Path: tt.originalPath,
				},
			}

			ConfigureRESTRequestPath(req, tt.providerPath, tt.networkName)

			if req.URL.Path != tt.expectedPath {
				t.Errorf("%s: expected path %s, got %s", tt.description, tt.expectedPath, req.URL.Path)
			}

			// Ensure RawPath is empty for REST paths
			if req.URL.RawPath != "" {
				t.Errorf("%s: expected empty raw path, got %s", tt.description, req.URL.RawPath)
			}
		})
	}
}

// Test url.JoinPath behavior directly to ensure it handles our use cases
func TestURLJoinPathBehavior(t *testing.T) {
	tests := []struct {
		base     string
		elem     string
		expected string
		hasError bool
	}{
		{
			base:     "https://example.com/ethbeacon/",
			elem:     "/eth/v1/config/spec",
			expected: "https://example.com/ethbeacon/eth/v1/config/spec",
			hasError: false,
		},
		{
			base:     "https://example.com/ethbeacon",
			elem:     "/eth/v1/config/spec",
			expected: "https://example.com/ethbeacon/eth/v1/config/spec",
			hasError: false,
		},
		{
			base:     "https://example.com/ethbeacon///",
			elem:     "///eth/v1/config/spec",
			expected: "https://example.com/ethbeacon/eth/v1/config/spec",
			hasError: false,
		},
		{
			base:     "/api/v1",
			elem:     "beacon/blocks",
			expected: "/api/v1/beacon/blocks",
			hasError: false,
		},
		{
			base:     "/api/v1/",
			elem:     "/beacon/blocks",
			expected: "/api/v1/beacon/blocks",
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.base+"_"+tt.elem, func(t *testing.T) {
			result, err := url.JoinPath(tt.base, tt.elem)
			if tt.hasError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.hasError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}