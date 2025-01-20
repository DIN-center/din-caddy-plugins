package nftoptions

import (
	"math/big"
	"github.com/umbracle/ethgo"
	"testing"
)

//Owner: 
//Contract address: 0x5FbDB2315678afecb367f032d93F642f64180aa3

var (
	tokenOwner = ethgo.HexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
)

func TestWithHardhat(t *testing.T) {
	c, err := NewNftOptionsContract("0x5FbDB2315678afecb367f032d93F642f64180aa3", "http://localhost:8545")
	if err != nil {
		t.Fatalf(err.Error())
	}
	addr, err := c.OwnerOf(new(big.Int))
	if err != nil {
		t.Fatalf(err.Error())
	}
	if addr != tokenOwner {
		t.Errorf("Unexpected owner. Wanted %v, got %v", tokenOwner, addr)
	}
	ids, err := c.TokenIDsByOwner(addr)
	if err != nil {
		t.Errorf(err.Error())
	}
	if len(ids) != 1 {
		t.Errorf("Unexpected ID count, wanted 1 got %v", len(ids))
	}
	if ids[0].Int64() != 0 {
		t.Fatalf("Unexpected ID")
	}
	metadata, err := c.GetTokenMetadata(ids[0])
	if err != nil {
		t.Fatalf(err.Error())
	}
	// 0:0 1:35 2:1737147299 3:0x0000000000000000000000000000000000000FfF 4:15
	if metadata.ServiceId.Int64() != 0 {
		t.Errorf("Unexpected service id")
	}
	if metadata.RequestsPerSecondLimit != 35 {
		t.Errorf("Unexpected RPS")
	}
	if metadata.Expiration != 1837147299 {
		t.Errorf("Unexpected exp")
	}
	if metadata.TokenAddress != ethgo.HexToAddress("0x0000000000000000000000000000000000000FfF") {
		t.Errorf("Unexpected tokenAddress")
	}
	if metadata.CostPerRequest.Int64() != 15 {
		t.Errorf("Unexpected cpr")
	}


}
