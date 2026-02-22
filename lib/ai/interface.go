package ai

import (
	"context"
	"net/http"
)

// IStreamingHTTPClient defines the HTTP client interface for AI provider communication.
// It supports both buffered (non-streaming) and streaming request modes.
// All methods accept a context for cancellation propagation.
type IStreamingHTTPClient interface {
	// Post sends a request and returns the full response body.
	// Used for non-streaming requests.
	Post(ctx context.Context, url string, headers map[string]string, payload []byte) ([]byte, int, error)

	// PostStream sends a request and returns the raw http.Response.
	// Used for streaming requests where the caller reads the body incrementally.
	// Caller is responsible for closing Response.Body.
	PostStream(ctx context.Context, url string, headers map[string]string, payload []byte) (*http.Response, error)
}
