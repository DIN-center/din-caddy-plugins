package dinregistry

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/umbracle/ethgo"
)

func TestName(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockContractHandler := NewMockIContractHandler(mockCtrl)

	providerHandler := &ProviderHandler{
		ContractHandler: mockContractHandler,
	}

	type smartContractData struct {
		output map[string]interface{}
		err    error
	}

	tests := []struct {
		name              string
		smartContractData smartContractData
		output            string
		hasErr            bool
	}{
		{
			name: "Name, success",
			smartContractData: smartContractData{
				output: map[string]interface{}{
					"0": "test_name",
				},
				err: nil,
			},
			output: "test_name",
			hasErr: false,
		},
		{
			name: "Name, failure",
			smartContractData: smartContractData{
				output: nil,
				err:    errors.New("error"),
			},
			output: "",
			hasErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockContractHandler.EXPECT().Call(Name).Return(tt.smartContractData.output, tt.smartContractData.err).Times(1)

			got, err := providerHandler.GetName()
			if (err != nil) != tt.hasErr {
				t.Errorf("Name() error = %v, wantErr %v", err, tt.hasErr)
			}
			if got != tt.output {
				t.Errorf("Name() got = %v, want %v", got, tt.output)
			}
		})
	}
}

func TestProviderOwner(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockContractHandler := NewMockIContractHandler(mockCtrl)

	providerHandler := &ProviderHandler{
		ContractHandler: mockContractHandler,
	}

	type smartContractData struct {
		output map[string]interface{}
		err    error
	}

	address1 := ethgo.HexToAddress("0x0000000000000000000000000000000000000123")

	tests := []struct {
		name              string
		smartContractData smartContractData
		output            *ethgo.Address
		hasErr            bool
	}{
		{
			name: "ProviderOwner, success",
			smartContractData: smartContractData{
				output: map[string]interface{}{
					"0": address1,
				},
				err: nil,
			},
			output: &address1,
			hasErr: false,
		},
		{
			name: "ProviderOwner, failure",
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
			mockContractHandler.EXPECT().Call(ProviderOwner).Return(tt.smartContractData.output, tt.smartContractData.err).Times(1)

			got, err := providerHandler.GetProviderOwner()
			if (err != nil) != tt.hasErr {
				t.Errorf("Owner() error = %v, wantErr %v", err, tt.hasErr)
			}
			if tt.hasErr {
				assert.Nil(t, got)
			} else {
				assert.Equal(t, tt.output, got)
			}
		})
	}
}

func TestGetAllNetworkServiceAddresses(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockContractHandler := NewMockIContractHandler(mockCtrl)

	providerHandler := &ProviderHandler{
		ContractHandler: mockContractHandler,
	}

	tests := []struct {
		name              string
		callReturnData    interface{}
		callReturnError   error
		expectedAddresses []ethgo.Address
		expectedError     string
	}{
		{
			name: "Successful case",
			callReturnData: map[string]interface{}{
				"allServices": []ethgo.Address{
					ethgo.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"),
					ethgo.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcdef"),
				},
			},
			callReturnError: nil,
			expectedAddresses: []ethgo.Address{
				ethgo.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"),
				ethgo.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcdef"),
			},
			expectedError: "",
		},
		{
			name:              "Failed call to ContractHandler",
			callReturnData:    nil,
			callReturnError:   errors.New("call failed"),
			expectedAddresses: nil,
			expectedError:     "failed call to ProviderHandler AllServices Call: call failed",
		},
		{
			name:              "Mismatched type for allServicesStruct",
			callReturnData:    "invalidType",
			callReturnError:   nil,
			expectedAddresses: nil,
			expectedError:     "mismatched type for allServicesStruct",
		},
		{
			name:              "Missing data in response",
			callReturnData:    map[string]interface{}{},
			callReturnError:   nil,
			expectedAddresses: nil,
			expectedError:     "unexpected data structure returned in ProviderHandler AllServices",
		},
		{
			name: "Mismatched type for serviceAddresses",
			callReturnData: map[string]interface{}{
				"allServices": "invalidType", // Invalid type for serviceAddresses (string instead of []ethgo.Address)
			},
			callReturnError:   nil,
			expectedAddresses: nil,
			expectedError:     "mismatched type for serviceAddresses",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock the ContractHandler
			mockContractHandler.EXPECT().Call(GetAllNetworkServices).Return(tt.callReturnData, tt.callReturnError).Times(1)

			// Call the method being tested
			serviceAddresses, err := providerHandler.GetAllNetworkServiceAddresses()

			// Validate the result
			if tt.expectedError == "" {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedAddresses, serviceAddresses)
			} else {
				assert.EqualError(t, err, tt.expectedError)
				assert.Nil(t, serviceAddresses)
			}
		})
	}
}

func TestGetAuthConfig(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockContractHandler := NewMockIContractHandler(mockCtrl)

	providerHandler := &ProviderHandler{
		ContractHandler: mockContractHandler,
	}

	tests := []struct {
		name               string
		callReturnData     interface{}
		callReturnError    error
		expectedAuthConfig *NetworkServiceAuthConfig
		expectedError      string
	}{
		{
			name: "Successful case",
			callReturnData: map[string]interface{}{
				"auth": uint8(1),
				"url":  "https://auth.example.com",
			},
			callReturnError: nil,
			expectedAuthConfig: &NetworkServiceAuthConfig{
				Type: SIWE,
				Url:  "https://auth.example.com",
			},
			expectedError: "",
		},
		{
			name:               "Failed call to ContractHandler",
			callReturnData:     nil,
			callReturnError:    errors.New("call failed"),
			expectedAuthConfig: nil,
			expectedError:      "failed call to ProviderHandler AuthConfig: call failed",
		},
		{
			name:               "Mismatched type for authConfigStruct",
			callReturnData:     "invalidType",
			callReturnError:    nil,
			expectedAuthConfig: nil,
			expectedError:      "mismatched type for authConfigStruct",
		},
		{
			name:               "Missing authConfig data in response",
			callReturnData:     map[string]interface{}{},
			callReturnError:    nil,
			expectedAuthConfig: nil,
			expectedError:      "mismatched type for auth",
		},
		{
			name: "Mismatched type for auth",
			callReturnData: map[string]interface{}{
				"auth": "invalidType", // Invalid type for auth
				"url":  "https://auth.example.com",
			},
			callReturnError:    nil,
			expectedAuthConfig: nil,
			expectedError:      "mismatched type for auth",
		},
		{
			name: "Mismatched type for authUrl",
			callReturnData: map[string]interface{}{
				"auth": uint8(1),
				"url":  123, // Invalid type for URL
			},
			callReturnError:    nil,
			expectedAuthConfig: nil,
			expectedError:      "mismatched type for authUrl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock the ContractHandler
			mockContractHandler.EXPECT().Call(AuthConfig).Return(tt.callReturnData, tt.callReturnError).Times(1)

			// Call the method being tested
			authConfig, err := providerHandler.GetAuthConfig()

			// Validate the result
			if tt.expectedError == "" {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedAuthConfig, authConfig)
			} else {
				assert.EqualError(t, err, tt.expectedError)
				assert.Nil(t, authConfig)
			}
		})
	}
}
