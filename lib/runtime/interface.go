package runtime

import (
	"github.com/openrelayxyz/din-caddy-plugins/auth"
)

type IRuntimeClient interface {
	GetLatestBlockNumber(httpUrl string, headers map[string]string, signer auth.AuthClient) (int64, int, error)
}
