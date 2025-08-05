package network

import (
	"net/http"
	"testing"
)

func TestSolanaHandler_GetType(t *testing.T) {
	handler := NewSolanaHandler(&NetworkConfig{})

	if handler.GetType() != "solana" {
		t.Errorf("Expected type 'solana', got '%s'", handler.GetType())
	}
}

func TestSolanaHandler_GetName(t *testing.T) {
	handler := NewSolanaHandler(&NetworkConfig{})

	expected := "Solana JSON-RPC Handler"
	if handler.GetName() != expected {
		t.Errorf("Expected name '%s', got '%s'", expected, handler.GetName())
	}
}

func TestSolanaHandler_GetRequestType(t *testing.T) {
	handler := NewSolanaHandler(&NetworkConfig{})

	if handler.GetRequestType() != RequestTypeRPC {
		t.Errorf("Expected RequestTypeRPC, got %v", handler.GetRequestType())
	}
}

func TestSolanaHandler_ValidateChainID(t *testing.T) {
	handler := NewSolanaHandler(&NetworkConfig{})

	tests := []struct {
		name    string
		chainID string
		valid   bool
	}{
		{
			name:    "valid_mainnet",
			chainID: "solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdpKuc147dw2N9d",
			valid:   true,
		},
		{
			name:    "valid_devnet",
			chainID: "solana:EtWTRABZaYq6iMfeYKouRu166VU2xqa1wcaWoxPkrZBG",
			valid:   true,
		},
		{
			name:    "invalid_prefix",
			chainID: "ethereum:0x1",
			valid:   false,
		},
		{
			name:    "invalid_format",
			chainID: "solana",
			valid:   false,
		},
		{
			name:    "empty_hash",
			chainID: "solana:",
			valid:   false,
		},
		{
			name:    "hash_too_short",
			chainID: "solana:abc123",
			valid:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handler.ValidateChainID(tt.chainID)
			if tt.valid && err != nil {
				t.Errorf("Expected valid chain ID '%s', but got error: %v", tt.chainID, err)
			}
			if !tt.valid && err == nil {
				t.Errorf("Expected invalid chain ID '%s', but no error returned", tt.chainID)
			}
		})
	}
}

func TestSolanaHandler_ValidateRequest(t *testing.T) {
	handler := NewSolanaHandler(&NetworkConfig{})

	tests := []struct {
		name        string
		method      string
		contentType string
		expectError bool
	}{
		{
			name:        "valid_request",
			method:      "POST",
			contentType: "application/json",
			expectError: false,
		},
		{
			name:        "invalid_method",
			method:      "GET",
			contentType: "application/json",
			expectError: true,
		},
		{
			name:        "invalid_content_type",
			method:      "POST",
			contentType: "text/plain",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest(tt.method, "/test", nil)
			req.Header.Set("Content-Type", tt.contentType)

			err := handler.ValidateRequest(req)
			if tt.expectError && err == nil {
				t.Errorf("Expected error for test '%s', but got none", tt.name)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error for test '%s', but got: %v", tt.name, err)
			}
		})
	}
}
