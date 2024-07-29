package http

import (
	"github.com/openrelayxyz/din-caddy-plugins/auth"
)

type IHTTPClient interface {
	Post(url string, headers map[string]string, payload []byte, auth auth.AuthClient) ([]byte, *int, error)
}
