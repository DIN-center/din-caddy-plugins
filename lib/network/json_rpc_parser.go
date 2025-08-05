// lib/network/json_rpc_parser.go
package network

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// JSONRPCParser provides parsing logic for JSON-RPC responses
type JSONRPCParser struct{}

// NewJSONRPCParser creates a new JSON-RPC parser
func NewJSONRPCParser() *JSONRPCParser {
	return &JSONRPCParser{}
}

// ParseResponse parses a JSON-RPC response and checks for errors
func (p *JSONRPCParser) ParseResponse(body []byte, statusCode int) error {
	// First check HTTP status
	if statusCode < 200 || statusCode >= 300 {
		return fmt.Errorf("HTTP error: %d", statusCode)
	}

	// Parse JSON-RPC response
	var response JSONRPCResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to parse JSON-RPC response: %w", err)
	}

	// Check for JSON-RPC error
	if response.Error != nil {
		return fmt.Errorf("JSON-RPC error %d: %s", response.Error.Code, response.Error.Message)
	}

	return nil
}

// ParseBlockNumberResponse parses a JSON-RPC response containing a block number
func (p *JSONRPCParser) ParseBlockNumberResponse(body []byte, statusCode int) (int64, error) {
	// Check HTTP status first
	if statusCode >= 400 {
		if statusCode == 429 {
			return 0, fmt.Errorf("rate limit error (status code: %d)", statusCode)
		}
		return 0, fmt.Errorf("error status code: %d", statusCode)
	}

	// Parse JSON-RPC response
	var response JSONRPCResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return 0, fmt.Errorf("failed to parse JSON-RPC response: %w", err)
	}

	// Check for JSON-RPC error
	if response.Error != nil {
		return 0, fmt.Errorf("JSON-RPC error %d: %s", response.Error.Code, response.Error.Message)
	}

	// Parse the result - this will be network-specific
	// Each handler should implement its own logic for extracting the block number
	return 0, fmt.Errorf("block number parsing must be implemented by specific handler")
}

// ParseHexBlockNumber parses a hex-encoded block number (common for EVM chains)
func (p *JSONRPCParser) ParseHexBlockNumber(result json.RawMessage) (int64, error) {
	var hexStr string
	if err := json.Unmarshal(result, &hexStr); err != nil {
		return 0, fmt.Errorf("failed to unmarshal block number: %w", err)
	}

	if hexStr == "" || len(hexStr) < 2 || hexStr[:2] != "0x" {
		return 0, fmt.Errorf("invalid hex block number: %s", hexStr)
	}

	blockNumber, err := strconv.ParseInt(hexStr[2:], 16, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse hex block number: %w", err)
	}

	return blockNumber, nil
}

// ParseNumericBlockNumber parses a numeric block number (common for Solana and Starknet)
func (p *JSONRPCParser) ParseNumericBlockNumber(result json.RawMessage) (int64, error) {
	var num float64
	if err := json.Unmarshal(result, &num); err != nil {
		return 0, fmt.Errorf("failed to unmarshal numeric block number: %w", err)
	}

	return int64(num), nil
}

// ParsePayload extracts method and params from a JSON-RPC payload
func (p *JSONRPCParser) ParsePayload(payload []byte) (method string, params json.RawMessage, err error) {
	var tempReq struct {
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}

	if err := json.Unmarshal(payload, &tempReq); err != nil {
		return "", nil, fmt.Errorf("failed to parse JSON-RPC payload: %w", err)
	}

	return tempReq.Method, tempReq.Params, nil
}

// CreateRequestContext creates a JSON-RPC specific request context
// This contains the JSON-RPC parsing logic moved from modules/helpers.go
func (p *JSONRPCParser) CreateRequestContext(payload []byte, method string) (map[string]interface{}, error) {
	context := make(map[string]interface{})

	// Extract params from payload for accurate JSONRPCRequest
	var params json.RawMessage
	var tempReq struct {
		Params json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(payload, &tempReq); err == nil && len(tempReq.Params) > 0 {
		params = tempReq.Params
	} else {
		// Default to empty array for requests without params
		params = json.RawMessage(`[]`)
	}

	// Store RPC-specific context
	context["jsonrpc"] = "2.0"
	context["method"] = method
	context["id"] = json.RawMessage(`1`)
	context["params"] = params

	return context, nil
}

// Package-level functions for backward compatibility

var defaultParser = NewJSONRPCParser()

// ParseJSONRPCResponse parses a JSON-RPC response and checks for errors
// This is shared logic for all JSON-RPC based handlers (EVM, Starknet, Solana)
func ParseJSONRPCResponse(body []byte, statusCode int) error {
	return defaultParser.ParseResponse(body, statusCode)
}

// ParseJSONRPCBlockNumberResponse parses a JSON-RPC response containing a block number
// This is shared logic for JSON-RPC based handlers (EVM, Starknet, Solana)
func ParseJSONRPCBlockNumberResponse(body []byte, statusCode int) (int64, error) {
	return defaultParser.ParseBlockNumberResponse(body, statusCode)
}

// ParseHexBlockNumber parses a hex-encoded block number (common for EVM chains)
func ParseHexBlockNumber(result json.RawMessage) (int64, error) {
	return defaultParser.ParseHexBlockNumber(result)
}

// ParseNumericBlockNumber parses a numeric block number (common for Solana and Starknet)
func ParseNumericBlockNumber(result json.RawMessage) (int64, error) {
	return defaultParser.ParseNumericBlockNumber(result)
}

// ParseJSONRPCPayload extracts method and params from a JSON-RPC payload
func ParseJSONRPCPayload(payload []byte) (method string, params json.RawMessage, err error) {
	return defaultParser.ParsePayload(payload)
}

// CreateJSONRPCRequestContext creates a JSON-RPC specific request context
func CreateJSONRPCRequestContext(payload []byte, method string) (map[string]interface{}, error) {
	return defaultParser.CreateRequestContext(payload, method)
}