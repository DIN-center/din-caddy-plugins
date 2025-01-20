package nftoptions

import (
	"fmt"
	"net/url"
	"math/big"
	"github.com/umbracle/ethgo"
)

type NFTMetadata struct {
	// Contract Metadata
	TokenId *big.Int
	NFTAddress ethgo.Address
	ChainID *big.Int

	// Token Metadata
	ServiceId *big.Int
	RequestsPerSecondLimit uint32
	Expiration uint64
	TokenAddress ethgo.Address
	CostPerRequest *big.Int
}

func (n *NFTMetadata) URL() url.URL {
	result, _ := url.Parse(fmt.Sprintf("din://%v@%v", n.TokenId, n.NFTAddress))
	return *result
}


type INftOptions interface {
	OwnerOf(*big.Int) (ethgo.Address, error)
	GetTokenMetadata(*big.Int) (*NFTMetadata, error)
	TokenIDsByOwner(ethgo.Address) ([]*big.Int, error)
}
