package http

import (
	"encoding/json"
)

type JSONRPCRequest struct {
	Method  string          `json:"method"`
	Params  json.RawMessage   `json:"params"`
	ID      json.RawMessage `json:"id"`
	JSONRPC string          `json:"jsonrpc"`
}
