// lib/network/handler_test.go
package network

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
)

// Mock provider for testing
type MockProvider struct {
	url      string
	headers  map[string]string
	path     string
	host     string
	priority int
}

func (m *MockProvider) GetURL() string                { return m.url }
func (m *MockProvider) GetHeaders() map[string]string { return m.headers }
func (m *MockProvider) GetPath() string               { return m.path }
func (m *MockProvider) GetHost() string               { return m.host }
func (m *MockProvider) GetPriority() int              { return m.priority }

var _ NetworkHandler = (*MockHandler)(nil)

// Mock handler for testing
type MockHandler struct {
	handlerType string
	name        string
	version     string
	requestType RequestType
	initialized bool
}

func (m *MockHandler) GetType() string                        { return m.handlerType }
func (m *MockHandler) GetName() string                        { return m.name }
func (m *MockHandler) GetVersion() string                     { return m.version }
func (m *MockHandler) GetRequestType() RequestType            { return m.requestType }
func (m *MockHandler) Initialize(config *NetworkConfig) error { m.initialized = true; return nil }
func (m *MockHandler) Shutdown() error                        { return nil }

func (m *MockHandler) ProcessRequest(req *http.Request) error {
	return nil
}

func (m *MockHandler) ExtractMethod(req *http.Request, body []byte) (string, error) {
	return "mock_method", nil
}

func (m *MockHandler) ConfigureRequestPath(req *http.Request, providerPath string, networkName string) error {
	// Mock implementation - just return nil
	return nil
}

func (m *MockHandler) ParseResponse(body []byte, statusCode int) error {
	return nil
}

func (m *MockHandler) IsRetryableError(err error, statusCode int) bool {
	return false
}

// === NEW: Network-Specific Methods (Mock implementations) ===

// Chain ID and Namespace
func (m *MockHandler) GetNamespace() string {
	return "mock"
}

func (m *MockHandler) ValidateChainID(chainID string) error {
	return nil
}

func (m *MockHandler) GetChainID(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int) (string, error) {
	return "mock:1", nil
}

func (m *MockHandler) ExtractChainReference(result interface{}) (string, error) {
	return "mockchain", nil
}

// Block Operations
func (m *MockHandler) FormatBlockHeight(blockNum int64) string {
	return "12345"
}

func (m *MockHandler) CreateBlockRequest(method string, blockNum int64, includeTransactions bool) ([]byte, error) {
	return []byte(`{"jsonrpc":"2.0","method":"mock_method","id":1}`), nil
}

func (m *MockHandler) ParseBlockResponse(body []byte) (interface{}, error) {
	return map[string]interface{}{"mock": "response"}, nil
}

// Archive Mode
func (m *MockHandler) SupportsArchiveMode() bool {
	return true
}

func (m *MockHandler) GetArchiveMethod() string {
	return "mock_archive"
}

func (m *MockHandler) CreateArchivePayload(method string, blockHeight string) ([]byte, error) {
	return []byte(`{"jsonrpc":"2.0","method":"mock_archive","id":1}`), nil
}

func (m *MockHandler) ParseArchiveResponse(body []byte) error {
	return nil
}

// Network Capabilities
func (m *MockHandler) SupportsGetBlockByNumber() bool {
	return true
}

func (m *MockHandler) GetSupportedMethods() []string {
	return []string{"mockMethod"}
}

func (m *MockHandler) GetBlockByNumberMethod() string {
	return "mockGetBlockByNumber"
}

// Data Format Conversions
func (m *MockHandler) ExtractBlockHash(blockData interface{}) string {
	return "0xmockhash"
}

func (m *MockHandler) ExtractBlockNumber(response []byte) (int64, error) {
	return 12345, nil
}

// Health Check Specifics
func (m *MockHandler) GetHealthCheckMethod() string {
	return "mock_health"
}

func (m *MockHandler) GetHealthCheckHTTPMethod() string {
	return "POST" // Mock handler defaults to POST
}

func (m *MockHandler) GetChainIDMethod() string {
	return "mock_chainid"
}

func (m *MockHandler) CreateHealthCheckPayload(method string) ([]byte, error) {
	return []byte(`{"jsonrpc":"2.0","method":"mock_health","id":1}`), nil
}

func (m *MockHandler) ParseHealthCheckResponse(body []byte) (*BlockInfo, error) {
	return &BlockInfo{Number: 12345}, nil
}

func (m *MockHandler) GetLatestBlockNumber(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int) (*LatestBlockResult, error) {
	return &LatestBlockResult{
		BlockNumber:    12345,
		HealthStatus:   Healthy,
		ResponseStatus: 200,
		Metadata:       make(map[string]interface{}),
	}, nil
}

func (m *MockHandler) PerformArchiveCheck(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int, blockHeight string) error {
	// Mock implementation for testing
	return nil
}

func (m *MockHandler) PerformGetBlockByNumber(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int, blockNumber int64) (interface{}, error) {
	// Mock implementation for testing - return a simple block object
	return map[string]interface{}{
		"number": blockNumber,
		"hash":   "0x123abc",
	}, nil
}

func (m *MockHandler) ParseBlockNumberResponse(body []byte, statusCode int) (int64, error) {
	if statusCode >= 400 {
		return 0, fmt.Errorf("error status code: %d", statusCode)
	}
	return 12345, nil
}

func (m *MockHandler) ParseChainIDResponse(body []byte, statusCode int) (string, error) {
	if statusCode >= 400 {
		return "", fmt.Errorf("error status code: %d", statusCode)
	}
	return "mock:1", nil
}

func (m *MockHandler) RequiresSeparateBlockInfoCall() bool {
	return false
}

func (m *MockHandler) GetBlockInfoMethod() string {
	return "mock_blockinfo"
}

func TestHandlerRegistry_RegisterHandler(t *testing.T) {
	registry := NewHandlerRegistry()

	// Test successful registration
	factory := func(config *NetworkConfig) (NetworkHandler, error) {
		return &MockHandler{
			handlerType: "test",
			name:        "Test Handler",
			version:     "1.0.0",
		}, nil
	}

	err := registry.RegisterHandler("test", factory)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test duplicate registration (should be allowed)
	err = registry.RegisterHandler("test", factory)
	if err != nil {
		t.Errorf("Expected no error for duplicate registration, got %v", err)
	}
}

func TestHandlerRegistry_GetHandler(t *testing.T) {
	registry := NewHandlerRegistry()

	// Register a test handler
	factory := func(config *NetworkConfig) (NetworkHandler, error) {
		return &MockHandler{
			handlerType: "test",
			name:        "Test Handler",
			version:     "1.0.0",
		}, nil
	}

	err := registry.RegisterHandler("test", factory)
	if err != nil {
		t.Fatalf("Failed to register handler: %v", err)
	}

	// Test getting handler
	config := &NetworkConfig{
		Name: "test-network",
		Type: "test",
	}

	handler, err := registry.GetHandler("test", config)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if handler.GetType() != "test" {
		t.Errorf("Expected handler type 'test', got %s", handler.GetType())
	}

	// Test getting the same handler again (should return cached)
	handler2, err := registry.GetHandler("test", config)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if handler != handler2 {
		t.Error("Expected cached handler to be returned")
	}

	// Test getting non-existent handler
	nonExistentConfig := &NetworkConfig{
		Name: "nonexistent-network",
		Type: "nonexistent",
	}
	_, err = registry.GetHandler("nonexistent", nonExistentConfig)
	if err == nil {
		t.Error("Expected error for non-existent handler, got none")
	}
}

func TestHandlerRegistry_ListHandlers(t *testing.T) {
	registry := NewHandlerRegistry()

	// Register multiple handlers
	handlers := []string{"test1", "test2", "test3"}
	for _, handlerType := range handlers {
		factory := func(config *NetworkConfig) (NetworkHandler, error) {
			return &MockHandler{handlerType: handlerType}, nil
		}
		err := registry.RegisterHandler(handlerType, factory)
		if err != nil {
			t.Fatalf("Failed to register handler %s: %v", handlerType, err)
		}
	}

	// Test listing handlers
	listed := registry.ListHandlers()
	if len(listed) != len(handlers) {
		t.Errorf("Expected %d handlers, got %d", len(handlers), len(listed))
	}

	// Check that all handlers are listed
	for _, expected := range handlers {
		found := false
		for _, actual := range listed {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected handler %s not found in list", expected)
		}
	}
}

func TestHandlerRegistry_GetHandlerInfo(t *testing.T) {
	registry := NewHandlerRegistry()

	// Register a test handler
	factory := func(config *NetworkConfig) (NetworkHandler, error) {
		return &MockHandler{
			handlerType: "test",
			name:        "Test Handler",
			version:     "1.0.0",
		}, nil
	}

	err := registry.RegisterHandler("test", factory)
	if err != nil {
		t.Fatalf("Failed to register handler: %v", err)
	}

	// Test getting handler info
	info, err := registry.GetHandlerInfo("test")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if info.Type != "test" {
		t.Errorf("Expected type 'test', got %s", info.Type)
	}

	if info.Name != "Test Handler" {
		t.Errorf("Expected name 'Test Handler', got %s", info.Name)
	}

}

func TestHandlerRegistry_ShutdownAll(t *testing.T) {
	registry := NewHandlerRegistry()

	// Register and create handlers
	factory := func(config *NetworkConfig) (NetworkHandler, error) {
		return &MockHandler{handlerType: "test"}, nil
	}

	err := registry.RegisterHandler("test", factory)
	if err != nil {
		t.Fatalf("Failed to register handler: %v", err)
	}

	// Create some handlers
	config1 := &NetworkConfig{Name: "test1", Type: "test"}
	config2 := &NetworkConfig{Name: "test2", Type: "test"}

	_, err = registry.GetHandler("test", config1)
	if err != nil {
		t.Fatalf("Failed to get handler: %v", err)
	}

	_, err = registry.GetHandler("test", config2)
	if err != nil {
		t.Fatalf("Failed to get handler: %v", err)
	}

	// Shutdown all handlers
	err = registry.ShutdownAll()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify handlers are cleared
	if len(registry.handlers) != 0 {
		t.Errorf("Expected no handlers after shutdown, got %d", len(registry.handlers))
	}
}

func TestEVMHandler_Basic(t *testing.T) {
	config := &NetworkConfig{
		Name: "test-evm",
		Type: "evm",
	}

	handler := NewEVMHandler(config)

	// Test metadata
	if handler.GetType() != "evm" {
		t.Errorf("Expected type 'evm', got %s", handler.GetType())
	}

	if handler.GetName() != "EVM JSON-RPC Handler" {
		t.Errorf("Expected name 'EVM JSON-RPC Handler', got %s", handler.GetName())
	}

	if handler.GetRequestType() != RequestTypeRPC {
		t.Errorf("Expected request type RPC, got %d", handler.GetRequestType())
	}

	// Test initialization
	err := handler.Initialize(config)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test shutdown
	err = handler.Shutdown()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestBeaconChainHandler_Basic(t *testing.T) {
	config := &NetworkConfig{
		Name: "test-beacon",
		Type: "beacon-chain",
	}

	handler := NewBeaconChainHandler(config)

	// Test metadata
	if handler.GetType() != "beacon-chain" {
		t.Errorf("Expected type 'beacon-chain', got %s", handler.GetType())
	}

	if handler.GetName() != "Ethereum Beacon Chain Handler" {
		t.Errorf("Expected name 'Ethereum Beacon Chain Handler', got %s", handler.GetName())
	}

	if handler.GetRequestType() != RequestTypeREST {
		t.Errorf("Expected request type REST, got %d", handler.GetRequestType())
	}

	// Test initialization
	err := handler.Initialize(config)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test shutdown
	err = handler.Shutdown()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestDefaultRegistry_Initialization(t *testing.T) {
	// Test that default registry is initialized with built-in handlers
	handlers := DefaultRegistry.ListHandlers()

	expectedHandlers := []string{"evm", "beacon-chain", "starknet", "solana", "bitcoin", "bitcoin-esplora"}

	if len(handlers) != len(expectedHandlers) {
		t.Errorf("Expected %d handlers, got %d", len(expectedHandlers), len(handlers))
	}

	for _, expected := range expectedHandlers {
		found := false
		for _, actual := range handlers {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected handler %s not found in default registry", expected)
		}
	}
}
