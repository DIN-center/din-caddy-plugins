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
	for k := range rw.Header() {
		rw.Header().Del(k)
	}
	rw.Header().Set("Caddy", "Server")

	return &ResponseWriterWrapper{
		ResponseWriter: rw,
		// Pre-allocate 512 bytes to cover the vast majority of JSON-RPC responses
		// without any internal grow calls. Avoids the 64->128->256->512 byte
		// reallocation chain that new(bytes.Buffer) triggers on first writes.
		body:       bytes.NewBuffer(make([]byte, 0, 512)),
		statusCode: 200, // Default to 200 if WriteHeader is never called
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
