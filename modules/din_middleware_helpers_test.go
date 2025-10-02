package modules

import (
	"crypto/rand"
	"net/url"
	"sync"
	"testing"
	"time"

	ws "github.com/DIN-center/din-caddy-plugins/lib/watcherscore"
	din "github.com/DIN-center/din-sc/apps/din-go/lib/din"
	"github.com/pkg/errors"

	"github.com/DIN-center/din-caddy-plugins/lib/auth/siwe"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	"github.com/DIN-center/din-caddy-plugins/lib/utils"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
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
		Registry: RegistryConfig{
			BlockEpoch:                  10,
			lastUpdatedEpochBlockNumber: 40,
		},
		logger:      logger,
		DingoClient: mockDingoClient,
		testMode:    true,
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
			dinMiddleware.Registry.lastUpdatedEpochBlockNumber = tt.registryLastUpdatedEpochBlockNumber

			// Create a mock Web3Client
			mockWeb3Client := &MockWeb3Client{mockLatestBlockNumber: tt.latestBlockNumber}

			// Set up expectations for GetRegistryData only if we expect an update
			if tt.expectedUpdateCall {
				mockDingoClient.EXPECT().GetRegistryData().Return(&din.DinRegistryData{}, nil).Times(1)
			}

			// Call the function being tested using the mock middleware
			dinMiddleware.syncRegistryWithLatestBlock(mockWeb3Client)

			// Validate that registryLastUpdatedEpochBlockNumber is updated correctly
			if dinMiddleware.Registry.lastUpdatedEpochBlockNumber != tt.expectedBlockFloorByEpoch {
				t.Errorf("Expected registryLastUpdatedEpochBlockNumber = %v, got %v",
					tt.expectedBlockFloorByEpoch, dinMiddleware.Registry.lastUpdatedEpochBlockNumber)
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
					// Method fields removed, handlers provide these now
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
					// Method fields removed, handlers provide these now
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
			require.NoError(t, dinMiddleware.addNetworkWithRegistryData(tt.regNetwork))

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

func TestAddNetworkFromRegistryDataWorksWithDynamicLoadBalancing(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	tests := []struct {
		name                        string
		regNetwork                  *din.Network
		dynamicLoadBalancingEnabled bool
		networkAdded                bool
	}{
		{
			name: "Network removed, and DLB enabled",
			regNetwork: &din.Network{
				ProxyName: "test-network",
				Providers: map[string]*din.Provider{
					"Provider1": {
						NetworkServices: map[string]*din.NetworkService{
							"http://new-provider.com": {
								Url:    "http://new-provider.com",
								Status: din.NetworkServiceStatusActive,
							},
						},
					},
				},
				NetworkConfig: &din.NetworkOperationsConfig{},
			},
			dynamicLoadBalancingEnabled: true,
			networkAdded:                true,
		},
		{
			name: "No network added, and DLB enabled",
			regNetwork: &din.Network{
				ProxyName:     "test-network",
				Providers:     map[string]*din.Provider{},
				Status:        din.NetworkStatusOnboarding,
				NetworkConfig: &din.NetworkOperationsConfig{},
			},
			dynamicLoadBalancingEnabled: true,
			networkAdded:                false,
		},
		{
			name: "network added, and DLB disabled",
			regNetwork: &din.Network{
				ProxyName: "test-network",
				Providers: map[string]*din.Provider{
					"Provider1": {
						NetworkServices: map[string]*din.NetworkService{
							"http://new-provider.com": {
								Url:    "http://new-provider.com",
								Status: din.NetworkServiceStatusActive,
							},
						},
					},
				},
				Status:        din.NetworkStatusActive,
				NetworkConfig: &din.NetworkOperationsConfig{},
			},
			dynamicLoadBalancingEnabled: false,
			networkAdded:                false,
		},
		{
			name: "No network added, and DLB disabled",
			regNetwork: &din.Network{
				ProxyName:     "test-network",
				Providers:     map[string]*din.Provider{},
				Status:        din.NetworkStatusOnboarding,
				NetworkConfig: &din.NetworkOperationsConfig{},
			},
			dynamicLoadBalancingEnabled: false,
			networkAdded:                false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock DingoClient and other dependencies
			mockDingoClient := din.NewMockIDinClient(mockCtrl)

			// Create a mock WatcherScoreManager
			mockWatcherScoreManager := ws.NewMockIWatcherScoreManager(mockCtrl)

			// Create logger
			logger := logger.NewLoggerClient(zaptest.NewLogger(t), utils.Environment("test"))

			// Create DinMiddleware instance with proper initialization
			dinMiddleware := &DinMiddleware{
				DingoClient:                mockDingoClient,
				logger:                     logger,
				Networks:                   make(map[string]*network),
				testMode:                   true,
				Env:                        utils.Environment("test"),
				CaddyPort:                  "8080",
				machineID:                  "test-machine-id",
				DynamicLoadBalacingEnabled: tt.dynamicLoadBalancingEnabled,
				watcherScoreManager:        mockWatcherScoreManager,
			}

			// Set up the expectations according to the test case
			if tt.networkAdded && tt.dynamicLoadBalancingEnabled {
				mockWatcherScoreManager.EXPECT().AddNetworkWithBuiltInFormula(tt.regNetwork.ProxyName, dinMiddleware.GetOrCreateWatcherClient()).Return(nil).
					Times(1)
			} else {
				mockWatcherScoreManager.EXPECT().AddNetworkWithBuiltInFormula(tt.regNetwork.ProxyName, dinMiddleware.GetOrCreateWatcherClient()).Return(nil).
					Times(0)
			}

			// Call the function being tested
			dinMiddleware.addNetworkWithRegistryData(tt.regNetwork)
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
					// Method fields removed, handlers provide these now
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
					// Method fields removed, handlers provide these now
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
				Name:          "test-network",
				NetworkConfig: &din.NetworkOperationsConfig{
					// Method fields removed, handlers provide these now
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
			require.NoError(t, dinMiddleware.updateNetworkWithRegistryData(tt.regNetwork, tt.newNetwork))

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
				DingoClient:      mockDingoClient,
				SiweSignerClient: mockSiweSignerClient,
				Registry: RegistryConfig{
					Priority: 10,
				},
				logger:            logger.NewLoggerClient(zaptest.NewLogger(t), utils.Environment("test")),
				testMode:          true,
				DefaultSiweSigner: defaultSigner,
				Networks: map[string]*network{
					"test-network": {
						Providers: make(map[string]*provider),
					},
				},
			}

			// Call the function being tested
			createdProvider, err := dinMiddleware.createNewProvider("test-network", tt.provider, tt.authConfig, tt.networkService)

			// Assert results
			if tt.expectedError != nil {
				assert.Equal(t, err.Error(), tt.expectedError.Error())
				assert.Nil(t, createdProvider)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, createdProvider)

				// Verify that the provider was updated correctly
				assert.Equal(t, expectedMethodsMap(tt.expectedMethods), createdProvider.Methods)
				assert.Equal(t, dinMiddleware.Registry.Priority, createdProvider.Priority)
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
		"api.example.com",      // First gets base name
		"api.example.com-5678", // Second gets suffix (first updated retroactively to -1234)
		"api.example.com-9abc", // Third gets suffix
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
