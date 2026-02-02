package network

import (
	"net/http"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
)

// Handler is the core interface that all network handlers must implement.
// This is the minimal set of methods required for any handler.
type Handler interface {
	// GetType returns the handler type identifier (e.g., "evm", "beacon-chain")
	GetType() string
	// GetName returns a human-readable name for the handler
	GetName() string
	// GetRequestType returns whether this handler uses RPC, REST, or GraphQL
	GetRequestType() RequestType
	// Initialize sets up the handler with configuration
	Initialize(config *NetworkConfig) error
}

// RequestProcessor handles incoming request validation and transformation
type RequestProcessor interface {
	// ProcessRequest validates and optionally transforms the incoming request
	ProcessRequest(req *http.Request) error
	// ExtractMethod extracts the method name from the request for logging/metrics
	ExtractMethod(req *http.Request, body []byte) (string, error)
}

// PathConfigurer handles URL path configuration for proxied requests
type PathConfigurer interface {
	// ConfigureRequestPath configures the request URL path for the upstream provider
	ConfigureRequestPath(req *http.Request, providerPath string, providerQuery string, networkName string) error
}

// ResponseHandler handles response parsing and error detection
type ResponseHandler interface {
	// ParseResponse parses the response body and returns an error if the response indicates failure
	ParseResponse(body []byte, statusCode int) error
	// IsRetryableError determines if an error should trigger a retry
	IsRetryableError(err error, statusCode int) bool
}

// HealthChecker provides health check capability for providers
type HealthChecker interface {
	// GetHealthCheckMethod returns the method/endpoint used for health checks
	GetHealthCheckMethod() string
	// GetHealthCheckHTTPMethod returns "GET" or "POST" for the health check request
	GetHealthCheckHTTPMethod() string
	// CreateHealthCheckPayload creates the request payload for health checks (nil for GET requests)
	CreateHealthCheckPayload(method string) ([]byte, error)
	// GetLatestBlockNumber retrieves the latest block number from the provider
	GetLatestBlockNumber(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int) (*LatestBlockResult, error)
}

// BlockNumberParser can parse block numbers from responses
type BlockNumberParser interface {
	// ParseBlockNumberResponse parses the block number from a raw response
	ParseBlockNumberResponse(body []byte, statusCode int) (int64, error)
}

// ChainIdentifier provides chain ID validation capability
type ChainIdentifier interface {
	// GetChainID retrieves the chain ID from the provider
	GetChainID(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int) (string, error)
	// ValidateChainID validates that the chain ID matches expected value
	ValidateChainID(chainID string) error
}

// BlockFetcher provides block retrieval capability
type BlockFetcher interface {
	// SupportsGetBlockByNumber returns true if this handler supports fetching blocks by number
	SupportsGetBlockByNumber() bool
	// GetBlockByNumberMethod returns the method/endpoint used to get blocks by number
	GetBlockByNumberMethod() string
	// CreateBlockRequest creates the request payload for fetching a block
	CreateBlockRequest(method string, blockNum int64, includeTransactions bool) ([]byte, error)
	// ParseBlockResponse parses the block data from a response
	ParseBlockResponse(body []byte) (interface{}, error)
	// FormatBlockHeight formats a block number for use in requests
	FormatBlockHeight(blockNum int64) string
	// ExtractBlockHash extracts the block hash from block data
	ExtractBlockHash(blockData interface{}) string
}

// ArchiveChecker provides archive node verification capability
type ArchiveChecker interface {
	// SupportsArchiveMode returns true if this handler supports archive mode checks
	SupportsArchiveMode() bool
	// PerformArchiveCheck verifies the provider has historical data at the specified block height
	PerformArchiveCheck(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int, blockHeight string) error
}

// DynamicBlockLagSupport provides dynamic block lag calculation capability
type DynamicBlockLagSupport interface {
	// SupportsDynamicBlockLag returns true if this handler supports dynamic block lag calculation
	SupportsDynamicBlockLag() bool
	// GetBlockTimestamp retrieves the Unix timestamp for a specific block
	GetBlockTimestamp(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int, blockNumber int64) (int64, error)
}

// SeparateBlockInfoProvider indicates the handler needs a separate call for block info
type SeparateBlockInfoProvider interface {
	// RequiresSeparateBlockInfoCall returns true if health endpoint doesn't provide complete block info
	RequiresSeparateBlockInfoCall() bool
	// GetBlockInfoMethod returns the endpoint to get additional block information
	GetBlockInfoMethod() string
}

// MethodProvider returns supported methods for the handler
type MethodProvider interface {
	// GetSupportedMethods returns the list of methods/endpoints this handler supports
	GetSupportedMethods() []string
}

// Helper functions for capability checking

// SupportsChainID checks if a handler implements ChainIdentifier
func SupportsChainID(h interface{}) bool {
	_, ok := h.(ChainIdentifier)
	return ok
}

// SupportsArchiveMode checks if a handler implements ArchiveChecker
func SupportsArchiveMode(h interface{}) bool {
	if ac, ok := h.(ArchiveChecker); ok {
		return ac.SupportsArchiveMode()
	}
	return false
}

// SupportsDynamicBlockLag checks if a handler implements DynamicBlockLagSupport
func SupportsDynamicBlockLag(h interface{}) bool {
	if dbl, ok := h.(DynamicBlockLagSupport); ok {
		return dbl.SupportsDynamicBlockLag()
	}
	return false
}

// SupportsBlockFetching checks if a handler implements BlockFetcher
func SupportsBlockFetching(h interface{}) bool {
	if bf, ok := h.(BlockFetcher); ok {
		return bf.SupportsGetBlockByNumber()
	}
	return false
}
