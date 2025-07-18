package modules

import (
	"fmt"

	netlib "github.com/DIN-center/din-caddy-plugins/lib/network"
	"go.uber.org/zap"
)

// ResponseProcessor handles response processing using network-specific handlers
type ResponseProcessor struct {
	handler netlib.NetworkHandler
	logger  *zap.Logger
}

// NewResponseProcessor creates a new response processor for a given handler
func NewResponseProcessor(handler netlib.NetworkHandler, logger *zap.Logger) *ResponseProcessor {
	return &ResponseProcessor{
		handler: handler,
		logger:  logger,
	}
}

// ProcessResponse processes the response using the network handler's ParseResponse method
func (rp *ResponseProcessor) ProcessResponse(body []byte, statusCode int) error {
	err := rp.handler.ParseResponse(body, statusCode)

	return err
}

// IsRetryableError determines if an error is retryable using the network handler
func (rp *ResponseProcessor) IsRetryableError(err error, statusCode int) bool {
	retryable := rp.handler.IsRetryableError(err, statusCode)
	return retryable
}

// CheckForApplicationError checks for application-level errors in the response
// For JSON-RPC: checks for JSON-RPC errors
// For REST: relies on HTTP status codes and handler validation
func (rp *ResponseProcessor) CheckForApplicationError(responseBody []byte, statusCode int, reqContext *RequestProcessor) error {
	// For successful HTTP responses, check for application-level errors
	if statusCode >= 200 && statusCode < 300 {
		// Let the handler check for application-specific errors
		return rp.ProcessResponse(responseBody, statusCode)
	}

	// For non-2xx responses, create an HTTP error
	return fmt.Errorf("HTTP error: %d", statusCode)
}
