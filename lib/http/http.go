package http

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/pkg/errors"
)

type HTTPClient struct{}

func NewHTTPClient() *HTTPClient {
	return &HTTPClient{}
}

func (h *HTTPClient) GetLatestBlockNumber(url string, latestBlockNumberMethod string) (*int64, error) {
	payload := []byte(`{"jsonrpc":"2.0","method":"` + latestBlockNumberMethod + `","params":[],"id":1}`)

	// Send the POST request
	resp, err := h.Post(url, []byte(payload))
	if err != nil {
		return nil, errors.Wrap(err, "Error sending POST request")
	}

	// Parse the response
	var result struct {
		Jsonrpc string `json:"jsonrpc"`
		Id      int    `json:"id"`
		Result  string `json:"result"`
	}

	err = json.Unmarshal(resp, &result)
	if err != nil {
		return nil, errors.Wrap(err, "Error unmarshalling response")
	}

	// Convert the hexadecimal string to an int64
	blockNumber, err := strconv.ParseInt(result.Result[2:], 16, 64)
	if err != nil {
		return nil, errors.Wrap(err, "Error converting block number")
	}

	return aws.Int64(blockNumber), nil
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
