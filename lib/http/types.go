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

type EVMTransactionDetail struct {
	BlockHash             string        `json:"blockHash"`
	BlockNumber           string        `json:"blockNumber"`
	From                  string        `json:"from"`
	Gas                   string        `json:"gas"`
	GasPrice              string        `json:"gasPrice"`
	Hash                  string        `json:"hash"`
	Input                 string        `json:"input"`
	Nonce                 string        `json:"nonce"`
	To                    string        `json:"to"`
	TransactionIndex      string        `json:"transactionIndex"`
	Value                 string        `json:"value"`
	Type                  string        `json:"type"`
	V                     string        `json:"v"`
	R                     string        `json:"r"`
	S                     string        `json:"s"`
	SourceHash            string        `json:"sourceHash,omitempty"`
	Mint                  string        `json:"mint,omitempty"`
	DepositReceiptVersion string        `json:"depositReceiptVersion,omitempty"`
	MaxFeePerGas          string        `json:"maxFeePerGas,omitempty"`
	MaxPriorityFeePerGas  string        `json:"maxPriorityFeePerGas,omitempty"`
	AccessList            []interface{} `json:"accessList,omitempty"`
	ChainID               string        `json:"chainId,omitempty"`
	YParity               string        `json:"yParity,omitempty"`
}

type EVMWithdrawal struct {
	Index          string `json:"index"`          // A monotonically increasing index starting from 0
	ValidatorIndex string `json:"validatorIndex"` // The validator index on the consensus layer
	Address        string `json:"address"`        // The recipient address (20 bytes)
	Amount         string `json:"amount"`         // The amount withdrawn (in Gwei)
}

type EVMBlockResult struct {
	BaseFeePerGas         string                 `json:"baseFeePerGas"`
	BlobGasUsed           string                 `json:"blobGasUsed"`
	Difficulty            string                 `json:"difficulty"`
	ExcessBlobGas         string                 `json:"excessBlobGas"`
	ExtraData             string                 `json:"extraData"`
	GasLimit              string                 `json:"gasLimit"`
	GasUsed               string                 `json:"gasUsed"`
	Hash                  string                 `json:"hash"`
	LogsBloom             string                 `json:"logsBloom"`
	Miner                 string                 `json:"miner"`
	MixHash               string                 `json:"mixHash"`
	Nonce                 string                 `json:"nonce"`
	Number                string                 `json:"number"`
	ParentBeaconBlockRoot string                 `json:"parentBeaconBlockRoot"`
	ParentHash            string                 `json:"parentHash"`
	ReceiptsRoot          string                 `json:"receiptsRoot"`
	Sha3Uncles            string                 `json:"sha3Uncles"`
	Size                  string                 `json:"size"`
	StateRoot             string                 `json:"stateRoot"`
	Timestamp             string                 `json:"timestamp"`
	TotalDifficulty       string                 `json:"totalDifficulty"`
	Transactions          []EVMTransactionDetail `json:"transactions"`
	TransactionsRoot      string                 `json:"transactionsRoot"`
	Uncles                []string               `json:"uncles"`
	Withdrawals           []EVMWithdrawal        `json:"withdrawals"`
	WithdrawalsRoot       string                 `json:"withdrawalsRoot"`
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
