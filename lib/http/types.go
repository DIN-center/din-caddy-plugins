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
	BaseFeePerGas         string `json:"baseFeePerGas"`
	BlobGasUsed           string `json:"blobGasUsed"`
	Difficulty            string `json:"difficulty"`
	ExcessBlobGas         string `json:"excessBlobGas"`
	ExtraData             string `json:"extraData"`
	GasLimit              string `json:"gasLimit"`
	GasUsed               string `json:"gasUsed"`
	Hash                  string `json:"hash"`
	LogsBloom             string `json:"logsBloom"`
	Miner                 string `json:"miner"`
	MixHash               string `json:"mixHash"`
	Nonce                 string `json:"nonce"`
	Number                string `json:"number"`
	ParentBeaconBlockRoot string `json:"parentBeaconBlockRoot"`
	ParentHash            string `json:"parentHash"`
	ReceiptsRoot          string `json:"receiptsRoot"`
	Sha3Uncles            string `json:"sha3Uncles"`
	Size                  string `json:"size"`
	StateRoot             string `json:"stateRoot"`
	Timestamp             string `json:"timestamp"`
	TotalDifficulty       string `json:"totalDifficulty"`
	TransactionsRoot      string `json:"transactionsRoot"`
	WithdrawalsRoot       string `json:"withdrawalsRoot"`
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
	BlockHeight       int    `json:"blockHeight"`
	BlockTime         int    `json:"blockTime"`
	Blockhash         string `json:"blockhash"`
	ParentSlot        int    `json:"parentSlot"`
	PreviousBlockhash string `json:"previousBlockhash"`
}
