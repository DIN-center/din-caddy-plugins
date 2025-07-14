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
	// Metadata for handler registry
	GetType() string
	GetName() string
	GetVersion() string

	// Request processing
	ProcessRequest(req *http.Request, provider Provider) error
	ValidateRequest(req *http.Request) error
	TranslatePath(gatewayPath string, provider Provider) (string, error)

	// Health checks
	GetLatestBlock(provider Provider) (*BlockInfo, error)
	CheckHealth(provider Provider) (*HealthStatus, error)

	// Metrics and normalization
	NormalizeEndpoint(path string) string
	GetRequestType() RequestType

	// Response handling
	ParseResponse(body []byte, statusCode int) error
	IsRetryableError(err error, statusCode int) bool

	// Lifecycle management
	Initialize(config *NetworkConfig) error
	Shutdown() error
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
