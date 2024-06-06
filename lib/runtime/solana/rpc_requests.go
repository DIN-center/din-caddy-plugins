package solana

import (
	"encoding/json"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/openrelayxyz/din-caddy-plugins/lib/http"
	"github.com/pkg/errors"
)

type SolanaRuntimeClient struct {
	HTTPClient *http.HTTPClient
}

func NewSolanaRuntimeClient() *SolanaRuntimeClient {
	return &SolanaRuntimeClient{
		HTTPClient: http.NewHTTPClient(),
	}
}

func (e *SolanaRuntimeClient) GetLatestBlock(url string) (*int64, error) {
	// Send the POST request to get the latest block
	payload := []byte(`{"jsonrpc":"2.0", "method":"getBlockHeight","params":[],"id":1}`)
	resp, err := e.HTTPClient.Post(url, payload)
	if err != nil {
		return nil, err
	}

	// Parse the response body
	var respBody map[string]string
	err = json.Unmarshal(resp, &respBody)
	if err != nil {
		return nil, errors.Wrap(err, "Error parsing response body")
	}

	// Convert hex block number to integer
	blockNumber, err := strconv.ParseInt(respBody["result"][2:], 16, 64)
	if err != nil {
		return nil, errors.Wrap(err, "Error converting hex to int")
	}

	return aws.Int64(blockNumber), nil
}
