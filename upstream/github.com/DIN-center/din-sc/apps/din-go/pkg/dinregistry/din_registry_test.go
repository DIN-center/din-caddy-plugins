package dinregistry

import (
	"math/big"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/umbracle/ethgo"
	"github.com/umbracle/ethgo/jsonrpc"
)

func TestDinRegistryGetNetworkOperationsConfig(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockContractHandler := NewMockIContractHandler(mockCtrl)

	dinRegistryHandler := &DinRegistryHandler{
		ContractHandler: mockContractHandler,
	}

	tests := []struct {
		name            string
		network         string
		callReturnData  interface{}
		callReturnError error
		expectedConfig  *NetworkConfig
		expectedError   string
	}{
		{
			name:    "Successful case",
			network: "test-network",
			callReturnData: map[string]interface{}{
				"opsConfig": map[string]interface{}{
					"healthcheckMethodBit":    uint8(1),
					"healthcheckIntervalSec":  uint8(30),
					"blockLagLimit":           uint8(5),
					"requestAttemptCount":     uint8(3),
					"maxRequestPayloadSizeKb": uint16(1024),
				},
			},
			callReturnError: nil,
			expectedConfig: &NetworkConfig{
				HealthcheckMethodBit:    1,
				HealthcheckIntervalSec:  30,
				BlockLagLimit:           5,
				RequestAttemptCount:     3,
				MaxRequestPayloadSizeKb: 1024,
			},
			expectedError: "",
		},
		{
			name:            "Failed call to ContractHandler",
			network:         "test-network",
			callReturnData:  nil,
			callReturnError: errors.New("call failed"),
			expectedConfig:  nil,
			expectedError:   "failed call to DinRegistryHandler GetNetworkOperationsConfig: call failed",
		},
		{
			name:            "Mismatched type for networkConfigMap",
			network:         "test-network",
			callReturnData:  "invalidType",
			callReturnError: nil,
			expectedConfig:  nil,
			expectedError:   "mismatched type for networkConfigMap",
		},
		{
			name:            "Missing opsConfig in response",
			network:         "test-network",
			callReturnData:  map[string]interface{}{},
			callReturnError: nil,
			expectedConfig:  nil,
			expectedError:   "unexpected data structure returned in opsConfig",
		},
		{
			name:            "Mismatched type for opsConfig",
			network:         "test-network",
			callReturnData:  map[string]interface{}{"opsConfig": "invalidType"},
			callReturnError: nil,
			expectedConfig:  nil,
			expectedError:   "mismatched type for opsConfig",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock the ContractHandler
			mockContractHandler.EXPECT().Call(GetNetworkOperationsConfig, tt.network).Return(tt.callReturnData, tt.callReturnError).Times(1)

			// Call the method being tested
			config, err := dinRegistryHandler.GetNetworkOperationsConfig(tt.network)

			// Validate the result
			if tt.expectedError == "" {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedConfig, config)
			} else {
				assert.EqualError(t, err, tt.expectedError)
				assert.Nil(t, config)
			}
		})
	}
}

func TestGetAllNetworkMethodNames(t *testing.T) {
	// TestListAllMethodsByNetwork tests the GetAllMethodsByNetwork function.
	mockCtrl := gomock.NewController(t)
	mockContractHandler := NewMockIContractHandler(mockCtrl)

	dinRegistryHandler := &DinRegistryHandler{
		ContractHandler: mockContractHandler,
	}

	type smartContractData struct {
		network string
		output  map[string]interface{}
		err     error
	}

	tests := []struct {
		name              string
		smartContractData smartContractData
		output            []string
		hasErr            bool
	}{
		{
			name: "ListAllMethodsByNetwork, success",
			smartContractData: smartContractData{
				network: "0x",
				output:  map[string]interface{}{"methods": []string{"method1", "method2"}},
				err:     nil,
			},
			output: []string{"method1", "method2"},
			hasErr: false,
		},
		{
			name: "ListAllMethodsByNetwork, failure",
			smartContractData: smartContractData{
				network: "0x",
				output:  nil,
				err:     errors.New("error"),
			},
			output: nil,
			hasErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockContractHandler.EXPECT().Call(GetAllNetworkMethodNames, tt.smartContractData.network).Return(tt.smartContractData.output, tt.smartContractData.err).Times(1)

			got, err := dinRegistryHandler.GetAllNetworkMethodNames(tt.smartContractData.network)
			if (err != nil) != tt.hasErr {
				t.Errorf("ListAllMethodsByNetwork() error = %v, wantErr %v", err, tt.hasErr)
				return
			}
			if !reflect.DeepEqual(got, tt.output) {
				t.Errorf("ListAllMethodsByNetwork() = %v, want %v", got, tt.output)
			}
		})
	}
}

func TestGetAllNetworkMethods(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockContractHandler := NewMockIContractHandler(mockCtrl)

	dinRegistryHandler := &DinRegistryHandler{
		ContractHandler: mockContractHandler,
	}

	type smartContractData struct {
		network string
		output  map[string]interface{}
		err     error
	}

	tests := []struct {
		name              string
		smartContractData smartContractData
		output            []Method
		hasErr            bool
	}{
		{
			name: "GetAllMethodsByNetwork, success",
			smartContractData: smartContractData{
				network: "0x",
				output: map[string]interface{}{
					"methods": []map[string]interface{}{
						{
							"name":        "method1",
							"bit":         uint8(1),
							"deactivated": false,
						},
						{
							"name":        "method2",
							"bit":         uint8(2),
							"deactivated": false,
						},
					},
				},
				err: nil,
			},
			output: []Method{
				{
					Name:        "method1",
					Bit:         1,
					Deactivated: false,
				},
				{
					Name:        "method2",
					Bit:         2,
					Deactivated: false,
				},
			},
			hasErr: false,
		},
		{
			name: "GetAllMethodsByNetwork, failure",
			smartContractData: smartContractData{
				network: "0x",
				output:  nil,
				err:     errors.New("error"),
			},
			output: nil,
			hasErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockContractHandler.EXPECT().Call(GetAllNetworkMethods, tt.smartContractData.network).Return(tt.smartContractData.output, tt.smartContractData.err).Times(1)

			got, err := dinRegistryHandler.GetAllNetworkMethods(tt.smartContractData.network)
			if (err != nil) != tt.hasErr {
				t.Errorf("GetAllMethodsByNetwork() error = %v, wantErr %v", err, tt.hasErr)
				return
			}
			if !reflect.DeepEqual(got, tt.output) {
				t.Errorf("GetAllMethodsByNetwork() = %v, want %v", got, tt.output)
			}
		})
	}
}

func TestGetNetworkAddressByName(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockContractHandler := NewMockIContractHandler(mockCtrl)

	dinRegistryHandler := &DinRegistryHandler{
		ContractHandler: mockContractHandler,
	}

	ethgoAddress := ethgo.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")

	tests := []struct {
		name            string
		network         string
		callReturnData  interface{}
		callReturnError error
		expectedAddress *ethgo.Address
		expectedError   string
	}{
		{
			name:    "Successful case",
			network: "test-network",
			callReturnData: map[string]interface{}{
				"0": ethgoAddress,
			},
			callReturnError: nil,
			expectedAddress: &ethgoAddress,
			expectedError:   "",
		},
		{
			name:            "Failed call to ContractHandler",
			network:         "test-network",
			callReturnData:  nil,
			callReturnError: errors.New("call failed"),
			expectedAddress: nil,
			expectedError:   "failed call to DinRegistryHandler GetNetworkByName: call failed",
		},
		{
			name:            "Mismatched type for addressStruct",
			network:         "test-network",
			callReturnData:  "invalidType",
			callReturnError: nil,
			expectedAddress: nil,
			expectedError:   "mismatched type for addressStruct GetNetworkByName",
		},
		{
			name:            "Missing address in response",
			network:         "test-network",
			callReturnData:  map[string]interface{}{},
			callReturnError: nil,
			expectedAddress: nil,
			expectedError:   "unexpected data structure returned in DinRegistry GetNetworkByName",
		},
		{
			name:    "Mismatched type for serviceAddress",
			network: "test-network",
			callReturnData: map[string]interface{}{
				"0": "invalidType", // Invalid type for serviceAddress
			},
			callReturnError: nil,
			expectedAddress: nil,
			expectedError:   "mismatched type for serviceAddress",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock the ContractHandler
			mockContractHandler.EXPECT().Call(GetNetworkFromName, tt.network).Return(tt.callReturnData, tt.callReturnError).Times(1)

			// Call the method being tested
			address, err := dinRegistryHandler.GetNetworkAddressByName(tt.network)

			// Validate the result
			if tt.expectedError == "" {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedAddress, address)
			} else {
				assert.EqualError(t, err, tt.expectedError)
				assert.Nil(t, address)
			}
		})
	}
}

func TestGetAllNetworks(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockContractHandler := NewMockIContractHandler(mockCtrl)

	dinRegistryHandler := &DinRegistryHandler{
		ContractHandler: mockContractHandler,
	}

	type smartContractData struct {
		output map[string]interface{}
		err    error
	}

	tests := []struct {
		name              string
		smartContractData smartContractData
		output            []ethgo.Address
		hasErr            bool
	}{
		{
			name: "GetAllNetworks, success",
			smartContractData: smartContractData{
				output: map[string]interface{}{
					"0": []ethgo.Address{
						ethgo.HexToAddress("0x1"),
						ethgo.HexToAddress("0x2"),
					},
				},
				err: nil,
			},
			output: []ethgo.Address{
				ethgo.HexToAddress("0x1"),
				ethgo.HexToAddress("0x2"),
			},
			hasErr: false,
		},
		{
			name: "GetAllNetworks, failure",
			smartContractData: smartContractData{
				output: nil,
				err:    errors.New("error"),
			},
			output: nil,
			hasErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockContractHandler.EXPECT().Call(GetAllNetworks).Return(tt.smartContractData.output, tt.smartContractData.err).Times(1)

			got, err := dinRegistryHandler.GetAllNetworkAddresses()
			if (err != nil) != tt.hasErr {
				t.Errorf("GetAllNetworks() error = %v, wantErr %v", err, tt.hasErr)
				return
			}
			if !reflect.DeepEqual(got, tt.output) {
				t.Errorf("GetAllNetworks() = %v, want %v", got, tt.output)
			}
		})
	}
}

func TestGetNetworkCapabilities(t *testing.T) {
	// TestGetNetworkCapabilities tests the GetNetworkCapabilities function.
	mockCtrl := gomock.NewController(t)
	mockContractHandler := NewMockIContractHandler(mockCtrl)

	dinRegistryHandler := &DinRegistryHandler{
		ContractHandler: mockContractHandler,
	}

	type smartContractData struct {
		network string
		output  map[string]interface{}
		err     error
	}

	tests := []struct {
		name              string
		smartContractData smartContractData
		output            *big.Int
		hasErr            bool
	}{
		{
			name: "GetNetworkCapabilities, success",
			smartContractData: smartContractData{
				network: "0x",
				output:  map[string]interface{}{"caps": big.NewInt(0)},
				err:     nil,
			},
			output: big.NewInt(0),
			hasErr: false,
		},
		{
			name: "GetNetworkCapabilities, failure",
			smartContractData: smartContractData{
				network: "0x",
				output:  nil,
				err:     errors.New("error"),
			},
			output: nil,
			hasErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockContractHandler.EXPECT().Call(GetRegNetworkCapabilities, tt.smartContractData.network).Return(tt.smartContractData.output, tt.smartContractData.err).Times(1)

			got, err := dinRegistryHandler.GetNetworkCapabilities(tt.smartContractData.network)
			if (err != nil) != tt.hasErr {
				t.Errorf("GetNetworkCapabilities() error = %v, wantErr %v", err, tt.hasErr)
				return
			}
			if !reflect.DeepEqual(got, tt.output) {
				t.Errorf("GetNetworkCapabilities() = %v, want %v", got, tt.output)
			}
		})
	}
}

func TestGetAllProviders(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockContractHandler := NewMockIContractHandler(mockCtrl)

	dinRegistryHandler := &DinRegistryHandler{
		ContractHandler: mockContractHandler,
	}

	type smartContractData struct {
		output map[string]interface{}
		err    error
	}

	tests := []struct {
		name              string
		smartContractData smartContractData
		outputLength      int
		callsEthClient    bool
		hasErr            bool
	}{
		{
			name: "GetAllProviders, success",
			smartContractData: smartContractData{
				output: map[string]interface{}{
					"0": []ethgo.Address{
						ethgo.HexToAddress("0x1"),
						ethgo.HexToAddress("0x2"),
					},
				},
				err: nil,
			},
			outputLength:   2,
			callsEthClient: true,
			hasErr:         false,
		},
		{
			name: "GetAllProviders, failure",
			smartContractData: smartContractData{
				output: nil,
				err:    errors.New("error"),
			},
			outputLength:   0,
			callsEthClient: false,
			hasErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ethClient, err := jsonrpc.NewClient("http://localhost:8545")
			if err != nil {
				t.Errorf("NewClient() error = %v", err)
			}

			mockContractHandler.EXPECT().Call(GetAllProviders).Return(tt.smartContractData.output, tt.smartContractData.err).Times(1)
			if tt.callsEthClient {
				mockContractHandler.EXPECT().GetEthClient().Return(ethClient).Times(2)
			}

			got, err := dinRegistryHandler.GetAllProviders()
			if (err != nil) != tt.hasErr {
				t.Errorf("GetAllProviders() error = %v, wantErr %v", err, tt.hasErr)
				return
			}
			if !reflect.DeepEqual(len(got), tt.outputLength) {
				t.Errorf("GetAllProviders() = %v, want %v", len(got), tt.outputLength)
			}
		})
	}
}

func TestGetProvidersByNetwork(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockContractHandler := NewMockIContractHandler(mockCtrl)

	dinRegistryHandler := &DinRegistryHandler{
		ContractHandler: mockContractHandler,
	}

	type smartContractData struct {
		output  map[string]interface{}
		network string
		err     error
	}

	tests := []struct {
		name              string
		smartContractData smartContractData
		outputLength      int
		callsEthClient    bool
		hasErr            bool
	}{
		{
			name: "GetAllProviders, success",
			smartContractData: smartContractData{
				output: map[string]interface{}{
					"0": []ethgo.Address{
						ethgo.HexToAddress("0x1"),
						ethgo.HexToAddress("0x2"),
					},
				},
				network: "0x",
				err:     nil,
			},
			outputLength:   2,
			callsEthClient: true,
			hasErr:         false,
		},
		{
			name: "GetAllProviders, failure",
			smartContractData: smartContractData{
				output:  nil,
				network: "0x",
				err:     errors.New("error"),
			},
			outputLength:   0,
			callsEthClient: false,
			hasErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ethClient, err := jsonrpc.NewClient("http://localhost:8545")
			if err != nil {
				t.Errorf("NewClient() error = %v", err)
			}

			mockContractHandler.EXPECT().Call(GetProviders, tt.smartContractData.network).Return(tt.smartContractData.output, tt.smartContractData.err).Times(1)
			if tt.callsEthClient {
				mockContractHandler.EXPECT().GetEthClient().Return(ethClient).Times(2)
			}

			got, err := dinRegistryHandler.GetProvidersByNetwork(tt.smartContractData.network)
			if (err != nil) != tt.hasErr {
				t.Errorf("GetProvidersByNetwork() error = %v, wantErr %v", err, tt.hasErr)
				return
			}
			if !reflect.DeepEqual(len(got), tt.outputLength) {
				t.Errorf("GetProvidersByNetwork() = %v, want %v", len(got), tt.outputLength)
			}
		})
	}
}
