package starknet

import (
	"encoding/json"
	"strconv"

	"github.com/openrelayxyz/din-caddy-plugins/lib/http"
	"github.com/pkg/errors"
)

type StarknetClient struct {
	HTTPClient *http.HTTPClient
}

func NewStarknetClient(httpClient *http.HTTPClient) *StarknetClient {
	return &StarknetClient{
		HTTPClient: httpClient,
	}
}

func (e *StarknetClient) GetLatestBlockNumber(httpUrl string, headers map[string]string) (int64, int, error) {
	payload := []byte(`{"jsonrpc":"2.0","method"starknet_blockNumber","params":[],"id":1}`)

	// Send the POST request
	resBytes, statusCode, err := e.HTTPClient.Post(httpUrl, headers, []byte(payload))
	if err != nil {
		return 0, 0, errors.Wrap(err, "Error sending POST request")
	}

	// response struct
	var result struct {
		Jsonrpc string `json:"jsonrpc"`
		Id      int    `json:"id"`
		Result  string `json:"result"`
	}

	// Unmarshal the response
	err = json.Unmarshal(resBytes, &result)
	if err != nil {
		return 0, 0, errors.Wrap(err, "Error unmarshalling response")
	}

	// Convert the hexadecimal string to an int64
	blockNumber, err := strconv.ParseInt(result.Result[2:], 16, 64)
	if err != nil {
		return 0, 0, errors.Wrap(err, "Error converting block number")
	}

	return blockNumber, *statusCode, nil
}