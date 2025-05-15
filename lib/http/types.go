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

type EVMBlockResult struct {
	Hash string `json:"hash"`
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
