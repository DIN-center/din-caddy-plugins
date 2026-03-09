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
	Hash      string `json:"hash"`
	Number    string `json:"number"`
	Timestamp string `json:"timestamp"` // Hex-encoded Unix timestamp
}

type JSONRPCEVMBlockResponse struct {
	Jsonrpc string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  EVMBlockResult  `json:"result"`
}

type JSONRPCSolanaBlockResponse struct {
	Jsonrpc string            `json:"jsonrpc"`
	ID      json.RawMessage   `json:"id"`
	Result  SolanaBlockResult `json:"result"`
}

type SolanaBlockResult struct {
	Blockhash string `json:"blockhash"`
	BlockTime *int64 `json:"blockTime"` // Unix timestamp (can be null for old blocks)
}

type StellarLedger struct {
	Hash            string `json:"hash"`
	Sequence        int64  `json:"sequence"`
	LedgerCloseTime string `json:"ledgerCloseTime"`
	HeaderXdr       string `json:"headerXdr"`
	MetadataXdr     string `json:"metadataXdr"`
}

type StellarLedgerResult struct {
	Ledgers               []StellarLedger `json:"ledgers"`
	LatestLedger          int64           `json:"latestLedger"`
	LatestLedgerCloseTime int64           `json:"latestLedgerCloseTime"`
	OldestLedger          int64           `json:"oldestLedger"`
	OldestLedgerCloseTime int64           `json:"oldestLedgerCloseTime"`
	Cursor                string          `json:"cursor"`
}

type JSONRPCStellarLedgerResponse struct {
	Jsonrpc string              `json:"jsonrpc"`
	ID      json.RawMessage     `json:"id"`
	Result  StellarLedgerResult `json:"result"`
}

type JSONRPCStellarGetHealthResponse struct {
	Jsonrpc string                 `json:"jsonrpc"`
	ID      json.RawMessage        `json:"id"`
	Result  StellarGetHealthResult `json:"result"`
}

type StellarGetHealthResult struct {
	Status                string `json:"status"`
	LatestLedger          int64  `json:"latestLedger"`
	OldestLedger          int64  `json:"oldestLedger"`
	LedgerRetentionWindow int64  `json:"ledgerRetentionWindow"`
}
