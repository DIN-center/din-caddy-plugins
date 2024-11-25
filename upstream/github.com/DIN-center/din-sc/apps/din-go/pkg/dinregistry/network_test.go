package dinregistry

import (
	"math/big"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestGetMethodId(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockContractHandler := NewMockIContractHandler(mockCtrl)

	networkHandler := &NetworkHandler{
		ContractHandler: mockContractHandler,
	}

	type smartContractData struct {
		output map[string]interface{}
		name   string
		err    error
	}

	tests := []struct {
		name              string
		smartContractData smartContractData
		output            uint8
		hasErr            bool
	}{
		{
			name: "GetMethodId, success",
			smartContractData: smartContractData{
				name: "test_name_input",
				output: map[string]interface{}{
					"0": uint8(1),
				},
				err: nil,
			},
			output: 1,
			hasErr: false,
		},
		{
			name: "GetMethodId, failure",
			smartContractData: smartContractData{
				name: "test_name_input",
				output: map[string]interface{}{
					"0": uint8(0),
				},
				err: errors.New("error"),
			},
			output: 0,
			hasErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockContractHandler.EXPECT().Call(GetMethodId, tt.smartContractData.name).Return(tt.smartContractData.output, tt.smartContractData.err).Times(1)

			got, err := networkHandler.GetMethodId(tt.smartContractData.name)
			if (err != nil) != tt.hasErr {
				t.Errorf("GetMethodId() error = %v, wantErr %v", err, tt.hasErr)
			}
			if got != tt.output {
				t.Errorf("GetMethodId() got = %v, want %v", got, tt.output)
			}
		})
	}
}

func TestGetMethodName(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockContractHandler := NewMockIContractHandler(mockCtrl)

	networkHandler := &NetworkHandler{
		ContractHandler: mockContractHandler,
	}

	type smartContractData struct {
		output map[string]interface{}
		bit    uint8
		err    error
	}

	tests := []struct {
		name              string
		smartContractData smartContractData
		output            string
		hasErr            bool
	}{
		{
			name: "GetMethodName, success",
			smartContractData: smartContractData{
				bit: 1,
				output: map[string]interface{}{
					"name": "test_name_output",
				},
				err: nil,
			},
			output: "test_name_output",
			hasErr: false,
		},
		{
			name: "GetMethodName, failure",
			smartContractData: smartContractData{
				bit: 1,
				output: map[string]interface{}{
					"name": "",
				},
				err: errors.New("error"),
			},
			output: "",
			hasErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockContractHandler.EXPECT().Call(GetMethodName, tt.smartContractData.bit).Return(tt.smartContractData.output, tt.smartContractData.err).Times(1)

			got, err := networkHandler.GetMethodName(tt.smartContractData.bit)
			if (err != nil) != tt.hasErr {
				t.Errorf("GetMethodName() error = %v, wantErr %v", err, tt.hasErr)
			}
			if got != tt.output {
				t.Errorf("GetMethodName() got = %v, want %v", got, tt.output)
			}
		})
	}
}

func TestGetCapabilities(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockContractHandler := NewMockIContractHandler(mockCtrl)

	networkHandler := &NetworkHandler{
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
					"0": big.NewInt(1),
				},
				err: nil,
			},
			output: big.NewInt(1),
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
			mockContractHandler.EXPECT().Call(GetNetworkCapabilities).Return(tt.smartContractData.output, tt.smartContractData.err).Times(1)

			got, err := networkHandler.GetCapabilities()
			if (err != nil) != tt.hasErr {
				t.Errorf("GetCapabilities() error = %v, wantErr %v", err, tt.hasErr)
			}
			if got.Cmp(tt.output) != 0 {
				t.Errorf("GetCapabilities() got = %v, want %v", got, tt.output)
			}
		})
	}
}

func TestGetAllMethods(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockContractHandler := NewMockIContractHandler(mockCtrl)

	networkHandler := &NetworkHandler{
		ContractHandler: mockContractHandler,
	}

	type smartContractData struct {
		output map[string]interface{}
		err    error
	}

	tests := []struct {
		name              string
		smartContractData smartContractData
		output            []Method
		hasErr            bool
	}{
		{
			name: "AllMethods, success",
			smartContractData: smartContractData{
				output: map[string]interface{}{
					"0": []map[string]interface{}{
						{
							"name":        "test_name",
							"bit":         uint8(1),
							"deactivated": false,
						},
					},
				},
				err: nil,
			},
			output: []Method{
				{
					Name:        "test_name",
					Bit:         1,
					Deactivated: false,
				},
			},
			hasErr: false,
		},
		{
			name: "GetAllMethods, failure",
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
			mockContractHandler.EXPECT().Call(GetAllMethods).Return(tt.smartContractData.output, tt.smartContractData.err).Times(1)

			got, err := networkHandler.GetAllMethods()
			if (err != nil) != tt.hasErr {
				t.Errorf("GetAllMethods() error = %v, wantErr %v", err, tt.hasErr)
			}
			if !reflect.DeepEqual(got, tt.output) {
				t.Errorf("GetAllMethods() = %v, want %v", got, tt.output)
			}
		})
	}
}

func TestIsMethodSupported(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockContractHandler := NewMockIContractHandler(mockCtrl)

	networkHandler := &NetworkHandler{
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
					"supported": true,
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
					"supported": false,
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

			got, err := networkHandler.IsMethodSupported(tt.smartContractData.bit)
			if (err != nil) != tt.hasErr {
				t.Errorf("IsMethodSupported() error = %v, wantErr %v", err, tt.hasErr)
			}
			if !reflect.DeepEqual(got, tt.output) {
				t.Errorf("IsMethodSupported() = %v, want %v", got, tt.output)
			}
		})
	}
}

func TestParseNetworkOperationsConfig(t *testing.T) {
	tests := []struct {
		name          string
		input         map[string]interface{}
		expected      *NetworkConfig
		expectedError error
	}{
		{
			name: "Valid input",
			input: map[string]interface{}{
				"healthcheckMethodBit":    uint8(1),
				"healthcheckIntervalSec":  uint8(30),
				"blockLagLimit":           uint8(5),
				"requestAttemptCount":     uint8(3),
				"maxRequestPayloadSizeKb": uint16(1024),
			},
			expected: &NetworkConfig{
				HealthcheckMethodBit:    1,
				HealthcheckIntervalSec:  30,
				BlockLagLimit:           5,
				RequestAttemptCount:     3,
				MaxRequestPayloadSizeKb: 1024,
			},
			expectedError: nil,
		},
		{
			name: "Missing field: healthcheckMethodBit",
			input: map[string]interface{}{
				"healthcheckIntervalSec":  uint8(30),
				"blockLagLimit":           uint8(5),
				"requestAttemptCount":     uint8(3),
				"maxRequestPayloadSizeKb": uint16(1024),
			},
			expected:      nil,
			expectedError: errors.New("mismatched type for healthcheckMethodBit"),
		},
		{
			name: "Invalid blockLagLimit type",
			input: map[string]interface{}{
				"healthcheckMethodBit":    uint8(1),
				"healthcheckIntervalSec":  uint8(30),
				"blockLagLimit":           "5", // wrong type
				"requestAttemptCount":     uint8(3),
				"maxRequestPayloadSizeKb": uint16(1024),
			},
			expected:      nil,
			expectedError: errors.New("mismatched type for blockLagLimit"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseNetworkOperationsConfig(tt.input)

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected result %+v, got %+v", tt.expected, result)
			}

			if tt.expectedError == nil && err != nil {
				t.Errorf("expected no error, but got %v", err)
			} else if tt.expectedError != nil && err == nil {
				t.Errorf("expected error %v, but got nil", tt.expectedError)
			} else if tt.expectedError != nil && err != nil && err.Error() != tt.expectedError.Error() {
				t.Errorf("expected error %v, but got %v", tt.expectedError, err)
			}
		})
	}
}

func TestGetNetworkOperationsConfig(t *testing.T) {

	mockCtrl := gomock.NewController(t)
	mockContractHandler := NewMockIContractHandler(mockCtrl)

	networkHandler := &NetworkHandler{
		ContractHandler: mockContractHandler,
	}

	tests := []struct {
		name            string
		callReturnData  interface{}
		callReturnError error
		expectedConfig  *NetworkConfig
		expectedError   string
	}{
		{
			name: "Successful case",
			callReturnData: map[string]interface{}{
				"config": map[string]interface{}{
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
			callReturnData:  nil,
			callReturnError: errors.New("call failed"),
			expectedConfig:  nil,
			expectedError:   "failed call to DinRegistryHandler GetNetworkOperationsConfig: call failed",
		},
		{
			name:            "Mismatched type for networkConfigData",
			callReturnData:  "invalidType",
			callReturnError: nil,
			expectedConfig:  nil,
			expectedError:   "mismatched type for networkConfigMap",
		},
		{
			name:            "Missing config in response",
			callReturnData:  map[string]interface{}{},
			callReturnError: nil,
			expectedConfig:  nil,
			expectedError:   "unexpected data structure returned in config",
		},
		{
			name:            "Mismatched type for config",
			callReturnData:  map[string]interface{}{"config": "invalidType"},
			callReturnError: nil,
			expectedConfig:  nil,
			expectedError:   "mismatched type for config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock the ContractHandler
			mockContractHandler.EXPECT().Call(GetNetworkOperationsConfig).Return(tt.callReturnData, tt.callReturnError).Times(1)

			// Call the method being tested
			config, err := networkHandler.GetNetworkOperationsConfig()

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

// TestGetNetworkStatus tests the GetNetworkStatus function with different scenarios
func TestGetNetworkStatus(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockContractHandler := NewMockIContractHandler(mockCtrl)

	networkHandler := &NetworkHandler{
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
			name: "successful call with Active status",
			mockResponse: map[string]interface{}{
				"status": uint8(2),
			},
			mockError:     nil,
			expected:      "Active", // Assuming Active corresponds to uint8(2)
			expectedError: "",
		},
		{
			name:          "contract call error",
			mockResponse:  nil,
			mockError:     errors.New("contract call failed"),
			expected:      "",
			expectedError: "failed call to NetworkHandler GetNetworkStatus: contract call failed",
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
				"unknown_field": uint8(1),
			},
			mockError:     nil,
			expected:      "",
			expectedError: "unexpected data structure returned in NetworkHandler GetNetworkStatus",
		},
		{
			name: "mismatched status type",
			mockResponse: map[string]interface{}{
				"status": "invalid type",
			},
			mockError:     nil,
			expected:      "",
			expectedError: "mismatched type for networkStatusTypeCode",
		},
		{
			name: "invalid status code",
			mockResponse: map[string]interface{}{
				"status": uint8(99), // Unknown status code
			},
			mockError:     nil,
			expected:      "",
			expectedError: "Invalid Network Status Code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock the ContractHandler
			mockContractHandler.EXPECT().Call(GetNetworkStatus).Return(tt.mockResponse, tt.mockError).Times(1)

			// Execute the function
			result, err := networkHandler.GetNetworkStatus()

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
