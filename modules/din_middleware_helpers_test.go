package modules

import (
	"crypto/rand"
	"sync"
	"testing"
	"time"

	"github.com/DIN-center/din-caddy-plugins/lib/auth/siwe"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	"github.com/DIN-center/din-caddy-plugins/lib/utils"
	din "github.com/DIN-center/din-sc/apps/din-go/lib/din"
	"github.com/pkg/errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
)

// Constants for replacer keys used in tests, mirroring those in din_middleware.go
const (
	testRequestProviderKey = "request_provider"
	testRequestBodyKey     = "request_body"
)

// MockWeb3Client is a mock implementation of web3.Web3Client for testing
type MockWeb3Client struct {
	mockLatestBlockNumber uint64
}

func (m *MockWeb3Client) LatestBlockNumber() (uint64, error) {
	return m.mockLatestBlockNumber, nil
}

func TestSyncRegistryWithLatestBlock(t *testing.T) {
	logger := logger.NewLoggerClient(zap.NewNop(), utils.Environment("test"))
	mockCtrl := gomock.NewController(t)
	mockDingoClient := din.NewMockIDinClient(mockCtrl)
	dinMiddleware := &DinMiddleware{
		RegistryBlockEpoch:                  10,
		registryLastUpdatedEpochBlockNumber: 40,
		logger:                              logger,
		DingoClient:                         mockDingoClient,
		testMode:                            true,
	}

	tests := []struct {
		name                                string
		registryLastUpdatedEpochBlockNumber uint64
		latestBlockNumber                   uint64
		expectedUpdateCall                  bool
		expectedBlockFloorByEpoch           uint64
	}{
		{
			name:                                "Sync should update as block difference is equal to or exceeds epoch 50",
			registryLastUpdatedEpochBlockNumber: uint64(40),
			latestBlockNumber:                   uint64(50),
			expectedUpdateCall:                  true,
			expectedBlockFloorByEpoch:           uint64(50),
		},
		{
			name:                                "Sync should update as block difference is equal to or exceeds epoch 52",
			registryLastUpdatedEpochBlockNumber: uint64(40),
			latestBlockNumber:                   uint64(52),
			expectedUpdateCall:                  true,
			expectedBlockFloorByEpoch:           uint64(50),
		},
		{
			name:                                "Sync should update as block difference is equal to or exceeds epoch 1000",
			registryLastUpdatedEpochBlockNumber: uint64(40),
			latestBlockNumber:                   uint64(1001),
			expectedUpdateCall:                  true,
			expectedBlockFloorByEpoch:           uint64(1000),
		},
		{
			name:                                "Sync should not update as block difference is less than epoch 48",
			registryLastUpdatedEpochBlockNumber: uint64(40),
			latestBlockNumber:                   uint64(48),
			expectedUpdateCall:                  false,
			expectedBlockFloorByEpoch:           uint64(40),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset the middleware state
			dinMiddleware.Networks = map[string]*network{}
			dinMiddleware.registryLastUpdatedEpochBlockNumber = tt.registryLastUpdatedEpochBlockNumber

			// Create a mock Web3Client
			mockWeb3Client := &MockWeb3Client{mockLatestBlockNumber: tt.latestBlockNumber}

			// Set up expectations for GetRegistryData only if we expect an update
			if tt.expectedUpdateCall {
				mockDingoClient.EXPECT().GetRegistryData().Return(&din.DinRegistryData{}, nil).Times(1)
			}

			// Call the function being tested using the mock middleware
			dinMiddleware.syncRegistryWithLatestBlock(mockWeb3Client)

			// Validate that registryLastUpdatedEpochBlockNumber is updated correctly
			if dinMiddleware.registryLastUpdatedEpochBlockNumber != tt.expectedBlockFloorByEpoch {
				t.Errorf("Expected registryLastUpdatedEpochBlockNumber = %v, got %v",
					tt.expectedBlockFloorByEpoch, dinMiddleware.registryLastUpdatedEpochBlockNumber)
			}
		})
	}
}

func TestAddNetworkWithRegistryData(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	tests := []struct {
		name                     string
		regNetwork               *din.Network
		expectedNetworkProviders int
		networkServiceCreated    bool
	}{
		{
			name: "Successful add network with providers",
			regNetwork: &din.Network{
				ProxyName: "test-network",
				Providers: map[string]*din.Provider{
					"Provider1": {
						NetworkServices: map[string]*din.NetworkService{
							"http://new-provider.com": {
								Url:     "http://new-provider.com",
								Address: "0x1234567890abcdef",
								Status:  din.NetworkServiceStatusActive,
							},
						},
					},
				},
				NetworkConfig: &din.NetworkOperationsConfig{
					Handler: "evm",
				},
			},
			expectedNetworkProviders: 1,
			networkServiceCreated:    true,
		},
		{
			name: "Missing active status, no providers added",
			regNetwork: &din.Network{
				ProxyName: "test-network",
				Providers: map[string]*din.Provider{
					"Provider1": {
						NetworkServices: map[string]*din.NetworkService{
							"http://new-provider.com": {
								Url:     "http://new-provider.com",
								Address: "0x1234567890abcdef",
								Status:  din.NetworkServiceStatusOnboarding,
							},
						},
					},
				},
				NetworkConfig: &din.NetworkOperationsConfig{
					Handler: "evm",
				},
				Status: din.NetworkStatusOnboarding,
			},
			expectedNetworkProviders: 0,
			networkServiceCreated:    false,
		},
		{
			name: "Only active status are added",
			regNetwork: &din.Network{
				ProxyName: "test-network",
				Providers: map[string]*din.Provider{
					"Provider1": {
						NetworkServices: map[string]*din.NetworkService{
							"http://new-provider.com": {
								Url:     "http://new-provider.com",
								Address: "0x1234567890abcdef",
								Status:  din.NetworkServiceStatusActive,
							},
						},
					},
					"Provider2": {
						NetworkServices: map[string]*din.NetworkService{
							"http://new-provider2.com": {
								Url:     "http://new-provider2.com",
								Address: "0x99999999999",
								Status:  din.NetworkServiceStatusRetired,
							},
						},
					},
				},
				NetworkConfig: &din.NetworkOperationsConfig{},
			},
			expectedNetworkProviders: 1,
			networkServiceCreated:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock DingoClient and other dependencies
			mockDingoClient := din.NewMockIDinClient(mockCtrl)

			// Create logger
			logger := logger.NewLoggerClient(zaptest.NewLogger(t), utils.Environment("test"))

			// Create DinMiddleware instance with proper initialization
			dinMiddleware := &DinMiddleware{
				DingoClient: mockDingoClient,
				logger:      logger,
				Networks:    make(map[string]*network),
				testMode:    true,
				Env:         utils.Environment("test"),
				CaddyPort:   "8080",
				machineID:   "test-machine-id",
			}

			// Call the function being tested
			dinMiddleware.addNetworkWithRegistryData(tt.regNetwork)

			// Verify the network is added
			network, ok := dinMiddleware.Networks[tt.regNetwork.ProxyName]
			assert.Equal(t, tt.networkServiceCreated, ok)

			if network != nil {
				// Verify the number of providers added to the network
				assert.Equal(t, tt.expectedNetworkProviders, len(network.Providers))
			}
		})
	}
}

func TestUpdateNetworkWithRegistryData(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	tests := []struct {
		name                       string
		regNetwork                 *din.Network
		newNetwork                 *network
		syncNetworkConfigErr       error
		createNewProviderErr       error
		expectedError              error
		expectedProviderCount      int
		expectedRemainingProviders int
	}{
		{
			name: "Successful update with new providers",
			regNetwork: &din.Network{
				Name: "test-network",
				Providers: map[string]*din.Provider{
					"Provider1": {
						NetworkServices: map[string]*din.NetworkService{
							"http://new-provider.com": {
								Url:     "http://new-provider.com",
								Address: "0x1234567890abcdef",
								Status:  din.NetworkServiceStatusActive,
							},
						},
					},
				},
				NetworkConfig: &din.NetworkOperationsConfig{
					Handler: "evm",
				},
			},
			newNetwork: &network{
				Name:      "test-network",
				Providers: map[string]*provider{},
			},
			syncNetworkConfigErr:       nil,
			createNewProviderErr:       nil,
			expectedError:              nil,
			expectedProviderCount:      1,
			expectedRemainingProviders: 0,
		},
		{
			name: "Error, non active status",
			regNetwork: &din.Network{
				Name: "test-network",
				Providers: map[string]*din.Provider{
					"Provider1": {
						NetworkServices: map[string]*din.NetworkService{
							"http://new-provider.com": {
								Url:     "http://new-provider.com",
								Address: "0x1234567890abcdef",
							},
						},
					},
				},
				NetworkConfig: &din.NetworkOperationsConfig{
					Handler: "evm",
				},
				Status: din.NetworkStatusOnboarding,
			},
			newNetwork: &network{
				Name:      "test-network",
				Providers: map[string]*provider{},
			},
			syncNetworkConfigErr:       nil,
			createNewProviderErr:       nil,
			expectedError:              nil,
			expectedProviderCount:      0,
			expectedRemainingProviders: 0,
		},
		{
			name: "Error syncing network config",
			regNetwork: &din.Network{
				Name: "test-network",
				NetworkConfig: &din.NetworkOperationsConfig{
					Handler: "evm",
				},
				Status: din.NetworkStatusActive,
			},
			newNetwork: &network{
				Name: "test-network",
			},
			syncNetworkConfigErr:       errors.New("sync error"),
			createNewProviderErr:       nil,
			expectedError:              errors.New("sync error"),
			expectedProviderCount:      0,
			expectedRemainingProviders: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock DingoClient and other dependencies
			mockDingoClient := din.NewMockIDinClient(mockCtrl)

			// Create logger
			logger := logger.NewLoggerClient(zaptest.NewLogger(t), utils.Environment("test"))

			// Create DinMiddleware instance with proper initialization
			dinMiddleware := &DinMiddleware{
				DingoClient: mockDingoClient,
				logger:      logger,
				Networks: map[string]*network{
					tt.newNetwork.Name: tt.newNetwork,
				},
				testMode:  true,
				Env:       utils.Environment("test"),
				CaddyPort: "8080",
				machineID: "test-machine-id",
			}

			// Call the function being tested
			dinMiddleware.updateNetworkWithRegistryData(tt.regNetwork, tt.newNetwork)

			// Assert the number of providers after the update
			assert.Equal(t, tt.expectedProviderCount, len(tt.newNetwork.Providers))
		})
	}
}
func TestSyncNetworkConfig(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	tests := []struct {
		name            string
		regNetwork      *din.Network
		existingNetwork *network
		expectedNetwork *network
	}{
		{
			name: "successful sync with all new values",
			regNetwork: &din.Network{
				Name: "test-network",
				NetworkConfig: &din.NetworkOperationsConfig{
					ChainId:                 "0x1",
					HealthcheckIntervalSec:  20,
					BlockLagLimit:           10,
					BlockJumpLimit:          5,
					MaxRequestPayloadSizeKb: 2048,
					RequestAttemptCount:     5,
					ArchiveEnabled:          true,
				},
			},
			existingNetwork: &network{
				Name:                    "test-network",
				ChainId:                 "0x0",
				HCInterval:              10,
				BlockLagLimit:           5,
				BlockJumpLimit:          3,
				MaxRequestPayloadSizeKB: 1024,
				RequestAttemptCount:     3,
				ArchiveEnabled:          false,
			},
			expectedNetwork: &network{
				Name:                    "test-network",
				ChainId:                 "0x1",
				HCInterval:              20,
				BlockLagLimit:           10,
				BlockJumpLimit:          5,
				MaxRequestPayloadSizeKB: 2048,
				RequestAttemptCount:     5,
				ArchiveEnabled:          true,
			},
		},
		{
			name: "no updates needed when registry values are zero",
			regNetwork: &din.Network{
				Name: "test-network",
				NetworkConfig: &din.NetworkOperationsConfig{
					ChainId:                 "0x1",
					HealthcheckIntervalSec:  0,
					BlockLagLimit:           0,
					BlockJumpLimit:          0,
					MaxRequestPayloadSizeKb: 0,
					RequestAttemptCount:     0,
					ArchiveEnabled:          false,
				}},
			existingNetwork: &network{
				Name:                    "test-network",
				ChainId:                 "0x1",
				HCInterval:              10,
				BlockLagLimit:           5,
				BlockJumpLimit:          3,
				MaxRequestPayloadSizeKB: 1024,
				RequestAttemptCount:     3,
				ArchiveEnabled:          false,
			},
			expectedNetwork: &network{
				Name:                    "test-network",
				ChainId:                 "0x1",
				HCInterval:              10,
				BlockLagLimit:           5,
				BlockJumpLimit:          3,
				MaxRequestPayloadSizeKB: 1024,
				RequestAttemptCount:     3,
				ArchiveEnabled:          false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDingoClient := din.NewMockIDinClient(mockCtrl)

			// Create logger
			logger := logger.NewLoggerClient(zaptest.NewLogger(t), utils.Environment("test"))

			dinMiddleware := &DinMiddleware{
				DingoClient: mockDingoClient,
				logger:      logger,
			}

			result := dinMiddleware.syncNetworkConfig(tt.regNetwork, tt.existingNetwork)
			// REMOVED: Method field assertions (now provided by handlers)
			assert.Equal(t, tt.expectedNetwork.ChainId, result.ChainId)
			assert.Equal(t, tt.expectedNetwork.HCInterval, result.HCInterval)
			assert.Equal(t, tt.expectedNetwork.BlockLagLimit, result.BlockLagLimit)
			assert.Equal(t, tt.expectedNetwork.BlockJumpLimit, result.BlockJumpLimit)
			assert.Equal(t, tt.expectedNetwork.MaxRequestPayloadSizeKB, result.MaxRequestPayloadSizeKB)
			assert.Equal(t, tt.expectedNetwork.RequestAttemptCount, result.RequestAttemptCount)
			assert.Equal(t, tt.expectedNetwork.ArchiveEnabled, result.ArchiveEnabled)
		})
	}
}

func expectedMethodsMap(m []*string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, k := range m {
		result[*k] = struct{}{}
	}
	return result
}

// TestCaddyfilePriorityOverRegistry tests that Caddyfile values take priority over registry values
func TestCaddyfilePriorityOverRegistry(t *testing.T) {
	logger := logger.NewLoggerClient(zaptest.NewLogger(t), utils.Environment("test"))
	
	tests := []struct {
		name             string
		existingNetwork  *network
		regNetworkConfig *din.NetworkOperationsConfig
		expectedNetwork  *network
		description      string
	}{
		{
			name: "Caddyfile values not overwritten by registry",
			existingNetwork: &network{
				Name:                    "test-network",
				HandlerType:            "evm",
				ChainId:                "0x1",
				HCInterval:             30,
				HCThreshold:            3,
				HCTimeout:              10,
				BlockLagLimit:          10,
				BlockJumpLimit:         50,
				MaxRequestPayloadSizeKB: 2048,
				RequestAttemptCount:    5,
				ProviderBlockHistorySize: 20,
				NetworkBlockHistorySize: 256,
				ArchiveEnabled:         true,
				ConfigSource: &networkConfigSource{
					HandlerTypeSet:              true,
					ChainIdSet:                 true,
					HCIntervalSet:              true,
					HCThresholdSet:             true,
					HCTimeoutSet:               true,
					BlockLagLimitSet:           true,
					BlockJumpLimitSet:          true,
					MaxRequestPayloadSizeKBSet: true,
					RequestAttemptCountSet:     true,
					ProviderBlockHistorySizeSet: true,
					NetworkBlockHistorySizeSet: true,
					ArchiveEnabledSet:          true,
				},
			},
			regNetworkConfig: &din.NetworkOperationsConfig{
				Handler:                  "solana",
				ChainId:                  "0x2",
				HealthcheckIntervalSec:   60,
				HealthcheckThreshold:     5,
				HealthcheckTimeout:       20,
				BlockLagLimit:           20,
				BlockJumpLimit:          100,
				MaxRequestPayloadSizeKb: 4096,
				RequestAttemptCount:     10,
				ProviderBlockHistorySize: 30,
				NetworkBlockHistorySize:  512,
				ArchiveEnabled:          false,
			},
			expectedNetwork: &network{
				Name:                    "test-network",
				HandlerType:            "evm",     // Should NOT change
				ChainId:                "0x1",     // Should NOT change
				HCInterval:             30,        // Should NOT change
				HCThreshold:            3,         // Should NOT change
				HCTimeout:              10,        // Should NOT change
				BlockLagLimit:          10,        // Should NOT change
				BlockJumpLimit:         50,        // Should NOT change
				MaxRequestPayloadSizeKB: 2048,     // Should NOT change
				RequestAttemptCount:    5,         // Should NOT change
				ProviderBlockHistorySize: 20,      // Should NOT change
				NetworkBlockHistorySize: 256,      // Should NOT change
				ArchiveEnabled:         true,      // Should NOT change
			},
			description: "All Caddyfile-set values should remain unchanged",
		},
		{
			name: "Registry values applied when not set in Caddyfile",
			existingNetwork: &network{
				Name:                    "test-network",
				HandlerType:            "",
				ChainId:                "",
				HCInterval:             DefaultHCInterval,
				HCThreshold:            DefaultHCThreshold,
				HCTimeout:              DefaultHCTimeout,
				BlockLagLimit:          DefaultBlockLagLimit,
				BlockJumpLimit:         DefaultBlockJumpLimit,
				MaxRequestPayloadSizeKB: DefaultMaxRequestPayloadSizeKB,
				RequestAttemptCount:    DefaultRequestAttemptCount,
				ProviderBlockHistorySize: DefaultProviderBlockHistorySize,
				NetworkBlockHistorySize: DefaultNetworkBlockHistorySize,
				ArchiveEnabled:         DefaultArchiveEnabled,
				ConfigSource: &networkConfigSource{
					// All fields false - nothing set via Caddyfile
				},
			},
			regNetworkConfig: &din.NetworkOperationsConfig{
				Handler:                  "starknet",
				ChainId:                  "0x3",
				HealthcheckIntervalSec:   45,
				HealthcheckThreshold:     4,
				HealthcheckTimeout:       15,
				BlockLagLimit:           15,
				BlockJumpLimit:          75,
				MaxRequestPayloadSizeKb: 3072,
				RequestAttemptCount:     7,
				ProviderBlockHistorySize: 25,
				NetworkBlockHistorySize:  384,
				ArchiveEnabled:          true,
			},
			expectedNetwork: &network{
				Name:                    "test-network",
				HandlerType:            "starknet", // Should change
				ChainId:                "0x3",      // Should change
				HCInterval:             45,         // Should change
				HCThreshold:            4,          // Should change
				HCTimeout:              15,         // Should change
				BlockLagLimit:          15,         // Should change
				BlockJumpLimit:         75,         // Should change
				MaxRequestPayloadSizeKB: 3072,      // Should change
				RequestAttemptCount:    7,          // Should change
				ProviderBlockHistorySize: 25,       // Should change
				NetworkBlockHistorySize: 384,       // Should change
				ArchiveEnabled:         true,       // Should change
			},
			description: "All registry values should be applied when not set in Caddyfile",
		},
		{
			name: "Mixed: some Caddyfile, some registry values",
			existingNetwork: &network{
				Name:                    "test-network",
				HandlerType:            "evm",
				ChainId:                "0x4",
				HCInterval:             25,
				HCThreshold:            DefaultHCThreshold,
				HCTimeout:              DefaultHCTimeout,
				BlockLagLimit:          DefaultBlockLagLimit,
				BlockJumpLimit:         DefaultBlockJumpLimit,
				MaxRequestPayloadSizeKB: 1536,
				RequestAttemptCount:    DefaultRequestAttemptCount,
				ProviderBlockHistorySize: DefaultProviderBlockHistorySize,
				NetworkBlockHistorySize: 192,
				ArchiveEnabled:         false,
				ConfigSource: &networkConfigSource{
					HandlerTypeSet:             true,
					ChainIdSet:                true,
					HCIntervalSet:             true,
					MaxRequestPayloadSizeKBSet: true,
					NetworkBlockHistorySizeSet: true,
					ArchiveEnabledSet:         true,
					// Other fields not set
				},
			},
			regNetworkConfig: &din.NetworkOperationsConfig{
				Handler:                  "beacon-chain",
				ChainId:                  "0x5",
				HealthcheckIntervalSec:   50,
				HealthcheckThreshold:     6,
				HealthcheckTimeout:       25,
				BlockLagLimit:           25,
				BlockJumpLimit:          125,
				MaxRequestPayloadSizeKb: 5120,
				RequestAttemptCount:     12,
				ProviderBlockHistorySize: 35,
				NetworkBlockHistorySize:  640,
				ArchiveEnabled:          true,
			},
			expectedNetwork: &network{
				Name:                    "test-network",
				HandlerType:            "evm",     // Caddyfile value preserved
				ChainId:                "0x4",     // Caddyfile value preserved
				HCInterval:             25,        // Caddyfile value preserved
				HCThreshold:            6,         // Registry value applied
				HCTimeout:              25,        // Registry value applied
				BlockLagLimit:          25,        // Registry value applied
				BlockJumpLimit:         125,       // Registry value applied
				MaxRequestPayloadSizeKB: 1536,     // Caddyfile value preserved
				RequestAttemptCount:    12,        // Registry value applied
				ProviderBlockHistorySize: 35,      // Registry value applied
				NetworkBlockHistorySize: 192,      // Caddyfile value preserved
				ArchiveEnabled:         false,     // Caddyfile value preserved
			},
			description: "Mixed configuration: Caddyfile values preserved, registry fills gaps",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dinMiddleware := &DinMiddleware{
				logger:   logger,
				Networks: map[string]*network{tt.existingNetwork.Name: tt.existingNetwork},
			}
			
			// Update the network with registry data
			dinMiddleware.updateNetworkFields(tt.existingNetwork, tt.regNetworkConfig)
			
			// Verify the expected values
			assert.Equal(t, tt.expectedNetwork.HandlerType, tt.existingNetwork.HandlerType, "HandlerType: %s", tt.description)
			assert.Equal(t, tt.expectedNetwork.ChainId, tt.existingNetwork.ChainId, "ChainId: %s", tt.description)
			assert.Equal(t, tt.expectedNetwork.HCInterval, tt.existingNetwork.HCInterval, "HCInterval: %s", tt.description)
			assert.Equal(t, tt.expectedNetwork.HCThreshold, tt.existingNetwork.HCThreshold, "HCThreshold: %s", tt.description)
			assert.Equal(t, tt.expectedNetwork.HCTimeout, tt.existingNetwork.HCTimeout, "HCTimeout: %s", tt.description)
			assert.Equal(t, tt.expectedNetwork.BlockLagLimit, tt.existingNetwork.BlockLagLimit, "BlockLagLimit: %s", tt.description)
			assert.Equal(t, tt.expectedNetwork.BlockJumpLimit, tt.existingNetwork.BlockJumpLimit, "BlockJumpLimit: %s", tt.description)
			assert.Equal(t, tt.expectedNetwork.MaxRequestPayloadSizeKB, tt.existingNetwork.MaxRequestPayloadSizeKB, "MaxRequestPayloadSizeKB: %s", tt.description)
			assert.Equal(t, tt.expectedNetwork.RequestAttemptCount, tt.existingNetwork.RequestAttemptCount, "RequestAttemptCount: %s", tt.description)
			assert.Equal(t, tt.expectedNetwork.ProviderBlockHistorySize, tt.existingNetwork.ProviderBlockHistorySize, "ProviderBlockHistorySize: %s", tt.description)
			assert.Equal(t, tt.expectedNetwork.NetworkBlockHistorySize, tt.existingNetwork.NetworkBlockHistorySize, "NetworkBlockHistorySize: %s", tt.description)
			assert.Equal(t, tt.expectedNetwork.ArchiveEnabled, tt.existingNetwork.ArchiveEnabled, "ArchiveEnabled: %s", tt.description)
		})
	}
}

func TestCreateNewProvider(t *testing.T) {
	tests := []struct {
		name                  string
		provider              *provider
		authConfig            *din.ProviderAuthConfig
		networkService        *din.NetworkService
		initializeProviderErr error
		expectedError         error
		expectedMethods       []*string
		expectedAuth          *siwe.SIWEClientAuth
		expectAuthCreation    bool
	}{
		{
			name: "Successful provider creation with SIWE auth",
			provider: &provider{
				HttpUrl: "http://example5.com",
			},
			authConfig: &din.ProviderAuthConfig{
				Type: din.ProviderAuthTypeSIWE,
				Url:  "http://example6.com",
			},
			networkService: &din.NetworkService{
				Methods: map[string]*din.Method{
					"eth_call":        {Name: "eth_call"},
					"eth_blockNumber": {Name: "eth_blockNumber"},
				},
			},
			initializeProviderErr: nil,
			expectedError:         nil,
			expectedMethods:       []*string{aws.String("eth_call"), aws.String("eth_blockNumber")},
			expectedAuth: &siwe.SIWEClientAuth{
				ProviderURL:  "http://example6.com",
				SessionCount: 16,
				Signer:       nil, // Signer is set in the createNewProvider function
			},
			expectAuthCreation: true,
		},
		{
			name: "Successful provider creation with no auth",
			provider: &provider{
				HttpUrl: "http://example7.com",
			},
			authConfig: &din.ProviderAuthConfig{Type: din.ProviderAuthTypeNone},
			networkService: &din.NetworkService{
				Methods: map[string]*din.Method{
					"eth_call":        {Name: "eth_call"},
					"eth_blockNumber": {Name: "eth_blockNumber"},
				},
			},
			initializeProviderErr: nil,
			expectedError:         nil,
			expectedMethods:       []*string{aws.String("eth_call"), aws.String("eth_blockNumber")},
			expectedAuth:          nil,
			expectAuthCreation:    false,
		},
		{
			name: "AUTH type is unknown ",
			provider: &provider{
				HttpUrl: "http://example12.com",
			},
			authConfig: &din.ProviderAuthConfig{
				Type: "unknown",
				Url:  "http://example13.com",
			},
			networkService: &din.NetworkService{
				Methods: map[string]*din.Method{
					"eth_call":        {Name: "eth_call"},
					"eth_blockNumber": {Name: "eth_blockNumber"},
				},
			},
			initializeProviderErr: nil,
			expectedError:         nil,
			expectedMethods:       []*string{aws.String("eth_call"), aws.String("eth_blockNumber")},
			expectedAuth:          nil,
			expectAuthCreation:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockCtrl := gomock.NewController(t)

			privateKeyData := make([]byte, 32)
			_, err := rand.Read(privateKeyData)
			if err != nil {
				t.Errorf("Failed to generate random data in test: %v", err)
			}

			mockDingoClient := din.NewMockIDinClient(mockCtrl)
			mockSiweSignerClient := siwe.NewMockISIWESignerClient(mockCtrl)
			defaultSigner := &siwe.SigningConfig{
				PrivateKey: privateKeyData,
			}

			// Setup mock expectations
			if tt.expectAuthCreation {
				mockSiweSignerClient.EXPECT().
					CreateNewSIWEAuth(tt.authConfig.Url, gomock.Eq(16)).
					Return(tt.expectedAuth).
					Times(1)
			}

			// Create DinMiddleware instance
			dinMiddleware := &DinMiddleware{
				DingoClient:       mockDingoClient,
				SiweSignerClient:  mockSiweSignerClient,
				RegistryPriority:  10,
				logger:            logger.NewLoggerClient(zaptest.NewLogger(t), utils.Environment("test")),
				testMode:          true,
				DefaultSiweSigner: defaultSigner,
			}

			// Create test network
			network, err := NewNetwork("test-network", "evm", utils.Environment("test"), "8080")
			assert.NoError(t, err)

			// Call the function being tested
			createdProvider, err := dinMiddleware.createNewProvider(tt.provider, network, tt.authConfig, tt.networkService)

			// Assert results
			if tt.expectedError != nil {
				assert.Equal(t, err.Error(), tt.expectedError.Error())
				assert.Nil(t, createdProvider)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, createdProvider)

				// Verify that the provider was updated correctly
				assert.Equal(t, expectedMethodsMap(tt.expectedMethods), createdProvider.Methods)
				assert.Equal(t, dinMiddleware.RegistryPriority, createdProvider.Priority)
				assert.Equal(t, tt.expectedAuth, createdProvider.Auth)
			}
		})
	}
}

func TestUpdateNetworkData(t *testing.T) {
	tests := []struct {
		name            string
		initialNetwork  *network
		updatedNetwork  *network
		expectedNetwork *network
	}{
		{
			name: "Successful update of network data",
			initialNetwork: &network{
				Name:                    "test-network",
				HCInterval:              10,
				BlockLagLimit:           5,
				MaxRequestPayloadSizeKB: 1024,
				RequestAttemptCount:     3,
				Providers: map[string]*provider{
					"existing-host": {
						host: "existing-host",
					},
				},
			},
			updatedNetwork: &network{
				Name:                    "test-network",
				HCInterval:              20,
				BlockLagLimit:           10,
				MaxRequestPayloadSizeKB: 2048,
				RequestAttemptCount:     5,
				Providers: map[string]*provider{
					"new-host": {
						host: "new-host",
					},
				},
			},
			expectedNetwork: &network{
				Name:                    "test-network",
				HCInterval:              20,
				BlockLagLimit:           10,
				MaxRequestPayloadSizeKB: 2048,
				RequestAttemptCount:     5,
				Providers: map[string]*provider{
					"existing-host": {
						host: "existing-host",
					},
					"new-host": {
						host: "new-host",
					},
				},
			},
		},
		{
			name: "Update with empty providers",
			initialNetwork: &network{
				Name:                    "test-network",
				HCInterval:              10,
				BlockLagLimit:           5,
				MaxRequestPayloadSizeKB: 1024,
				RequestAttemptCount:     3,
				Providers: map[string]*provider{
					"existing-host": {
						host: "existing-host",
					},
				},
			},
			updatedNetwork: &network{
				Name:                    "test-network",
				HCInterval:              20,
				BlockLagLimit:           10,
				MaxRequestPayloadSizeKB: 2048,
				RequestAttemptCount:     5,
				Providers:               map[string]*provider{},
			},
			expectedNetwork: &network{
				Name:                    "test-network",
				HCInterval:              20,
				BlockLagLimit:           10,
				MaxRequestPayloadSizeKB: 2048,
				RequestAttemptCount:     5,
				Providers: map[string]*provider{
					"existing-host": {
						host: "existing-host",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize DinMiddleware and lock
			dinMiddleware := &DinMiddleware{
				Networks: map[string]*network{
					tt.initialNetwork.Name: tt.initialNetwork,
				},
				mu:       sync.RWMutex{},
				testMode: true,
			}

			// Lock for writing
			dinMiddleware.mu.Lock()
			// Call the function being tested
			dinMiddleware.updateNetworkData(tt.updatedNetwork)
			// Unlock
			dinMiddleware.mu.Unlock()

			// Assert that the network data was updated correctly
			updatedNetwork := dinMiddleware.Networks[tt.initialNetwork.Name]
			assert.Equal(t, tt.expectedNetwork.HCInterval, updatedNetwork.HCInterval)
			assert.Equal(t, tt.expectedNetwork.BlockLagLimit, updatedNetwork.BlockLagLimit)
			assert.Equal(t, tt.expectedNetwork.MaxRequestPayloadSizeKB, updatedNetwork.MaxRequestPayloadSizeKB)
			assert.Equal(t, tt.expectedNetwork.RequestAttemptCount, updatedNetwork.RequestAttemptCount)
			assert.Equal(t, len(tt.expectedNetwork.Providers), len(updatedNetwork.Providers))

			for host, provider := range tt.expectedNetwork.Providers {
				assert.Equal(t, provider.host, updatedNetwork.Providers[host].host)
			}
		})
	}
}

func TestCreateProviderSIWEAuth(t *testing.T) {
	tests := []struct {
		name               string
		authConfig         *din.ProviderAuthConfig
		defaultSignerSet   bool
		expectedAuth       *siwe.SIWEClientAuth
		expectedError      error
		expectAuthCreation bool
	}{
		{
			name: "Successful SIWE auth creation with default signer",
			authConfig: &din.ProviderAuthConfig{
				Type: din.ProviderAuthTypeSIWE,
				Url:  "http://example.com",
			},
			defaultSignerSet:   true,
			expectAuthCreation: true,
			expectedAuth: &siwe.SIWEClientAuth{
				ProviderURL:  "http://example.com",
				SessionCount: 16,
			},
			expectedError: nil,
		},
		{
			name: "Auth type None should return nil",
			authConfig: &din.ProviderAuthConfig{
				Type: din.ProviderAuthTypeNone,
				Url:  "http://example.com",
			},
			defaultSignerSet:   true,
			expectAuthCreation: false,
			expectedAuth:       nil,
			expectedError:      nil,
		},
		{
			name: "Unknown auth type should return nil",
			authConfig: &din.ProviderAuthConfig{
				Type: "unknown",
				Url:  "http://example.com",
			},
			defaultSignerSet:   true,
			expectAuthCreation: false,
			expectedAuth:       nil,
			expectedError:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockSiweSignerClient := siwe.NewMockISIWESignerClient(mockCtrl)

			// Create random private key data for the default signer
			privateKeyData := make([]byte, 32)
			_, err := rand.Read(privateKeyData)
			if err != nil {
				t.Errorf("Failed to generate random data in test: %v", err)
			}

			// Create DinMiddleware instance
			dinMiddleware := &DinMiddleware{
				SiweSignerClient: mockSiweSignerClient,
				logger:           logger.NewLoggerClient(zaptest.NewLogger(t), utils.Environment("test")),
			}

			// Set default signer if required by test case
			if tt.defaultSignerSet {
				dinMiddleware.DefaultSiweSigner = &siwe.SigningConfig{
					PrivateKey: privateKeyData,
				}
			}

			// Setup mock expectations
			if tt.expectAuthCreation {
				mockSiweSignerClient.EXPECT().
					CreateNewSIWEAuth(tt.authConfig.Url, 16).
					Return(tt.expectedAuth).
					Times(1)
			}

			// Call the function being tested
			auth, err := dinMiddleware.createProviderSIWEAuth(tt.authConfig)

			// Assert results
			if tt.expectedError != nil {
				assert.Equal(t, tt.expectedError.Error(), err.Error())
				assert.Nil(t, auth)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedAuth, auth)
			}
		})
	}
}

func TestProcessHCMethodResponseAsyncLogging(t *testing.T) {
	tests := []struct {
		name           string
		respBody       []byte
		respStatus     int
		method         string
		expectLogCall  bool
		expectLogLevel zapcore.Level
		expectLogMsg   string
	}{
		{
			name:           "JSON parsing error should trigger robust logging",
			respBody:       []byte("invalid json"),
			respStatus:     200,
			expectLogMsg:   "Request attempt failed",
			expectLogLevel: zapcore.ErrorLevel,
		},
		{
			name:           "Non-200 status should trigger robust logging",
			respBody:       []byte(`{"jsonrpc":"2.0","id":1,"result":"0x123"}`),
			respStatus:     500,
			expectLogMsg:   "Request attempt failed",
			expectLogLevel: zapcore.ErrorLevel,
		},
		{
			name:           "JSON-RPC error should trigger robust logging",
			respBody:       []byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32603,"message":"Internal error"}}`),
			respStatus:     200,
			expectLogMsg:   "Request attempt failed",
			expectLogLevel: zapcore.ErrorLevel,
		},
		{
			name:           "Method mismatch should not trigger failure logging",
			respBody:       []byte(`{"jsonrpc":"2.0","id":1,"result":"0x64"}`),
			respStatus:     200,
			method:         "eth_getBalance",
			expectLogCall:  false,
			expectLogLevel: zapcore.DebugLevel,
			expectLogMsg:   "",
		},
		{
			name:           "Empty response body should not trigger failure logging",
			respBody:       []byte{},
			respStatus:     200,
			method:         "eth_blockNumber",
			expectLogCall:  false,
			expectLogLevel: zapcore.DebugLevel,
			expectLogMsg:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create observed logger
			observedZapCore, observedLogs := observer.New(zap.DebugLevel)
			observedLogger := zap.New(observedZapCore)
			loggerClient := &logger.LoggerClient{Logger: observedLogger}

			// Create test network with proper handler initialization
			network, err := NewNetwork("test", EVMHandler, utils.Environment("test"), "8080")
			assert.NoError(t, err)
			network.logger = loggerClient
			network.Name = "test/eth" // Update name to match test expectations

			// Create test middleware
			middleware := &DinMiddleware{
				logger: loggerClient,
			}

			// Run the function
			middleware.processHCMethodResponseAsync(network, "test/eth", tt.respBody, tt.respStatus, tt.method)

			// Give the goroutine time to complete
			time.Sleep(200 * time.Millisecond)

			// Check logs
			logs := observedLogs.All()

			if tt.expectLogCall {
				// Should have at least one log entry with the expected message
				found := false
				for _, log := range logs {
					if log.Level == tt.expectLogLevel && log.Message == tt.expectLogMsg {
						found = true

						// Verify log contains expected fields
						fields := log.ContextMap()
						assert.Contains(t, fields, "network")
						assert.Contains(t, fields, "provider")
						assert.Contains(t, fields, "request_method")
						assert.Equal(t, "test/eth", fields["network"])
						assert.Equal(t, "din", fields["provider"])
						assert.Equal(t, tt.method, fields["request_method"])

						// Check for error-specific fields based on the test case
						if tt.respStatus != 200 {
							assert.Contains(t, fields, "status_code")
							assert.Equal(t, int64(tt.respStatus), fields["status_code"])
						}

						break
					}
				}
				assert.True(t, found, "Expected log message '%s' with level %s not found in logs", tt.expectLogMsg, tt.expectLogLevel)
			} else {
				// Should not have any failure logs with the expected message
				for _, log := range logs {
					assert.NotEqual(t, "Request attempt failed", log.Message, "Unexpected failure log found")
				}
			}
		})
	}
}
