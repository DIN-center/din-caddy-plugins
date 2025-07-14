// lib/network/beacon_path_normalizer.go
package network

import (
	"regexp"
)

// BeaconPathNormalizer handles path normalization for Beacon Chain REST APIs
type BeaconPathNormalizer struct {
	patterns []CompiledPathPattern
}

// CompiledPathPattern represents a compiled path pattern
type CompiledPathPattern struct {
	regex       *regexp.Regexp
	replacement string
	category    string
}

// NewBeaconPathNormalizer creates a new beacon path normalizer with default patterns
func NewBeaconPathNormalizer() *BeaconPathNormalizer {
	return NewBeaconPathNormalizerWithPatterns(getDefaultBeaconPatterns())
}

// NewBeaconPathNormalizerWithPatterns creates a normalizer with custom patterns
func NewBeaconPathNormalizerWithPatterns(patterns []PathPattern) *BeaconPathNormalizer {
	compiled := make([]CompiledPathPattern, len(patterns))
	for i, pattern := range patterns {
		compiled[i] = CompiledPathPattern{
			regex:       regexp.MustCompile(pattern.Regex),
			replacement: pattern.Replacement,
			category:    pattern.Category,
		}
	}

	return &BeaconPathNormalizer{
		patterns: compiled,
	}
}

// getDefaultBeaconPatterns returns the default set of beacon chain path patterns
func getDefaultBeaconPatterns() []PathPattern {
	return []PathPattern{
		// State endpoints - normalize state identifiers
		{
			Regex:       `/states/(head|genesis|finalized|justified|\d+|0x[a-fA-F0-9]+)/`,
			Replacement: "/states/{state_id}/",
			Category:    "state",
		},
		// Block endpoints - normalize block identifiers
		{
			Regex:       `/blocks/(head|genesis|finalized|\d+|0x[a-fA-F0-9]+)(\b|/|$)`,
			Replacement: "/blocks/{block_id}$2",
			Category:    "block",
		},
		// Validator endpoints - normalize validator identifiers
		{
			Regex:       `/validators/(\d+|0x[a-fA-F0-9]+)(\b|/|$)`,
			Replacement: "/validators/{validator_id}$2",
			Category:    "validator",
		},
		// Headers endpoints - normalize block identifiers
		{
			Regex:       `/headers/(head|\d+|0x[a-fA-F0-9]+)(\b|/|$)`,
			Replacement: "/headers/{block_id}$2",
			Category:    "header",
		},
		// Epoch endpoints - normalize epoch numbers
		{
			Regex:       `/epochs/(\d+)/`,
			Replacement: "/epochs/{epoch}/",
			Category:    "epoch",
		},
		// Committee endpoints - normalize committee identifiers
		{
			Regex:       `/committees/(\d+)/`,
			Replacement: "/committees/{committee_id}/",
			Category:    "committee",
		},
		// Attestation endpoints - normalize attestation identifiers
		{
			Regex:       `/attestations/(\d+|0x[a-fA-F0-9]+)(\b|/|$)`,
			Replacement: "/attestations/{attestation_id}$2",
			Category:    "attestation",
		},
		// Deposit endpoints - normalize deposit identifiers
		{
			Regex:       `/deposits/(\d+)`,
			Replacement: "/deposits/{deposit_id}",
			Category:    "deposit",
		},
		// Voluntary exits - normalize exit identifiers
		{
			Regex:       `/voluntary_exits/(\d+)`,
			Replacement: "/voluntary_exits/{exit_id}",
			Category:    "voluntary_exit",
		},
		// Sync committee endpoints
		{
			Regex:       `/sync_committees/(\d+)/`,
			Replacement: "/sync_committees/{period}/",
			Category:    "sync_committee",
		},
	}
}

// NormalizePath normalizes a path by applying all registered patterns
func (n *BeaconPathNormalizer) NormalizePath(path string) string {
	normalized := path
	for _, pattern := range n.patterns {
		normalized = pattern.regex.ReplaceAllString(normalized, pattern.replacement)
	}
	return normalized
}

// AddPattern adds a new pattern to the normalizer
func (n *BeaconPathNormalizer) AddPattern(pattern PathPattern) {
	compiled := CompiledPathPattern{
		regex:       regexp.MustCompile(pattern.Regex),
		replacement: pattern.Replacement,
		category:    pattern.Category,
	}
	n.patterns = append(n.patterns, compiled)
}

// GetPatterns returns the compiled patterns
func (n *BeaconPathNormalizer) GetPatterns() []CompiledPathPattern {
	return n.patterns
}

// GetCategory returns the category for a given normalized path
// Returns the most specific match (longest match)
func (n *BeaconPathNormalizer) GetCategory(path string) string {
	var bestMatch string
	var bestLength int

	for _, pattern := range n.patterns {
		if pattern.regex.MatchString(path) {
			// Find the match to determine its length
			match := pattern.regex.FindString(path)
			if len(match) > bestLength {
				bestMatch = pattern.category
				bestLength = len(match)
			}
		}
	}

	if bestMatch != "" {
		return bestMatch
	}
	return "unknown"
}
