package http

import (
	"github.com/DIN-center/din-caddy-plugins/lib/auth"
)

type IHTTPClient interface {
	Post(url string, headers map[string]string, payload []byte, auth auth.IAuthClient) ([]byte, *int, error)
}
