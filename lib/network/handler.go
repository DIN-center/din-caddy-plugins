// lib/network/handler.go
package network

import (
	"net/http"
	"time"
)

// Provider represents a generic provider interface for handlers
type Provider interface {
	GetURL() string
	GetHeaders() map[string]string
	GetPath() string
	GetHost() string
	GetPriority() int
}

// NetworkHandler defines the interface for handling different network types
type NetworkHandler interface {
	// === Existing Registry Methods ===
	GetType() string
	GetName() string
	GetVersion() string
	GetRequestType() RequestType

	// === Request Processing Methods ===
	ProcessRequest(req *http.Request, provider Provider) error
	ValidateRequest(req *http.Request) error
	TranslatePath(gatewayPath string, provider Provider) (string, error)
	NormalizeEndpoint(path string) string

	// === Health Check Methods ===
	GetLatestBlock(provider Provider) (*BlockInfo, error)
	CheckHealth(provider Provider) (*HealthStatus, error)

	// === Response Handling Methods ===
	ParseResponse(body []byte, statusCode int) error
	IsRetryableError(err error, statusCode int) bool

	// === Lifecycle Methods ===
	Initialize(config *NetworkConfig) error
	Shutdown() error

	// === NEW: Network-Specific Methods (Migration Target) ===

	// Chain ID and Namespace
	GetNamespace() string
	ValidateChainID(chainID string) error
	FormatChainID(networkReference string) string
	ExtractChainReference(result interface{}) (string, error)

	// Block Operations
	FormatBlockHeight(blockNum int64) string
	CreateBlockRequest(method string, blockNum int64, includeTransactions bool) ([]byte, error)
	ParseBlockResponse(body []byte) (interface{}, error)

	// Archive Mode
	SupportsArchiveMode() bool
	GetArchiveMethod() string
	CreateArchivePayload(method string, blockHeight string) ([]byte, error)
	ParseArchiveResponse(body []byte) error

	// Network Capabilities
	SupportsGetBlockByNumber() bool
	GetSupportedMethods() []string

	// Data Format Conversions
	ExtractBlockHash(blockData interface{}) string
	ExtractBlockNumber(response []byte) (int64, error)

	// Health Check Specifics
	GetHealthCheckMethod() string
	GetChainIDMethod() string
	CreateHealthCheckPayload(method string) ([]byte, error)
	ParseHealthCheckResponse(body []byte) (*BlockInfo, error)
}

// BlockInfo represents block information across different network types
type BlockInfo struct {
	Number    int64     `json:"number"`
	Hash      string    `json:"hash"`
	Timestamp time.Time `json:"timestamp"`
	// For beacon chain specific fields
	Slot  int64 `json:"slot,omitempty"`
	Epoch int64 `json:"epoch,omitempty"`
}

// HealthStatus represents the health status of a provider
type HealthStatus struct {
	Healthy     bool
	BlockNumber int64
	Latency     time.Duration
	Error       error
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
	HealthEndpoint string
	MaxPayloadSize int64
	RequestTimeout time.Duration
	Custom         map[string]interface{}
}

// PathPattern represents a path normalization pattern
type PathPattern struct {
	Regex       string
	Replacement string
	Category    string
}
