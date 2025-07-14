// lib/network/beacon_path_normalizer_test.go
package network

import (
	"testing"
)

func TestBeaconPathNormalizer_NormalizePath(t *testing.T) {
	normalizer := NewBeaconPathNormalizer()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "state with numeric ID",
			input:    "/eth/v1/beacon/states/12345/validators",
			expected: "/eth/v1/beacon/states/{state_id}/validators",
		},
		{
			name:     "state with head identifier",
			input:    "/eth/v1/beacon/states/head/validators",
			expected: "/eth/v1/beacon/states/{state_id}/validators",
		},
		{
			name:     "state with finalized identifier",
			input:    "/eth/v1/beacon/states/finalized/validators",
			expected: "/eth/v1/beacon/states/{state_id}/validators",
		},
		{
			name:     "state with hex hash",
			input:    "/eth/v1/beacon/states/0x1234abcd5678ef90/validators",
			expected: "/eth/v1/beacon/states/{state_id}/validators",
		},
		{
			name:     "block with numeric ID",
			input:    "/eth/v1/beacon/blocks/5432100",
			expected: "/eth/v1/beacon/blocks/{block_id}",
		},
		{
			name:     "block with head identifier",
			input:    "/eth/v1/beacon/blocks/head",
			expected: "/eth/v1/beacon/blocks/{block_id}",
		},
		{
			name:     "block with genesis identifier",
			input:    "/eth/v1/beacon/blocks/genesis",
			expected: "/eth/v1/beacon/blocks/{block_id}",
		},
		{
			name:     "block with hex hash",
			input:    "/eth/v1/beacon/blocks/0xabcdef1234567890/root",
			expected: "/eth/v1/beacon/blocks/{block_id}/root",
		},
		{
			name:     "header with numeric ID",
			input:    "/eth/v1/beacon/headers/5432100",
			expected: "/eth/v1/beacon/headers/{block_id}",
		},
		{
			name:     "header with head identifier",
			input:    "/eth/v1/beacon/headers/head",
			expected: "/eth/v1/beacon/headers/{block_id}",
		},
		{
			name:     "validator with numeric ID",
			input:    "/eth/v1/beacon/states/head/validators/123456",
			expected: "/eth/v1/beacon/states/{state_id}/validators/{validator_id}",
		},
		{
			name:     "validator with hex pubkey",
			input:    "/eth/v1/beacon/states/head/validators/0x123456789abcdef",
			expected: "/eth/v1/beacon/states/{state_id}/validators/{validator_id}",
		},
		{
			name:     "epoch endpoint",
			input:    "/eth/v1/beacon/epochs/12345/committees",
			expected: "/eth/v1/beacon/epochs/{epoch}/committees",
		},
		{
			name:     "committee endpoint",
			input:    "/eth/v1/beacon/committees/987654/attestations",
			expected: "/eth/v1/beacon/committees/{committee_id}/attestations",
		},
		{
			name:     "attestation endpoint",
			input:    "/eth/v1/beacon/attestations/0xabcdef123456",
			expected: "/eth/v1/beacon/attestations/{attestation_id}",
		},
		{
			name:     "deposit endpoint",
			input:    "/eth/v1/beacon/deposits/54321",
			expected: "/eth/v1/beacon/deposits/{deposit_id}",
		},
		{
			name:     "voluntary exit endpoint",
			input:    "/eth/v1/beacon/voluntary_exits/11111",
			expected: "/eth/v1/beacon/voluntary_exits/{exit_id}",
		},
		{
			name:     "sync committee endpoint",
			input:    "/eth/v1/beacon/sync_committees/333/duties",
			expected: "/eth/v1/beacon/sync_committees/{period}/duties",
		},
		{
			name:     "unchanged path",
			input:    "/eth/v1/beacon/genesis",
			expected: "/eth/v1/beacon/genesis",
		},
		{
			name:     "complex path with multiple patterns",
			input:    "/eth/v1/beacon/states/12345/validators/67890/balances",
			expected: "/eth/v1/beacon/states/{state_id}/validators/{validator_id}/balances",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizer.NormalizePath(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizePath(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBeaconPathNormalizer_GetCategory(t *testing.T) {
	normalizer := NewBeaconPathNormalizer()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "state endpoint",
			input:    "/eth/v1/beacon/states/12345/validators",
			expected: "state",
		},
		{
			name:     "block endpoint",
			input:    "/eth/v1/beacon/blocks/head",
			expected: "block",
		},
		{
			name:     "header endpoint",
			input:    "/eth/v1/beacon/headers/5432100",
			expected: "header",
		},
		{
			name:     "validator endpoint",
			input:    "/eth/v1/beacon/states/head/validators/123456",
			expected: "validator",
		},
		{
			name:     "epoch endpoint",
			input:    "/eth/v1/beacon/epochs/12345/committees",
			expected: "epoch",
		},
		{
			name:     "committee endpoint",
			input:    "/eth/v1/beacon/committees/987654/attestations",
			expected: "committee",
		},
		{
			name:     "unknown endpoint",
			input:    "/eth/v1/beacon/something/unknown",
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizer.GetCategory(tt.input)
			if result != tt.expected {
				t.Errorf("GetCategory(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBeaconPathNormalizer_AddPattern(t *testing.T) {
	normalizer := NewBeaconPathNormalizer()

	// Get initial pattern count
	initialPatterns := len(normalizer.GetPatterns())

	// Add a custom pattern
	customPattern := PathPattern{
		Regex:       `/custom/(\d+)/endpoint`,
		Replacement: "/custom/{id}/endpoint",
		Category:    "custom",
	}

	normalizer.AddPattern(customPattern)

	// Check that pattern was added
	newPatterns := len(normalizer.GetPatterns())
	if newPatterns != initialPatterns+1 {
		t.Errorf("Expected %d patterns after adding custom pattern, got %d", initialPatterns+1, newPatterns)
	}

	// Test the custom pattern works
	result := normalizer.NormalizePath("/custom/12345/endpoint")
	expected := "/custom/{id}/endpoint"

	if result != expected {
		t.Errorf("Custom pattern normalization failed: got %q, expected %q", result, expected)
	}
}

func TestBeaconPathNormalizer_CustomPatterns(t *testing.T) {
	// Test creating normalizer with custom patterns
	customPatterns := []PathPattern{
		{
			Regex:       `/test/(\d+)/custom`,
			Replacement: "/test/{id}/custom",
			Category:    "test",
		},
		{
			Regex:       `/another/([a-f0-9]+)/pattern`,
			Replacement: "/another/{hash}/pattern",
			Category:    "another",
		},
	}

	normalizer := NewBeaconPathNormalizerWithPatterns(customPatterns)

	// Test first custom pattern
	result1 := normalizer.NormalizePath("/test/12345/custom")
	expected1 := "/test/{id}/custom"

	if result1 != expected1 {
		t.Errorf("First custom pattern failed: got %q, expected %q", result1, expected1)
	}

	// Test second custom pattern
	result2 := normalizer.NormalizePath("/another/abc123/pattern")
	expected2 := "/another/{hash}/pattern"

	if result2 != expected2 {
		t.Errorf("Second custom pattern failed: got %q, expected %q", result2, expected2)
	}

	// Test that default patterns are not present
	result3 := normalizer.NormalizePath("/eth/v1/beacon/states/12345/validators")
	expected3 := "/eth/v1/beacon/states/12345/validators" // Should be unchanged

	if result3 != expected3 {
		t.Errorf("Expected default patterns to be absent, but got normalized result: %q", result3)
	}
}

func TestBeaconPathNormalizer_DefaultPatterns(t *testing.T) {
	patterns := getDefaultBeaconPatterns()

	// Test that we have the expected number of default patterns
	expectedPatternCount := 10 // Update this if you add more patterns
	if len(patterns) != expectedPatternCount {
		t.Errorf("Expected %d default patterns, got %d", expectedPatternCount, len(patterns))
	}

	// Test that all patterns have required fields
	for i, pattern := range patterns {
		if pattern.Regex == "" {
			t.Errorf("Pattern %d has empty regex", i)
		}
		if pattern.Replacement == "" {
			t.Errorf("Pattern %d has empty replacement", i)
		}
		if pattern.Category == "" {
			t.Errorf("Pattern %d has empty category", i)
		}
	}

	// Test that we have all expected categories
	categories := make(map[string]bool)
	for _, pattern := range patterns {
		categories[pattern.Category] = true
	}

	expectedCategories := []string{
		"state", "block", "validator", "header", "epoch",
		"committee", "attestation", "deposit", "voluntary_exit", "sync_committee",
	}

	for _, expectedCategory := range expectedCategories {
		if !categories[expectedCategory] {
			t.Errorf("Expected category %q not found in default patterns", expectedCategory)
		}
	}
}
