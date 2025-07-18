package modules

import (
	"bytes"
	"net/http"
)

type ResponseWriterWrapper struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
}

func NewResponseWriterWrapper(rw http.ResponseWriter) *ResponseWriterWrapper {
	// NOTE: We should NOT delete headers here! The upstream proxy will set
	// important headers like Content-Type, Content-Length, etc. that need to
	// be preserved for the response to work properly.
	for k := range rw.Header() {
		rw.Header().Del(k)
	}
	rw.Header().Set("Caddy", "Server")

	return &ResponseWriterWrapper{
		ResponseWriter: rw,
		body:           new(bytes.Buffer),
		statusCode:     200, // Default to 200 if WriteHeader is never called
	}
}

func (rww *ResponseWriterWrapper) WriteHeader(statusCode int) {
	rww.statusCode = statusCode
	// Note: We DON'T call the underlying WriteHeader here because we want to
	// capture the response and write it later after potential retries
}

func (rww *ResponseWriterWrapper) Write(b []byte) (int, error) {
	return rww.body.Write(b)
}
