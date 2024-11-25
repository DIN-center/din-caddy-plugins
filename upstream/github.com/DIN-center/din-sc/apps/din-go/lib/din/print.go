package din

import (
	"fmt"
	"log"

	din "github.com/DIN-center/din-sc/apps/din-go/pkg/dinregistry"

	"github.com/pkg/errors"
)

func (d *DinClient) PrintGetAllMethodsByNetwork(network string) error {
	fmt.Println("=============== GetAllMethodsByNetwork ===============")
	methods, err := d.DinRegistry.GetAllNetworkMethods(network)
	if err != nil {
		return errors.Wrap(err, "failed call to GetAllMethodsByNetwork")
	}

	for _, method := range methods {
		fmt.Printf("Name: %s, Bit: %d, Deactivated: %t\n", method.Name, method.Bit, method.Deactivated)
	}
	return nil
}

func (d *DinClient) PrintListAllMethodsByNetwork(network string) error {
	fmt.Println("\n\n=============== ListAllMethodsByNetwork ===============")
	methods, err := d.DinRegistry.GetAllNetworkMethodNames(network)
	if err != nil {
		return errors.Wrap(err, "failed call to ListAllMethodsByNetwork")
	}

	for _, method := range methods {
		fmt.Printf("Method: %s\n", method)
	}
	return nil
}

func (d *DinClient) PrintGetAllNetworks() error {
	fmt.Println("\n\n =============== PrintGetAllNetworks ===============")
	networks, err := d.DinRegistry.GetAllNetworkAddresses()
	if err != nil {
		return errors.Wrap(err, "failed call to GetAllNetworks")
	}

	for _, network := range networks {
		networkHandler, err := din.NewNetworkHandler(d.ethClient, network.String())
		if err != nil {
			return errors.Wrap(err, "failed call to NewNetworkHandler")
		}

		networkAddress := networkHandler.ContractHandler.GetContractAddress()

		name, err := networkHandler.GetNetworkName()
		if err != nil {
			return errors.Wrap(err, "failed call to GetNetworkName")
		}

		owner, err := networkHandler.GetNetworkOwner()
		if err != nil {
			return errors.Wrap(err, "failed call to GetNetworkOwnerAddress")
		}

		networkData := &din.Network{
			Name:         name,
			Description:  "",
			OwnerAddress: owner.String(),
		}
		err = d.PrintNetworkData(networkAddress.String(), networkData)
		if err != nil {
			return errors.Wrap(err, "failed call to PrintNetworkData")
		}
		fmt.Print("\n")
	}
	return nil
}

func (d *DinClient) PrintNetworkData(networkAddress string, networkData *din.Network) error {
	fmt.Printf("Description: %s\n", networkData.Description)
	fmt.Printf("OwnerAddress: %s\n", networkData.OwnerAddress)

	network, err := din.NewNetworkHandler(d.ethClient, networkAddress)
	if err != nil {
		return errors.Wrap(err, "failed call to NewNetwork")
	}

	capabilities, err := network.GetCapabilities()
	if err != nil {
		return errors.Wrap(err, "failed call to GetCapabilities")
	}
	fmt.Printf("\tCapabilities: %s\n", capabilities.String())

	bit2 := uint8(1) << 2
	supported, err := network.IsMethodSupported(bit2)
	if err != nil {
		return errors.Wrap(err, "failed call to IsMethodSupported")
	}
	fmt.Printf("\t2 Supported?: %t\n", supported)

	methods, err := network.GetAllMethods()
	if err != nil {
		return errors.Wrap(err, "failed call to AllMethods")
	}
	for _, method := range methods {
		fmt.Printf("\t: %s\n", method.Name)
	}

	networkStatus, err := network.GetNetworkStatus()
	if err != nil {
		return errors.Wrap(err, "failed call to GetNetworkStatus")
	}

	networkConfig, err := network.GetNetworkOperationsConfig()
	if err != nil {
		return errors.Wrap(err, "failed call to GetNetworkConfig")
	}

	fmt.Printf("NetworkStatusTypeCode: %s\n", networkStatus)
	fmt.Printf("HealthcheckMethodBit: %d\n", networkConfig.HealthcheckMethodBit)
	fmt.Printf("HealtcheckIntervalSec: %d\n", networkConfig.HealthcheckIntervalSec)
	fmt.Printf("BlockLagLimit: %d\n", networkConfig.BlockLagLimit)
	fmt.Printf("RequestAttemptCount: %d\n", networkConfig.RequestAttemptCount)
	fmt.Printf("MaxRequestPayloadSizeKb: %d\n", networkConfig.MaxRequestPayloadSizeKb)

	return nil
}

func (d *DinClient) PrintGetNetworkCapabilities(network string) error {
	fmt.Println("\n\n =============== PrintGetNetworkCapabilities ===============")
	capabilities, err := d.DinRegistry.GetNetworkCapabilities(network)
	if err != nil {
		return errors.Wrap(err, "failed call to GetNetworkCapabilities")
	}
	fmt.Printf("Capabilities [%s]: %s\n", "ethereum://mainnet", capabilities.String())
	return nil
}

func (d *DinClient) PrintGetAllProviders(network string) error {
	fmt.Println("\n\n =============== PrintGetAllProviders ===============")
	providers, err := d.DinRegistry.GetAllProviders()
	if err != nil {
		return errors.Wrap(err, "failed call to GetAllProviders")
	}

	for _, provider := range providers {
		name, err := provider.GetName()
		if err != nil {
			return errors.Wrap(err, "failed call to provider.Name()")
		}
		owner, err := provider.GetProviderOwner()
		if err != nil {
			return errors.Wrap(err, "failed call to provider.Owner()")
		}
		fmt.Printf("Name: %s, Owner: %s\n", name, owner)
	}
	return nil
}

func (d *DinClient) PrintRegistryData() {
	registryData, err := d.GetRegistryData()
	if err != nil {
		log.Fatalf("Error getting registry data: %v", err)
	}

	fmt.Println("\n =============== PrintRegistryData ===============")
	fmt.Println("# of Networks in Registry: ", len(registryData.Networks))
	fmt.Println()
	for _, network := range registryData.Networks {
		fmt.Println("Network: ", network.Name)
		fmt.Println("ProxyName: ", network.ProxyName)
		fmt.Println("Capabilities: ", network.Capabilities)
		fmt.Println("NetworkConfig: ")
		fmt.Println("  NetworkStatusTypeCode: ", network.Status)
		fmt.Println("  HealthcheckMethodBit: ", network.NetworkConfig.HealthcheckMethodBit)
		fmt.Println("  HealtcheckIntervalSec: ", network.NetworkConfig.HealthcheckIntervalSec)
		fmt.Println("  BlockLagLimit: ", network.NetworkConfig.BlockLagLimit)
		fmt.Println("  RequestAttemptCount: ", network.NetworkConfig.RequestAttemptCount)
		fmt.Println("  MaxRequestPayloadSizeKb: ", network.NetworkConfig.MaxRequestPayloadSizeKb)
		fmt.Println("# of Providers: ", len(network.Providers))
		for providerName, provider := range network.Providers {
			fmt.Println("  Provider: ", providerName)
			fmt.Println("  Address: ", provider.Address)
			fmt.Println("  Owner: ", provider.Owner)
			fmt.Println("  AuthTypeCode: ", provider.AuthConfig.Type)
			fmt.Println("  AuthURL: ", provider.AuthConfig.Url)

			for _, networkService := range provider.NetworkServices {
				fmt.Println("    NetworkService: ", networkService.Url)
				fmt.Println("    Capabilities: ", networkService.Capabilities)
				// fmt.Println("    # of Methods: ", len(networkService.Methods))
				// for _, method := range networkService.Methods {
				// 	fmt.Println("      Method: ", method.Name)
				// 	fmt.Println("      Bit: ", method.Bit)
				// 	fmt.Println("      Deactivated: ", method.Deactivated)
				// }

			}
		}
		fmt.Println("")
	}
}

func (d *DinClient) PrintGetProvidersByNetwork(network string) error {
	fmt.Printf("\n\n =============== GetProvidersByNetwork %v ===============\n", network)
	providers, err := d.DinRegistry.GetProvidersByNetwork(network)
	if err != nil {
		return errors.Wrap(err, "failed call toGetProvidersByNetwork")
	}

	for _, provider := range providers {
		name, err := provider.GetName()
		if err != nil {
			return errors.Wrap(err, "failed call provider.Name()")
		}
		owner, err := provider.GetProviderOwner()
		if err != nil {
			return errors.Wrap(err, "failed call provider.Owner()")
		}
		fmt.Printf("Name: %s, Owner: %s\n", name, owner)

		networkServiceAddresses, err := provider.GetAllNetworkServiceAddresses()
		if err != nil {
			return errors.Wrap(err, "failed call AllServices")
		}

		for _, networkServiceAddress := range networkServiceAddresses {
			networkServiceHandler, err := din.NewNetworkServiceHandler(d.ethClient, networkServiceAddress.String())
			if err != nil {
				return errors.Wrap(err, "failed call NewNetworkServiceHandler")
			}
			capabilities, err := networkServiceHandler.GetCapabilities()
			if err != nil {
				return errors.Wrap(err, "failed call GetCapabilities")
			}
			fmt.Printf("Capabilities: %d\n", capabilities)

			d.PrintIsServiceSupported(*networkServiceHandler, 1)
			d.PrintIsServiceSupported(*networkServiceHandler, 2)
			d.PrintIsServiceSupported(*networkServiceHandler, 3)

			methods, err := networkServiceHandler.GetAllMethodNames()
			if err != nil {
				return errors.Wrap(err, "failed call GetAllMethodNames")
			}
			for _, method := range methods {
				fmt.Printf("\t: %s\n", method)
			}
		}
	}
	return nil
}

func (d *DinClient) PrintIsServiceSupported(service din.NetworkServiceHandler, bit uint8) error {
	supported, err := service.IsMethodSupported(bit)
	if err != nil {
		return errors.Wrap(err, "failed call IsMethodSupported")
	}
	fmt.Printf("Is bit %d Supported?: %v\n", bit, supported)
	return nil
}
