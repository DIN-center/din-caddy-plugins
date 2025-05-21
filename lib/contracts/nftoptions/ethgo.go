package nftoptions

import (
	"errors"
	"fmt"
	"math/big"
	// "net/url"
	"github.com/umbracle/ethgo"
	"github.com/umbracle/ethgo/abi"
	"github.com/umbracle/ethgo/contract"
	"github.com/umbracle/ethgo/jsonrpc"
	// "fmt"
	// "reflect"
)

type NftOptionsContract struct {
	contract *contract.Contract
	address ethgo.Address
	chainid  *big.Int
}

var functions = []string{
	"function balanceOf(address owner) external view returns (uint256)",
	"function tokenOfOwnerByIndex(address owner, uint256 index) external view returns (uint256)",
	"function getTokenMetadata(uint256 tokenId) external view returns (tuple(uint256, uint32, uint64, address, uint256))",
	"function ownerOf(uint256 _tokenId) external view returns (address)",
}

func getAndCast[T any](inMap map[string]interface{}, key string) (T, error) {
	var zero T
	if v, ok := inMap[key]; ok {
		if cv, ok := v.(T); ok {
			return cv, nil
		}
		return zero, errors.New("type mismatch")
	}
	return zero, errors.New("key missing")
}

func NewNftOptionsContract(addr string, endpoint string) (*NftOptionsContract, error) {
	abiContract, err := abi.NewABIFromList(functions)
	if err != nil {
		return nil, err
	}
	client, err := jsonrpc.NewClient(endpoint)
	if err != nil {
		return nil, err
	}
	chainid, err := client.Eth().ChainID()
	if err != nil {
		return nil, err
	}

	c := contract.NewContract(ethgo.HexToAddress(addr), abiContract, contract.WithJsonRPC(client.Eth()))
	return &NftOptionsContract{contract: c, address: ethgo.HexToAddress(addr), chainid: chainid}, nil
}

func (c *NftOptionsContract) OwnerOf(tokenId *big.Int) (ethgo.Address, error) {
	res, err := c.contract.Call("ownerOf", ethgo.Latest, tokenId)
	if err != nil {
		return ethgo.Address{}, err
	}
	v, err := getAndCast[ethgo.Address](res, "0")
	if err != nil {
		fmt.Println(res)
	}
	return v, err
}
func (c *NftOptionsContract) GetTokenMetadata(tokenId *big.Int) (*NFTMetadata, error) {
	res, err := c.contract.Call("getTokenMetadata", ethgo.Latest, tokenId)
	if err != nil {
		return nil, nil
	}
	res, err = getAndCast[map[string]interface{}](res, "0")
	if err != nil {
		return nil, fmt.Errorf("Tuple: %v", err)
	}
	v := &NFTMetadata{
		TokenId: tokenId,
		NFTAddress: c.address,
		ChainID: c.chainid,
	}
	v.ServiceId, err = getAndCast[*big.Int](res, "0")
	if err != nil {
		return nil, fmt.Errorf("ServiceId: %v", err)
	}
	v.RequestsPerSecondLimit, err = getAndCast[uint32](res, "1")
	if err != nil {
		return nil, fmt.Errorf("RequestsPerSecondLimit: %v", err)
	}
	v.Expiration, err = getAndCast[uint64](res, "2")
	if err != nil {
		return nil, fmt.Errorf("ServExpirationiceId: %v", err)
	}
	v.TokenAddress, err = getAndCast[ethgo.Address](res, "3")
	if err != nil {
		return nil, fmt.Errorf("TokenAddress: %v", err)
	}
	v.CostPerRequest, err = getAndCast[*big.Int](res, "4")
	return v, err
}
func (c *NftOptionsContract) TokenIDsByOwner(owner ethgo.Address) ([]*big.Int, error) {
	res, err := c.contract.Call("balanceOf", ethgo.Latest, owner)
	if err != nil {
		return nil, err
	}
	balance, err := getAndCast[*big.Int](res, "0")
	if err != nil {
		return nil, err
	}
	tokens := make([]*big.Int, int(balance.Int64()))
	for i := 0; i < len(tokens); i++ {
		res, err := c.contract.Call("tokenOfOwnerByIndex", ethgo.Latest, owner, big.NewInt(int64(i)))
		if err != nil {
			return nil, err
		}
		tokens[i], err = getAndCast[*big.Int](res, "0")
		if err != nil {
			return nil, err
		}
	}
	return tokens, nil
}
