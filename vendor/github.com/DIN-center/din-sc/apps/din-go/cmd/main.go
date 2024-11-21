package main

import (
	"log"
	"os"

	"github.com/DIN-center/din-sc/apps/din-go/lib/din"
	"go.uber.org/zap"
)

func main() {
	arg := os.Args[1]
	dinClient, err := din.NewDinClient(zap.NewNop(), "", "")
	if err != nil {
		log.Fatalf("Error creating DinClient: %v", err)
	}

	var defaultNetwork = "ethereum://mainnet"

	switch arg {
	case "KickTheTires":
		err := dinClient.KickTheTires(defaultNetwork)
		if err != nil {
			log.Fatalf("Error printing DIN registry data: %v", err)
		}
	case "PrintGetAllMethodsByNetwork":
		err := dinClient.PrintGetAllMethodsByNetwork(defaultNetwork)
		if err != nil {
			log.Fatalf("Error printing all methods by network: %v", err)
		}
	case "PrintListAllMethodsByNetwork":
		err := dinClient.PrintListAllMethodsByNetwork(defaultNetwork)
		if err != nil {
			log.Fatalf("Error printing all methods by network: %v", err)
		}
	case "PrintGetAllNetworks":
		err := dinClient.PrintGetAllNetworks()
		if err != nil {
			log.Fatalf("Error printing all networks: %v", err)
		}
	case "PrintGetNetworkCapabilities":
		err := dinClient.PrintGetNetworkCapabilities(defaultNetwork)
		if err != nil {
			log.Fatalf("Error printing network capabilities: %v", err)
		}
	case "PrintGetAllProviders":
		err := dinClient.PrintGetAllProviders(defaultNetwork)
		if err != nil {
			log.Fatalf("Error printing all providers: %v", err)
		}
	case "PrintGetProvidersByNetwork":
		err := dinClient.PrintGetProvidersByNetwork(defaultNetwork)
		if err != nil {
			log.Fatalf("Error printing providers by network: %v", err)
		}
	case "GetRegistryData":
		dinClient.PrintRegistryData()
		if err != nil {
			log.Fatalf("Error printing registry data: %v", err)
		}

	default:
		log.Fatalf("Invalid argument: %v", arg)
	}
}
