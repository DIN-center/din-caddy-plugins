package http

import (
	"bytes"
	"compress/gzip"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/pkg/errors"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
)

type HTTPClient struct {
	httpClient *http.Client
}

// decompressGzipIfNecessary decompresses gzip content  if the Content-Encoding header indicates gzip
func decompressGzipIfNecessary(resp *http.Response, body []byte) ([]byte, error) {
	var err error

	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		gzipReader, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return body, err // Return original body if decompression fails
		}

		defer func() {
			err = gzipReader.Close()
		}()

		decompressed, err := io.ReadAll(gzipReader)
		if err != nil {
			return body, err // Return original body if decompression fails
		}
		return decompressed, nil
	}

	return body, err
}

func NewHTTPClient(timeout time.Duration) *HTTPClient {
	return &HTTPClient{
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
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
			},
		},
	}
}

func (h *HTTPClient) Post(url string, headers map[string]string, payload []byte, auth auth.IAuthClient) ([]byte, *int, error) {
	var err error

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

	defer func() {
		err = res.Body.Close()
	}()

	// Read the response body
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Error reading response body")
	}

	// Decompress gzip if necessary
	body, err = decompressGzipIfNecessary(res, body)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Error decompressing gzip response")
	}

	return body, aws.Int(res.StatusCode), err
}

func (h *HTTPClient) Get(url string, headers map[string]string, auth auth.IAuthClient) ([]byte, *int, error) {
	var err error

	// Send the GET request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Error making GET request")
	}

	// Set headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if auth != nil {
		if err := auth.Sign(req); err != nil {
			return nil, nil, errors.Wrap(err, "Error authenticating GET request")
		}
	}
	res, err := h.httpClient.Do(req)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Error sending GET request")
	}

	defer func() {
		err = res.Body.Close()
	}()

	// Read the response body
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Error reading response body")
	}

	// Decompress gzip if necessary
	body, err = decompressGzipIfNecessary(res, body)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Error decompressing gzip response")
	}

	return body, aws.Int(res.StatusCode), err
}
