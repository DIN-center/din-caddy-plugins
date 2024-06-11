package http

import (
	"bytes"
	"io"
	"net/http"

	"github.com/pkg/errors"
)

type HTTPClient struct {
	httpClient *http.Client
}

func NewHTTPClient() *HTTPClient {
	return &HTTPClient{
		httpClient: &http.Client{},
	}
}

func (h *HTTPClient) Post(url string, headers map[string]string, payload []byte) ([]byte, error) {
	// Send the POST request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, errors.Wrap(err, "Error making POST request")
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := h.httpClient.Do(req)
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
