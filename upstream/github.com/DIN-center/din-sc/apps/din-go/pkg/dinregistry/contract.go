package dinregistry

import (
	"log"

	"github.com/umbracle/ethgo"
	"github.com/umbracle/ethgo/abi"
	"github.com/umbracle/ethgo/contract"
	"github.com/umbracle/ethgo/jsonrpc"
)

// ContractHandler holds common fields for contract interaction.
type ContractHandler struct {
	Contract  *contract.Contract
	ethClient *jsonrpc.Client
	address   *ethgo.Address
}

// NewContractHandler creates a new ContractHandler struct
// It is used as a common fields for contract interaction.
func NewContractHandler(ethClient *jsonrpc.Client, contractAddress string, abiStr string) (*ContractHandler, error) {
	address := ethgo.HexToAddress(contractAddress)

	parsedABI, err := abi.NewABI(abiStr)
	if err != nil {
		log.Fatalf("Failed to parse ABI: %v", err)
	}

	contract := contract.NewContract(address, parsedABI, contract.WithJsonRPC(ethClient.Eth()))

	return &ContractHandler{
		Contract:  contract,
		ethClient: ethClient,
		address:   &address,
	}, nil
}

func (c *ContractHandler) GetEthClient() *jsonrpc.Client {
	return c.ethClient
}

func (c *ContractHandler) GetContractAddress() *ethgo.Address {
	return c.address
}

func (c *ContractHandler) Call(method string, args ...interface{}) (interface{}, error) {
	return c.Contract.Call(method, ethgo.Latest, args...)
}
