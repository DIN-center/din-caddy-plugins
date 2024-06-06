package http

import (
	"bytes"
	"io"
	"net/http"

	"github.com/pkg/errors"
)

type HTTPClient struct{}

func NewHTTPClient() *HTTPClient {
	return &HTTPClient{}
}

func (h *HTTPClient) Post(url string, payload []byte) ([]byte, error) {
	// Send the POST request
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return nil, errors.Wrap(err, "Error sending POST request")
	}

	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "Error reading response body")
	}

	return body, nil
}
