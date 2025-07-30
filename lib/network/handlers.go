// lib/network/handlers.go
package network

import (
	"fmt"
	"net/http"
	"time"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
)

func init() {
	RegisterBuiltinHandlers()
}

// NetworkHandler defines the interface for handling different network types
type NetworkHandler interface {
	// === Core Identification ===
	GetType() string
	GetName() string
	GetRequestType() RequestType

	// === Request Processing ===
	ProcessRequest(req *http.Request) error

	// === Response Handling ===
	ParseResponse(body []byte, statusCode int) error
	IsRetryableError(err error, statusCode int) bool

	// === Block Operations ===
	FormatBlockHeight(blockNum int64) string
	CreateBlockRequest(method string, blockNum int64, includeTransactions bool) ([]byte, error)
	ParseBlockResponse(body []byte) (interface{}, error)
	ExtractBlockHash(blockData interface{}) string
	SupportsGetBlockByNumber() bool
	GetSupportedMethods() []string
	GetBlockByNumberMethod() string

	// === Health Check ===
	GetHealthCheckMethod() string
	GetHealthCheckHTTPMethod() string // "GET" or "POST"
	CreateHealthCheckPayload(method string) ([]byte, error)
	ParseHealthCheckResponse(body []byte) (*BlockInfo, error)

	// GetLatestBlockNumber retrieves the latest block number from the provider
	// This abstracts the entire process of getting latest block number per network type
	GetLatestBlockNumber(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int) (*LatestBlockResult, error)

	// === Separate Block Info Call (for REST APIs like Beacon) ===
	// RequiresSeparateBlockInfoCall returns true if health endpoint doesn't provide block info
	RequiresSeparateBlockInfoCall() bool
	// GetBlockInfoMethod returns the endpoint to get current block/slot information
	GetBlockInfoMethod() string

	// === Block Number Parsing ===
	// ParseBlockNumberResponse parses the raw response from a block number query
	// Returns the block number extracted from the response
	ParseBlockNumberResponse(body []byte, statusCode int) (int64, error)

	// === Chain ID Operations ===
	GetChainIDMethod() string
	ParseChainIDResponse(body []byte, statusCode int) (string, error)
	ValidateChainID(chainID string) error
	// GetChainID retrieves the chain ID from the provider
	// This abstracts the chain ID retrieval logic per network type
	GetChainID(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int) (string, error)

	// === Archive Mode ===
	SupportsArchiveMode() bool
	GetArchiveMethod() string
	CreateArchivePayload(method string, blockHeight string) ([]byte, error)
	ParseArchiveResponse(body []byte) error
	// PerformArchiveCheck performs the complete archive mode check for JSON-RPC handlers
	// This abstracts the entire archive check process per network type
	PerformArchiveCheck(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int, blockHeight string) error

	// PerformGetBlockByNumber performs the complete get block by number operation
	// This abstracts the entire get block by number process per network type
	PerformGetBlockByNumber(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int, blockNumber int64) (interface{}, error)

	// === Lifecycle ===
	Initialize(config *NetworkConfig) error
}

// BlockInfo represents block information across different network types
type BlockInfo struct {
	Number    int64     `json:"number"`
	Hash      string    `json:"hash"`
	Timestamp time.Time `json:"timestamp"`
	// For beacon chain specific fields
	Slot                int64 `json:"slot,omitempty"`
	Epoch               int64 `json:"epoch,omitempty"`
	ExecutionOptimistic bool  `json:"execution_optimistic,omitempty"`
	Finalized           bool  `json:"finalized,omitempty"`
}

// LatestBlockResult represents the result of getting the latest block number
type LatestBlockResult struct {
	BlockNumber    int64
	HealthStatus   HealthStatus
	ResponseStatus int
	// Additional context that might be useful for debugging
	Extra map[string]interface{}
}

// HealthStatus represents the health status of a provider
type HealthStatus int

const (
	Healthy HealthStatus = iota
	Warning
	Unhealthy
)

// String returns the string representation of HealthStatus
func (h HealthStatus) String() string {
	switch h {
	case Healthy:
		return "healthy"
	case Warning:
		return "warning"
	case Unhealthy:
		return "unhealthy"
	default:
		return "unknown"
	}
}

// HealthCheckResult represents the result of a health check operation
type HealthCheckResult struct {
	BlockNumber  int64
	HealthStatus int // Maps to the HealthStatus constants in modules
}

// RequestType enum for different request types
type RequestType int

const (
	RequestTypeRPC RequestType = iota
	RequestTypeREST
	RequestTypeGraphQL
)

// NetworkConfig contains configuration for a network handler
type NetworkConfig struct {
	Name           string
	Type           string
	ChainID        string
	MaxPayloadSize int64
	RequestTimeout time.Duration
	Logger         *logger.LoggerClient
	Custom         map[string]interface{}
}

// RegisterBuiltinHandlers registers all built-in network handlers
func RegisterBuiltinHandlers() {
	if err := DefaultRegistry.RegisterHandler("evm", func(config *NetworkConfig) (NetworkHandler, error) {
		return NewEVMHandler(config), nil
	}); err != nil {
		panic(fmt.Sprintf("Failed to register EVM handler: %v", err))
	}

	if err := DefaultRegistry.RegisterHandler("starknet", func(config *NetworkConfig) (NetworkHandler, error) {
		return NewStarknetHandler(config), nil
	}); err != nil {
		panic(fmt.Sprintf("Failed to register Starknet handler: %v", err))
	}

	if err := DefaultRegistry.RegisterHandler("solana", func(config *NetworkConfig) (NetworkHandler, error) {
		return NewSolanaHandler(config), nil
	}); err != nil {
		panic(fmt.Sprintf("Failed to register Solana handler: %v", err))
	}

	if err := DefaultRegistry.RegisterHandler("beacon-chain", func(config *NetworkConfig) (NetworkHandler, error) {
		return NewBeaconChainHandler(config), nil
	}); err != nil {
		panic(fmt.Sprintf("Failed to register Beacon Chain handler: %v", err))
	}
}
