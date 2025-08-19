package modules

import (
	"net/url"
	"testing"
	
	"github.com/stretchr/testify/assert"
)

func TestEnsureUniqueProviderHost(t *testing.T) {
	tests := []struct {
		name              string
		networkName       string
		urlString         string
		headers           map[string]string
		existingProviders map[string]*provider
		expected          string
		description       string
	}{
		// Basic cases
		{
			name:              "first_provider_no_suffix_needed",
			networkName:       "bsc-mainnet",
			urlString:         "https://mainnet.bsc.validationcloud.io/v1/Q8LyK-Pm9VIVeosS6Eqbm6MlZM2ip7ir9bC_nkcXiFY",
			headers:           nil,
			existingProviders: map[string]*provider{},
			expected:          "mainnet.bsc.validationcloud.io",
			description:       "First provider with a host should use base host without suffix",
		},
		{
			name:        "second_provider_with_path_api_key",
			networkName: "bsc-mainnet",
			urlString:   "https://mainnet.bsc.validationcloud.io/v1/j5nPy9ZVeu0_XMsQQaRwOrpaB5qMcCMmAxdbChVH3bI",
			headers:     nil,
			existingProviders: map[string]*provider{
				"mainnet.bsc.validationcloud.io": {},
			},
			expected:    "mainnet.bsc.validationcloud.io-H3bI",
			description: "Second provider should extract last 4 chars from API key in path",
		},
		{
			name:        "third_provider_with_path_api_key",
			networkName: "bsc-mainnet",
			urlString:   "https://mainnet.bsc.validationcloud.io/v1/QoPnp-DVCrhoM-5Xf2_cvP0IrnBiKn-1Yvox7M7kKUg",
			headers:     nil,
			existingProviders: map[string]*provider{
				"mainnet.bsc.validationcloud.io":      {},
				"mainnet.bsc.validationcloud.io-H3bI": {},
			},
			expected:    "mainnet.bsc.validationcloud.io-kKUg",
			description: "Third provider should also use unique suffix",
		},

		// Header-based API key cases
		{
			name:        "first_provider_header_api_key",
			networkName: "arb-sepolia",
			urlString:   "https://arb-sepolia-din.us.nodefleet.net",
			headers: map[string]string{
				"X-API-Key": "08e7eb7f1b52f8da9c6fd30656d8903353304036ab0ec022a995537724cb1ffc",
			},
			existingProviders: map[string]*provider{},
			expected:          "arb-sepolia-din.us.nodefleet.net",
			description:       "First provider doesn't need suffix even with API key in header",
		},
		{
			name:        "second_provider_header_api_key",
			networkName: "arb-sepolia",
			urlString:   "https://arb-sepolia-din.us.nodefleet.net",
			headers: map[string]string{
				"X-API-Key": "12345678901234567890123456789012345678901234567890123456789abcde",
			},
			existingProviders: map[string]*provider{
				"arb-sepolia-din.us.nodefleet.net": {},
			},
			expected:    "arb-sepolia-din.us.nodefleet.net-bcde",
			description: "Second provider with header API key should use last 4 chars",
		},

		// Case insensitive header handling
		{
			name:        "case_insensitive_header",
			networkName: "eth-mainnet",
			urlString:   "https://eth-mainnet.nodefleet.net",
			headers: map[string]string{
				"x-api-key": "test1234", // lowercase
			},
			existingProviders: map[string]*provider{
				"eth-mainnet.nodefleet.net": {},
			},
			expected:    "eth-mainnet.nodefleet.net-1234",
			description: "Should handle lowercase x-api-key header",
		},
		{
			name:        "mixed_case_header",
			networkName: "eth-mainnet",
			urlString:   "https://eth-mainnet.nodefleet.net",
			headers: map[string]string{
				"X-Api-Key": "test5678", // mixed case
			},
			existingProviders: map[string]*provider{
				"eth-mainnet.nodefleet.net": {},
			},
			expected:    "eth-mainnet.nodefleet.net-5678",
			description: "Should handle mixed case X-Api-Key header",
		},

		// Edge cases
		{
			name:        "short_api_key_in_path",
			networkName: "test-net",
			urlString:   "https://test.provider.com/v1/abc", // Less than 4 chars
			headers:     nil,
			existingProviders: map[string]*provider{
				"test.provider.com": {},
			},
			expected:    "test.provider.com-1",
			description: "Should fallback to counter when API key is too short",
		},
		{
			name:        "short_api_key_in_header",
			networkName: "test-net",
			urlString:   "https://test.provider.com",
			headers: map[string]string{
				"X-API-Key": "123", // Less than 4 chars
			},
			existingProviders: map[string]*provider{
				"test.provider.com": {},
			},
			expected:    "test.provider.com-1",
			description: "Should fallback to counter when header API key is too short",
		},
		{
			name:        "empty_path_empty_headers",
			networkName: "test-net",
			urlString:   "https://simple.provider.com",
			headers:     nil,
			existingProviders: map[string]*provider{
				"simple.provider.com": {},
			},
			expected:    "simple.provider.com-1",
			description: "Should use counter when no API key available",
		},
		{
			name:        "whitespace_in_header_value",
			networkName: "test-net",
			urlString:   "https://test.provider.com",
			headers: map[string]string{
				"X-API-Key": "  abcd1234efgh5678  ", // whitespace
			},
			existingProviders: map[string]*provider{
				"test.provider.com": {},
			},
			expected:    "test.provider.com-5678",
			description: "Should trim whitespace from header value",
		},

		// Collision handling
		{
			name:        "suffix_collision_resolution",
			networkName: "test-net",
			urlString:   "https://test.provider.com/v1/different_key_same_1234",
			headers:     nil,
			existingProviders: map[string]*provider{
				"test.provider.com":      {},
				"test.provider.com-1234": {}, // Collision!
			},
			expected:    "test.provider.com-1234-1",
			description: "Should handle suffix collisions by appending counter",
		},
		{
			name:        "multiple_suffix_collisions",
			networkName: "test-net",
			urlString:   "https://test.provider.com/v1/another_key_1234",
			headers:     nil,
			existingProviders: map[string]*provider{
				"test.provider.com":        {},
				"test.provider.com-1234":   {},
				"test.provider.com-1234-1": {},
			},
			expected:    "test.provider.com-1234-2",
			description: "Should increment collision counter",
		},

		// Complex path structures
		{
			name:        "nested_path_with_api_key",
			networkName: "test-net",
			urlString:   "https://api.provider.com/v2/mainnet/rpc/key123456789",
			headers:     nil,
			existingProviders: map[string]*provider{
				"api.provider.com": {},
			},
			expected:    "api.provider.com-6789",
			description: "Should extract from last path segment",
		},
		{
			name:        "path_with_query_params",
			networkName: "test-net",
			urlString:   "https://api.provider.com/endpoint/apikey9876?param=value",
			headers:     nil,
			existingProviders: map[string]*provider{
				"api.provider.com": {},
			},
			expected:    "api.provider.com-9876",
			description: "Should handle URLs with query parameters",
		},

		// Nil and empty cases
		{
			name:              "nil_url",
			networkName:       "test-net",
			urlString:         "",
			headers:           nil,
			existingProviders: map[string]*provider{},
			expected:          "",
			description:       "Should handle nil URL gracefully",
		},
		{
			name:              "empty_host",
			networkName:       "test-net",
			urlString:         "http:///path/to/resource", // Invalid URL with empty host
			headers:           nil,
			existingProviders: map[string]*provider{},
			expected:          "",
			description:       "Should handle empty host",
		},

		// Priority: path over header
		{
			name:        "both_path_and_header_prefers_path",
			networkName: "test-net",
			urlString:   "https://api.provider.com/v1/pathkey1234",
			headers: map[string]string{
				"X-API-Key": "headerkey5678",
			},
			existingProviders: map[string]*provider{
				"api.provider.com": {},
			},
			expected:    "api.provider.com-1234",
			description: "Should prefer path API key over header API key",
		},

		// Port handling
		{
			name:        "provider_with_port",
			networkName: "test-net",
			urlString:   "https://api.provider.com:8545/v1/key9999",
			headers:     nil,
			existingProviders: map[string]*provider{
				"api.provider.com:8545": {},
			},
			expected:    "api.provider.com:8545-9999",
			description: "Should handle hosts with ports correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			d := &DinMiddleware{
				Networks: map[string]*network{
					tt.networkName: {
						Providers: tt.existingProviders,
					},
				},
			}

			parsedUrl, _ := url.Parse(tt.urlString)

			// Execute
			result := d.ensureUniqueProviderHost(tt.networkName, parsedUrl, tt.headers)

			// Assert
			assert.Equal(t, tt.expected, result, tt.description)
		})
	}
}

// Test concurrent provider additions
func TestEnsureUniqueProviderHostConcurrent(t *testing.T) {
	d := &DinMiddleware{
		Networks: map[string]*network{
			"test-net": {
				Providers: make(map[string]*provider),
			},
		},
	}

	// Simulate adding multiple providers sequentially (simulating concurrent scenario)
	urls := []string{
		"https://api.example.com/apikey1234",
		"https://api.example.com/apikey5678",
		"https://api.example.com/apikey9abc",
	}

	expectedHosts := []string{
		"api.example.com",       // First gets base name
		"api.example.com-5678",  // Second gets suffix (first updated retroactively to -1234)
		"api.example.com-9abc",  // Third gets suffix
	}

	for i, urlStr := range urls {
		parsedUrl, _ := url.Parse(urlStr)
		result := d.ensureUniqueProviderHost("test-net", parsedUrl, nil)

		// Update the provider map to simulate real usage
		d.Networks["test-net"].Providers[result] = &provider{
			HttpUrl: urlStr,
			host:    result,
		}

		// For the first provider, it should get base name initially
		if i == 0 && result != expectedHosts[i] {
			t.Errorf("Provider %d: expected host %s, got %s", i, expectedHosts[i], result)
		}
	}

	// After all additions, verify the final state:
	// First provider should have been retroactively updated
	if _, exists := d.Networks["test-net"].Providers["api.example.com"]; exists {
		t.Error("First provider still has base name without suffix after retroactive update")
	}
	
	// All three should have suffixes now
	expectedFinalHosts := []string{
		"api.example.com-1234",
		"api.example.com-5678",
		"api.example.com-9abc",
	}
	
	for _, expectedHost := range expectedFinalHosts {
		if _, exists := d.Networks["test-net"].Providers[expectedHost]; !exists {
			t.Errorf("Expected provider with host %s not found", expectedHost)
		}
	}

	// Verify all hosts are unique
	seen := make(map[string]bool)
	for host := range d.Networks["test-net"].Providers {
		if seen[host] {
			t.Errorf("Duplicate host found: %s", host)
		}
		seen[host] = true
	}
}

// Test the providerHostExists helper function
func TestProviderHostExists(t *testing.T) {
	d := &DinMiddleware{
		Networks: map[string]*network{
			"test-net": {
				Providers: map[string]*provider{
					"existing.provider.com":      {},
					"existing.provider.com-1234": {},
				},
			},
		},
	}

	tests := []struct {
		name        string
		networkName string
		host        string
		expected    bool
	}{
		{
			name:        "existing_host",
			networkName: "test-net",
			host:        "existing.provider.com",
			expected:    true,
		},
		{
			name:        "existing_host_with_suffix",
			networkName: "test-net",
			host:        "existing.provider.com-1234",
			expected:    true,
		},
		{
			name:        "non_existing_host",
			networkName: "test-net",
			host:        "new.provider.com",
			expected:    false,
		},
		{
			name:        "non_existing_network",
			networkName: "other-net",
			host:        "existing.provider.com",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := d.providerHostExists(tt.networkName, tt.host)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test retroactive update functionality
func TestEnsureUniqueProviderHostRetroactiveUpdate(t *testing.T) {
	// Create a middleware without logger to avoid nil pointer issues
	d := &DinMiddleware{
		Networks: map[string]*network{
			"test-net": {
				Providers: make(map[string]*provider),
			},
		},
	}

	// Test case 1: Add first provider - should get base name without suffix
	url1, _ := url.Parse("https://validation.cloud/v1/bsc/apikey1234")
	host1 := d.ensureUniqueProviderHost("test-net", url1, nil)
	
	// First provider should get base name (no suffix initially)
	assert.Equal(t, "validation.cloud", host1, "First provider should get base name without suffix")
	
	// Add first provider to the map
	d.Networks["test-net"].Providers[host1] = &provider{
		HttpUrl: url1.String(),
		host:    host1,
	}
	
	// Test case 2: Add second provider with same base host - should trigger retroactive update
	url2, _ := url.Parse("https://validation.cloud/v1/bsc/apikey5678") 
	host2 := d.ensureUniqueProviderHost("test-net", url2, nil)
	
	// Second provider should get suffix
	assert.Equal(t, "validation.cloud-5678", host2, "Second provider should get suffix")
	
	// Check if first provider was retroactively updated
	if _, exists := d.Networks["test-net"].Providers["validation.cloud"]; exists {
		t.Error("First provider still has base name without suffix after retroactive update")
	}
	
	if firstProvider, exists := d.Networks["test-net"].Providers["validation.cloud-1234"]; !exists {
		t.Error("First provider not found with expected suffix after retroactive update")
	} else {
		assert.Equal(t, "validation.cloud-1234", firstProvider.host, "First provider host field should be updated")
	}
	
	// Add second provider to the map
	d.Networks["test-net"].Providers[host2] = &provider{
		HttpUrl: url2.String(),
		host:    host2,
	}
	
	// Test case 3: Add third provider - should NOT trigger another retroactive update
	url3, _ := url.Parse("https://validation.cloud/v1/bsc/apikey9abc")
	
	// Store current state of first two providers
	firstProviderBefore := d.Networks["test-net"].Providers["validation.cloud-1234"]
	secondProviderBefore := d.Networks["test-net"].Providers["validation.cloud-5678"]
	
	host3 := d.ensureUniqueProviderHost("test-net", url3, nil)
	
	// Third provider should get suffix
	assert.Equal(t, "validation.cloud-9abc", host3, "Third provider should get suffix")
	
	// Verify first and second providers were NOT changed again
	firstProviderAfter := d.Networks["test-net"].Providers["validation.cloud-1234"]
	secondProviderAfter := d.Networks["test-net"].Providers["validation.cloud-5678"]
	
	assert.Equal(t, firstProviderBefore, firstProviderAfter, "First provider should not be modified when adding third provider")
	assert.Equal(t, secondProviderBefore, secondProviderAfter, "Second provider should not be modified when adding third provider")
	
	// Add third provider
	d.Networks["test-net"].Providers[host3] = &provider{
		HttpUrl: url3.String(),
		host:    host3,
	}
	
	// Test case 4: Add fourth provider - should also NOT trigger retroactive updates
	url4, _ := url.Parse("https://validation.cloud/v1/bsc/apikeyDEF0")
	
	// Store state before adding fourth
	stateBefore := make(map[string]*provider)
	for k, v := range d.Networks["test-net"].Providers {
		stateBefore[k] = v
	}
	
	host4 := d.ensureUniqueProviderHost("test-net", url4, nil)
	
	// Fourth provider should get suffix
	assert.Equal(t, "validation.cloud-DEF0", host4, "Fourth provider should get suffix")
	
	// Verify no existing providers were changed
	for k, v := range stateBefore {
		if currentProvider, exists := d.Networks["test-net"].Providers[k]; !exists {
			t.Errorf("Provider %s was removed when adding fourth provider", k)
		} else if currentProvider != v {
			t.Errorf("Provider %s was modified when adding fourth provider", k)
		}
	}
	
	// Add fourth provider to the map
	d.Networks["test-net"].Providers[host4] = &provider{
		HttpUrl: url4.String(),
		host:    host4,
	}
	
	// Verify all four providers have unique suffixed names
	expectedProviders := map[string]bool{
		"validation.cloud-1234": true,
		"validation.cloud-5678": true,
		"validation.cloud-9abc": true,
		"validation.cloud-DEF0": true,
	}
	
	for expectedHost := range expectedProviders {
		if _, exists := d.Networks["test-net"].Providers[expectedHost]; !exists {
			t.Errorf("Expected provider with host '%s' not found", expectedHost)
		}
	}
	
	// Ensure no provider has the base name without suffix
	if _, exists := d.Networks["test-net"].Providers["validation.cloud"]; exists {
		t.Error("Provider with base name (no suffix) still exists after adding multiple providers")
	}
	
	// Verify total count
	assert.Equal(t, 4, len(d.Networks["test-net"].Providers), "Should have exactly 4 providers")
}