package modules

import (
	"crypto/rand"
	"sync"
	"testing"

	"github.com/DIN-center/din-caddy-plugins/lib/auth/siwe"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
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
	logger := logger.NewLoggerClient(zap.NewNop(), nil)
	mockCtrl := gomock.NewController(t)
	mockDingoClient := din.NewMockIDingoClient(mockCtrl)
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
			dinMiddleware.Networks = map[string]*network{}
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
								Status:  dinreg.Active,
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
			networkServiceCreated:    true,
		},
		{
			name: "Error, missing active status",
			regNetwork: &din.Network{
				ProxyName: "test-network",
				Providers: map[string]*din.Provider{
					"Provider1": {
						NetworkServices: map[string]*din.NetworkService{
							"http://new-provider.com": {
								Url:     "http://new-provider.com",
								Address: "0x1234567890abcdef",
								Status:  dinreg.Onboarding,
							},
						},
					},
				},
				NetworkConfig: &dinreg.NetworkConfig{
					HealthcheckMethodBit: uint8(1),
				},
				Status: dinreg.Onboarding,
			},
			methodByBitErr:           nil,
			syncNetworkConfigErr:     nil,
			createNewProviderErr:     nil,
			expectedError:            nil,
			expectedNetworkProviders: 0,
			networkServiceCreated:    false,
		},
		{
			name: "Error syncing network config",
			regNetwork: &din.Network{
				ProxyName: "test-network",
				Providers: map[string]*din.Provider{
					"Provider1": {
						NetworkServices: map[string]*din.NetworkService{
							"http://new-provider.com": {
								Url:     "http://new-provider.com",
								Address: "0x1234567890abcdef",
								Status:  dinreg.Active,
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
			networkServiceCreated:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock DingoClient and other dependencies
			mockDingoClient := din.NewMockIDingoClient(mockCtrl)

			mockDingoClient.EXPECT().GetNetworkMethodNameByBit(gomock.Any(), gomock.Any()).Return("new-method", tt.methodByBitErr).AnyTimes()
			mockDingoClient.EXPECT().GetNetworkServiceMethods(gomock.Any()).Return([]*string{aws.String("eth_call"), aws.String("eth_blockNumber")}, nil).AnyTimes()

			// Create logger
			logger := logger.NewLoggerClient(zaptest.NewLogger(t), nil)

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
				assert.Equal(t, tt.networkServiceCreated, ok)

				if network != nil {
					// Verify the number of providers added to the network
					assert.Equal(t, tt.expectedNetworkProviders, len(network.Providers))
				}
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
		blockNumberMethodByBitErr  error
		chainIdMethodByBitErr      error
		callContractMethodByBitErr error
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
								Status:  dinreg.Active,
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
			blockNumberMethodByBitErr:  nil,
			chainIdMethodByBitErr:      nil,
			callContractMethodByBitErr: nil,
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
				NetworkConfig: &dinreg.NetworkConfig{
					HealthcheckMethodBit: 1,
				},
				Status: dinreg.Onboarding,
			},
			newNetwork: &network{
				Name:      "test-network",
				Providers: map[string]*provider{},
			},
			blockNumberMethodByBitErr:  nil,
			chainIdMethodByBitErr:      nil,
			callContractMethodByBitErr: nil,
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
				NetworkConfig: &dinreg.NetworkConfig{
					HealthcheckMethodBit: 1,
				},
				Status: dinreg.Active,
			},
			newNetwork: &network{
				Name: "test-network",
			},
			blockNumberMethodByBitErr:  errors.New("sync error"),
			chainIdMethodByBitErr:      errors.New("sync error"),
			callContractMethodByBitErr: errors.New("sync error"),
			syncNetworkConfigErr:       nil,
			createNewProviderErr:       nil,
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
			logger := logger.NewLoggerClient(zaptest.NewLogger(t), nil)

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
			mockDingoClient.EXPECT().GetNetworkMethodNameByBit(gomock.Any(), gomock.Any()).Return("new-method", tt.blockNumberMethodByBitErr).AnyTimes()
			mockDingoClient.EXPECT().GetNetworkMethodNameByBit(gomock.Any(), gomock.Any()).Return("new-method", tt.chainIdMethodByBitErr).AnyTimes()
			mockDingoClient.EXPECT().GetNetworkMethodNameByBit(gomock.Any(), gomock.Any()).Return("new-method", tt.callContractMethodByBitErr).AnyTimes()

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
		name                     string
		regNetwork               *din.Network
		existingNetwork          *network
		hcMethodName             string
		chainIDMethodName        string
		callContractMethodName   string
		archiveEnabled           bool
		getHCMethodErr           error
		callsHealthcheckMethod   bool
		getChainIDMethodErr      error
		callsChainIDMethod       bool
		getCallContractMethodErr error
		callsCallContractMethod  bool
		expectedError            error
		expectedNetwork          *network
	}{
		{
			name: "successful sync with all new values",
			regNetwork: &din.Network{
				Name: "test-network",
				NetworkConfig: &dinreg.NetworkConfig{
					HealthcheckMethodBit:    1,
					ChainIdMethodBit:        1,
					CallContractMethodBit:   1,
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
				HCMethod:                "old-method",
				ChainIdMethod:           "old-chain-method",
				CallContractMethod:      "old-call-method",
				ChainId:                 "0x0",
				HCInterval:              10,
				BlockLagLimit:           5,
				BlockJumpLimit:          3,
				MaxRequestPayloadSizeKB: 1024,
				RequestAttemptCount:     3,
				ArchiveEnabled:          false,
			},
			hcMethodName:            "eth_blockNumber",
			callsHealthcheckMethod:  true,
			chainIDMethodName:       "eth_chainId",
			callsChainIDMethod:      true,
			callContractMethodName:  "eth_call",
			callsCallContractMethod: true,
			expectedNetwork: &network{
				Name:                    "test-network",
				HCMethod:                "eth_blockNumber",
				ChainIdMethod:           "eth_chainId",
				CallContractMethod:      "eth_call",
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
			name: "error getting healthcheck method",
			regNetwork: &din.Network{
				Name: "test-network",
				NetworkConfig: &dinreg.NetworkConfig{
					HealthcheckMethodBit: 1,
				},
			},
			existingNetwork: &network{
				Name: "test-network",
			},
			getHCMethodErr:         errors.New("failed to get healthcheck method"),
			callsHealthcheckMethod: true,
			expectedError:          errors.New("failed to get network healthcheck method"),
		},
		{
			name: "error getting chain ID method",
			regNetwork: &din.Network{
				Name: "test-network",
				NetworkConfig: &dinreg.NetworkConfig{
					ChainIdMethodBit: 1,
				},
			},
			existingNetwork:        &network{Name: "test-network"},
			callsHealthcheckMethod: true,
			getChainIDMethodErr:    errors.New("failed to get chain ID method"),
			callsChainIDMethod:     true,
			expectedError:          errors.New("failed to get network chain ID method"),
		},
		{
			name: "error getting call contract method",
			regNetwork: &din.Network{
				Name: "test-network",
				NetworkConfig: &dinreg.NetworkConfig{
					CallContractMethodBit: 1,
				},
			},
			existingNetwork: &network{
				Name: "test-network",
			},
			callsHealthcheckMethod:   true,
			getCallContractMethodErr: errors.New("failed to get call contract method"),
			callsChainIDMethod:       true,
			callsCallContractMethod:  true,
			expectedError:            errors.New("failed to get network call contract method"),
		},
		{
			name: "no updates needed when registry values are zero",
			regNetwork: &din.Network{
				Name: "test-network",
				NetworkConfig: &dinreg.NetworkConfig{
					HealthcheckMethodBit:    1,
					ChainIdMethodBit:        1,
					ChainId:                 "0x1",
					HealthcheckIntervalSec:  0,
					BlockLagLimit:           0,
					BlockJumpLimit:          0,
					MaxRequestPayloadSizeKb: 0,
					RequestAttemptCount:     0,
					ArchiveEnabled:          false,
				},
			},
			existingNetwork: &network{
				Name:                    "test-network",
				HCMethod:                "eth_blockNumber",
				ChainIdMethod:           "eth_chainId",
				CallContractMethod:      "eth_call",
				ChainId:                 "0x1",
				HCInterval:              10,
				BlockLagLimit:           5,
				BlockJumpLimit:          3,
				MaxRequestPayloadSizeKB: 1024,
				RequestAttemptCount:     3,
				ArchiveEnabled:          false,
			},
			callsHealthcheckMethod:  true,
			callsChainIDMethod:      true,
			callsCallContractMethod: true,
			hcMethodName:            "eth_blockNumber",
			chainIDMethodName:       "eth_chainId",
			callContractMethodName:  "eth_call",
			expectedNetwork: &network{
				Name:                    "test-network",
				HCMethod:                "eth_blockNumber",
				ChainIdMethod:           "eth_chainId",
				CallContractMethod:      "eth_call",
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
			mockDingoClient := din.NewMockIDingoClient(mockCtrl)

			// Create logger
			logger := logger.NewLoggerClient(zaptest.NewLogger(t), nil)

			if tt.callsHealthcheckMethod {
				mockDingoClient.EXPECT().
					GetNetworkMethodNameByBit(tt.regNetwork.Name, tt.regNetwork.NetworkConfig.HealthcheckMethodBit).
					Return(tt.hcMethodName, tt.getHCMethodErr).Times(1)
			}

			if tt.callsChainIDMethod {
				mockDingoClient.EXPECT().
					GetNetworkMethodNameByBit(tt.regNetwork.Name, tt.regNetwork.NetworkConfig.ChainIdMethodBit).
					Return(tt.chainIDMethodName, tt.getChainIDMethodErr).Times(1)
			}

			if tt.callsCallContractMethod {
				mockDingoClient.EXPECT().
					GetNetworkMethodNameByBit(tt.regNetwork.Name, tt.regNetwork.NetworkConfig.CallContractMethodBit).
					Return(tt.callContractMethodName, tt.getCallContractMethodErr).Times(1)
			}

			dinMiddleware := &DinMiddleware{
				DingoClient: mockDingoClient,
				logger:      logger,
			}

			result, err := dinMiddleware.syncNetworkConfig(tt.regNetwork, tt.existingNetwork)
			if tt.expectedError != nil {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedNetwork.HCMethod, result.HCMethod)
			assert.Equal(t, tt.expectedNetwork.ChainIdMethod, result.ChainIdMethod)
			assert.Equal(t, tt.expectedNetwork.ChainId, result.ChainId)
			assert.Equal(t, tt.expectedNetwork.CallContractMethod, result.CallContractMethod)
			assert.Equal(t, tt.expectedNetwork.HCInterval, result.HCInterval)
			assert.Equal(t, tt.expectedNetwork.BlockLagLimit, result.BlockLagLimit)
			assert.Equal(t, tt.expectedNetwork.BlockJumpLimit, result.BlockJumpLimit)
			assert.Equal(t, tt.expectedNetwork.MaxRequestPayloadSizeKB, result.MaxRequestPayloadSizeKB)
			assert.Equal(t, tt.expectedNetwork.RequestAttemptCount, result.RequestAttemptCount)
			assert.Equal(t, tt.expectedNetwork.ArchiveEnabled, result.ArchiveEnabled)
		})
	}
}

func TestCreateNewProvider(t *testing.T) {
	tests := []struct {
		name                  string
		provider              *provider
		authConfig            *dinreg.NetworkServiceAuthConfig
		networkServiceAddress string
		initializeProviderErr error
		getMethodsErr         error
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
			authConfig: &dinreg.NetworkServiceAuthConfig{
				Type: dinreg.SIWE,
				Url:  "http://example6.com",
			},
			networkServiceAddress: "0x1234567890abcdef",
			initializeProviderErr: nil,
			getMethodsErr:         nil,
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
			authConfig:            &dinreg.NetworkServiceAuthConfig{Type: dinreg.None},
			networkServiceAddress: "0x1234567890abcdef",
			initializeProviderErr: nil,
			getMethodsErr:         nil,
			expectedError:         nil,
			expectedMethods:       []*string{aws.String("eth_call"), aws.String("eth_blockNumber")},
			expectedAuth:          nil,
			expectAuthCreation:    false,
		},
		{
			name: "Error fetching network service methods",
			provider: &provider{
				HttpUrl: "http://example8.com",
			},
			authConfig:            &dinreg.NetworkServiceAuthConfig{Type: dinreg.SIWE, Url: "http://example9.com"},
			networkServiceAddress: "0x1234567890abcdef",
			initializeProviderErr: nil,
			getMethodsErr:         errors.New("failed to fetch methods"),
			expectedError:         errors.New("failed to get network service methods: failed to fetch methods"),
			expectedMethods:       nil,
			expectedAuth: &siwe.SIWEClientAuth{
				ProviderURL:  "http://example9.com",
				SessionCount: 16,
				Signer:       nil,
			},
			expectAuthCreation: true,
		},
		{
			name: "AUTH type is unknown ",
			provider: &provider{
				HttpUrl: "http://example12.com",
			},
			authConfig: &dinreg.NetworkServiceAuthConfig{
				Type: "unknown",
				Url:  "http://example13.com",
			},
			networkServiceAddress: "0x1234567890abcdef",
			initializeProviderErr: nil,
			getMethodsErr:         nil,
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
			defer mockCtrl.Finish()

			privateKeyData := make([]byte, 32)
			_, err := rand.Read(privateKeyData)
			if err != nil {
				t.Errorf("Failed to generate random data in test: %v", err)
			}

			mockDingoClient := din.NewMockIDingoClient(mockCtrl)
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
				logger:            logger.NewLoggerClient(zaptest.NewLogger(t), nil),
				testMode:          true,
				DefaultSiweSigner: defaultSigner,
			}

			// Mock GetNetworkServiceMethods
			mockDingoClient.EXPECT().
				GetNetworkServiceMethods(tt.networkServiceAddress).
				Return(tt.expectedMethods, tt.getMethodsErr).
				Times(1)

			// Call the function being tested
			createdProvider, err := dinMiddleware.createNewProvider(tt.provider, tt.authConfig, tt.networkServiceAddress)

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

func TestCreateProviderSIWEAuth(t *testing.T) {
	tests := []struct {
		name               string
		authConfig         *dinreg.NetworkServiceAuthConfig
		defaultSignerSet   bool
		expectedAuth       *siwe.SIWEClientAuth
		expectedError      error
		expectAuthCreation bool
	}{
		{
			name: "Successful SIWE auth creation with default signer",
			authConfig: &dinreg.NetworkServiceAuthConfig{
				Type: dinreg.SIWE,
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
			authConfig: &dinreg.NetworkServiceAuthConfig{
				Type: dinreg.None,
				Url:  "http://example.com",
			},
			defaultSignerSet:   true,
			expectAuthCreation: false,
			expectedAuth:       nil,
			expectedError:      nil,
		},
		{
			name: "Unknown auth type should return nil",
			authConfig: &dinreg.NetworkServiceAuthConfig{
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
				logger:           logger.NewLoggerClient(zaptest.NewLogger(t), nil),
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
