package modules

import (
	"bytes"
	"net/http"
)

// ResponseWriterWrapper is a wrapper around http.ResponseWriter that captures the response body.
type ResponseWriterWrapper struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
}

// NewCustomResponseWriter creates a new CustomResponseWriter.
func NewResponseWriterWrapper(rw http.ResponseWriter) *ResponseWriterWrapper {
	return &ResponseWriterWrapper{ResponseWriter: rw, body: new(bytes.Buffer)}
}

// WriteHeader captures the status code.
func (rww *ResponseWriterWrapper) WriteHeader(statusCode int) {
	rww.statusCode = statusCode
	rww.ResponseWriter.WriteHeader(statusCode)
}

// Write captures the response body and writes it to the response writer wrapper.
// we do not write to the original response writer here because we defer this until after the request is attempted multiple times if needed.
func (rww *ResponseWriterWrapper) Write(b []byte) (int, error) {
	return rww.body.Write(b)
}
