package modules

import (
	"encoding/json"
	"testing"

	dinHttp "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/caddyserver/caddy/v2"
	"github.com/stretchr/testify/assert"
)

func TestGetRequestBody(t *testing.T) {
	tests := []struct {
		name          string
		setupReplacer func(repl *caddy.Replacer)
		wantRequest   *dinHttp.JSONRPCRequest
		wantErr       bool
		expectErrStr  string
	}{
		{
			name: "Successful retrieval and unmarshal",
			setupReplacer: func(repl *caddy.Replacer) {
				req := dinHttp.JSONRPCRequest{Method: "test_method", JSONRPC: "2.0"}
				bodyBytes, _ := json.Marshal(req)
				repl.Set(RequestBodyKey, bodyBytes)
			},
			wantRequest: &dinHttp.JSONRPCRequest{
				Method:  "test_method",
				Params:  json.RawMessage("null"),
				ID:      json.RawMessage("null"),
				JSONRPC: "2.0",
			},
			wantErr: false,
		},
		{
			name: "RequestBodyKey present but value is not []byte",
			setupReplacer: func(repl *caddy.Replacer) {
				repl.Set(RequestBodyKey, "not a byte array")
			},
			wantRequest:  nil,
			wantErr:      true,
			expectErrStr: "request body is not a byte array",
		},
		{
			name: "RequestBodyKey present, value is []byte, but JSON unmarshal fails",
			setupReplacer: func(repl *caddy.Replacer) {
				repl.Set(RequestBodyKey, []byte("invalid json"))
			},
			wantRequest:  nil,
			wantErr:      true,
			expectErrStr: "failed to unmarshal request body: invalid character 'i' looking for beginning of value",
		},
		{
			name: "RequestBodyKey not present",
			setupReplacer: func(repl *caddy.Replacer) {
				// Do nothing, key is not set
			},
			wantRequest: nil,
			wantErr:     false, // Function returns nil, nil in this case
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repl := caddy.NewReplacer()
			if tt.setupReplacer != nil {
				tt.setupReplacer(repl)
			}

			gotRequest, err := getRequestBody(repl)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectErrStr != "" {
					assert.Contains(t, err.Error(), tt.expectErrStr)
				}
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantRequest, gotRequest)
		})
	}
}

func TestGetRequestMethod(t *testing.T) {
	tests := []struct {
		name          string
		setupReplacer func(repl *caddy.Replacer)
		wantMethod    string
		wantErr       bool
		expectErrStr  string
	}{
		{
			name: "Successful retrieval of method string",
			setupReplacer: func(repl *caddy.Replacer) {
				repl.Set(RequestMethodKey, "eth_call")
			},
			wantMethod: "eth_call",
			wantErr:    false,
		},
		{
			name: "RequestMethodKey present but value is not string",
			setupReplacer: func(repl *caddy.Replacer) {
				repl.Set(RequestMethodKey, 123)
			},
			wantMethod:   "",
			wantErr:      true,
			expectErrStr: "request method is not a string",
		},
		{
			name: "RequestMethodKey not present",
			setupReplacer: func(repl *caddy.Replacer) {
				// Do nothing, key is not set
			},
			wantMethod:   "",
			wantErr:      true,
			expectErrStr: "request method not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repl := caddy.NewReplacer()
			if tt.setupReplacer != nil {
				tt.setupReplacer(repl)
			}

			gotMethod, err := getRequestMethod(repl)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectErrStr != "" {
					assert.Contains(t, err.Error(), tt.expectErrStr)
				}
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantMethod, gotMethod)
		})
	}
}
