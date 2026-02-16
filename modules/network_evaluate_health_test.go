package modules

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	networklib "github.com/DIN-center/din-caddy-plugins/lib/network"
	"github.com/DIN-center/din-caddy-plugins/lib/utils"
)

// TestEvaluateProviderHealth tests the provider health evaluation logic
func TestEvaluateProviderHealth(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name               string
		currentBlock       int64
		latestNetworkBlock int64
		initialStatus      HealthStatus
		providerHistory    []blockHistoryEntry
		chainIDValid       bool
		archiveSupported   bool
		expectedStatus     HealthStatus
		networkConfig      struct {
			blockLagLimit  int64
			blockJumpLimit int64
		}
	}{
		{
			name:               "healthy_provider_no_lag",
			currentBlock:       100,
			latestNetworkBlock: 100,
			initialStatus:      Healthy,
			chainIDValid:       true,
			archiveSupported:   false,
			expectedStatus:     Healthy,
			networkConfig: struct {
				blockLagLimit  int64
				blockJumpLimit int64
			}{
				blockLagLimit:  5,
				blockJumpLimit: 10,
			},
		},
		{
			name:               "provider_with_block_lag",
			currentBlock:       90,
			latestNetworkBlock: 100,
			initialStatus:      Healthy,
			chainIDValid:       true,
			archiveSupported:   false,
			expectedStatus:     Warning,
			networkConfig: struct {
				blockLagLimit  int64
				blockJumpLimit int64
			}{
				blockLagLimit:  5, // Lagging by 10, exceeds limit of 5
				blockJumpLimit: 10,
			},
		},
		{
			name:               "provider_with_block_jump",
			currentBlock:       115,
			latestNetworkBlock: 100,
			initialStatus:      Healthy,
			chainIDValid:       true,
			archiveSupported:   false,
			expectedStatus:     Unhealthy,
			networkConfig: struct {
				blockLagLimit  int64
				blockJumpLimit int64
			}{
				blockLagLimit:  5,
				blockJumpLimit: 10, // Ahead by 15, exceeds limit of 10
			},
		},
		{
			name:               "provider_wrong_chain_id",
			currentBlock:       100,
			latestNetworkBlock: 100,
			initialStatus:      Healthy,
			chainIDValid:       false,
			archiveSupported:   false,
			expectedStatus:     Unhealthy,
			networkConfig: struct {
				blockLagLimit  int64
				blockJumpLimit int64
			}{
				blockLagLimit:  5,
				blockJumpLimit: 10,
			},
		},
		{
			name:               "provider_stalled",
			currentBlock:       95,
			latestNetworkBlock: 100,
			initialStatus:      Healthy,
			providerHistory: []blockHistoryEntry{
				{blockNumber: 95, healthStatus: Healthy},
				{blockNumber: 95, healthStatus: Healthy},
				{blockNumber: 95, healthStatus: Healthy},
				{blockNumber: 95, healthStatus: Healthy},
				{blockNumber: 95, healthStatus: Healthy},
			},
			chainIDValid:     true,
			archiveSupported: false,
			expectedStatus:   Warning,
			networkConfig: struct {
				blockLagLimit  int64
				blockJumpLimit int64
			}{
				blockLagLimit:  10,
				blockJumpLimit: 10,
			},
		},
		{
			name:               "provider_stalled_and_lagged",
			currentBlock:       85,
			latestNetworkBlock: 100,
			initialStatus:      Healthy,
			providerHistory: []blockHistoryEntry{
				{blockNumber: 85, healthStatus: Healthy},
				{blockNumber: 85, healthStatus: Healthy},
				{blockNumber: 85, healthStatus: Healthy},
				{blockNumber: 85, healthStatus: Healthy},
				{blockNumber: 85, healthStatus: Healthy},
			},
			chainIDValid:     true,
			archiveSupported: false,
			expectedStatus:   Unhealthy,
			networkConfig: struct {
				blockLagLimit  int64
				blockJumpLimit int64
			}{
				blockLagLimit:  10, // Lagging by 15, exceeds limit
				blockJumpLimit: 20,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock handler
			mockHandler := networklib.NewMockNetworkHandler(ctrl)

			// Create network
			n, err := NewNetwork("test-network", networklib.EVMHandlerType, utils.Environment("test"), "")
			assert.NoError(t, err)

			// Configure network
			n.ChainId = "0x1"
			n.BlockLagLimit = tt.networkConfig.blockLagLimit
			n.BlockJumpLimit = tt.networkConfig.blockJumpLimit
			n.ProviderBlockHistorySize = 5
			n.RequestAttemptCount = 1
			n.ArchiveEnabled = false

			// Set dependencies
			n.logger = logger.NewLoggerClient(zap.NewNop(), utils.EnvTest)
			n.handler = mockHandler

			// Create provider
			provider, err := NewProvider("http://test-provider.com")
			assert.NoError(t, err)

			// Pre-populate history if specified
			if len(tt.providerHistory) > 0 {
				for _, entry := range tt.providerHistory {
					provider.AddBlockEntry(entry.blockNumber, entry.healthStatus, n.ProviderBlockHistorySize)
				}
			} else {
				// Add at least one entry for the test
				provider.AddBlockEntry(tt.currentBlock-1, Healthy, n.ProviderBlockHistorySize)
			}

			// Create a second provider if testing block jump (need other healthy providers)
			// or if testing stalled provider (need other non-stalled providers)
			if tt.currentBlock > tt.latestNetworkBlock+tt.networkConfig.blockJumpLimit ||
				(tt.name == "provider_stalled") {
				otherProvider := createHealthyProvider(tt.latestNetworkBlock)
				n.Providers["test-provider"] = provider
				n.Providers["other-provider"] = otherProvider
			} else {
				n.Providers["test-provider"] = provider
			}

			// Setup mocks for chain ID validation
			if tt.chainIDValid {
				mockHandler.EXPECT().GetChainID(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("0x1", nil).AnyTimes()
				mockHandler.EXPECT().ValidateChainID("0x1").Return(nil).AnyTimes()
			} else {
				mockHandler.EXPECT().GetChainID(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("0x5", nil).AnyTimes()
				mockHandler.EXPECT().ValidateChainID("0x5").Return(errors.New("invalid chain ID")).AnyTimes()
			}

			// Setup mocks for archive mode
			mockHandler.EXPECT().SupportsArchiveMode().Return(tt.archiveSupported).AnyTimes()

			// Call the method under test
			result := n.evaluateProviderHealth(provider, tt.currentBlock, tt.initialStatus, tt.latestNetworkBlock)

			// Verify the result
			assert.Equal(t, tt.expectedStatus, result,
				"Expected status %s but got %s", tt.expectedStatus.String(), result.String())
		})
	}
}

// TestHandleErrorWithGracePeriod is already in network_test.go
// TestIsStalled is already in network_test.go
// TestGetLatestHealthyBlock is already in network_test.go

// Helper function to create a healthy provider
func createHealthyProvider(blockNumber int64) *provider {
	p, _ := NewProvider("http://other-provider.com")
	p.AddBlockEntry(blockNumber, Healthy, 5)
	return p
}
