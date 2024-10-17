package modules

import (
	"sync"
	"testing"

	"github.com/DIN-center/din-caddy-plugins/lib/auth/siwe"
	din "github.com/DIN-center/din-sc/apps/din-go/lib/din"
	dinreg "github.com/DIN-center/din-sc/apps/din-go/pkg/dinregistry"
	"github.com/pkg/errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/golang/mock/gomock"
	"github.com/zeebo/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestSyncRegistryWithLatestBlock(t *testing.T) {
	logger := zap.NewNop()
	mockCtrl := gomock.NewController(t)
	mockDingoClient := din.NewMockIDingoClient(mockCtrl)
	dinMiddleware := &DinMiddleware{
		RegistryEnv:                         LineaMainnet,
		RegistryBlockEpoch:                  10,
		registryLastUpdatedEpochBlockNumber: 40,
		logger:                              logger,
		DingoClient:                         mockDingoClient,
		testMode:                            true,
	}

	tests := []struct {
		name                                string
		registryLastUpdatedEpochBlockNumber int64
		latestBlockNumber                   int64
		expectedUpdateCall                  bool
		expectedBlockFloorByEpoch           int64
	}{
		{
			name:                                "Sync should update as block difference is equal to or exceeds epoch 50",
			registryLastUpdatedEpochBlockNumber: int64(40),
			latestBlockNumber:                   int64(50),
			expectedUpdateCall:                  true,
			expectedBlockFloorByEpoch:           int64(50),
		},
		{
			name:                                "Sync should update as block difference is equal to or exceeds epoch 52",
			registryLastUpdatedEpochBlockNumber: int64(40),
			latestBlockNumber:                   int64(52),
			expectedUpdateCall:                  true,
			expectedBlockFloorByEpoch:           int64(50),
		},
		{
			name:                                "Sync should update as block difference is equal to or exceeds epoch 1000",
			registryLastUpdatedEpochBlockNumber: int64(40),
			latestBlockNumber:                   int64(1001),
			expectedUpdateCall:                  true,
			expectedBlockFloorByEpoch:           int64(1000),
		},
		{
			name:                                "Sync should not update as block difference is less than epoch 48",
			registryLastUpdatedEpochBlockNumber: int64(40),
			latestBlockNumber:                   int64(48),
			expectedUpdateCall:                  false,
			expectedBlockFloorByEpoch:           int64(40),
		},
		{
			name:                                "Sync should not update as block difference is less than epoch 30",
			registryLastUpdatedEpochBlockNumber: int64(40),
			latestBlockNumber:                   int64(30),
			expectedUpdateCall:                  false,
			expectedBlockFloorByEpoch:           int64(40),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			dinMiddleware.Networks = map[string]*network{
				LineaMainnet: {
					latestBlockNumber: tt.latestBlockNumber,
				},
			}
			dinMiddleware.registryLastUpdatedEpochBlockNumber = tt.registryLastUpdatedEpochBlockNumber

			mockDingoClient.EXPECT().GetLatestBlockNumber().Return(tt.latestBlockNumber, nil).Times(1)

			// Check if update was called as expected
			if tt.expectedUpdateCall {
				mockDingoClient.EXPECT().GetRegistryData().Return(&din.DinRegistryData{}, nil).Times(1)
			}
			// Call the function
			dinMiddleware.syncRegistryWithLatestBlock()

			// Validate that registryLastUpdatedEpochBlockNumber is updated correctly
			if dinMiddleware.registryLastUpdatedEpochBlockNumber != tt.expectedBlockFloorByEpoch {
				t.Errorf("Expected registryLastUpdatedEpochBlockNumber = %v, got %v", tt.expectedBlockFloorByEpoch, dinMiddleware.registryLastUpdatedEpochBlockNumber)
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
		syncNetworkConfigErr     error
		createNewProviderErr     error
		methodByBitErr           error
		expectedError            error
		expectedNetworkProviders int
	}{
		{
			name: "Successful add network with providers",
			regNetwork: &din.Network{
				ProxyName: "test-network",
				Providers: map[string]*din.Provider{
					"Provider1": {
						NetworkServices: map[string]*din.NetworkService{
							"http://new-provider.com": {
								Url:           "http://new-provider.com",
								Address:       "0x1234567890abcdef",
								NetworkStatus: dinreg.Active,
							},
						},
					},
				},
				NetworkConfig: &dinreg.NetworkConfig{
					HealthcheckMethodBit: uint8(1),
				},
			},
			methodByBitErr:           nil,
			syncNetworkConfigErr:     nil,
			createNewProviderErr:     nil,
			expectedError:            nil,
			expectedNetworkProviders: 1,
		},
		{
			name: "Error, missing active status",
			regNetwork: &din.Network{
				ProxyName: "test-network",
				Providers: map[string]*din.Provider{
					"Provider1": {
						NetworkServices: map[string]*din.NetworkService{
							"http://new-provider.com": {
								Url:           "http://new-provider.com",
								Address:       "0x1234567890abcdef",
								NetworkStatus: dinreg.Onboarding,
							},
						},
					},
				},
				NetworkConfig: &dinreg.NetworkConfig{
					HealthcheckMethodBit: uint8(1),
				},
			},
			methodByBitErr:           nil,
			syncNetworkConfigErr:     nil,
			createNewProviderErr:     nil,
			expectedError:            nil,
			expectedNetworkProviders: 0,
		},
		{
			name: "Error syncing network config",
			regNetwork: &din.Network{
				ProxyName: "test-network",
				Providers: map[string]*din.Provider{
					"Provider1": {
						NetworkServices: map[string]*din.NetworkService{
							"http://new-provider.com": {
								Url:           "http://new-provider.com",
								Address:       "0x1234567890abcdef",
								NetworkStatus: dinreg.Active,
							},
						},
					},
				},
				NetworkConfig: &dinreg.NetworkConfig{
					HealthcheckMethodBit: uint8(1),
				},
			},
			methodByBitErr:           errors.New(""),
			syncNetworkConfigErr:     nil,
			createNewProviderErr:     nil,
			expectedError:            errors.New(""),
			expectedNetworkProviders: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock DingoClient and other dependencies
			mockDingoClient := din.NewMockIDingoClient(mockCtrl)

			mockDingoClient.EXPECT().GetNetworkMethodNameByBit(gomock.Any(), gomock.Any()).Return("new-method", tt.methodByBitErr).AnyTimes()
			mockDingoClient.EXPECT().GetNetworkServiceMethods(gomock.Any()).Return([]*string{aws.String("eth_call"), aws.String("eth_blockNumber")}, nil).AnyTimes()

			// Create logger
			logger := zaptest.NewLogger(t)

			// Create DinMiddleware instance
			dinMiddleware := &DinMiddleware{
				DingoClient: mockDingoClient,
				logger:      logger,
				Networks:    make(map[string]*network),
				testMode:    true,
			}

			// Call the function being tested
			err := dinMiddleware.addNetworkWithRegistryData(tt.regNetwork)

			// Assert expected error
			if tt.expectedError != nil {
				assert.Equal(t, err.Error(), tt.expectedError.Error())
			} else {
				assert.NoError(t, err)

				// Verify the network is added
				network, ok := dinMiddleware.Networks[tt.regNetwork.ProxyName]
				assert.Equal(t, true, ok)

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
		methodByBitErr             error
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
								Url:           "http://new-provider.com",
								Address:       "0x1234567890abcdef",
								NetworkStatus: dinreg.Active,
							},
						},
					},
				},
				NetworkConfig: &dinreg.NetworkConfig{
					HealthcheckMethodBit: 1,
				},
			},
			newNetwork: &network{
				Name:      "test-network",
				Providers: map[string]*provider{},
			},
			methodByBitErr:             nil,
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
								Url:           "http://new-provider.com",
								Address:       "0x1234567890abcdef",
								NetworkStatus: dinreg.Onboarding,
							},
						},
					},
				},
				NetworkConfig: &dinreg.NetworkConfig{
					HealthcheckMethodBit: 1,
				},
			},
			newNetwork: &network{
				Name:      "test-network",
				Providers: map[string]*provider{},
			},
			methodByBitErr:             nil,
			createNewProviderErr:       nil,
			expectedError:              nil,
			expectedProviderCount:      0,
			expectedRemainingProviders: 0,
		},
		{
			name: "Error syncing network config",
			regNetwork: &din.Network{
				Name: "test-network",
				NetworkConfig: &dinreg.NetworkConfig{
					HealthcheckMethodBit: 1,
					NetworkStatus:        dinreg.Active,
				},
			},
			newNetwork: &network{
				Name: "test-network",
			},
			methodByBitErr:             errors.New("sync error"),
			expectedError:              errors.New("sync error"),
			expectedProviderCount:      0,
			expectedRemainingProviders: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock DingoClient and other dependencies
			mockDingoClient := din.NewMockIDingoClient(mockCtrl)

			// Create logger
			logger := zaptest.NewLogger(t)

			// Create DinMiddleware instance
			dinMiddleware := &DinMiddleware{
				DingoClient: mockDingoClient,
				logger:      logger,
				Networks: map[string]*network{
					tt.newNetwork.Name: tt.newNetwork,
				},
				testMode: true,
			}

			mockDingoClient.EXPECT().GetNetworkServiceMethods(gomock.Any()).Return([]*string{aws.String("eth_call"), aws.String("eth_blockNumber")}, nil).AnyTimes()
			mockDingoClient.EXPECT().GetNetworkMethodNameByBit(gomock.Any(), gomock.Any()).Return("new-method", tt.methodByBitErr).AnyTimes()

			// Call the function being tested
			err := dinMiddleware.updateNetworkWithRegistryData(tt.regNetwork, tt.newNetwork)

			// Assert expected error
			if tt.expectedError != nil {
				assert.Equal(t, err.Error(), tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}

			// Assert the number of providers after the update
			assert.Equal(t, tt.expectedProviderCount, len(tt.newNetwork.Providers))
		})
	}
}

func TestSyncNetworkConfig(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	tests := []struct {
		name                          string
		regNetwork                    *din.Network
		network                       *network
		getNetworkMethodNameByBitErr  error
		expectedNetwork               *network
		expectedError                 error
		expectedHCMethod              string
		expectedHCInterval            uint64
		expectedBlockLagLimit         uint64
		expectedMaxRequestPayloadSize uint64
		expectedRequestAttemptCount   int
	}{
		{
			name: "Successful sync with all changes",
			regNetwork: &din.Network{
				Name: "test-network",
				NetworkConfig: &dinreg.NetworkConfig{
					HealthcheckMethodBit:    1,
					HealthcheckIntervalSec:  30,
					BlockLagLimit:           5,
					MaxRequestPayloadSizeKb: 2048,
					RequestAttemptCount:     3,
				},
			},
			network: &network{
				Name:                    "test-network",
				HCMethod:                "old-method",
				HCInterval:              10,
				BlockLagLimit:           3,
				MaxRequestPayloadSizeKB: 1024,
				RequestAttemptCount:     1,
			},
			expectedHCMethod:              "new-method",
			expectedHCInterval:            30,
			expectedBlockLagLimit:         5,
			expectedMaxRequestPayloadSize: 2048,
			expectedRequestAttemptCount:   3,
			expectedNetwork: &network{
				Name:                    "test-network",
				HCMethod:                "new-method",
				HCInterval:              30,
				BlockLagLimit:           5,
				MaxRequestPayloadSizeKB: 2048,
				RequestAttemptCount:     3,
			},
			expectedError: nil,
		},
		{
			name: "Error getting network method",
			regNetwork: &din.Network{
				Name: "test-network",
				NetworkConfig: &dinreg.NetworkConfig{
					HealthcheckMethodBit: 1,
				},
			},
			network: &network{
				Name:     "test-network",
				HCMethod: "old-method",
			},
			getNetworkMethodNameByBitErr: errors.New("failed to get method"),
			expectedError:                errors.New("failed to get method"),
			expectedNetwork:              nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock DingoClient
			mockDingoClient := din.NewMockIDingoClient(mockCtrl)

			// Create logger
			logger := zaptest.NewLogger(t)

			// Create DinMiddleware instance
			dinMiddleware := &DinMiddleware{
				DingoClient: mockDingoClient,
				logger:      logger,
				testMode:    true,
			}

			// Mock GetNetworkMethodNameByBit method
			mockDingoClient.EXPECT().
				GetNetworkMethodNameByBit(tt.regNetwork.Name, tt.regNetwork.NetworkConfig.HealthcheckMethodBit).
				Return(tt.expectedHCMethod, tt.getNetworkMethodNameByBitErr).
				AnyTimes()

			// Call syncNetworkConfig
			updatedNetwork, err := dinMiddleware.syncNetworkConfig(tt.regNetwork, tt.network)

			// Assert results
			if tt.expectedError != nil {
				assert.Equal(t, err.Error(), tt.expectedError.Error())
				assert.Nil(t, updatedNetwork)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, updatedNetwork)

				// Verify that the network was updated correctly
				assert.Equal(t, tt.expectedHCMethod, updatedNetwork.HCMethod)
				assert.Equal(t, tt.expectedHCInterval, updatedNetwork.HCInterval)
				assert.Equal(t, tt.expectedBlockLagLimit, updatedNetwork.BlockLagLimit)
				assert.Equal(t, tt.expectedMaxRequestPayloadSize, updatedNetwork.MaxRequestPayloadSizeKB)
				assert.Equal(t, tt.expectedRequestAttemptCount, updatedNetwork.RequestAttemptCount)
			}
		})
	}
}

func TestCreateNewProvider(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	tests := []struct {
		name                  string
		provider              *provider
		authConfig            *dinreg.NetworkServiceAuthConfig
		networkServiceAddress string
		initializeProviderErr error
		getMethodsErr         error
		expectedError         error
		expectedMethods       []*string
	}{
		{
			name: "Successful provider creation",
			provider: &provider{
				HttpUrl: "http://example.com",
			},
			authConfig: &dinreg.NetworkServiceAuthConfig{
				Type: "siwe",
				Url:  "http://example.com",
			},
			networkServiceAddress: "0x1234567890abcdef",
			initializeProviderErr: nil,
			getMethodsErr:         nil,
			expectedError:         nil,
			expectedMethods:       []*string{aws.String("eth_call"), aws.String("eth_blockNumber")},
		},
		{
			name: "Error fetching network service methods",
			provider: &provider{
				HttpUrl: "http://example.com",
			},
			networkServiceAddress: "0x1234567890abcdef",
			initializeProviderErr: nil,
			getMethodsErr:         errors.New("failed to fetch methods"),
			expectedError:         errors.New("failed to get network service methods: failed to fetch methods"),
			expectedMethods:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockDingoClient := din.NewMockIDingoClient(mockCtrl)

			// Create DinMiddleware and mock logger
			logger := zaptest.NewLogger(t)
			dinMiddleware := &DinMiddleware{
				DingoClient:      mockDingoClient,
				RegistryPriority: 10,
				logger:           logger,
				testMode:         true,
			}

			// Mock GetNetworkServiceMethods
			mockDingoClient.EXPECT().
				GetNetworkServiceMethods(tt.networkServiceAddress).
				Return(tt.expectedMethods, tt.getMethodsErr).
				Times(1)

			// Call the function being tested
			createdProvider, err := dinMiddleware.createNewProvider(tt.provider, nil, tt.networkServiceAddress)

			// Assert results
			if tt.expectedError != nil {
				assert.Equal(t, err.Error(), tt.expectedError.Error())
				assert.Nil(t, createdProvider)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, createdProvider)

				// Verify that the provider was updated correctly
				assert.DeepEqual(t, tt.expectedMethods, createdProvider.Methods)
				assert.Equal(t, dinMiddleware.RegistryPriority, createdProvider.Priority)
			}
		})
	}
}

func TestUpdateProviderData(t *testing.T) {
	tests := []struct {
		name             string
		networkName      string
		initialProviders map[string]*provider
		updatedProvider  *provider
		expectedProvider *provider
	}{
		{
			name:        "Successful update of provider data",
			networkName: "test-network",
			initialProviders: map[string]*provider{
				"provider1": {
					host: "provider1",
					Auth: &siwe.SIWEClientAuth{
						ProviderURL: "initial-url",
					},
					Methods: []*string{aws.String("eth_call")},
				},
			},
			updatedProvider: &provider{
				host: "provider1",
				Auth: &siwe.SIWEClientAuth{
					ProviderURL: "updated-url",
				},
				Methods: []*string{aws.String("eth_blockNumber")},
			},
			expectedProvider: &provider{
				host: "provider1",
				Auth: &siwe.SIWEClientAuth{
					ProviderURL: "updated-url",
				},
				Methods: []*string{aws.String("eth_blockNumber")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize DinMiddleware with the network and providers
			dinMiddleware := &DinMiddleware{
				Networks: map[string]*network{
					tt.networkName: {
						Name:      tt.networkName,
						Providers: tt.initialProviders,
					},
				},
				mu:       sync.RWMutex{},
				testMode: true,
			}

			// Lock for writing
			dinMiddleware.mu.Lock()
			// Call the function being tested
			dinMiddleware.updateProviderData(tt.networkName, tt.updatedProvider)
			// Unlock
			dinMiddleware.mu.Unlock()

			// Assert that the provider data was updated correctly
			updatedProvider := dinMiddleware.Networks[tt.networkName].Providers[tt.updatedProvider.host]
			assert.Equal(t, tt.expectedProvider.Auth.ProviderURL, updatedProvider.Auth.ProviderURL)
			assert.DeepEqual(t, tt.expectedProvider.Methods, updatedProvider.Methods)
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
				HCMethod:                "initial-method",
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
				HCMethod:                "new-method",
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
				HCMethod:                "new-method",
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
				HCMethod:                "initial-method",
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
				HCMethod:                "new-method",
				HCInterval:              20,
				BlockLagLimit:           10,
				MaxRequestPayloadSizeKB: 2048,
				RequestAttemptCount:     5,
				Providers:               map[string]*provider{},
			},
			expectedNetwork: &network{
				Name:                    "test-network",
				HCMethod:                "new-method",
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
			assert.Equal(t, tt.expectedNetwork.HCMethod, updatedNetwork.HCMethod)
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
