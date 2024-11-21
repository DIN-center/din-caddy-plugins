package dinregistry

import (
	"math/big"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/umbracle/ethgo"
)

func TestGetAllMethodNames(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockContractHandler := NewMockIContractHandler(mockCtrl)

	dinServiceHandler := &NetworkServiceHandler{
		ContractHandler: mockContractHandler,
	}

	type smartContractData struct {
		output map[string]interface{}
		err    error
	}

	tests := []struct {
		name              string
		smartContractData smartContractData
		output            []string
		hasErr            bool
	}{
		{
			name: "GetAllMethodNames, success",
			smartContractData: smartContractData{
				output: map[string]interface{}{
					"methods": []string{"method1", "method2"},
				},
				err: nil,
			},
			output: []string{"method1", "method2"},
			hasErr: false,
		},
		{
			name: "GetAllMethodNames, failure",
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
			mockContractHandler.EXPECT().Call(GetAllMethodNames).Return(tt.smartContractData.output, tt.smartContractData.err).Times(1)

			got, err := dinServiceHandler.GetAllMethodNames()
			if (err != nil) != tt.hasErr {
				t.Errorf("GetAllMethodNames() error = %v, wantErr %v", err, tt.hasErr)
			}
			if !reflect.DeepEqual(got, tt.output) {
				t.Errorf("GetAllMethodNames() = %v, want %v", got, tt.output)
			}
		})
	}
}

func TestNetworkServiceGetCapabilities(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockContractHandler := NewMockIContractHandler(mockCtrl)

	dinServiceHandler := &NetworkServiceHandler{
		ContractHandler: mockContractHandler,
	}

	type smartContractData struct {
		output map[string]interface{}
		err    error
	}

	tests := []struct {
		name              string
		smartContractData smartContractData
		output            *big.Int
		hasErr            bool
	}{
		{
			name: "GetCapabilities, success",
			smartContractData: smartContractData{
				output: map[string]interface{}{
					"0": big.NewInt(0),
				},
				err: nil,
			},
			output: big.NewInt(0),
			hasErr: false,
		},
		{
			name: "GetCapabilities, failure",
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
			mockContractHandler.EXPECT().Call(GetNetworkServiceCapabilities).Return(tt.smartContractData.output, tt.smartContractData.err).Times(1)

			got, err := dinServiceHandler.GetCapabilities()
			if (err != nil) != tt.hasErr {
				t.Errorf("GetCapabilities() error = %v, wantErr %v", err, tt.hasErr)
			}
			if !reflect.DeepEqual(got, tt.output) {
				t.Errorf("GetCapabilities() = %v, want %v", got, tt.output)
			}
		})
	}
}

func TestNetworkServiceIsMethodSupported(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockContractHandler := NewMockIContractHandler(mockCtrl)

	dinServiceHandler := &NetworkServiceHandler{
		ContractHandler: mockContractHandler,
	}

	type smartContractData struct {
		bit    uint8
		output map[string]interface{}
		err    error
	}

	tests := []struct {
		name              string
		smartContractData smartContractData
		output            bool
		hasErr            bool
	}{
		{
			name: "IsMethodSupported, success",
			smartContractData: smartContractData{
				bit: uint8(0),
				output: map[string]interface{}{
					"0": true,
				},
				err: nil,
			},
			output: true,
			hasErr: false,
		},
		{
			name: "IsMethodSupported, failure",
			smartContractData: smartContractData{
				bit: uint8(0),
				output: map[string]interface{}{
					"0": false,
				},
				err: errors.New("error"),
			},
			output: false,
			hasErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockContractHandler.EXPECT().Call(IsMethodSupported, uint8(0)).Return(tt.smartContractData.output, tt.smartContractData.err).Times(1)

			got, err := dinServiceHandler.IsMethodSupported(tt.smartContractData.bit)
			if (err != nil) != tt.hasErr {
				t.Errorf("IsMethodSupported() error = %v, wantErr %v", err, tt.hasErr)
			}
			if !reflect.DeepEqual(got, tt.output) {
				t.Errorf("IsMethodSupported() = %v, want %v", got, tt.output)
			}
		})
	}
}

func TestGetNetworkServiceURL(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockContractHandler := NewMockIContractHandler(mockCtrl)

	networkServiceHandler := &NetworkServiceHandler{
		ContractHandler: mockContractHandler,
	}

	tests := []struct {
		name            string
		callReturnData  interface{}
		callReturnError error
		expectedURL     string
		expectedError   string
	}{
		{
			name: "Successful case",
			callReturnData: map[string]interface{}{
				"0": "https://service.example.com",
			},
			callReturnError: nil,
			expectedURL:     "https://service.example.com",
			expectedError:   "",
		},
		{
			name:            "Failed call to ContractHandler",
			callReturnData:  nil,
			callReturnError: errors.New("call failed"),
			expectedURL:     "",
			expectedError:   "failed call to NetworkService GetServiceURL: call failed",
		},
		{
			name:            "Mismatched type for urlStruct",
			callReturnData:  "invalidType",
			callReturnError: nil,
			expectedURL:     "",
			expectedError:   "mismatched type for urlStruct",
		},
		{
			name:            "Missing URL data in response",
			callReturnData:  map[string]interface{}{},
			callReturnError: nil,
			expectedURL:     "",
			expectedError:   "unexpected data structure returned in ServiceHandler GetServiceURL",
		},
		{
			name:            "Mismatched type for serviceURL",
			callReturnData:  map[string]interface{}{"0": 12345}, // Invalid type for URL (int instead of string)
			callReturnError: nil,
			expectedURL:     "",
			expectedError:   "mismatched type for serviceURL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock the ContractHandler
			mockContractHandler.EXPECT().Call(ServiceURL).Return(tt.callReturnData, tt.callReturnError).Times(1)

			// Call the method being tested
			serviceURL, err := networkServiceHandler.GetNetworkServiceURL()

			// Validate the result
			if tt.expectedError == "" {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedURL, serviceURL)
			} else {
				assert.EqualError(t, err, tt.expectedError)
				assert.Equal(t, "", serviceURL)
			}
		})
	}
}

func TestGetNetworkAddress(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockContractHandler := NewMockIContractHandler(mockCtrl)

	networkServiceHandler := &NetworkServiceHandler{
		ContractHandler: mockContractHandler,
	}
	ethgoAddress := ethgo.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")

	tests := []struct {
		name            string
		callReturnData  interface{}
		callReturnError error
		expectedAddress *ethgo.Address
		expectedError   string
	}{
		{
			name: "Successful case",
			callReturnData: map[string]interface{}{
				"0": ethgoAddress,
			},
			callReturnError: nil,
			expectedAddress: &ethgoAddress,
			expectedError:   "",
		},
		{
			name:            "Failed call to ContractHandler",
			callReturnData:  nil,
			callReturnError: errors.New("call failed"),
			expectedAddress: nil,
			expectedError:   "failed call to NetworkService GetNetworkAddress: call failed",
		},
		{
			name:            "Mismatched type for nameStruct",
			callReturnData:  "invalidType",
			callReturnError: nil,
			expectedAddress: nil,
			expectedError:   "mismatched type for nameStruct",
		},
		{
			name:            "Missing address data in response",
			callReturnData:  map[string]interface{}{},
			callReturnError: nil,
			expectedAddress: nil,
			expectedError:   "unexpected data structure returned in NetworkService GetNetworkAddress",
		},
		{
			name:            "Mismatched type for serviceAddress",
			callReturnData:  map[string]interface{}{"0": "invalidType"}, // Invalid type for serviceAddress (string instead of ethgo.Address)
			callReturnError: nil,
			expectedAddress: nil,
			expectedError:   "mismatched type for serviceAddress",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock the ContractHandler
			mockContractHandler.EXPECT().Call(GetNetworkAddress).Return(tt.callReturnData, tt.callReturnError).Times(1)

			// Call the method being tested
			serviceAddress, err := networkServiceHandler.GetNetworkAddress()

			// Validate the result
			if tt.expectedError == "" {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedAddress, serviceAddress)
			} else {
				assert.EqualError(t, err, tt.expectedError)
				assert.Nil(t, serviceAddress)
			}
		})
	}
}

func TestGetNetworkServiceStatus(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockContractHandler := NewMockIContractHandler(mockCtrl)

	networkServiceHandler := &NetworkServiceHandler{
		ContractHandler: mockContractHandler,
	}
	tests := []struct {
		name          string
		mockResponse  interface{}
		mockError     error
		expected      string
		expectedError string
	}{
		{
			name: "successful call",
			mockResponse: map[string]interface{}{
				"status": uint8(1),
			},
			mockError:     nil,
			expected:      Onboarding,
			expectedError: "",
		},
		{
			name:          "contract call failure",
			mockResponse:  nil,
			mockError:     errors.New("contract call failed"),
			expected:      "",
			expectedError: "failed call to NetworkService GetNetworkServiceStatus: contract call failed",
		},
		{
			name: "mismatched data structure",
			mockResponse: struct {
				Field string
			}{Field: "invalid"},
			mockError:     nil,
			expected:      "",
			expectedError: "mismatched type for statusStruct",
		},
		{
			name: "missing status in response",
			mockResponse: map[string]interface{}{
				"missing_status": uint8(1),
			},
			mockError:     nil,
			expected:      "",
			expectedError: "unexpected data structure returned in NetworkService GetNetworkServiceStatus",
		},
		{
			name: "mismatched status type",
			mockResponse: map[string]interface{}{
				"status": "invalid type",
			},
			mockError:     nil,
			expected:      "",
			expectedError: "mismatched type for status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock the ContractHandler
			mockContractHandler.EXPECT().Call(GetNetworkServiceStatus).Return(tt.mockResponse, tt.mockError).Times(1)

			// Execute the function
			result, err := networkServiceHandler.GetNetworkServiceStatus()

			// Validate the results
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expected, result)
		})
	}
}
