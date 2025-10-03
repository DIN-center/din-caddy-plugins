package nftoptions

import (
	"math/big"
	snft "github.com/DIN-center/din-sc/apps/din-go/lib/superfluidnft"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"errors"
	"go.uber.org/zap"
)

type Config struct {
	Type string
	Options map[string]string
	opts snft.INftOptions
	logger *zap.Logger
}

func (cfg *Config) Init() (error) {
	if cfg.opts != nil {
		return nil
	}
	switch cfg.Type {
	case "dinsc":
		addr, ok := cfg.Options["contract_address"]
		if !ok {
			return errors.New("dinsc nftoptions require a contract address")
		}
		endpoint, ok := cfg.Options["endpoint"]
		if !ok {
			return errors.New("dinsc nftoptions require an endpoint")
		}

		client, err := ethclient.Dial(endpoint)

		if err != nil {
			return err
		}

		opts, err := snft.NewSuperfluidNFTClient(common.HexToAddress(addr), client, cfg.logger)
		if err == nil {
			cfg.opts = opts
		}
		return err
	default:
		return errors.New("unknown nftoptions type")
	}
}

func (cfg *Config) OwnerOf(tokenid *big.Int) (common.Address, error) {
	err := cfg.Init()
	if err != nil {
		return common.Address{}, err
	}
	return cfg.opts.OwnerOf(tokenid)
}
func (cfg *Config) GetTokenMetadata(tokenid *big.Int) (*snft.NFTMetadata, error) {
	err := cfg.Init()
	if err != nil {
		return nil, err
	}
	return cfg.opts.GetTokenMetadata(tokenid)
}
func (cfg *Config) TokenIDsByOwner(addr common.Address) ([]*big.Int, error) {
	err := cfg.Init()
	if err != nil {
		return nil, err
	}
	return cfg.opts.TokenIDsByOwner(addr) 
}