package network

import (
	"encoding/json"
	"testing"
)

func TestParseHexBlockNumber(t *testing.T) {
	tests := []struct {
		name     string
		input    json.RawMessage
		expected int64
		wantErr  bool
	}{
		{
			name:     "valid hex number",
			input:    json.RawMessage(`"0x10"`),
			expected: 16,
			wantErr:  false,
		},
		{
			name:     "large hex number",
			input:    json.RawMessage(`"0x1234567"`),
			expected: 19088743,
			wantErr:  false,
		},
		{
			name:     "zero hex",
			input:    json.RawMessage(`"0x0"`),
			expected: 0,
			wantErr:  false,
		},
		{
			name:     "invalid format - missing 0x",
			input:    json.RawMessage(`"123"`),
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "empty string",
			input:    json.RawMessage(`""`),
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "not a string",
			input:    json.RawMessage(`123`),
			expected: 0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseHexBlockNumber(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseHexBlockNumber() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != tt.expected {
				t.Errorf("ParseHexBlockNumber() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestParseNumericBlockNumber(t *testing.T) {
	tests := []struct {
		name     string
		input    json.RawMessage
		expected int64
		wantErr  bool
	}{
		{
			name:     "valid integer",
			input:    json.RawMessage(`123`),
			expected: 123,
			wantErr:  false,
		},
		{
			name:     "valid float",
			input:    json.RawMessage(`123.0`),
			expected: 123,
			wantErr:  false,
		},
		{
			name:     "zero",
			input:    json.RawMessage(`0`),
			expected: 0,
			wantErr:  false,
		},
		{
			name:     "not a number",
			input:    json.RawMessage(`"abc"`),
			expected: 0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseNumericBlockNumber(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseNumericBlockNumber() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != tt.expected {
				t.Errorf("ParseNumericBlockNumber() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestParseJSONRPCPayload(t *testing.T) {
	tests := []struct {
		name           string
		payload        []byte
		expectedMethod string
		expectedParams json.RawMessage
		wantErr        bool
	}{
		{
			name:           "valid payload with params",
			payload:        []byte(`{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`),
			expectedMethod: "eth_blockNumber",
			expectedParams: json.RawMessage(`[]`),
			wantErr:        false,
		},
		{
			name:           "valid payload with complex params",
			payload:        []byte(`{"jsonrpc":"2.0","method":"eth_call","params":[{"to":"0x123"},"latest"],"id":1}`),
			expectedMethod: "eth_call",
			expectedParams: json.RawMessage(`[{"to":"0x123"},"latest"]`),
			wantErr:        false,
		},
		{
			name:           "invalid JSON",
			payload:        []byte(`{invalid json}`),
			expectedMethod: "",
			expectedParams: nil,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method, params, err := ParseJSONRPCPayload(tt.payload)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseJSONRPCPayload() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if method != tt.expectedMethod {
				t.Errorf("ParseJSONRPCPayload() method = %v, want %v", method, tt.expectedMethod)
			}
			if !tt.wantErr && string(params) != string(tt.expectedParams) {
				t.Errorf("ParseJSONRPCPayload() params = %v, want %v", string(params), string(tt.expectedParams))
			}
		})
	}
}

func TestCreateJSONRPCRequestContext(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		method  string
	}{
		{
			name:    "basic request context",
			payload: []byte(`{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`),
			method:  "eth_blockNumber",
		},
		{
			name:    "request with params",
			payload: []byte(`{"jsonrpc":"2.0","method":"eth_call","params":[{"to":"0x123"}],"id":1}`),
			method:  "eth_call",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			context, err := CreateJSONRPCRequestContext(tt.payload, tt.method)
			if err != nil {
				t.Errorf("CreateJSONRPCRequestContext() error = %v", err)
				return
			}

			// Verify context contains required fields
			if context["jsonrpc"] != "2.0" {
				t.Errorf("Expected jsonrpc = 2.0, got %v", context["jsonrpc"])
			}
			if context["method"] != tt.method {
				t.Errorf("Expected method = %s, got %v", tt.method, context["method"])
			}
			if _, ok := context["id"]; !ok {
				t.Error("Expected context to contain id field")
			}
			if _, ok := context["params"]; !ok {
				t.Error("Expected context to contain params field")
			}
		})
	}
}