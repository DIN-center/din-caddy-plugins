package nftoptions

import (
	"math/big"
	"github.com/umbracle/ethgo"
	"errors"
)

type Config struct {
	Type string
	Options map[string]string
	opts INftOptions
}

func (cfg *Config) Init() (error) {
	if cfg.opts != nil {
		return nil
	}
	switch cfg.Type {
	case "ethgo":
		addr, ok := cfg.Options["contract_address"]
		if !ok {
			return errors.New("ethgo nftoptions require a contract address")
		}
		endpoint, ok := cfg.Options["endpoint"]
		if !ok {
			return errors.New("ethgo nftoptions require an endpoint")
		}
		opts, err := NewNftOptionsContract(addr, endpoint)
		if err == nil {
			cfg.opts = opts
		}
		return err
	default:
		return errors.New("unknown nftoptions type")
	}
}

func (cfg *Config) OwnerOf(tokenid *big.Int) (ethgo.Address, error) {
	err := cfg.Init()
	if err != nil {
		return ethgo.Address{}, err
	}
	return cfg.opts.OwnerOf(tokenid)
}
func (cfg *Config) GetTokenMetadata(tokenid *big.Int) (*NFTMetadata, error) {
	err := cfg.Init()
	if err != nil {
		return nil, err
	}
	return cfg.opts.GetTokenMetadata(tokenid)
}
func (cfg *Config) TokenIDsByOwner(addr ethgo.Address) ([]*big.Int, error) {
	err := cfg.Init()
	if err != nil {
		return nil, err
	}
	return cfg.opts.TokenIDsByOwner(addr) 
}