package network

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
)

func TestConfigureJSONRPCRequestPath(t *testing.T) {
	tests := []struct {
		name         string
		originalPath string
		providerPath string
		expectedPath string
		expectedRaw  string
	}{
		{
			name:         "empty provider path",
			originalPath: "/ethereum",
			providerPath: "",
			expectedPath: "/ethereum",
			expectedRaw:  "",
		},
		{
			name:         "provider path set",
			originalPath: "/ethereum",
			providerPath: "/rpc/v1",
			expectedPath: "/rpc/v1",
			expectedRaw:  "/rpc/v1",
		},
		{
			name:         "provider path with special characters",
			originalPath: "/ethereum",
			providerPath: "/rpc/v1/key%20test",
			expectedPath: "/rpc/v1/key test",
			expectedRaw:  "/rpc/v1/key%20test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				URL: &url.URL{
					Path: tt.originalPath,
				},
			}

			ConfigureJSONRPCRequestPath(req, tt.providerPath)

			if req.URL.Path != tt.expectedPath {
				t.Errorf("expected path %s, got %s", tt.expectedPath, req.URL.Path)
			}

			if req.URL.RawPath != tt.expectedRaw {
				t.Errorf("expected raw path %s, got %s", tt.expectedRaw, req.URL.RawPath)
			}
		})
	}
}

func TestConfigureRESTRequestPath(t *testing.T) {
	tests := []struct {
		name         string
		originalPath string
		providerPath string
		networkName  string
		expectedPath string
	}{
		{
			name:         "basic REST path stripping",
			originalPath: "/beacon-mainnet/eth/v1/beacon/genesis",
			providerPath: "",
			networkName:  "beacon-mainnet",
			expectedPath: "/eth/v1/beacon/genesis",
		},
		{
			name:         "REST path stripping with provider base",
			originalPath: "/beacon-test/eth/v2/beacon/blocks/head",
			providerPath: "/api",
			networkName:  "beacon-test",
			expectedPath: "/api/eth/v2/beacon/blocks/head",
		},
		{
			name:         "no network prefix in path",
			originalPath: "/eth/v1/beacon/genesis",
			providerPath: "",
			networkName:  "beacon-mainnet",
			expectedPath: "/eth/v1/beacon/genesis",
		},
		{
			name:         "provider path with trailing slash",
			originalPath: "/beacon/eth/v1/node/health",
			providerPath: "/base/",
			networkName:  "beacon",
			expectedPath: "/base/eth/v1/node/health",
		},
		{
			name:         "root provider path",
			originalPath: "/beacon/eth/v1/node/version",
			providerPath: "/",
			networkName:  "beacon",
			expectedPath: "/eth/v1/node/version",
		},
		{
			name:         "network name not at start of path",
			originalPath: "/api/beacon/eth/v1/test",
			providerPath: "",
			networkName:  "beacon",
			expectedPath: "/api/beacon/eth/v1/test",
		},
		{
			name:         "single segment path",
			originalPath: "/beacon",
			providerPath: "",
			networkName:  "beacon",
			expectedPath: "/beacon",
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
				t.Errorf("expected path %s, got %s", tt.expectedPath, req.URL.Path)
			}

			// REST paths should not have RawPath set
			if req.URL.RawPath != "" {
				t.Errorf("expected empty raw path, got %s", req.URL.RawPath)
			}
		})
	}
}

func TestIsRetryableJSONRPCError_GasAndRevertHandling(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		statusCode int
		expected   bool
	}{
		// Gas-related errors should NOT be retryable even with -32000 error code
		{
			name:       "insufficient gas with -32000",
			err:        fmt.Errorf("json-rpc error -32000: insufficient gas"),
			statusCode: 200,
			expected:   false, // Should not retry gas errors
		},
		{
			name:       "out of gas error",
			err:        fmt.Errorf("out of gas"),
			statusCode: 200,
			expected:   false,
		},
		{
			name:       "gas limit exceeded",
			err:        fmt.Errorf("gas limit exceeded"),
			statusCode: 200,
			expected:   false,
		},
		{
			name:       "intrinsic gas too low",
			err:        fmt.Errorf("intrinsic gas too low"),
			statusCode: 200,
			expected:   false,
		},
		// Revert-related errors should NOT be retryable
		{
			name:       "execution reverted with -32000",
			err:        fmt.Errorf("json-rpc error -32000: execution reverted"),
			statusCode: 200,
			expected:   false, // Should not retry revert errors
		},
		{
			name:       "vm execution error",
			err:        fmt.Errorf("vm execution error"),
			statusCode: 200,
			expected:   false,
		},
		{
			name:       "simple revert",
			err:        fmt.Errorf("revert"),
			statusCode: 200,
			expected:   false,
		},
		// Other -32000 errors should be retryable
		{
			name:       "generic -32000 server error",
			err:        fmt.Errorf("json-rpc error -32000: server busy"),
			statusCode: 200,
			expected:   true, // Should retry non-gas/non-revert -32000 errors
		},
		{
			name:       "network timeout -32001",
			err:        fmt.Errorf("json-rpc error -32001: network timeout"),
			statusCode: 200,
			expected:   true,
		},
		// Retryable patterns should still work
		{
			name:       "connection error",
			err:        fmt.Errorf("connection timeout"),
			statusCode: 200,
			expected:   true,
		},
		{
			name:       "rate limit error",
			err:        fmt.Errorf("rate limit exceeded"),
			statusCode: 200,
			expected:   true,
		},
		// HTTP errors should still be retryable
		{
			name:       "HTTP 500 error",
			err:        fmt.Errorf("internal server error"),
			statusCode: 500,
			expected:   true,
		},
		{
			name:       "HTTP 429 rate limit",
			err:        fmt.Errorf("too many requests"),
			statusCode: 429,
			expected:   true,
		},
		// Traditional non-retryable errors should still not be retryable
		{
			name:       "method not found",
			err:        fmt.Errorf("method not found"),
			statusCode: 200,
			expected:   false,
		},
		{
			name:       "insufficient funds",
			err:        fmt.Errorf("insufficient funds"),
			statusCode: 200,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableJSONRPCError(tt.err, tt.statusCode)
			if result != tt.expected {
				t.Errorf("IsRetryableJSONRPCError(%v, %d) = %v, expected %v",
					tt.err, tt.statusCode, result, tt.expected)
			}
		})
	}
}
