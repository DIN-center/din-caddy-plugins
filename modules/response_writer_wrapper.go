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

// Write captures the response body and writes it to the original ResponseWriter.
func (rww *ResponseWriterWrapper) Write(b []byte) (int, error) {
	rww.body.Write(b) // Capture the response body
	return rww.ResponseWriter.Write(b)
}

func (rww *ResponseWriterWrapper) ResetBody() {
	rww.body.Reset()
}
