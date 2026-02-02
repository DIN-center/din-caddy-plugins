package network

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestHandlerCapabilities verifies that each handler implements the expected capability interfaces
func TestHandlerCapabilities(t *testing.T) {
	testCases := []struct {
		name                   string
		handlerType            string
		expectHandler          bool
		expectRequestProcessor bool
		expectPathConfigurer   bool
		expectResponseHandler  bool
		expectHealthChecker    bool
		expectBlockNumberParser bool
		expectChainIdentifier  bool
		expectBlockFetcher     bool
		expectArchiveChecker   bool
		expectDynamicBlockLag  bool
		expectMethodProvider   bool
	}{
		{
			name:                   "EVM handler implements all capabilities",
			handlerType:            "evm",
			expectHandler:          true,
			expectRequestProcessor: true,
			expectPathConfigurer:   true,
			expectResponseHandler:  true,
			expectHealthChecker:    true,
			expectBlockNumberParser: true,
			expectChainIdentifier:  true,
			expectBlockFetcher:     true,
			expectArchiveChecker:   true,
			expectDynamicBlockLag:  true,
			expectMethodProvider:   true,
		},
		{
			name:                   "Beacon Chain handler implements REST capabilities",
			handlerType:            "beacon-chain",
			expectHandler:          true,
			expectRequestProcessor: true,
			expectPathConfigurer:   true,
			expectResponseHandler:  true,
			expectHealthChecker:    true,
			expectBlockNumberParser: true,
			expectChainIdentifier:  true,
			expectBlockFetcher:     true,
			expectArchiveChecker:   true,  // Returns false from SupportsArchiveMode
			expectDynamicBlockLag:  true,  // Returns false from SupportsDynamicBlockLag
			expectMethodProvider:   true,
		},
		{
			name:                   "Bitcoin Esplora handler implements REST capabilities",
			handlerType:            "bitcoin-esplora",
			expectHandler:          true,
			expectRequestProcessor: true,
			expectPathConfigurer:   true,
			expectResponseHandler:  true,
			expectHealthChecker:    true,
			expectBlockNumberParser: true,
			expectChainIdentifier:  true,
			expectBlockFetcher:     true,
			expectArchiveChecker:   true,  // Returns false from SupportsArchiveMode
			expectDynamicBlockLag:  true,  // Returns false from SupportsDynamicBlockLag
			expectMethodProvider:   true,
		},
		{
			name:                   "Starknet handler implements RPC capabilities",
			handlerType:            "starknet",
			expectHandler:          true,
			expectRequestProcessor: true,
			expectPathConfigurer:   true,
			expectResponseHandler:  true,
			expectHealthChecker:    true,
			expectBlockNumberParser: true,
			expectChainIdentifier:  true,
			expectBlockFetcher:     true,
			expectArchiveChecker:   true,
			expectDynamicBlockLag:  true,
			expectMethodProvider:   true,
		},
		{
			name:                   "Solana handler implements RPC capabilities",
			handlerType:            "solana",
			expectHandler:          true,
			expectRequestProcessor: true,
			expectPathConfigurer:   true,
			expectResponseHandler:  true,
			expectHealthChecker:    true,
			expectBlockNumberParser: true,
			expectChainIdentifier:  true,
			expectBlockFetcher:     true,
			expectArchiveChecker:   true,
			expectDynamicBlockLag:  true,
			expectMethodProvider:   true,
		},
		{
			name:                   "Tron handler implements RPC capabilities",
			handlerType:            "tron-full-node",
			expectHandler:          true,
			expectRequestProcessor: true,
			expectPathConfigurer:   true,
			expectResponseHandler:  true,
			expectHealthChecker:    true,
			expectBlockNumberParser: true,
			expectChainIdentifier:  true,
			expectBlockFetcher:     true,
			expectArchiveChecker:   true,
			expectDynamicBlockLag:  true,
			expectMethodProvider:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Registry uses compound key (name:type), so same name with different types works
			config := &NetworkConfig{
				Name: "test-network",
				Type: tc.handlerType,
			}

			handler, err := DefaultRegistry.GetHandler(tc.handlerType, config)
			assert.NoError(t, err, "Failed to get handler for type %s", tc.handlerType)
			assert.NotNil(t, handler, "Handler should not be nil")

			// Test Handler interface
			_, ok := handler.(Handler)
			assert.Equal(t, tc.expectHandler, ok, "Handler interface implementation mismatch")

			// Test RequestProcessor interface
			_, ok = handler.(RequestProcessor)
			assert.Equal(t, tc.expectRequestProcessor, ok, "RequestProcessor interface implementation mismatch")

			// Test PathConfigurer interface
			_, ok = handler.(PathConfigurer)
			assert.Equal(t, tc.expectPathConfigurer, ok, "PathConfigurer interface implementation mismatch")

			// Test ResponseHandler interface
			_, ok = handler.(ResponseHandler)
			assert.Equal(t, tc.expectResponseHandler, ok, "ResponseHandler interface implementation mismatch")

			// Test HealthChecker interface
			_, ok = handler.(HealthChecker)
			assert.Equal(t, tc.expectHealthChecker, ok, "HealthChecker interface implementation mismatch")

			// Test BlockNumberParser interface
			_, ok = handler.(BlockNumberParser)
			assert.Equal(t, tc.expectBlockNumberParser, ok, "BlockNumberParser interface implementation mismatch")

			// Test ChainIdentifier interface
			_, ok = handler.(ChainIdentifier)
			assert.Equal(t, tc.expectChainIdentifier, ok, "ChainIdentifier interface implementation mismatch")

			// Test BlockFetcher interface
			_, ok = handler.(BlockFetcher)
			assert.Equal(t, tc.expectBlockFetcher, ok, "BlockFetcher interface implementation mismatch")

			// Test ArchiveChecker interface
			_, ok = handler.(ArchiveChecker)
			assert.Equal(t, tc.expectArchiveChecker, ok, "ArchiveChecker interface implementation mismatch")

			// Test DynamicBlockLagSupport interface
			_, ok = handler.(DynamicBlockLagSupport)
			assert.Equal(t, tc.expectDynamicBlockLag, ok, "DynamicBlockLagSupport interface implementation mismatch")

			// Test MethodProvider interface
			_, ok = handler.(MethodProvider)
			assert.Equal(t, tc.expectMethodProvider, ok, "MethodProvider interface implementation mismatch")
		})
	}
}

// TestCapabilitySupportsFlags verifies that Supports* methods return correct values
func TestCapabilitySupportsFlags(t *testing.T) {
	testCases := []struct {
		name                      string
		handlerType               string
		expectSupportsArchive     bool
		expectSupportsDynamicLag  bool
		expectSupportsBlockFetch  bool
	}{
		{
			name:                     "EVM supports all capabilities",
			handlerType:              "evm",
			expectSupportsArchive:    true,
			expectSupportsDynamicLag: true,
			expectSupportsBlockFetch: true,
		},
		{
			name:                     "Beacon Chain does not support archive, dynamic lag, or block fetch",
			handlerType:              "beacon-chain",
			expectSupportsArchive:    false,
			expectSupportsDynamicLag: false,
			expectSupportsBlockFetch: false,
		},
		{
			name:                     "Bitcoin Esplora does not support archive or dynamic lag",
			handlerType:              "bitcoin-esplora",
			expectSupportsArchive:    false,
			expectSupportsDynamicLag: false,
			expectSupportsBlockFetch: true,
		},
		{
			name:                     "Starknet supports archive and dynamic lag but not block fetch",
			handlerType:              "starknet",
			expectSupportsArchive:    true,
			expectSupportsDynamicLag: true,
			expectSupportsBlockFetch: false,
		},
		{
			name:                     "Solana supports dynamic lag and block fetch but not archive",
			handlerType:              "solana",
			expectSupportsArchive:    false,
			expectSupportsDynamicLag: true,
			expectSupportsBlockFetch: true,
		},
		{
			name:                     "Tron supports dynamic lag and block fetch but not archive",
			handlerType:              "tron-full-node",
			expectSupportsArchive:    false,
			expectSupportsDynamicLag: true,
			expectSupportsBlockFetch: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Registry uses compound key (name:type), so same name with different types works
			config := &NetworkConfig{
				Name: "test-network",
				Type: tc.handlerType,
			}

			handler, err := DefaultRegistry.GetHandler(tc.handlerType, config)
			assert.NoError(t, err)

			// Test SupportsArchiveMode helper
			assert.Equal(t, tc.expectSupportsArchive, SupportsArchiveMode(handler),
				"SupportsArchiveMode mismatch for %s", tc.handlerType)

			// Test SupportsDynamicBlockLag helper
			assert.Equal(t, tc.expectSupportsDynamicLag, SupportsDynamicBlockLag(handler),
				"SupportsDynamicBlockLag mismatch for %s", tc.handlerType)

			// Test SupportsBlockFetching helper
			assert.Equal(t, tc.expectSupportsBlockFetch, SupportsBlockFetching(handler),
				"SupportsBlockFetching mismatch for %s", tc.handlerType)
		})
	}
}

// TestCapabilityHelperFunctions verifies the helper functions work correctly
func TestCapabilityHelperFunctions(t *testing.T) {
	config := &NetworkConfig{
		Name: "test-network",
		Type: "evm",
	}

	handler, err := DefaultRegistry.GetHandler("evm", config)
	assert.NoError(t, err)

	t.Run("SupportsChainID returns true for ChainIdentifier implementers", func(t *testing.T) {
		assert.True(t, SupportsChainID(handler))
	})

	t.Run("SupportsChainID returns false for non-implementers", func(t *testing.T) {
		nonHandler := struct{}{}
		assert.False(t, SupportsChainID(nonHandler))
	})

	t.Run("SupportsArchiveMode returns correct value based on handler method", func(t *testing.T) {
		// EVM supports archive mode
		assert.True(t, SupportsArchiveMode(handler))

		// Non-handler returns false
		nonHandler := struct{}{}
		assert.False(t, SupportsArchiveMode(nonHandler))
	})

	t.Run("SupportsDynamicBlockLag returns correct value based on handler method", func(t *testing.T) {
		// EVM supports dynamic block lag
		assert.True(t, SupportsDynamicBlockLag(handler))

		// Non-handler returns false
		nonHandler := struct{}{}
		assert.False(t, SupportsDynamicBlockLag(nonHandler))
	})

	t.Run("SupportsBlockFetching returns correct value based on handler method", func(t *testing.T) {
		// EVM supports block fetching
		assert.True(t, SupportsBlockFetching(handler))

		// Non-handler returns false
		nonHandler := struct{}{}
		assert.False(t, SupportsBlockFetching(nonHandler))
	})
}

// TestNetworkHandlerSatisfiesCapabilities verifies NetworkHandler satisfies capability interfaces
func TestNetworkHandlerSatisfiesCapabilities(t *testing.T) {
	// This test verifies that any type implementing NetworkHandler
	// also satisfies all the capability interfaces (due to method overlap)

	var handler NetworkHandler = &MockHandler{
		handlerType: "mock",
		name:        "Mock Handler",
	}

	t.Run("NetworkHandler satisfies Handler", func(t *testing.T) {
		_, ok := handler.(Handler)
		assert.True(t, ok, "NetworkHandler should satisfy Handler interface")
	})

	t.Run("NetworkHandler satisfies RequestProcessor", func(t *testing.T) {
		_, ok := handler.(RequestProcessor)
		assert.True(t, ok, "NetworkHandler should satisfy RequestProcessor interface")
	})

	t.Run("NetworkHandler satisfies ResponseHandler", func(t *testing.T) {
		_, ok := handler.(ResponseHandler)
		assert.True(t, ok, "NetworkHandler should satisfy ResponseHandler interface")
	})

	t.Run("NetworkHandler satisfies HealthChecker", func(t *testing.T) {
		_, ok := handler.(HealthChecker)
		assert.True(t, ok, "NetworkHandler should satisfy HealthChecker interface")
	})

	t.Run("NetworkHandler satisfies BlockNumberParser", func(t *testing.T) {
		_, ok := handler.(BlockNumberParser)
		assert.True(t, ok, "NetworkHandler should satisfy BlockNumberParser interface")
	})

	t.Run("NetworkHandler satisfies ChainIdentifier", func(t *testing.T) {
		_, ok := handler.(ChainIdentifier)
		assert.True(t, ok, "NetworkHandler should satisfy ChainIdentifier interface")
	})

	t.Run("NetworkHandler satisfies BlockFetcher", func(t *testing.T) {
		_, ok := handler.(BlockFetcher)
		assert.True(t, ok, "NetworkHandler should satisfy BlockFetcher interface")
	})

	t.Run("NetworkHandler satisfies ArchiveChecker", func(t *testing.T) {
		_, ok := handler.(ArchiveChecker)
		assert.True(t, ok, "NetworkHandler should satisfy ArchiveChecker interface")
	})

	t.Run("NetworkHandler satisfies DynamicBlockLagSupport", func(t *testing.T) {
		_, ok := handler.(DynamicBlockLagSupport)
		assert.True(t, ok, "NetworkHandler should satisfy DynamicBlockLagSupport interface")
	})
}
