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
	// delete all leftover headers and reset the Caddy header
	for k := range rw.Header() {
		rw.Header().Del(k)
	}
	rw.Header().Set("Caddy", "Server")
	return &ResponseWriterWrapper{
		ResponseWriter: rw,
		body:           new(bytes.Buffer),
	}
}

func (rww *ResponseWriterWrapper) WriteHeader(statusCode int) {
	rww.statusCode = statusCode
}

func (rww *ResponseWriterWrapper) Write(b []byte) (int, error) {
	return rww.body.Write(b)
}
