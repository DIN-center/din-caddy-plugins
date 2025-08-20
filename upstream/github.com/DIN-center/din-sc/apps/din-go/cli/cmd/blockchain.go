package dincli

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/ethclient"
)

type EVMFeesEstimator struct {
	ethClient *ethclient.Client
}

func NewEVMFeesEstimator(ethClient *ethclient.Client) *EVMFeesEstimator {
	return &EVMFeesEstimator{
		ethClient: ethClient,
	}
}

func (e *EVMFeesEstimator) EstimateFees() (FeeData, error) {
	// Try EIP-1559 first
	tipCap, err := e.ethClient.SuggestGasTipCap(context.Background())
	if err == nil {
		header, err := e.ethClient.HeaderByNumber(context.Background(), nil)
		if err == nil && header.BaseFee != nil {
			// EIP-1559 supported: suggestion is maxFeePerGas = baseFee + tipCap + (baseFee * 0.1)
			// Add 10% margin to the base fee
			margin := new(big.Int).Div(header.BaseFee, big.NewInt(10))
			// Add the tip cap to the base fee
			maxFeePerGas := new(big.Int).Add(header.BaseFee, tipCap)
			// Add the margin to the max fee per gas
			maxFeePerGas.Add(maxFeePerGas, margin)
			// Return the max fee per gas and the tip cap
			return FeeData{
				GasFeeCap: maxFeePerGas,
				GasTipCap: tipCap,
			}, nil
		}
	}

	// Fallback to legacy
	gasPrice, err := e.ethClient.SuggestGasPrice(context.Background())
	if err != nil {
		return FeeData{}, err
	}
	return FeeData{
		GasPrice: gasPrice,
	}, nil
}
