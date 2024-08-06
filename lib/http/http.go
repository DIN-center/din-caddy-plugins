package http

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/openrelayxyz/din-caddy-plugins/lib/auth"
	"github.com/pkg/errors"
)

type HTTPClient struct {
	httpClient *http.Client
}

func NewHTTPClient() *HTTPClient {
	return &HTTPClient{
		httpClient: &http.Client{Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConnsPerHost:   16,
			MaxIdleConns:          16,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}},
	}
}

func (h *HTTPClient) Post(url string, headers map[string]string, payload []byte, auth auth.IAuthClient) ([]byte, *int, error) {
	// Send the POST request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, nil, errors.Wrap(err, "Error making POST request")
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if auth != nil {
		if err := auth.Sign(req); err != nil {
			return nil, nil, errors.Wrap(err, "Error authenticating POST request")
		}
	}
	res, err := h.httpClient.Do(req)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Error sending POST request")
	}
	defer res.Body.Close()

	// Read the response body
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Error reading response body")
	}

	return body, aws.Int(res.StatusCode), nil
}
