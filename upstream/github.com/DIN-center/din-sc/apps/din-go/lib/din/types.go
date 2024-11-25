package din

import (
	"math/big"

	din_reg "github.com/DIN-center/din-sc/apps/din-go/pkg/dinregistry"
)

type DinRegistryData struct {
	Networks map[string]*Network
}

type Network struct {
	Address       string
	Status        string
	Name          string
	ProxyName     string
	Methods       map[string]*din_reg.Method
	Providers     map[string]*Provider
	Capabilities  *big.Int
	NetworkConfig *din_reg.NetworkConfig
}

type Provider struct {
	Address         string
	Name            string
	Owner           string
	NetworkServices map[string]*NetworkService
	AuthConfig      *din_reg.NetworkServiceAuthConfig
}

type NetworkService struct {
	Address      string
	Status       string
	Url          string
	Capabilities *big.Int
	Methods      map[string]*din_reg.Method
}
