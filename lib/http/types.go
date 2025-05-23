package http

import (
	"encoding/json"
)

type JSONRPCRequest struct {
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	ID      json.RawMessage `json:"id"`
	JSONRPC string          `json:"jsonrpc"`
}

// JSONRPCError represents the error object in a JSON-RPC response
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// JSONRPCResponse represents a generic JSON-RPC response
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

type EVMBlockResult struct {
	Hash   string `json:"hash"`
	Number string `json:"number"`
}

type JSONRPCEVMBlockResponse struct {
	Jsonrpc string         `json:"jsonrpc"`
	ID      int            `json:"id"`
	Result  EVMBlockResult `json:"result"`
}

type JSONRPCSolanaBlockResponse struct {
	Jsonrpc string            `json:"jsonrpc"`
	ID      int               `json:"id"`
	Result  SolanaBlockResult `json:"result"`
}

type SolanaBlockResult struct {
	Blockhash string `json:"blockhash"`
}
