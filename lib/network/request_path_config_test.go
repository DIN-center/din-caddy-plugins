package network

import (
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
			expectedPath: "/",  // Empty provider path should set to root
			expectedRaw:  "",
		},
		{
			name:         "provider path set",
			originalPath: "/ethereum",
			providerPath: "/rpc/v1",
			expectedPath: "/rpc/v1",
			expectedRaw:  "",  // RawPath should be cleared
		},
		{
			name:         "provider path with special characters",
			originalPath: "/ethereum",
			providerPath: "/rpc/v1/key%20test",
			expectedPath: "/rpc/v1/key%20test",  // Path should be kept as-is
			expectedRaw:  "",  // RawPath should be cleared
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